package chunk

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	httpCore "github.com/example/wget2go/internal/core/http"
	"github.com/example/wget2go/internal/core/types"
	"github.com/example/wget2go/internal/core/utils"
)

// ChunkDownloader 分片下载器
type ChunkDownloader struct {
	client      *httpCore.Client
	config      *types.Config
	progressCh  chan types.ProgressInfo
	errorCh     chan error
	stopCh      chan struct{}
}

// NewChunkDownloader 创建分片下载器
func NewChunkDownloader(client *httpCore.Client, config *types.Config) *ChunkDownloader {
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

	// 打印文件信息和服务器支持状态
	fmt.Printf("文件大小: %d bytes\n", fileInfo.ContentLength)
	fmt.Printf("服务器范围请求支持: %v\n", fileInfo.AcceptRanges)

	// 确定输出路径
	finalOutputPath := cd.getOutputPath(outputPath, url, fileInfo)

	// 检查是否需要分片下载
	if cd.shouldUseChunks(fileInfo) {
		// 测试服务器是否真正支持范围请求
		fmt.Println("测试服务器分片下载支持...")
		// 尝试下载0-0字节来测试Range支持
		reader, _, rangeErr := cd.client.DownloadRange(ctx, url, 0, 0)
		if rangeErr != nil {
			if isRangeNotSupportedError(rangeErr) {
				fmt.Println("服务器不支持分片下载，使用单线程下载")
				return cd.downloadSingle(ctx, url, finalOutputPath)
			}
			// 其他错误（如网络问题），仍尝试分片下载
			fmt.Println("范围请求测试失败（网络问题），仍尝试分片下载")
		} else {
			reader.Close()
			fmt.Println("服务器支持分片下载，开始分片下载")
		}
		
		// 尝试分片下载
		err := cd.downloadWithChunks(ctx, url, finalOutputPath, fileInfo)
		if err != nil {
			// 检查是否是服务器不支持范围请求的错误
			if isRangeNotSupportedError(err) {
				// 服务器不支持分片下载，回退到单线程
				fmt.Println("服务器不支持分片下载，回退到单线程下载")
				return cd.downloadSingle(ctx, url, finalOutputPath)
			}
			// 其他错误，直接返回
			return err
		}
		// 分片下载成功
		return nil
	}

	// 单线程下载，打印原因
	fmt.Println("使用单线程下载:")
	if cd.config.ChunkSize <= 0 {
		fmt.Println("  - 未配置分片大小")
	} else if fileInfo.ContentLength <= cd.config.ChunkSize {
		fmt.Printf("  - 文件大小 (%d bytes) 小于分片大小 (%d bytes)\n", fileInfo.ContentLength, cd.config.ChunkSize)
	} else if !fileInfo.AcceptRanges {
		fmt.Println("  - 服务器不支持范围请求")
	}
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

	// 计算每个分片的大小
	chunkSize := fileInfo.ContentLength / int64(numChunks)
	lastChunkSize := fileInfo.ContentLength - chunkSize*(int64(numChunks)-1)

	// 打印分片计划
	fmt.Printf("分片下载计划:\n")
	fmt.Printf("  文件总大小: %d 字节\n", fileInfo.ContentLength)
	fmt.Printf("  分片数量: %d\n", numChunks)
	fmt.Printf("  分片大小: %d 字节\n", chunkSize)
	fmt.Printf("  最后一个分片大小: %d 字节\n", lastChunkSize)

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
			Completed: 0,
			Status:   types.TaskPending,
		}
		fmt.Printf("  分片 %d: 字节范围 %d-%d (大小: %d)\n", i, start, end, end-start+1)
	}

	// 临时文件路径
	tempPath := outputPath + ".tmp"
	var tempFile *os.File
	var err error

	// 检查是否需要断点续传
	if cd.config.Continue && utils.FileExists(tempPath) {
		// 尝试加载状态
		stateLoaded, err := loadDownloadState(outputPath, chunks)
		if err != nil {
			return fmt.Errorf("加载下载状态失败: %w", err)
		}
		
		if stateLoaded {
			// 状态加载成功，以追加模式打开临时文件
			tempFile, err = os.OpenFile(tempPath, os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return fmt.Errorf("打开临时文件失败: %w", err)
			}
			
			// 验证文件大小与状态是否匹配
			fileStat, err := tempFile.Stat()
			if err != nil {
				tempFile.Close()
				return fmt.Errorf("获取文件信息失败: %w", err)
			}
			
			actualSize := fileStat.Size()
			var expectedSize int64
			for _, chunk := range chunks {
				expectedSize += chunk.Completed
			}
			
			if actualSize != expectedSize {
				// 文件大小不匹配，可能需要重新下载
				// 这里我们选择继续下载，但记录警告
				fmt.Printf("警告: 临时文件大小与状态不匹配: 文件 %d 字节, 状态 %d 字节\n", actualSize, expectedSize)
			}
		} else {
			// 没有状态文件，但临时文件存在，可能需要重新下载
			// 删除临时文件重新开始
			os.Remove(tempPath)
			deleteStateFile(outputPath)
			tempFile, err = os.Create(tempPath)
			if err != nil {
				return fmt.Errorf("创建临时文件失败: %w", err)
			}
		}
	} else {
		// 不是断点续传或临时文件不存在，创建新文件
		// 确保删除可能存在的旧状态文件
		deleteStateFile(outputPath)
		tempFile, err = os.Create(tempPath)
		if err != nil {
			return fmt.Errorf("创建临时文件失败: %w", err)
		}
	}
	defer tempFile.Close()

	// 启动下载
	err = cd.downloadChunks(ctx, url, tempFile, chunks, outputPath)
	if err != nil {
		return err
	}
	
	// 下载完成后，验证文件大小
	fileStat, err := tempFile.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}
	
	actualSize := fileStat.Size()
	expectedSize := fileInfo.ContentLength // 来自HEAD请求的文件总大小（参数fileInfo）
	
	if actualSize != expectedSize {
		return fmt.Errorf("文件大小不匹配: 期望 %d 字节, 实际 %d 字节 (差异: %d 字节)", expectedSize, actualSize, expectedSize-actualSize)
	}
	
	// 删除状态文件
	deleteStateFile(outputPath)
	
	// 重命名临时文件为最终文件
	if err := os.Rename(tempPath, outputPath); err != nil {
		return fmt.Errorf("重命名文件失败: %w", err)
	}
	
	fmt.Printf("文件验证通过: %d 字节\n", actualSize)
	return nil
}

// downloadChunks 下载所有分片
func (cd *ChunkDownloader) downloadChunks(ctx context.Context, url string, file *os.File, chunks []*types.Chunk, outputPath string) error {
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
			
			// 记录分片开始下载
			mu.Lock()
			fmt.Printf("分片 %d 开始下载: 字节范围 %d-%d (大小: %d)\n", 
				chunk.Index, chunk.Start, chunk.End, chunk.Size)
			mu.Unlock()
			
			// 下载分片
			if err := cd.downloadChunk(ctx, url, file, chunk); err != nil {
				cd.errorCh <- fmt.Errorf("分片 %d 下载失败: %w", chunk.Index, err)
				chunk.Status = types.TaskFailed
				chunk.Error = err
				
				mu.Lock()
				fmt.Printf("分片 %d 下载失败: %v\n", chunk.Index, err)
				mu.Unlock()
				return
			}
			
			// 更新统计并保存状态
			mu.Lock()
			// 使用实际完成的字节数（chunk.Completed）而不是预期大小（chunk.Size）
			totalDownloaded += chunk.Completed
			chunk.Status = types.TaskCompleted
			fmt.Printf("分片 %d 下载完成: 已下载 %d 字节 (总计: %d/%d)\n", 
				chunk.Index, chunk.Completed, totalDownloaded, calculateTotalSize(chunks))
			// 保存状态
			if err := saveDownloadState(outputPath, chunks); err != nil {
				// 状态保存失败不影响下载，只记录警告
				fmt.Printf("警告: 保存分片 %d 状态失败: %v\n", chunk.Index, err)
			}
			mu.Unlock()
		}(chunk)
	}

	// 等待所有分片完成
	wg.Wait()
	
	// 检查是否有错误
	var firstError error
	// 读取所有错误
	for len(cd.errorCh) > 0 {
		select {
		case err := <-cd.errorCh:
			if firstError == nil {
				firstError = err
			}
		default:
			break
		}
	}
	if firstError != nil {
		return firstError
	}
	return nil
}

// downloadChunk 下载单个分片
func (cd *ChunkDownloader) downloadChunk(ctx context.Context, url string, file *os.File, chunk *types.Chunk) error {
	// 如果分片已经完成，直接返回
	if chunk.Status == types.TaskCompleted {
		return nil
	}
	
	// 计算需要下载的起始位置
	start := chunk.Start + chunk.Completed
	end := chunk.End
	
	// 如果已经下载完成，直接返回
	if start > end {
		chunk.Status = types.TaskCompleted
		return nil
	}
	
	// 下载数据
	reader, contentLength, err := cd.client.DownloadRange(ctx, url, start, end)
	if err != nil {
		return err
	}
	defer reader.Close()

	// 验证内容长度
	expectedSize := end - start + 1
	if contentLength != expectedSize {
		return fmt.Errorf("分片大小不匹配: 期望 %d, 实际 %d", expectedSize, contentLength)
	}

	// 使用WriteAt在指定偏移量处写入，避免并发Seek导致的文件指针竞争
	writer := &writeAtWriter{
		file:   file,
		offset: start,
		chunk:  chunk,
	}
	
	if _, err := io.Copy(writer, reader); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	// 验证实际写入的字节数
	if writer.written != expectedSize {
		return fmt.Errorf("分片写入大小不匹配: 期望 %d, 实际写入 %d", expectedSize, writer.written)
	}

	// 更新分片状态
	chunk.Status = types.TaskCompleted
	return nil
}

// writeAtWriter 使用WriteAt在指定偏移量处写入，支持并发写入
type writeAtWriter struct {
	file   *os.File
	offset int64
	chunk  *types.Chunk
	written int64 // 实际写入的字节数
}

func (w *writeAtWriter) Write(p []byte) (int, error) {
	n, err := w.file.WriteAt(p, w.offset)
	if n > 0 {
		w.offset += int64(n)
		w.written += int64(n)
		w.chunk.Completed += int64(n)
	}
	return n, err
}

// chunkTrackingWriter 包装writer，用于跟踪分片下载进度
type chunkTrackingWriter struct {
	writer io.Writer
	chunk  *types.Chunk
}

func (w *chunkTrackingWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if n > 0 {
		w.chunk.Completed += int64(n)
	}
	return n, err
}

// downloadSingle 单线程下载
func (cd *ChunkDownloader) downloadSingle(ctx context.Context, url, outputPath string) error {
	var rangeHeader string
	var file *os.File
	var err error
	var fileSize int64
	
	// 检查是否需要断点续传
	if cd.config.Continue && utils.FileExists(outputPath) {
		// 获取已下载文件大小
		fileSize, err = utils.GetFileSize(outputPath)
		if err != nil {
			return fmt.Errorf("获取文件大小失败: %w", err)
		}
		
		if fileSize > 0 {
			// 设置Range头，从断点处继续下载
			rangeHeader = fmt.Sprintf("bytes=%d-", fileSize)
		}
	}
	
	// 发送GET请求（可能带有Range头）
	resp, err := cd.client.Get(ctx, url, rangeHeader)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	
	// 检查响应状态码
	if rangeHeader != "" {
		// 如果发送了Range头，期望206 Partial Content
		if resp.StatusCode == http.StatusPartialContent {
			// 服务器支持范围请求，继续下载
		} else if resp.StatusCode == http.StatusOK {
			// 服务器返回200 OK，说明不支持范围请求或范围无效
			// 从头开始下载，覆盖已存在的文件
			fileSize = 0
			rangeHeader = ""
		} else {
			// 其他错误状态码
			return fmt.Errorf("HTTP错误: %d", resp.StatusCode)
		}
	} else {
		// 没有发送Range头，期望200 OK
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP错误: %d", resp.StatusCode)
		}
	}
	
	// 打开或创建文件
	if fileSize > 0 {
		// 断点续传：以追加模式打开文件
		file, err = os.OpenFile(outputPath, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("打开文件失败: %w", err)
		}
	} else {
		// 全新下载：创建文件（覆盖）
		file, err = os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("创建文件失败: %w", err)
		}
	}
	defer file.Close()
	
	// 处理可能的压缩内容
	bodyReader := resp.Body
	contentEncoding := resp.Header.Get("Content-Encoding")
	isCompressed := false
	
	// 根据Content-Encoding进行解压
	switch strings.ToLower(contentEncoding) {
	case "gzip", "x-gzip":
		gzipReader, err := gzip.NewReader(bodyReader)
		if err != nil {
			return fmt.Errorf("创建gzip解压器失败: %w", err)
		}
		defer gzipReader.Close()
		bodyReader = gzipReader
		isCompressed = true
	case "deflate":
		zlibReader, err := zlib.NewReader(bodyReader)
		if err != nil {
			return fmt.Errorf("创建zlib解压器失败: %w", err)
		}
		defer zlibReader.Close()
		bodyReader = zlibReader
		isCompressed = true
	case "identity", "":
		// 无压缩，使用原始body
	default:
		// 未知编码，但继续下载，可能服务器使用了我们不支持的压缩算法
		// 记录警告但继续
		fmt.Printf("警告: 未知的Content-Encoding: %s，按原始数据下载\n", contentEncoding)
	}
	
	// 复制数据
	copied, err := io.Copy(file, bodyReader)
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	
	// 验证下载大小（如果知道内容长度）
	contentLength := resp.ContentLength
	if contentLength > 0 && !isCompressed {
		// 只有当内容未压缩时才验证大小
		// 如果服务器返回压缩内容，Content-Length是压缩后的大小，但解压后大小不同
		if copied != contentLength {
			return fmt.Errorf("下载大小不匹配: 期望 %d, 实际 %d", contentLength, copied)
		}
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

// createStateFileName 创建状态文件名
func createStateFileName(outputPath string) string {
	return outputPath + ".wget2go.state"
}

// saveDownloadState 保存下载状态
func saveDownloadState(outputPath string, chunks []*types.Chunk) error {
	stateFile := createStateFileName(outputPath)
	
	// 创建状态数据结构
	type ChunkState struct {
		Index    int   `json:"index"`
		Start    int64 `json:"start"`
		End      int64 `json:"end"`
		Size     int64 `json:"size"`
		Completed int64 `json:"completed"`
		Status   int   `json:"status"`
	}
	
	var states []ChunkState
	for _, chunk := range chunks {
		states = append(states, ChunkState{
			Index:     chunk.Index,
			Start:     chunk.Start,
			End:       chunk.End,
			Size:      chunk.Size,
			Completed: chunk.Completed,
			Status:    int(chunk.Status),
		})
	}
	
	// 序列化为JSON
	data, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(stateFile, data, 0644)
}

// loadDownloadState 加载下载状态
func loadDownloadState(outputPath string, chunks []*types.Chunk) (bool, error) {
	stateFile := createStateFileName(outputPath)
	
	if !utils.FileExists(stateFile) {
		return false, nil
	}
	
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return false, err
	}
	
	// 反序列化JSON
	type ChunkState struct {
		Index    int   `json:"index"`
		Start    int64 `json:"start"`
		End      int64 `json:"end"`
		Size     int64 `json:"size"`
		Completed int64 `json:"completed"`
		Status   int   `json:"status"`
	}
	
	var states []ChunkState
	if err := json.Unmarshal(data, &states); err != nil {
		return false, err
	}
	
	// 创建状态映射
	stateMap := make(map[int]ChunkState)
	for _, state := range states {
		stateMap[state.Index] = state
	}
	
	// 恢复状态到chunks
	for _, chunk := range chunks {
		if state, exists := stateMap[chunk.Index]; exists {
			// 验证分片范围是否匹配
			if chunk.Start == state.Start && chunk.End == state.End {
				chunk.Completed = state.Completed
				chunk.Status = types.TaskStatus(state.Status)
			} else {
				// 分片范围不匹配，重置状态
				chunk.Completed = 0
				chunk.Status = types.TaskPending
			}
		}
	}
	
	return true, nil
}

// deleteStateFile 删除状态文件
func deleteStateFile(outputPath string) error {
	stateFile := createStateFileName(outputPath)
	if utils.FileExists(stateFile) {
		return os.Remove(stateFile)
	}
	return nil
}

// isRangeNotSupportedError 检查是否是服务器不支持范围请求的错误
func isRangeNotSupportedError(err error) bool {
	if err == nil {
		return false
	}
	// 检查错误信息是否包含关键词
	errStr := err.Error()
	return strings.Contains(errStr, "服务器不支持范围请求") || 
	       strings.Contains(errStr, "range not supported") ||
	       strings.Contains(errStr, "不支持范围请求")
}