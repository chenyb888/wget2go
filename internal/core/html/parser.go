package html

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/example/wget2go/internal/core/types"
	"golang.org/x/net/html"
)

// Parser HTML解析器
type Parser struct {
	FollowTags []string
	IgnoreTags []string
}

// NewParser 创建HTML解析器
func NewParser() *Parser {
	return &Parser{
		FollowTags: []string{"a", "link", "img", "script", "iframe", "frame", "embed", "object", "area", "base", "body", "input", "form", "meta"},
		IgnoreTags: []string{},
	}
}

// Parse 解析HTML并提取URL
func (p *Parser) Parse(htmlData []byte, baseURL string) (*types.ParsedResult, error) {
	result := &types.ParsedResult{
		URLs:     make([]*types.ParsedURL, 0),
		Follow:   true,
		Encoding: "utf-8",
		Links:    make(map[string]string),
	}

	// 检测BOM编码
	if len(htmlData) >= 3 && htmlData[0] == 0xEF && htmlData[1] == 0xBB && htmlData[2] == 0xBF {
		htmlData = htmlData[3:] // 移除UTF-8 BOM
		result.Encoding = "utf-8"
	}

	// 解析HTML
	doc, err := html.Parse(bytes.NewReader(htmlData))
	if err != nil {
		return nil, fmt.Errorf("解析HTML失败: %w", err)
	}

	// 遍历DOM树
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// 处理META robots标签
			if strings.ToLower(n.Data) == "meta" {
				p.processMetaTag(n, result)
			}

			// 跳过不处理的标签
			if p.shouldIgnoreTag(n.Data) {
				return
			}

			// 提取URL
			p.extractURLs(n, baseURL, result)
		}

		// 递归遍历子节点
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)

	return result, nil
}

// processMetaTag 处理META标签
func (p *Parser) processMetaTag(n *html.Node, result *types.ParsedResult) {
	var name, content string
	for _, attr := range n.Attr {
		switch strings.ToLower(attr.Key) {
		case "name":
			name = strings.ToLower(attr.Val)
		case "content":
			content = strings.ToLower(attr.Val)
		}
	}

	if name == "robots" {
		// 检查nofollow指令
		if strings.Contains(content, "nofollow") {
			result.Follow = false
		}
		if strings.Contains(content, "noindex") {
			result.Follow = false
		}
	}
}

// shouldIgnoreTag 检查是否应该忽略该标签
func (p *Parser) shouldIgnoreTag(tag string) bool {
	for _, ignoreTag := range p.IgnoreTags {
		if strings.EqualFold(tag, ignoreTag) {
			return true
		}
	}
	return false
}

// extractURLs 从节点中提取URL
func (p *Parser) extractURLs(n *html.Node, baseURL string, result *types.ParsedResult) {
	// 定义需要提取URL的属性
	urlAttrs := map[string]string{
		"a":       "href",
		"link":    "href",
		"img":     "src",
		"script":  "src",
		"iframe":  "src",
		"frame":   "src",
		"embed":   "src",
		"object":  "data",
		"area":    "href",
		"base":    "href",
		"body":    "background",
		"input":   "src",
		"form":    "action",
		"blockquote": "cite",
		"q":       "cite",
		"ins":     "cite",
		"del":     "cite",
	}

	tag := strings.ToLower(n.Data)
	attrName, ok := urlAttrs[tag]
	if !ok {
		return
	}

	// 获取属性值
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, attrName) {
			urlStr := strings.TrimSpace(attr.Val)
			if urlStr == "" || urlStr == "#" || strings.HasPrefix(urlStr, "javascript:") || strings.HasPrefix(urlStr, "data:") {
				continue
			}

			// 跳过action和formaction属性（这些不是要下载的链接）
			if attrName == "action" || attrName == "formaction" {
				continue
			}

			// 标准化URL
			normalizedURL, err := normalizeURL(urlStr, baseURL)
			if err != nil {
				continue
			}

			// 添加到结果
			parsedURL := &types.ParsedURL{
				URL:      normalizedURL,
				Attr:     attrName,
				Tag:      tag,
			}
			result.URLs = append(result.URLs, parsedURL)
			result.Links[urlStr] = normalizedURL
		}
	}

	// 处理srcset属性（用于img标签）
	if tag == "img" {
		for _, attr := range n.Attr {
			if strings.EqualFold(attr.Key, "srcset") {
				p.processSrcSet(attr.Val, baseURL, result)
			}
		}
	}

	// 处理style属性中的URL
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, "style") {
			p.processStyleURLs(attr.Val, baseURL, result)
		}
	}
}

// processSrcSet 处理srcset属性
func (p *Parser) processSrcSet(srcset, baseURL string, result *types.ParsedResult) {
	// srcset格式: "image1.jpg 1x, image2.jpg 2x"
	parts := strings.Split(srcset, ",")
	for _, part := range parts {
		// 提取URL（移除描述符）
		urlPart := strings.Fields(part)[0]
		urlStr := strings.TrimSpace(urlPart)
		if urlStr == "" {
			continue
		}

		normalizedURL, err := normalizeURL(urlStr, baseURL)
		if err != nil {
			continue
		}

		parsedURL := &types.ParsedURL{
			URL:  normalizedURL,
			Attr: "srcset",
			Tag:  "img",
		}
		result.URLs = append(result.URLs, parsedURL)
		result.Links[urlStr] = normalizedURL
	}
}

// processStyleURLs 处理style属性中的URL
func (p *Parser) processStyleURLs(style, baseURL string, result *types.ParsedResult) {
	// 查找url()模式
	re := regexp.MustCompile(`url\(['"]?([^'")\s]+)['"]?\)`)
	matches := re.FindAllStringSubmatch(style, -1)

	for _, match := range matches {
		if len(match) > 1 {
			urlStr := match[1]
			normalizedURL, err := normalizeURL(urlStr, baseURL)
			if err != nil {
				continue
			}

			parsedURL := &types.ParsedURL{
				URL:  normalizedURL,
				Attr: "style",
				Tag:  "*",
			}
			result.URLs = append(result.URLs, parsedURL)
			result.Links[urlStr] = normalizedURL
		}
	}
}

// normalizeURL 标准化URL
func normalizeURL(urlStr, baseURL string) (string, error) {
	if urlStr == "" {
		return "", fmt.Errorf("空URL")
	}

	// 解析URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	// 如果是相对URL，使用baseURL解析
	if !u.IsAbs() {
		if baseURL == "" {
			return "", fmt.Errorf("相对URL需要baseURL")
		}
		base, err := url.Parse(baseURL)
		if err != nil {
			return "", err
		}
		resolved := base.ResolveReference(u)
		return resolved.String(), nil
	}

	return u.String(), nil
}

// ParseReader 从io.Reader解析HTML
func (p *Parser) ParseReader(r io.Reader, baseURL string) (*types.ParsedResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("读取HTML失败: %w", err)
	}
	return p.Parse(data, baseURL)
}

// GetInlineURLs 获取内联URL（用于页面必需资源）
func (p *Parser) GetInlineURLs(htmlData []byte, baseURL string) ([]string, error) {
	result, err := p.Parse(htmlData, baseURL)
	if err != nil {
		return nil, err
	}

	var urls []string
	for _, parsed := range result.URLs {
		// 内联资源通常在img、script、link等标签中
		if parsed.Tag == "img" || parsed.Tag == "script" || parsed.Tag == "link" {
			urls = append(urls, parsed.URL)
		}
	}

	return urls, nil
}

// SetFollowTags 设置要跟随的标签
func (p *Parser) SetFollowTags(tags []string) {
	p.FollowTags = tags
}

// SetIgnoreTags 设置要忽略的标签
func (p *Parser) SetIgnoreTags(tags []string) {
	p.IgnoreTags = tags
}

// GetFollowedTags 获取当前跟随的标签列表
func (p *Parser) GetFollowedTags() []string {
	return p.FollowTags
}