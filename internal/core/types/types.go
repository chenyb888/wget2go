package types

import (
	"sync"
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
	ProxyURL        string
	
	// Proxy选项
	HTTPProxy       string
	HTTPSProxy      string
	NoProxy         string
	ProxyEnabled    bool
	ProxyUsername   string
	ProxyPassword   string
	
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

// Job 下载任务（用于递归下载）
type Job struct {
	ID              uint64
	ParentID        uint64
	URL             string
	OutputPath      string
	Level           int          // 递归深度级别
	RedirectionLevel int         // 重定向级别
	Flags           URLFlag      // URL标志
	Status          TaskStatus
	Error           error
	ContentType     string
	Encoding        string
	IsSitemap       bool         // 是否为sitemap
	IsRobotsTxt     bool         // 是否为robots.txt
	RequestedByUser bool         // 是否由用户直接请求
	StartTime       time.Time
	EndTime         time.Time
}

// URLFlag URL标志位
type URLFlag int

const (
	URLFlagNone URLFlag = iota
	URLFlagRedirection   // 重定向
	URLFlagRequisite     // 页面必需资源（CSS、图片等）
	URLFlagRecursive     // 递归下载
)

// ParsedURL 解析出的URL信息
type ParsedURL struct {
	URL      string
	Attr     string // HTML属性名（如href、src）
	Tag      string // HTML标签名
	Position int    // 在文档中的位置
}

// ParsedResult 解析结果（HTML/CSS）
type ParsedResult struct {
	URLs      []*ParsedURL
	Follow    bool // 是否允许跟随（基于META robots标签）
	Encoding  string
	Links     map[string]string // 原始URL到标准化URL的映射
}

// Conversion 链接转换信息
type Conversion struct {
	Filename  string
	BaseURL   string
	Encoding  string
	Result    *ParsedResult
	Converted bool
}

// RobotsRules robots.txt规则
type RobotsRules struct {
	UserAgent string
	Disallow  []string
	Allow     []string
	CrawlDelay int
	Sitemaps  []string
}

// RobotsParser robots.txt解析器
type RobotsParser struct {
	Rules    []*RobotsRules
	Default  *RobotsRules // User-agent: * 的规则
	Sitemaps []string
}

// URLQueue URL队列
type URLQueue struct {
	Jobs    []*Job
	Index   map[string]bool // URL黑名单，防止重复下载
	Mutex   sync.RWMutex
}

// NewURLQueue 创建URL队列
func NewURLQueue() *URLQueue {
	return &URLQueue{
		Jobs:  make([]*Job, 0),
		Index: make(map[string]bool),
	}
}

// Add 添加URL到队列
func (q *URLQueue) Add(job *Job) bool {
	q.Mutex.Lock()
	defer q.Mutex.Unlock()

	if q.Index[job.URL] {
		return false // 已存在
	}

	q.Jobs = append(q.Jobs, job)
	q.Index[job.URL] = true
	return true
}

// Pop 从队列中取出一个URL
func (q *URLQueue) Pop() *Job {
	q.Mutex.Lock()
	defer q.Mutex.Unlock()

	if len(q.Jobs) == 0 {
		return nil
	}

	job := q.Jobs[0]
	q.Jobs = q.Jobs[1:]
	return job
}

// Contains 检查URL是否在队列中
func (q *URLQueue) Contains(url string) bool {
	q.Mutex.RLock()
	defer q.Mutex.RUnlock()
	return q.Index[url]
}

// Size 获取队列大小
func (q *URLQueue) Size() int {
	q.Mutex.RLock()
	defer q.Mutex.RUnlock()
	return len(q.Jobs)
}

// IsEmpty 检查队列是否为空
func (q *URLQueue) IsEmpty() bool {
	q.Mutex.RLock()
	defer q.Mutex.RUnlock()
	return len(q.Jobs) == 0
}