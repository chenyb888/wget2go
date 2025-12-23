package types

import (
	"time"
)

// Config 全局配置
type Config struct {
	// 下载选项
	OutputFile      string
	OutputDocument  string
	Continue        bool
	ChunkSize       int64
	MaxThreads      int
	LimitRate       int64
	Timeout         time.Duration
	UserAgent       string
	Referer         string
	Headers         map[string]string
	Cookies         map[string]string
	
	// 递归下载选项
	Recursive       bool
	RecursiveLevel  int
	ConvertLinks    bool
	PageRequisites  bool
	
	// HTTP选项
	MaxRedirects    int
	FollowRedirects bool
	Insecure        bool
	
	// 输出选项
	Quiet           bool
	Verbose         bool
	Progress        bool
	
	// 其他选项
	Metalink        bool
	RobotsTxt       bool
}

// DownloadTask 下载任务
type DownloadTask struct {
	URL         string
	OutputPath  string
	Size        int64
	Completed   int64
	Status      TaskStatus
	Error       error
	StartTime   time.Time
	EndTime     time.Time
}

// TaskStatus 任务状态
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskDownloading
	TaskCompleted
	TaskFailed
	TaskPaused
)

// Chunk 文件分片
type Chunk struct {
	Index    int
	Start    int64
	End      int64
	Size     int64
	Completed int64
	Status   TaskStatus
	Error    error
}

// HTTPResponse HTTP响应信息
type HTTPResponse struct {
	StatusCode    int
	ContentLength int64
	ContentType   string
	LastModified  time.Time
	ETag          string
	AcceptRanges  bool
}

// ProgressInfo 进度信息
type ProgressInfo struct {
	TotalSize     int64
	Downloaded    int64
	Speed         int64 // bytes per second
	Percentage    float64
	RemainingTime time.Duration
	ActiveThreads int
}