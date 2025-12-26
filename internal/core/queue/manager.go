package queue

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/example/wget2go/internal/core/types"
)

// Manager URL队列管理器
type Manager struct {
	queue      *types.URLQueue
	blacklist  map[string]bool // URL黑名单
	visited    map[string]bool // 已访问的URL
	hostMap    map[string]*types.RobotsParser // 每个主机的robots.txt解析器
	mutex      sync.RWMutex
}

// NewManager 创建队列管理器
func NewManager() *Manager {
	return &Manager{
		queue:     types.NewURLQueue(),
		blacklist: make(map[string]bool),
		visited:   make(map[string]bool),
		hostMap:   make(map[string]*types.RobotsParser),
	}
}

// Add 添加URL到队列
func (m *Manager) Add(job *types.Job) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查黑名单
	if m.blacklist[job.URL] {
		return fmt.Errorf("URL在黑名单中: %s", job.URL)
	}

	// 检查是否已访问
	if m.visited[job.URL] {
		return fmt.Errorf("URL已访问: %s", job.URL)
	}

	// 添加到队列
	if !m.queue.Add(job) {
		return fmt.Errorf("URL已在队列中: %s", job.URL)
	}

	return nil
}

// Pop 从队列中取出一个URL
func (m *Manager) Pop() *types.Job {
	return m.queue.Pop()
}

// Contains 检查URL是否在队列中
func (m *Manager) Contains(url string) bool {
	return m.queue.Contains(url)
}

// Size 获取队列大小
func (m *Manager) Size() int {
	return m.queue.Size()
}

// IsEmpty 检查队列是否为空
func (m *Manager) IsEmpty() bool {
	return m.queue.IsEmpty()
}

// AddToBlacklist 添加URL到黑名单
func (m *Manager) AddToBlacklist(url string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.blacklist[url] = true
}

// IsInBlacklist 检查URL是否在黑名单中
func (m *Manager) IsInBlacklist(url string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.blacklist[url]
}

// MarkVisited 标记URL为已访问
func (m *Manager) MarkVisited(url string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.visited[url] = true
}

// IsVisited 检查URL是否已访问
func (m *Manager) IsVisited(url string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.visited[url]
}

// GetHost 获取URL的主机名
func (m *Manager) GetHost(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	return u.Hostname(), nil
}

// SetRobotsParser 设置主机的robots.txt解析器
func (m *Manager) SetRobotsParser(host string, parser *types.RobotsParser) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.hostMap[host] = parser
}

// GetRobotsParser 获取主机的robots.txt解析器
func (m *Manager) GetRobotsParser(host string) *types.RobotsParser {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.hostMap[host]
}

// IsAllowedByRobots 检查URL是否被robots.txt允许
func (m *Manager) IsAllowedByRobots(urlStr, userAgent string) bool {
	host, err := m.GetHost(urlStr)
	if err != nil {
		return true // 解析失败，默认允许
	}

	parser := m.GetRobotsParser(host)
	if parser == nil {
		return true // 没有robots.txt，默认允许
	}

	// 解析URL获取路径
	u, err := url.Parse(urlStr)
	if err != nil {
		return true // URL解析失败，默认允许
	}

	// 检查是否被禁止
	for _, rule := range parser.Rules {
		for _, disallow := range rule.Disallow {
			if strings.HasPrefix(u.Path, disallow) {
				return false // 被禁止
			}
		}
	}

	return true // 允许访问
}

// Clear 清空队列和黑名单
func (m *Manager) Clear() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.queue = types.NewURLQueue()
	m.blacklist = make(map[string]bool)
	m.visited = make(map[string]bool)
	m.hostMap = make(map[string]*types.RobotsParser)
}

// GetBlacklistSize 获取黑名单大小
func (m *Manager) GetBlacklistSize() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.blacklist)
}

// GetVisitedCount 获取已访问URL数量
func (m *Manager) GetVisitedCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.visited)
}

// GetHostCount 获取主机数量
func (m *Manager) GetHostCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.hostMap)
}

// AddBatch 批量添加URL到队列
func (m *Manager) AddBatch(jobs []*types.Job) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, job := range jobs {
		if m.blacklist[job.URL] {
			continue
		}

		if m.visited[job.URL] {
			continue
		}

		m.queue.Add(job)
	}

	return nil
}

// Peek 查看队列中的第一个URL（不移除）
func (m *Manager) Peek() *types.Job {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.queue.IsEmpty() {
		return nil
	}

	return m.queue.Jobs[0]
}

// Remove 从队列中移除URL
func (m *Manager) Remove(url string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.queue.Contains(url) {
		return false
	}

	// 查找并移除
	for i, job := range m.queue.Jobs {
		if job.URL == url {
			m.queue.Jobs = append(m.queue.Jobs[:i], m.queue.Jobs[i+1:]...)
			delete(m.queue.Index, url)
			return true
		}
	}

	return false
}

// GetPendingJobs 获取所有待处理的任务
func (m *Manager) GetPendingJobs() []*types.Job {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	jobs := make([]*types.Job, len(m.queue.Jobs))
	copy(jobs, m.queue.Jobs)
	return jobs
}

// GetStats 获取队列统计信息
func (m *Manager) GetStats() map[string]int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]int{
		"queue_size":     m.queue.Size(),
		"blacklist_size": len(m.blacklist),
		"visited_count":  len(m.visited),
		"host_count":     len(m.hostMap),
	}
}

// FilterByLevel 按层级过滤URL
func (m *Manager) FilterByLevel(maxLevel int) []*types.Job {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var jobs []*types.Job
	for _, job := range m.queue.Jobs {
		if job.Level <= maxLevel {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// GetJobsByLevel 获取指定层级的所有任务
func (m *Manager) GetJobsByLevel(level int) []*types.Job {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var jobs []*types.Job
	for _, job := range m.queue.Jobs {
		if job.Level == level {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// GetMaxLevel 获取队列中的最大层级
func (m *Manager) GetMaxLevel() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	maxLevel := 0
	for _, job := range m.queue.Jobs {
		if job.Level > maxLevel {
			maxLevel = job.Level
		}
	}
	return maxLevel
}

// HasRobotsForHost 检查主机是否有robots.txt
func (m *Manager) HasRobotsForHost(host string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	_, ok := m.hostMap[host]
	return ok
}

// GetAllHosts 获取所有主机列表
func (m *Manager) GetAllHosts() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	hosts := make([]string, 0, len(m.hostMap))
	for host := range m.hostMap {
		hosts = append(hosts, host)
	}
	return hosts
}

// ClearBlacklist 清空黑名单
func (m *Manager) ClearBlacklist() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.blacklist = make(map[string]bool)
}

// ClearVisited 清空已访问列表
func (m *Manager) ClearVisited() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.visited = make(map[string]bool)
}

// RemoveFromBlacklist 从黑名单中移除URL
func (m *Manager) RemoveFromBlacklist(url string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.blacklist, url)
}