package css

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/example/wget2go/internal/core/types"
)

// Parser CSS解析器
type Parser struct {
	baseURL string
}

// NewParser 创建CSS解析器
func NewParser() *Parser {
	return &Parser{}
}

// Parse 解析CSS并提取URL
func (p *Parser) Parse(cssData []byte, baseURL string) (*types.ParsedResult, error) {
	result := &types.ParsedResult{
		URLs:     make([]*types.ParsedURL, 0),
		Follow:   true,
		Encoding: "utf-8",
		Links:    make(map[string]string),
	}

	p.baseURL = baseURL

	// 解析@import规则
	p.parseImportRules(cssData, result)

	// 解析url()函数
	p.parseURLFunctions(cssData, result)

	return result, nil
}

// parseImportRules 解析@import规则
func (p *Parser) parseImportRules(cssData []byte, result *types.ParsedResult) {
	// 匹配@import规则
	// 格式: @import url("style.css"); 或 @import "style.css";
	re := regexp.MustCompile(`@import\s+(?:url\()?['"]?([^'")\s]+)['"]?\)?\s*;`)
	matches := re.FindAllSubmatch(cssData, -1)

	for _, match := range matches {
		if len(match) > 1 {
			urlStr := string(match[1])
			normalizedURL, err := p.normalizeURL(urlStr)
			if err != nil {
				continue
			}

			parsedURL := &types.ParsedURL{
				URL:  normalizedURL,
				Attr: "@import",
				Tag:  "@import",
			}
			result.URLs = append(result.URLs, parsedURL)
			result.Links[urlStr] = normalizedURL
		}
	}
}

// parseURLFunctions 解析url()函数
func (p *Parser) parseURLFunctions(cssData []byte, result *types.ParsedResult) {
	// 匹配url()函数
	// 格式: url('image.png') 或 url("image.png") 或 url(image.png)
	re := regexp.MustCompile(`url\(['"]?([^'")\s]+)['"]?\)`)
	matches := re.FindAllSubmatch(cssData, -1)

	for _, match := range matches {
		if len(match) > 1 {
			urlStr := string(match[1])
			normalizedURL, err := p.normalizeURL(urlStr)
			if err != nil {
				continue
			}

			parsedURL := &types.ParsedURL{
				URL:  normalizedURL,
				Attr: "url()",
				Tag:  "css",
			}
			result.URLs = append(result.URLs, parsedURL)
			result.Links[urlStr] = normalizedURL
		}
	}
}

// normalizeURL 标准化URL
func (p *Parser) normalizeURL(urlStr string) (string, error) {
	if urlStr == "" {
		return "", fmt.Errorf("空URL")
	}

	// 跳过data URL
	if strings.HasPrefix(urlStr, "data:") {
		return "", fmt.Errorf("跳过data URL")
	}

	// 解析URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	// 如果是相对URL，使用baseURL解析
	if !u.IsAbs() {
		if p.baseURL == "" {
			return "", fmt.Errorf("相对URL需要baseURL")
		}
		base, err := url.Parse(p.baseURL)
		if err != nil {
			return "", err
		}
		resolved := base.ResolveReference(u)
		return resolved.String(), nil
	}

	return u.String(), nil
}

// ParseReader 从io.Reader解析CSS
func (p *Parser) ParseReader(r io.Reader, baseURL string) (*types.ParsedResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("读取CSS失败: %w", err)
	}
	return p.Parse(data, baseURL)
}

// ParseBuffer 解析CSS缓冲区
func (p *Parser) ParseBuffer(data []byte, size int, baseURL string) (*types.ParsedResult, error) {
	if size > len(data) {
		size = len(data)
	}
	return p.Parse(data[:size], baseURL)
}

// GetURLs 获取CSS中的所有URL
func (p *Parser) GetURLs(cssData []byte, baseURL string) ([]string, error) {
	result, err := p.Parse(cssData, baseURL)
	if err != nil {
		return nil, err
	}

	urls := make([]string, 0, len(result.URLs))
	for _, parsed := range result.URLs {
		urls = append(urls, parsed.URL)
	}

	return urls, nil
}

// ExtractBackgroundURLs 提取background属性中的URL
func (p *Parser) ExtractBackgroundURLs(cssData []byte, baseURL string) ([]string, error) {
	result := &types.ParsedResult{
		URLs:  make([]*types.ParsedURL, 0),
		Links: make(map[string]string),
	}

	p.baseURL = baseURL

	// 匹配background和background-image属性
	re := regexp.MustCompile(`(?:background|background-image)\s*:\s*([^;]+);`)
	matches := re.FindAllSubmatch(cssData, -1)

	for _, match := range matches {
		if len(match) > 1 {
			value := string(match[1])
			p.parseURLFunctions([]byte(value), result)
		}
	}

	urls := make([]string, 0, len(result.URLs))
	for _, parsed := range result.URLs {
		urls = append(urls, parsed.URL)
	}

	return urls, nil
}

// ParseFontFace 解析@font-face规则
func (p *Parser) ParseFontFace(cssData []byte, baseURL string) ([]string, error) {
	result := &types.ParsedResult{
		URLs:  make([]*types.ParsedURL, 0),
		Links: make(map[string]string),
	}

	p.baseURL = baseURL

	// 匹配@font-face规则中的src属性
	re := regexp.MustCompile(`@font-face\s*\{[^}]*src\s*:\s*([^;]+);`)
	matches := re.FindAllSubmatch(cssData, -1)

	for _, match := range matches {
		if len(match) > 1 {
			value := string(match[1])
			p.parseURLFunctions([]byte(value), result)
		}
	}

	urls := make([]string, 0, len(result.URLs))
	for _, parsed := range result.URLs {
		urls = append(urls, parsed.URL)
	}

	return urls, nil
}

// IsCSSContent 检查是否为CSS内容
func (p *Parser) IsCSSContent(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/css")
}

// ParseStyleAttribute 解析HTML元素的style属性
func (p *Parser) ParseStyleAttribute(styleAttr, baseURL string) ([]string, error) {
	result := &types.ParsedResult{
		URLs:  make([]*types.ParsedURL, 0),
		Links: make(map[string]string),
	}

	p.baseURL = baseURL
	p.parseURLFunctions([]byte(styleAttr), result)

	urls := make([]string, 0, len(result.URLs))
	for _, parsed := range result.URLs {
		urls = append(urls, parsed.URL)
	}

	return urls, nil
}

// ParseInlineStyle 解析内联style标签
func (p *Parser) ParseInlineStyle(styleData []byte, baseURL string) (*types.ParsedResult, error) {
	return p.Parse(styleData, baseURL)
}

// GetEncoding 检测CSS编码
func (p *Parser) GetEncoding(cssData []byte) string {
	// 检查@charset规则
	re := regexp.MustCompile(`@charset\s+['"]([^'"]+)['"]\s*;`)
	matches := re.FindSubmatch(cssData)
	if len(matches) > 1 {
		return strings.ToLower(string(matches[1]))
	}

	// 默认使用UTF-8
	return "utf-8"
}

// ParseBufferWithEncoding 使用指定编码解析CSS缓冲区
func (p *Parser) ParseBufferWithEncoding(data []byte, size int, baseURL, encoding string) (*types.ParsedResult, error) {
	if size > len(data) {
		size = len(data)
	}
	
	// 简化处理，直接使用UTF-8
	// 实际实现中可能需要进行编码转换
	return p.Parse(data[:size], baseURL)
}

// GetURLCount 获取URL数量
func (p *Parser) GetURLCount(cssData []byte, baseURL string) (int, error) {
	result, err := p.Parse(cssData, baseURL)
	if err != nil {
		return 0, err
	}
	return len(result.URLs), nil
}

// ParseString 解析CSS字符串
func (p *Parser) ParseString(cssStr, baseURL string) (*types.ParsedResult, error) {
	return p.Parse([]byte(cssStr), baseURL)
}

// ParseBytes 解析CSS字节数组
func (p *Parser) ParseBytes(cssData []byte, baseURL string) (*types.ParsedResult, error) {
	return p.Parse(cssData, baseURL)
}

// ParseFromReader 从Reader解析CSS
func (p *Parser) ParseFromReader(r io.Reader, baseURL string) (*types.ParsedResult, error) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	if err != nil {
		return nil, fmt.Errorf("读取CSS失败: %w", err)
	}
	return p.Parse(buf.Bytes(), baseURL)
}