package http

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/example/wget2go/internal/core/types"
)

// ProxyConfig 代理配置
type ProxyConfig struct {
	HTTPProxy     *url.URL
	HTTPSProxy    *url.URL
	NoProxyList   []string
	ProxyUsername string
	ProxyPassword string
}

// ProxyManager 代理管理器
type ProxyManager struct {
	config      *ProxyConfig
	proxyMutex  sync.RWMutex
	httpIndex   int
	httpsIndex  int
	httpProxies []*url.URL
	httpsProxies []*url.URL
}

// NewProxyManager 创建代理管理器
func NewProxyManager(cfg *types.Config) (*ProxyManager, error) {
	pm := &ProxyConfig{
		ProxyUsername: cfg.ProxyUsername,
		ProxyPassword: cfg.ProxyPassword,
	}

	// 解析HTTP和HTTPS代理列表
	httpProxies := parseProxyList(cfg.HTTPProxy)
	httpsProxies := parseProxyList(cfg.HTTPSProxy)

	// 解析HTTP代理列表
	if cfg.HTTPProxy != "" && len(httpProxies) > 0 {
		pm.HTTPProxy = httpProxies[0]
	}

	// 解析HTTPS代理列表
	if cfg.HTTPSProxy != "" && len(httpsProxies) > 0 {
		pm.HTTPSProxy = httpsProxies[0]
	}

	// 解析no_proxy列表
	if cfg.NoProxy != "" {
		pm.NoProxyList = parseNoProxyList(cfg.NoProxy)
	}

	return &ProxyManager{
		config:        pm,
		httpProxies:   httpProxies,
		httpsProxies:  httpsProxies,
	}, nil
}

// parseProxyList 解析代理列表（逗号分隔）
func parseProxyList(proxyStr string) []*url.URL {
	if proxyStr == "" {
		return nil
	}

	var proxies []*url.URL
	parts := strings.Split(proxyStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 检查是否包含协议
		if !strings.Contains(part, "://") {
			// 默认使用http协议
			part = "http://" + part
		}

		u, err := url.Parse(part)
		if err != nil {
			continue
		}
		proxies = append(proxies, u)
	}

	return proxies
}

// parseNoProxyList 解析no_proxy列表
func parseNoProxyList(noProxyStr string) []string {
	if noProxyStr == "" {
		return nil
	}

	parts := strings.Split(noProxyStr, ",")
	var result []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

// GetProxyForURL 获取指定URL的代理
func (pm *ProxyManager) GetProxyForURL(targetURL *url.URL) (*url.URL, error) {
	if pm.config == nil {
		return nil, nil
	}

	// 检查是否在no_proxy列表中
	if pm.isNoProxy(targetURL.Hostname()) {
		return nil, nil
	}

	pm.proxyMutex.Lock()
	defer pm.proxyMutex.Unlock()

	// 根据协议选择代理
	if targetURL.Scheme == "https" {
		if len(pm.httpsProxies) > 0 {
			proxy := pm.httpsProxies[pm.httpsIndex%len(pm.httpsProxies)]
			pm.httpsIndex++
			return proxy, nil
		}
		// HTTPS如果没有专门的代理，使用HTTP代理
		if len(pm.httpProxies) > 0 {
			proxy := pm.httpProxies[pm.httpIndex%len(pm.httpProxies)]
			pm.httpIndex++
			return proxy, nil
		}
	} else {
		if len(pm.httpProxies) > 0 {
			proxy := pm.httpProxies[pm.httpIndex%len(pm.httpProxies)]
			pm.httpIndex++
			return proxy, nil
		}
	}

	return nil, nil
}

// isNoProxy 检查主机是否在no_proxy列表中
func (pm *ProxyManager) isNoProxy(host string) bool {
	if len(pm.config.NoProxyList) == 0 {
		return false
	}

	host = strings.ToLower(host)

	for _, pattern := range pm.config.NoProxyList {
		pattern = strings.ToLower(strings.TrimSpace(pattern))

		// 精确匹配
		if host == pattern {
			return true
		}

		// 域名后缀匹配（如 .example.com）
		if strings.HasPrefix(pattern, ".") {
			if host == pattern[1:] || strings.HasSuffix(host, pattern) {
				return true
			}
		}

		// IPv4 CIDR匹配
		if isIPv4CIDR(pattern) {
			if matchIPv4CIDR(host, pattern) {
				return true
			}
		}

		// IPv6 CIDR匹配
		if isIPv6CIDR(pattern) {
			if matchIPv6CIDR(host, pattern) {
				return true
			}
		}
	}

	return false
}

// isIPv4CIDR 检查是否为IPv4 CIDR格式
func isIPv4CIDR(pattern string) bool {
	parts := strings.Split(pattern, "/")
	if len(parts) != 2 {
		return false
	}
	ip := net.ParseIP(parts[0])
	return ip != nil && ip.To4() != nil
}

// isIPv6CIDR 检查是否为IPv6 CIDR格式
func isIPv6CIDR(pattern string) bool {
	parts := strings.Split(pattern, "/")
	if len(parts) != 2 {
		return false
	}
	ip := net.ParseIP(parts[0])
	return ip != nil && ip.To4() == nil
}

// matchIPv4CIDR IPv4 CIDR匹配
func matchIPv4CIDR(host, pattern string) bool {
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil {
		return false
	}

	_, ipNet, err := net.ParseCIDR(pattern)
	if err != nil {
		return false
	}

	return ipNet.Contains(ip)
}

// matchIPv6CIDR IPv6 CIDR匹配
func matchIPv6CIDR(host, pattern string) bool {
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() != nil {
		return false
	}

	_, ipNet, err := net.ParseCIDR(pattern)
	if err != nil {
		return false
	}

	return ipNet.Contains(ip)
}

// GetProxyAuthHeader 获取代理认证头
func (pm *ProxyManager) GetProxyAuthHeader() string {
	if pm.config.ProxyUsername == "" && pm.config.ProxyPassword == "" {
		return ""
	}

	auth := pm.config.ProxyUsername + ":" + pm.config.ProxyPassword
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

// NewProxyTransport 创建支持代理的Transport
func NewProxyTransport(pm *ProxyManager, insecure bool, timeout time.Duration) *http.Transport {
	transport := &http.Transport{
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  true,
	}

	// 如果允许不安全的SSL连接
	if insecure {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	// 设置代理函数
	if pm != nil {
		transport.Proxy = func(req *http.Request) (*url.URL, error) {
			return pm.GetProxyForURL(req.URL)
		}
	}

	return transport
}

// EstablishConnectForHTTPS 为HTTPS建立CONNECT隧道
func EstablishConnectForHTTPS(ctx context.Context, proxyURL, targetURL *url.URL, proxyAuth string, timeout time.Duration) (net.Conn, error) {
	// 连接到代理服务器
	dialer := &net.Dialer{
		Timeout: timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", proxyURL.Host)
	if err != nil {
		return nil, fmt.Errorf("连接代理服务器失败: %w", err)
	}

	// 发送CONNECT请求
	connectReq := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: targetURL.Host},
		Host:   targetURL.Host,
		Header: make(http.Header),
	}

	if proxyAuth != "" {
		connectReq.Header.Set("Proxy-Authorization", proxyAuth)
	}

	// 写入CONNECT请求
	if err := connectReq.Write(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("发送CONNECT请求失败: %w", err)
	}

	// 读取响应
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("读取代理响应失败: %w", err)
	}
	resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("代理CONNECT失败，状态码: %d", resp.StatusCode)
	}

	return conn, nil
}



// ParseProxyResponse 解析代理响应状态
func ParseProxyResponse(resp string) (int, string, error) {
	lines := strings.Split(resp, "\r\n")
	if len(lines) == 0 {
		return 0, "", fmt.Errorf("无效的代理响应")
	}

	// 解析状态行
	statusLine := lines[0]
	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		return 0, "", fmt.Errorf("无效的状态行: %s", statusLine)
	}

	statusCode, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, "", fmt.Errorf("无效的状态码: %s", parts[1])
	}

	message := ""
	if len(parts) > 2 {
		message = parts[2]
	}

	return statusCode, message, nil
}

// IsProxyAuthenticationRequired 检查是否需要代理认证
func IsProxyAuthenticationRequired(statusCode int) bool {
	return statusCode == http.StatusProxyAuthRequired
}

// GetProxyAuthChallenge 从响应中提取认证挑战
func GetProxyAuthChallenge(resp *http.Response) string {
	return resp.Header.Get("Proxy-Authenticate")
}

// AddProxyAuthHeader 添加代理认证头
func AddProxyAuthHeader(req *http.Request, username, password string) {
	if username == "" && password == "" {
		return
	}

	auth := username + ":" + password
	encoded := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Proxy-Authorization", "Basic "+encoded)
}

// ParseProxyAuthenticateHeader 解析Proxy-Authenticate头
func ParseProxyAuthenticateHeader(header string) (authScheme, realm string) {
	if header == "" {
		return "", ""
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) == 0 {
		return "", ""
	}

	authScheme = strings.TrimSpace(parts[0])

	if len(parts) > 1 {
		// 提取realm参数
		re := regexp.MustCompile(`realm\s*=\s*"([^"]*)"`)
		matches := re.FindStringSubmatch(parts[1])
		if len(matches) > 1 {
			realm = matches[1]
		}
	}

	return authScheme, realm
}

// ValidateProxyURL 验证代理URL是否有效
func ValidateProxyURL(proxyURL string) error {
	if proxyURL == "" {
		return nil
	}

	// 检查是否包含协议
	if !strings.Contains(proxyURL, "://") {
		proxyURL = "http://" + proxyURL
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("无效的代理URL: %w", err)
	}

	// 检查协议
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" {
		return fmt.Errorf("不支持的代理协议: %s", u.Scheme)
	}

	// 检查主机
	if u.Host == "" {
		return fmt.Errorf("代理URL缺少主机地址")
	}

	return nil
}

// GetProxyFromEnv 从环境变量获取代理配置
func GetProxyFromEnv() (httpProxy, httpsProxy, noProxy string) {
	httpProxy = osGetEnv("http_proxy", "")
	if httpProxy == "" {
		httpProxy = osGetEnv("HTTP_PROXY", "")
	}

	httpsProxy = osGetEnv("https_proxy", "")
	if httpsProxy == "" {
		httpsProxy = osGetEnv("HTTPS_PROXY", "")
	}

	noProxy = osGetEnv("no_proxy", "")
	if noProxy == "" {
		noProxy = osGetEnv("NO_PROXY", "")
	}

	return httpProxy, httpsProxy, noProxy
}

// osGetEnv 获取环境变量
func osGetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// ProxyDialer 代理拨号器
type ProxyDialer struct {
	proxyURL *url.URL
	dialer   *net.Dialer
}

// NewProxyDialer 创建代理拨号器
func NewProxyDialer(proxyURL *url.URL, timeout time.Duration) *ProxyDialer {
	return &ProxyDialer{
		proxyURL: proxyURL,
		dialer: &net.Dialer{
			Timeout: timeout,
		},
	}
}

// Dial 通过代理建立连接
func (pd *ProxyDialer) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	// 直接连接到代理服务器
	return pd.dialer.DialContext(ctx, network, pd.proxyURL.Host)
}

// DialContext 通过代理建立连接（带上下文）
func (pd *ProxyDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return pd.Dial(ctx, network, addr)
}

// CreateHTTPProxyClient 创建使用HTTP代理的客户端
func CreateHTTPProxyClient(proxyURL *url.URL, username, password string, timeout time.Duration) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// 如果有认证信息，需要自定义RoundTripper来添加认证头
	if username != "" || password != "" {
		transport.Proxy = func(req *http.Request) (*url.URL, error) {
			AddProxyAuthHeader(req, username, password)
			return proxyURL, nil
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

// GetProxyInfo 获取代理信息（用于调试）
func (pm *ProxyManager) GetProxyInfo() string {
	if pm == nil || pm.config == nil {
		return "代理未配置"
	}

	var info []string

	if pm.config.HTTPProxy != nil {
		info = append(info, fmt.Sprintf("HTTP代理: %s", pm.config.HTTPProxy.String()))
	}

	if pm.config.HTTPSProxy != nil {
		info = append(info, fmt.Sprintf("HTTPS代理: %s", pm.config.HTTPSProxy.String()))
	}

	if len(pm.config.NoProxyList) > 0 {
		info = append(info, fmt.Sprintf("No-Proxy: %s", strings.Join(pm.config.NoProxyList, ", ")))
	}

	if pm.config.ProxyUsername != "" {
		info = append(info, fmt.Sprintf("代理认证: 是 (用户名: %s)", pm.config.ProxyUsername))
	}

	if len(info) == 0 {
		return "代理未配置"
	}

	return strings.Join(info, "\n")
}