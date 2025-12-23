package chunk

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/example/wget2go/internal/core/http"
	"github.com/example/wget2go/internal/core/types"
	"github.com/example/wget2go/internal/core/utils"
)

// ChunkDownloader 分片下载器
type ChunkDownloader struct {
	client      *http.Client
	config      *types.Config
	progressCh  chan types.ProgressInfo
	errorCh     chan error
	stopCh      chan struct{}
}

// NewChunkDownloader 创建分片下载器
func NewChunkDownloader(client *http.Client, config *types.Config) *ChunkDownloader {
	return &ChunkDownloader{
		client:     client,
		config:     config,
		progressCh: make(chan types.ProgressInfo, 100),
		errorCh:    make(chan error, 100),
		stopCh:     make(chan struct{}),
	}
}

// Download 下载文件
func (cd *ChunkDownloader) Download(ctx context.Context, url, outputPath string) error {
	// 获取文件信息
	fileInfo, err := cd.getFileInfo(ctx, url)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 确定输出路径
	finalOutputPath := cd.getOutputPath(outputPath, url, fileInfo)

	// 检查是否需要分片下载
	if cd.shouldUseChunks(fileInfo) {
		return cd.downloadWithChunks(ctx, url, finalOutputPath, fileInfo)
	}

	// 单线程下载
	return cd.downloadSingle(ctx, url, finalOutputPath)
}

// getFileInfo 获取文件信息
func (cd *ChunkDownloader) getFileInfo(ctx context.Context, url string) (*types.HTTPResponse, error) {
	resp, err := cd.client.Head(ctx, url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP错误: %d", resp.StatusCode)
	}

	if resp.ContentLength <= 0 {
		return nil, fmt.Errorf("无法获取文件大小")
	}

	return resp, nil
}

// getOutputPath 确定输出路径
func (cd *ChunkDownloader) getOutputPath(outputPath, url string, fileInfo *types.HTTPResponse) string {
	if outputPath != "" {
		return outputPath
	}

	if cd.config.OutputFile != "" {
		return cd.config.OutputFile
	}

	if cd.config.OutputDocument != "" {
		return cd.config.OutputDocument
	}

	// 从URL提取文件名
	filename := cd.client.GetFileNameFromURL(url)
	return filename
}

// shouldUseChunks 判断是否需要分片下载
func (cd *ChunkDownloader) shouldUseChunks(fileInfo *types.HTTPResponse) bool {
	// 需要满足以下条件：
	// 1. 配置了chunk size
	// 2. 文件大小大于chunk size
	// 3. 服务器支持范围请求
	return cd.config.ChunkSize > 0 &&
		fileInfo.ContentLength > cd.config.ChunkSize &&
		fileInfo.AcceptRanges
}

// downloadWithChunks 使用分片下载
func (cd *ChunkDownloader) downloadWithChunks(ctx context.Context, url, outputPath string, fileInfo *types.HTTPResponse) error {
	// 计算分片数量
	numChunks := calculateNumChunks(fileInfo.ContentLength, cd.config.ChunkSize)
	
	// 限制最大线程数
	if numChunks > cd.config.MaxThreads {
		numChunks = cd.config.MaxThreads
	}

	// 创建临时文件
	tempFile, err := createTempFile(outputPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer tempFile.Close()

	// 计算每个分片的大小
	chunkSize := fileInfo.ContentLength / int64(numChunks)
	lastChunkSize := fileInfo.ContentLength - chunkSize*(int64(numChunks)-1)

	// 创建分片任务
	chunks := make([]*types.Chunk, numChunks)
	for i := 0; i < numChunks; i++ {
		start := int64(i) * chunkSize
		end := start + chunkSize - 1
		if i == numChunks-1 {
			end = start + lastChunkSize - 1
		}

		chunks[i] = &types.Chunk{
			Index:    i,
			Start:    start,
			End:      end,
			Size:     end - start + 1,
			Status:   types.TaskPending,
		}
	}

	// 启动下载
	return cd.downloadChunks(ctx, url, tempFile, chunks)
}

// downloadChunks 下载所有分片
func (cd *ChunkDownloader) downloadChunks(ctx context.Context, url string, file *os.File, chunks []*types.Chunk) error {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, cd.config.MaxThreads)
	
	var mu sync.Mutex
	totalDownloaded := int64(0)
	startTime := time.Now()

	// 启动进度报告
	go cd.reportProgress(ctx, len(chunks), chunks, &totalDownloaded, startTime)

	// 下载每个分片
	for _, chunk := range chunks {
		wg.Add(1)
		
		go func(chunk *types.Chunk) {
			defer wg.Done()
			
			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			// 下载分片
			if err := cd.downloadChunk(ctx, url, file, chunk); err != nil {
				cd.errorCh <- fmt.Errorf("分片 %d 下载失败: %w", chunk.Index, err)
				chunk.Status = types.TaskFailed
				chunk.Error = err
				return
			}
			
			// 更新统计
			mu.Lock()
			totalDownloaded += chunk.Size
			chunk.Status = types.TaskCompleted
			mu.Unlock()
		}(chunk)
	}

	// 等待所有分片完成
	wg.Wait()
	
	// 检查是否有错误
	select {
	case err := <-cd.errorCh:
		return err
	default:
		return nil
	}
}

// downloadChunk 下载单个分片
func (cd *ChunkDownloader) downloadChunk(ctx context.Context, url string, file *os.File, chunk *types.Chunk) error {
	// 设置范围头
	rangeHeader := fmt.Sprintf("bytes=%d-%d", chunk.Start, chunk.End)
	
	// 下载数据
	reader, contentLength, err := cd.client.DownloadRange(ctx, url, chunk.Start, chunk.End)
	if err != nil {
		return err
	}
	defer reader.Close()

	// 验证内容长度
	if contentLength != chunk.Size {
		return fmt.Errorf("分片大小不匹配: 期望 %d, 实际 %d", chunk.Size, contentLength)
	}

	// 定位到文件位置
	if _, err := file.Seek(chunk.Start, io.SeekStart); err != nil {
		return fmt.Errorf("文件定位失败: %w", err)
	}

	// 写入数据
	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// downloadSingle 单线程下载
func (cd *ChunkDownloader) downloadSingle(ctx context.Context, url, outputPath string) error {
	// 发送GET请求
	resp, err := cd.client.Get(ctx, url, "")
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP错误: %d", resp.StatusCode)
	}

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

	return nil
}

// reportProgress 报告下载进度
func (cd *ChunkDownloader) reportProgress(ctx context.Context, totalChunks int, chunks []*types.Chunk, totalDownloaded *int64, startTime time.Time) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cd.stopCh:
			return
		case <-ticker.C:
			// 计算进度
			downloaded := *totalDownloaded
			elapsed := time.Since(startTime)
			
			var speed int64
			if elapsed.Seconds() > 0 {
				speed = int64(float64(downloaded) / elapsed.Seconds())
			}

			// 计算完成的分片数
			completedChunks := 0
			for _, chunk := range chunks {
				if chunk.Status == types.TaskCompleted {
					completedChunks++
				}
			}

			// 发送进度信息
			cd.progressCh <- types.ProgressInfo{
				TotalSize:     calculateTotalSize(chunks),
				Downloaded:    downloaded,
				Speed:         speed,
				Percentage:    float64(downloaded) / float64(calculateTotalSize(chunks)) * 100,
				RemainingTime: utils.CalculateETA(calculateTotalSize(chunks), downloaded, speed),
				ActiveThreads: cd.config.MaxThreads,
			}
		}
	}
}

// calculateNumChunks 计算分片数量
func calculateNumChunks(fileSize, chunkSize int64) int {
	if chunkSize <= 0 {
		return 1
	}
	
	numChunks := int(fileSize / chunkSize)
	if fileSize%chunkSize != 0 {
		numChunks++
	}
	
	return numChunks
}

// createTempFile 创建临时文件
func createTempFile(outputPath string) (*os.File, error) {
	// 创建临时文件
	tempPath := outputPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return nil, err
	}
	
	return file, nil
}

// calculateTotalSize 计算总大小
func calculateTotalSize(chunks []*types.Chunk) int64 {
	var total int64
	for _, chunk := range chunks {
		total += chunk.Size
	}
	return total
}

// GetProgressChannel 获取进度通道
func (cd *ChunkDownloader) GetProgressChannel() <-chan types.ProgressInfo {
	return cd.progressCh
}

// GetErrorChannel 获取错误通道
func (cd *ChunkDownloader) GetErrorChannel() <-chan error {
	return cd.errorCh
}

// Stop 停止下载
func (cd *ChunkDownloader) Stop() {
	close(cd.stopCh)
}