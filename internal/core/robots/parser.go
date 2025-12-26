package robots

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/example/wget2go/internal/core/types"
)

// Parser robots.txt解析器
type Parser struct {
	rules    []*types.RobotsRules
	defaults *types.RobotsRules
	sitemaps []string
}

// NewParser 创建robots.txt解析器
func NewParser() *Parser {
	return &Parser{
		rules:    make([]*types.RobotsRules, 0),
		sitemaps: make([]string, 0),
	}
}

// Parse 解析robots.txt内容
func (p *Parser) Parse(data []byte, userAgent string) error {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	var currentRule *types.RobotsRules
	var inRecord bool

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 分割键值对
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch key {
		case "user-agent":
			// 开始新的规则记录
			value = strings.ToLower(value)
			if value == "*" {
				// 默认规则
				currentRule = &types.RobotsRules{
					UserAgent: "*",
					Disallow:  make([]string, 0),
					Allow:     make([]string, 0),
				}
				p.defaults = currentRule
			} else {
				// 特定user-agent的规则
				currentRule = &types.RobotsRules{
					UserAgent: value,
					Disallow:  make([]string, 0),
					Allow:     make([]string, 0),
				}
			}
			inRecord = true
			p.rules = append(p.rules, currentRule)

		case "disallow":
			if inRecord && currentRule != nil {
				if value == "" {
					// 空值表示允许所有
					currentRule.Disallow = make([]string, 0)
				} else {
					currentRule.Disallow = append(currentRule.Disallow, value)
				}
			}

		case "allow":
			if inRecord && currentRule != nil {
				currentRule.Allow = append(currentRule.Allow, value)
			}

		case "crawl-delay":
			if inRecord && currentRule != nil {
				// 解析延迟时间（秒）
				var delay int
				fmt.Sscanf(value, "%d", &delay)
				currentRule.CrawlDelay = delay
			}

		case "sitemap":
			// Sitemap是全局的，不属于特定user-agent
			p.sitemaps = append(p.sitemaps, value)
		}
	}

	return nil
}

// ParseReader 从io.Reader解析robots.txt
func (p *Parser) ParseReader(r io.Reader, userAgent string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("读取robots.txt失败: %w", err)
	}
	return p.Parse(data, userAgent)
}

// IsAllowed 检查URL是否被允许
func (p *Parser) IsAllowed(urlStr, userAgent string) bool {
	// 获取适用的规则
	rule := p.getRule(userAgent)
	if rule == nil {
		return true // 没有规则，默认允许
	}

	// 解析URL路径
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return true // URL解析失败，默认允许
	}

	path := parsedURL.Path

	// 检查Allow规则
	for _, allow := range rule.Allow {
		if p.matchPath(path, allow) {
			return true
		}
	}

	// 检查Disallow规则
	for _, disallow := range rule.Disallow {
		if p.matchPath(path, disallow) {
			return false
		}
	}

	return true // 没有匹配的规则，默认允许
}

// getRule 获取适用的规则
func (p *Parser) getRule(userAgent string) *types.RobotsRules {
	userAgent = strings.ToLower(userAgent)

	// 首先查找匹配的user-agent规则
	for _, rule := range p.rules {
		if rule.UserAgent == "*" {
			continue // 跳过默认规则
		}
		if strings.Contains(userAgent, rule.UserAgent) {
			return rule
		}
	}

	// 没有匹配的规则，使用默认规则
	return p.defaults
}

// matchPath 检查路径是否匹配规则
func (p *Parser) matchPath(path, pattern string) bool {
	// 转换为正则表达式
	// * 匹配任意字符
	// $ 匹配路径结束
	pattern = strings.ReplaceAll(pattern, "*", ".*")
	if !strings.HasSuffix(pattern, "$") {
		pattern += ".*"
	}

	re := regexp.MustCompile("^" + pattern + "$")
	return re.MatchString(path)
}

// GetSitemaps 获取sitemap列表
func (p *Parser) GetSitemaps() []string {
	return p.sitemaps
}

// GetCrawlDelay 获取爬取延迟
func (p *Parser) GetCrawlDelay(userAgent string) int {
	rule := p.getRule(userAgent)
	if rule == nil {
		return 0
	}
	return rule.CrawlDelay
}

// GetDisallowPaths 获取禁止访问的路径
func (p *Parser) GetDisallowPaths(userAgent string) []string {
	rule := p.getRule(userAgent)
	if rule == nil {
		return nil
	}
	return rule.Disallow
}

// GetAllowPaths 获取允许访问的路径
func (p *Parser) GetAllowPaths(userAgent string) []string {
	rule := p.getRule(userAgent)
	if rule == nil {
		return nil
	}
	return rule.Allow
}

// HasRules 检查是否有规则
func (p *Parser) HasRules() bool {
	return len(p.rules) > 0
}

// Clear 清空解析器状态
func (p *Parser) Clear() {
	p.rules = make([]*types.RobotsRules, 0)
	p.defaults = nil
	p.sitemaps = make([]string, 0)
}

// ParseString 解析robots.txt字符串
func (p *Parser) ParseString(robotsStr, userAgent string) error {
	return p.Parse([]byte(robotsStr), userAgent)
}

// ParseBytes 解析robots.txt字节数组
func (p *Parser) ParseBytes(data []byte, userAgent string) error {
	return p.Parse(data, userAgent)
}

// ParseFromReader 从Reader解析robots.txt
func (p *Parser) ParseFromReader(r io.Reader, userAgent string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("读取robots.txt失败: %w", err)
	}
	return p.Parse(data, userAgent)
}

// GetRules 获取所有规则
func (p *Parser) GetRules() []*types.RobotsRules {
	return p.rules
}

// GetDefaultRule 获取默认规则
func (p *Parser) GetDefaultRule() *types.RobotsRules {
	return p.defaults
}

// IsPathAllowed 检查路径是否被允许
func (p *Parser) IsPathAllowed(path, userAgent string) bool {
	rule := p.getRule(userAgent)
	if rule == nil {
		return true
	}

	// 检查Allow规则
	for _, allow := range rule.Allow {
		if p.matchPath(path, allow) {
			return true
		}
	}

	// 检查Disallow规则
	for _, disallow := range rule.Disallow {
		if p.matchPath(path, disallow) {
			return false
		}
	}

	return true
}

// GetRuleForUserAgent 获取特定user-agent的规则
func (p *Parser) GetRuleForUserAgent(userAgent string) *types.RobotsRules {
	return p.getRule(userAgent)
}

// AddRule 添加规则
func (p *Parser) AddRule(rule *types.RobotsRules) {
	p.rules = append(p.rules, rule)
	if rule.UserAgent == "*" {
		p.defaults = rule
	}
}

// AddSitemap 添加sitemap
func (p *Parser) AddSitemap(sitemapURL string) {
	p.sitemaps = append(p.sitemaps, sitemapURL)
}

// MatchUserAgent 检查user-agent是否匹配规则
func (p *Parser) MatchUserAgent(userAgent, ruleUserAgent string) bool {
	userAgent = strings.ToLower(userAgent)
	ruleUserAgent = strings.ToLower(ruleUserAgent)
	
	if ruleUserAgent == "*" {
		return true
	}
	
	return strings.Contains(userAgent, ruleUserAgent)
}

// ParseBuffer 解析robots.txt缓冲区
func (p *Parser) ParseBuffer(data []byte, size int, userAgent string) error {
	if size > len(data) {
		size = len(data)
	}
	return p.Parse(data[:size], userAgent)
}

// GetSitemapCount 获取sitemap数量
func (p *Parser) GetSitemapCount() int {
	return len(p.sitemaps)
}

// GetRuleCount 获取规则数量
func (p *Parser) GetRuleCount() int {
	return len(p.rules)
}

// Validate 验证robots.txt格式
func (p *Parser) Validate(data []byte) error {
	// 简单验证：检查是否包含有效的字段
	scanner := bufio.NewScanner(bytes.NewReader(data))
	hasValidField := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			if key == "user-agent" || key == "disallow" || key == "allow" || 
			   key == "crawl-delay" || key == "sitemap" {
				hasValidField = true
				break
			}
		}
	}

	if !hasValidField {
		return fmt.Errorf("无效的robots.txt格式")
	}

	return nil
}