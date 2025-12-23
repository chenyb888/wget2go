package parser

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// HTMLExtractor HTML解析器
type HTMLExtractor struct {
	baseURL *url.URL
}

// NewHTMLExtractor 创建HTML解析器
func NewHTMLExtractor(baseURL string) (*HTMLExtractor, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("解析基础URL失败: %w", err)
	}

	return &HTMLExtractor{
		baseURL: parsedURL,
	}, nil
}

// ExtractURLs 从HTML中提取URL
func (h *HTMLExtractor) ExtractURLs(htmlContent string) ([]string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("解析HTML失败: %w", err)
	}

	var urls []string
	h.extractURLsFromNode(doc, &urls)
	
	// 去重
	return h.uniqueURLs(urls), nil
}

// extractURLsFromNode 递归提取节点中的URL
func (h *HTMLExtractor) extractURLsFromNode(n *html.Node, urls *[]string) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "a", "link":
			h.extractAttribute(n, "href", urls)
		case "img", "script", "iframe", "frame", "embed", "object":
			h.extractAttribute(n, "src", urls)
		case "form":
			h.extractAttribute(n, "action", urls)
		case "meta":
			h.extractMetaRefresh(n, urls)
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		h.extractURLsFromNode(c, urls)
	}
}

// extractAttribute 提取属性值
func (h *HTMLExtractor) extractAttribute(n *html.Node, attrName string, urls *[]string) {
	for _, attr := range n.Attr {
		if attr.Key == attrName && attr.Val != "" {
			if absoluteURL := h.resolveURL(attr.Val); absoluteURL != "" {
				*urls = append(*urls, absoluteURL)
			}
		}
	}
}

// extractMetaRefresh 提取meta refresh标签
func (h *HTMLExtractor) extractMetaRefresh(n *html.Node, urls *[]string) {
	var httpEquiv, content string
	
	for _, attr := range n.Attr {
		switch attr.Key {
		case "http-equiv":
			httpEquiv = strings.ToLower(attr.Val)
		case "content":
			content = attr.Val
		}
	}

	if httpEquiv == "refresh" && content != "" {
		// 解析 content="5;url=http://example.com/"
		if urlStr := h.parseRefreshContent(content); urlStr != "" {
			if absoluteURL := h.resolveURL(urlStr); absoluteURL != "" {
				*urls = append(*urls, absoluteURL)
			}
		}
	}
}

// parseRefreshContent 解析refresh content
func (h *HTMLExtractor) parseRefreshContent(content string) string {
	// 匹配 URL
	re := regexp.MustCompile(`(?i)url\s*=\s*(['"]?)([^'"]+)\1`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 2 {
		return matches[2]
	}
	return ""
}

// resolveURL 解析相对URL为绝对URL
func (h *HTMLExtractor) resolveURL(relativeURL string) string {
	if relativeURL == "" {
		return ""
	}

	// 跳过JavaScript和mailto等
	if strings.HasPrefix(strings.ToLower(relativeURL), "javascript:") ||
		strings.HasPrefix(strings.ToLower(relativeURL), "mailto:") ||
		strings.HasPrefix(strings.ToLower(relativeURL), "tel:") ||
		strings.HasPrefix(strings.ToLower(relativeURL), "#") {
		return ""
	}

	parsed, err := url.Parse(relativeURL)
	if err != nil {
		return ""
	}

	// 如果已经是绝对URL，直接返回
	if parsed.IsAbs() {
		return parsed.String()
	}

	// 解析为相对于baseURL的URL
	resolved := h.baseURL.ResolveReference(parsed)
	return resolved.String()
}

// uniqueURLs URL去重
func (h *HTMLExtractor) uniqueURLs(urls []string) []string {
	seen := make(map[string]bool)
	var unique []string

	for _, urlStr := range urls {
		if !seen[urlStr] {
			seen[urlStr] = true
			unique = append(unique, urlStr)
		}
	}

	return unique
}

// ExtractCSSURLs 从CSS中提取URL
func (h *HTMLExtractor) ExtractCSSURLs(cssContent string) []string {
	var urls []string
	
	// 匹配 url() 中的URL
	re := regexp.MustCompile(`url\s*\(\s*['"]?([^'"()]+)['"]?\s*\)`)
	matches := re.FindAllStringSubmatch(cssContent, -1)
	
	for _, match := range matches {
		if len(match) > 1 {
			if absoluteURL := h.resolveURL(match[1]); absoluteURL != "" {
				urls = append(urls, absoluteURL)
			}
		}
	}
	
	return h.uniqueURLs(urls)
}

// ExtractText 提取纯文本
func (h *HTMLExtractor) ExtractText(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("解析HTML失败: %w", err)
	}

	var text strings.Builder
	h.extractTextFromNode(doc, &text)
	
	return strings.TrimSpace(text.String()), nil
}

// extractTextFromNode 递归提取文本
func (h *HTMLExtractor) extractTextFromNode(n *html.Node, text *strings.Builder) {
	if n.Type == html.TextNode {
		text.WriteString(n.Data)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		h.extractTextFromNode(c, text)
	}
}