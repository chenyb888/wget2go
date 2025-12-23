package multi_thread

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/example/wget2go/internal/core/types"
	"github.com/example/wget2go/internal/downloader/chunk"
	"github.com/example/wget2go/internal/core/http"
)

// DownloadManager 下载管理器
type DownloadManager struct {
	config      *types.Config
	httpClient  *http.Client
	downloader  *chunk.ChunkDownloader
	progressCh  chan types.ProgressInfo
	errorCh     chan error
	stopCh      chan struct{}
	tasks       map[string]*types.DownloadTask
	mu          sync.RWMutex
}

// NewDownloadManager 创建下载管理器
func NewDownloadManager(config *types.Config) *DownloadManager {
	httpClient := http.NewClient(config)
	downloader := chunk.NewChunkDownloader(httpClient, config)

	return &DownloadManager{
		config:     config,
		httpClient: httpClient,
		downloader: downloader,
		progressCh: make(chan types.ProgressInfo, 100),
		errorCh:    make(chan error, 100),
		stopCh:     make(chan struct{}),
		tasks:      make(map[string]*types.DownloadTask),
	}
}

// AddTask 添加下载任务
func (dm *DownloadManager) AddTask(url, outputPath string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 检查任务是否已存在
	if _, exists := dm.tasks[url]; exists {
		return fmt.Errorf("任务已存在: %s", url)
	}

	// 创建新任务
	task := &types.DownloadTask{
		URL:        url,
		OutputPath: outputPath,
		Status:     types.TaskPending,
		StartTime:  time.Now(),
	}

	dm.tasks[url] = task
	return nil
}

// Start 开始下载所有任务
func (dm *DownloadManager) Start(ctx context.Context) error {
	dm.mu.Lock()
	
	// 启动所有任务
	var wg sync.WaitGroup
	for url, task := range dm.tasks {
		if task.Status == types.TaskPending {
			wg.Add(1)
			go func(url string, task *types.DownloadTask) {
				defer wg.Done()
				dm.downloadTask(ctx, url, task)
			}(url, task)
		}
	}
	
	dm.mu.Unlock()

	// 等待所有任务完成
	wg.Wait()
	
	// 检查是否有错误
	select {
	case err := <-dm.errorCh:
		return err
	default:
		return nil
	}
}

// downloadTask 下载单个任务
func (dm *DownloadManager) downloadTask(ctx context.Context, url string, task *types.DownloadTask) {
	dm.mu.Lock()
	task.Status = types.TaskDownloading
	dm.mu.Unlock()

	// 开始下载
	err := dm.downloader.Download(ctx, url, task.OutputPath)
	
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	if err != nil {
		task.Status = types.TaskFailed
		task.Error = err
		dm.errorCh <- fmt.Errorf("下载失败 %s: %w", url, err)
	} else {
		task.Status = types.TaskCompleted
		task.EndTime = time.Now()
	}
}

// GetProgress 获取进度信息
func (dm *DownloadManager) GetProgress() <-chan types.ProgressInfo {
	return dm.downloader.GetProgressChannel()
}

// GetErrors 获取错误信息
func (dm *DownloadManager) GetErrors() <-chan error {
	return dm.errorCh
}

// Stop 停止所有下载
func (dm *DownloadManager) Stop() {
	close(dm.stopCh)
	dm.downloader.Stop()
}

// GetTaskStatus 获取任务状态
func (dm *DownloadManager) GetTaskStatus(url string) (*types.DownloadTask, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	task, exists := dm.tasks[url]
	return task, exists
}

// GetAllTasks 获取所有任务
func (dm *DownloadManager) GetAllTasks() []*types.DownloadTask {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	tasks := make([]*types.DownloadTask, 0, len(dm.tasks))
	for _, task := range dm.tasks {
		tasks = append(tasks, task)
	}
	
	return tasks
}

// RemoveTask 移除任务
func (dm *DownloadManager) RemoveTask(url string) bool {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	if _, exists := dm.tasks[url]; exists {
		delete(dm.tasks, url)
		return true
	}
	
	return false
}

// PauseTask 暂停任务
func (dm *DownloadManager) PauseTask(url string) bool {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	task, exists := dm.tasks[url]
	if exists && task.Status == types.TaskDownloading {
		task.Status = types.TaskPaused
		return true
	}
	
	return false
}

// ResumeTask 恢复任务
func (dm *DownloadManager) ResumeTask(url string) bool {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	task, exists := dm.tasks[url]
	if exists && task.Status == types.TaskPaused {
		task.Status = types.TaskPending
		return true
	}
	
	return false
}

// GetStatistics 获取统计信息
func (dm *DownloadManager) GetStatistics() *Statistics {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	stats := &Statistics{
		TotalTasks:    len(dm.tasks),
		Completed:     0,
		Downloading:   0,
		Failed:        0,
		Paused:        0,
		Pending:       0,
		TotalSize:     0,
		Downloaded:    0,
	}
	
	for _, task := range dm.tasks {
		switch task.Status {
		case types.TaskCompleted:
			stats.Completed++
			stats.Downloaded += task.Completed
			stats.TotalSize += task.Size
		case types.TaskDownloading:
			stats.Downloading++
			stats.Downloaded += task.Completed
			stats.TotalSize += task.Size
		case types.TaskFailed:
			stats.Failed++
		case types.TaskPaused:
			stats.Paused++
			stats.Downloaded += task.Completed
			stats.TotalSize += task.Size
		case types.TaskPending:
			stats.Pending++
		}
	}
	
	return stats
}

// Statistics 统计信息
type Statistics struct {
	TotalTasks   int
	Completed    int
	Downloading  int
	Failed       int
	Paused       int
	Pending      int
	TotalSize    int64
	Downloaded   int64
}

// Format 格式化统计信息
func (s *Statistics) Format() string {
	percentage := 0.0
	if s.TotalSize > 0 {
		percentage = float64(s.Downloaded) / float64(s.TotalSize) * 100
	}
	
	return fmt.Sprintf("任务: %d (完成: %d, 下载中: %d, 失败: %d, 暂停: %d, 等待: %d) | 进度: %.1f%%",
		s.TotalTasks, s.Completed, s.Downloading, s.Failed, s.Paused, s.Pending, percentage)
}