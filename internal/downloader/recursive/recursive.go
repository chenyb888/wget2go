package recursive

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/example/wget2go/internal/core/converter"
	"github.com/example/wget2go/internal/core/css"
	"github.com/example/wget2go/internal/core/html"
	"github.com/example/wget2go/internal/core/http"
	"github.com/example/wget2go/internal/core/queue"
	"github.com/example/wget2go/internal/core/robots"
	"github.com/example/wget2go/internal/core/types"
)

// RecursiveDownloader 递归下载器
type RecursiveDownloader struct {
	config           *types.Config
	httpClient       *http.Client
	queueManager     *queue.Manager
	htmlParser       *html.Parser
	cssParser        *css.Parser
	robotsParser     *robots.Parser
	linkConverter    *converter.Converter
	userAgent        string
	downloadedFiles  map[string]bool
	mutex            sync.RWMutex
	jobCounter       uint64
}

// NewRecursiveDownloader 创建递归下载器
func NewRecursiveDownloader(httpClient *http.Client, config *types.Config) *RecursiveDownloader {
	return &RecursiveDownloader{
		config:          config,
		httpClient:      httpClient,
		queueManager:    queue.NewManager(),
		htmlParser:      html.NewParser(),
		cssParser:       css.NewParser(),
		robotsParser:    robots.NewParser(),
		linkConverter:   converter.NewConverter(".", false),
		downloadedFiles: make(map[string]bool),
		userAgent:       getUserAgent(config),
		jobCounter:      0,
	}
}

// Download 执行递归下载
func (rd *RecursiveDownloader) Download(ctx context.Context, startURL string, outputDir string) error {
	// 创建输出目录
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 设置转换器的基础目录
	rd.linkConverter.SetBaseDir(outputDir)
	rd.linkConverter.SetBackup(rd.config.ConvertLinks)

	// 添加初始URL到队列
	initialJob := &types.Job{
		ID:              rd.nextJobID(),
		URL:             startURL,
		Level:           0,
		Flags:           types.URLFlagNone,
		Status:          types.TaskPending,
		RequestedByUser: true,
	}

	if err := rd.queueManager.Add(initialJob); err != nil {
		return fmt.Errorf("添加初始URL失败: %w", err)
	}

	// 下载并处理robots.txt
	if rd.config.RobotsTxt {
		if err := rd.downloadRobotsTxt(ctx, startURL); err != nil {
			if rd.config.Verbose {
				fmt.Printf("警告: 下载robots.txt失败: %v\n", err)
			}
		}
	}

	// 处理队列中的所有URL
	for !rd.queueManager.IsEmpty() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			job := rd.queueManager.Pop()
			if job == nil {
				continue
			}

			if err := rd.processJob(ctx, job, outputDir); err != nil {
				if rd.config.Verbose {
					fmt.Printf("处理URL失败: %s - %v\n", job.URL, err)
				}
			}
		}
	}

	// 转换链接
	if rd.config.ConvertLinks {
		if err := rd.linkConverter.ConvertAll(); err != nil {
			return fmt.Errorf("转换链接失败: %w", err)
		}
	}

	return nil
}

// processJob 处理单个下载任务
func (rd *RecursiveDownloader) processJob(ctx context.Context, job *types.Job, outputDir string) error {
	// 标记为已访问
	rd.queueManager.MarkVisited(job.URL)

	// 检查robots.txt
	if !rd.queueManager.IsAllowedByRobots(job.URL, rd.userAgent) {
		if rd.config.Verbose {
			fmt.Printf("URL被robots.txt禁止: %s\n", job.URL)
		}
		return nil
	}

	// 确定输出路径
	outputPath := rd.getOutputPath(job.URL, outputDir)

	// 下载文件
	if err := rd.downloadFile(ctx, job, outputPath); err != nil {
		return err
	}

	// 检查是否需要继续递归
	if !rd.shouldRecurse(job) {
		return nil
	}

	// 解析文件内容并提取URL
	if err := rd.parseAndQueueURLs(ctx, job, outputPath); err != nil {
		return err
	}

	return nil
}

// shouldRecurse 检查是否应该继续递归
func (rd *RecursiveDownloader) shouldRecurse(job *types.Job) bool {
	if !rd.config.Recursive {
		return false
	}

	// 检查递归深度
	if rd.config.RecursiveLevel > 0 && job.Level >= rd.config.RecursiveLevel {
		return false
	}

	// 检查是否是页面必需资源
	if job.Flags&types.URLFlagRequisite != 0 && rd.config.PageRequisites {
		return true
	}

	return true
}

// downloadFile 下载文件
func (rd *RecursiveDownloader) downloadFile(ctx context.Context, job *types.Job, outputPath string) error {
	// 创建输出目录
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 获取文件信息
	resp, err := rd.httpClient.Head(ctx, job.URL)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 检查内容类型
	contentType := strings.ToLower(resp.ContentType)
	if !strings.HasPrefix(contentType, "text/html") && 
	   !strings.HasPrefix(contentType, "text/css") &&
	   !strings.HasPrefix(contentType, "application/xml") {
		// 非文本文件，直接下载
		return rd.downloadBinaryFile(ctx, job, outputPath)
	}

	// 下载文本文件
	return rd.downloadTextFile(ctx, job, outputPath)
}

// downloadBinaryFile 下载二进制文件
func (rd *RecursiveDownloader) downloadBinaryFile(ctx context.Context, job *types.Job, outputPath string) error {
	resp, err := rd.httpClient.Get(ctx, job.URL, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 创建输出文件
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	// 复制数据
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	// 记录已下载文件
	rd.mutex.Lock()
	rd.downloadedFiles[outputPath] = true
	rd.mutex.Unlock()

	return nil
}

// downloadTextFile 下载文本文件
func (rd *RecursiveDownloader) downloadTextFile(ctx context.Context, job *types.Job, outputPath string) error {
	resp, err := rd.httpClient.Get(ctx, job.URL, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 读取数据
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取数据失败: %w", err)
	}

	// 保存编码信息
	job.Encoding = "utf-8"
	job.ContentType = resp.Header.Get("Content-Type")

	// 写入文件
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	// 记录已下载文件
	rd.mutex.Lock()
	rd.downloadedFiles[outputPath] = true
	rd.mutex.Unlock()

	return nil
}

// parseAndQueueURLs 解析文件内容并提取URL
func (rd *RecursiveDownloader) parseAndQueueURLs(ctx context.Context, job *types.Job, outputPath string) error {
	// 读取文件内容
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 根据内容类型选择解析器
	contentType := strings.ToLower(job.ContentType)

	var result *types.ParsedResult

	if strings.HasPrefix(contentType, "text/html") {
		result, err = rd.htmlParser.Parse(data, job.URL)
		if err != nil {
			return fmt.Errorf("解析HTML失败: %w", err)
		}

		// 检查META robots标签
		if rd.config.RobotsTxt && !result.Follow {
			return nil
		}

		// 添加到转换列表
		if rd.config.ConvertLinks {
			rd.linkConverter.AddConversion(outputPath, job.URL, result)
		}

	} else if strings.HasPrefix(contentType, "text/css") {
		result, err = rd.cssParser.Parse(data, job.URL)
		if err != nil {
			return fmt.Errorf("解析CSS失败: %w", err)
		}
	}

	// 将提取的URL添加到队列
	if result != nil {
		for _, parsedURL := range result.URLs {
			rd.queueURL(job, parsedURL)
		}
	}

	return nil
}

// queueURL 将URL添加到队列
func (rd *RecursiveDownloader) queueURL(parentJob *types.Job, parsedURL *types.ParsedURL) error {
	// 跳过非HTTP协议的URL
	if !strings.HasPrefix(parsedURL.URL, "http://") && !strings.HasPrefix(parsedURL.URL, "https://") {
		return nil
	}

	// 检查是否已被访问或已在队列中
	if rd.queueManager.IsVisited(parsedURL.URL) || rd.queueManager.Contains(parsedURL.URL) {
		return nil
	}

	// 检查是否在黑名单中
	if rd.queueManager.IsInBlacklist(parsedURL.URL) {
		return nil
	}

	// 确定URL标志
	flags := types.URLFlagNone
	if parsedURL.Attr == "src" || parsedURL.Attr == "href" || parsedURL.Tag == "img" || parsedURL.Tag == "script" {
		flags |= types.URLFlagRequisite
	}

	// 创建新任务
	newJob := &types.Job{
		ID:         rd.nextJobID(),
		ParentID:   parentJob.ID,
		URL:        parsedURL.URL,
		Level:      parentJob.Level + 1,
		Flags:      flags,
		Status:     types.TaskPending,
		Encoding:   "utf-8",
	}

	// 添加到队列
	if err := rd.queueManager.Add(newJob); err != nil {
		return err
	}

	return nil
}

// downloadRobotsTxt 下载robots.txt
func (rd *RecursiveDownloader) downloadRobotsTxt(ctx context.Context, urlStr string) error {
	// 解析URL获取主机
	u, err := url.Parse(urlStr)
	if err != nil {
		return err
	}

	host := u.Hostname()
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", u.Scheme, host)

	// 下载robots.txt
	resp, err := rd.httpClient.Get(ctx, robotsURL, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 读取内容
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 解析robots.txt
	if err := rd.robotsParser.Parse(data, rd.userAgent); err != nil {
		return err
	}

	// 保存到队列管理器 - 转换为types.RobotsParser类型
	robotsParser := &types.RobotsParser{
		Rules:    rd.robotsParser.GetRules(),
		Default:  nil, // 获取默认规则
		Sitemaps: rd.robotsParser.GetSitemaps(),
	}
	rd.queueManager.SetRobotsParser(host, robotsParser)

	if rd.config.Verbose {
		fmt.Printf("已下载并解析robots.txt: %s\n", robotsURL)
	}

	return nil
}

// getOutputPath 获取输出路径
func (rd *RecursiveDownloader) getOutputPath(urlStr, outputDir string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return filepath.Join(outputDir, "index.html")
	}

	path := u.Path
	if path == "" || path == "/" {
		path = "/index.html"
	}

	// 移除查询参数和锚点
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}
	if idx := strings.Index(path, "#"); idx != -1 {
		path = path[:idx]
	}

	// 转换为本地路径
	localPath := filepath.Join(outputDir, filepath.FromSlash(path))

	// 如果路径以/结尾，添加index.html
	if strings.HasSuffix(localPath, string(filepath.Separator)) {
		localPath = filepath.Join(localPath, "index.html")
	}

	return localPath
}

// nextJobID 生成下一个任务ID
func (rd *RecursiveDownloader) nextJobID() uint64 {
	rd.mutex.Lock()
	defer rd.mutex.Unlock()
	rd.jobCounter++
	return rd.jobCounter
}

// GetDownloadedFiles 获取已下载的文件列表
func (rd *RecursiveDownloader) GetDownloadedFiles() []string {
	rd.mutex.RLock()
	defer rd.mutex.RUnlock()

	files := make([]string, 0, len(rd.downloadedFiles))
	for file := range rd.downloadedFiles {
		files = append(files, file)
	}
	return files
}

// GetDownloadedCount 获取已下载文件数量
func (rd *RecursiveDownloader) GetDownloadedCount() int {
	rd.mutex.RLock()
	defer rd.mutex.RUnlock()
	return len(rd.downloadedFiles)
}

// GetStats 获取下载统计信息
func (rd *RecursiveDownloader) GetStats() map[string]int {
	return rd.queueManager.GetStats()
}

// getUserAgent 获取User-Agent
func getUserAgent(config *types.Config) string {
	if config.UserAgent != "" {
		return config.UserAgent
	}
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"
}