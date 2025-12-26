package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/http2"

	"github.com/example/wget2go/internal/core/types"
)

// Client HTTP客户端
type Client struct {
	httpClient   *http.Client
	config       *types.Config
	userAgent    string
	proxyManager *ProxyManager
}

// NewClient 创建新的HTTP客户端
func NewClient(config *types.Config) *Client {
	// 创建代理管理器
	var proxyManager *ProxyManager
	var err error

	if config.ProxyEnabled || config.HTTPProxy != "" || config.HTTPSProxy != "" {
		proxyManager, err = NewProxyManager(config)
		if err != nil {
			// 代理配置错误，记录警告但不阻止程序运行
			if config.Verbose {
				fmt.Printf("警告: 创建代理管理器失败: %v\n", err)
			}
		}
	}

	// 创建传输层配置
	var transport *http.Transport
	if proxyManager != nil {
		transport = NewProxyTransport(proxyManager, config.Insecure, config.Timeout)
	} else {
		transport = &http.Transport{
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			DisableCompression:  true, // 禁用自动解压，避免文件大小计算问题
		}

		// 如果允许不安全的SSL连接
		if config.Insecure {
			transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		}
	}

	// 启用HTTP/2
	http2.ConfigureTransport(transport)

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !config.FollowRedirects || len(via) >= config.MaxRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	return &Client{
		httpClient:   client,
		config:       config,
		userAgent:    getUserAgent(config),
		proxyManager: proxyManager,
	}
}

// getUserAgent 获取User-Agent
func getUserAgent(config *types.Config) string {
	if config.UserAgent != "" {
		return config.UserAgent
	}
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"
}

// Head 发送HEAD请求获取文件信息
func (c *Client) Head(ctx context.Context, urlStr string) (*types.HTTPResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("创建HEAD请求失败: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("执行HEAD请求失败: %w", err)
	}
	defer resp.Body.Close()

	return c.parseResponse(resp), nil
}

// Get 发送GET请求下载文件
func (c *Client) Get(ctx context.Context, urlStr string, rangeHeader string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("创建GET请求失败: %w", err)
	}

	c.setHeaders(req)

	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("执行GET请求失败: %w", err)
	}

	return resp, nil
}

// DownloadRange 下载指定范围的数据
func (c *Client) DownloadRange(ctx context.Context, urlStr string, start, end int64) (io.ReadCloser, int64, error) {
	rangeHeader := fmt.Sprintf("bytes=%d-%d", start, end)
	resp, err := c.Get(ctx, urlStr, rangeHeader)
	if err != nil {
		return nil, 0, err
	}

	if resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("服务器不支持范围请求，状态码: %d", resp.StatusCode)
	}

	return resp.Body, resp.ContentLength, nil
}

// setHeaders 设置请求头
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", c.userAgent)
	
	if c.config.Referer != "" {
		req.Header.Set("Referer", c.config.Referer)
	}

	// 设置自定义头部
	for key, value := range c.config.Headers {
		req.Header.Set(key, value)
	}

	// 设置Cookie
	if len(c.config.Cookies) > 0 {
		var cookies []string
		for name, value := range c.config.Cookies {
			cookies = append(cookies, fmt.Sprintf("%s=%s", name, value))
		}
		req.Header.Set("Cookie", strings.Join(cookies, "; "))
	}

	// 添加代理认证头（如果配置了代理认证）
	if c.proxyManager != nil && (c.config.ProxyUsername != "" || c.config.ProxyPassword != "") {
		proxyAuth := c.proxyManager.GetProxyAuthHeader()
		if proxyAuth != "" {
			req.Header.Set("Proxy-Authorization", proxyAuth)
		}
	}

	// 对于下载请求，总是要求不压缩，避免文件大小计算问题
	// 同时支持断点续传（identity编码确保范围请求正常工作）
	req.Header.Set("Accept-Encoding", "identity")
}

// parseResponse 解析HTTP响应
func (c *Client) parseResponse(resp *http.Response) *types.HTTPResponse {
	contentLength, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	
	var lastModified time.Time
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		lastModified, _ = time.Parse(time.RFC1123, lm)
	}

	acceptRanges := resp.Header.Get("Accept-Ranges") == "bytes"

	return &types.HTTPResponse{
		StatusCode:    resp.StatusCode,
		ContentLength: contentLength,
		ContentType:   resp.Header.Get("Content-Type"),
		LastModified:  lastModified,
		ETag:          resp.Header.Get("ETag"),
		AcceptRanges:  acceptRanges,
	}
}

// IsValidURL 验证URL是否有效
func (c *Client) IsValidURL(urlStr string) bool {
	_, err := url.ParseRequestURI(urlStr)
	return err == nil
}

// GetFileNameFromURL 从URL中提取文件名
func (c *Client) GetFileNameFromURL(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "download"
	}

	path := parsedURL.Path
	if path == "" || path == "/" {
		return "index.html"
	}

	// 获取路径的最后一部分
	parts := strings.Split(path, "/")
	filename := parts[len(parts)-1]
	
	if filename == "" {
		return "index.html"
	}

	return filename
}