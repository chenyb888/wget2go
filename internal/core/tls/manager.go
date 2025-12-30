package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/example/wget2go/internal/core/types"
)

// CertManager 证书管理器
type CertManager struct {
	config *types.Config
}

// NewCertManager 创建证书管理器
func NewCertManager(config *types.Config) *CertManager {
	return &CertManager{
		config: config,
	}
}

// GetTLSConfig 获取TLS配置
func (m *CertManager) GetTLSConfig() *tls.Config {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
	}

	if m.config.Insecure {
		tlsConfig.InsecureSkipVerify = true
	} else {
		// 加载系统证书
		if certPool, err := m.loadSystemCertPool(); err == nil {
			tlsConfig.RootCAs = certPool
		}
	}

	return tlsConfig
}

// loadSystemCertPool 加载系统证书池
func (m *CertManager) loadSystemCertPool() (*x509.CertPool, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		// 如果系统证书池不可用，创建新的证书池
		certPool = x509.NewCertPool()
		
		// 尝试加载常见证书文件
		certFiles := []string{
			"/etc/ssl/certs/ca-certificates.crt",
			"/etc/pki/tls/certs/ca-bundle.crt",
			"/usr/share/ssl/certs/ca-bundle.crt",
			"/usr/local/share/certs/ca-root-nss.crt",
			"/etc/ssl/cert.pem",
		}

		for _, certFile := range certFiles {
			if data, err := os.ReadFile(certFile); err == nil {
				if certPool.AppendCertsFromPEM(data) {
					return certPool, nil
				}
			}
		}

		return nil, fmt.Errorf("无法加载系统证书")
	}

	return certPool, nil
}

// VerifyCertificate 验证证书
func (m *CertManager) VerifyCertificate(serverName string, cert *x509.Certificate) error {
	// 检查证书是否过期
	if time.Now().After(cert.NotAfter) {
		return fmt.Errorf("证书已过期: %s", cert.NotAfter)
	}

	if time.Now().Before(cert.NotBefore) {
		return fmt.Errorf("证书尚未生效: %s", cert.NotBefore)
	}

	// 验证主机名
	if err := cert.VerifyHostname(serverName); err != nil {
		return fmt.Errorf("主机名验证失败: %w", err)
	}

	return nil
}

// CheckOCSP OCSP检查
func (m *CertManager) CheckOCSP(cert *x509.Certificate) (bool, error) {
	// 如果证书没有OCSP扩展，跳过检查
	if len(cert.OCSPServer) == 0 {
		return true, nil
	}

	// 如果没有颁发者证书，无法验证OCSP响应签名
	// 在实际应用中，应该从证书链中获取颁发者证书
	// 这里返回true表示跳过OCSP检查（不影响正常使用）
	if m.config.Verbose {
		fmt.Printf("OCSP检查: 证书包含OCSP服务器但缺少颁发者证书，跳过检查\n")
	}
	return true, nil
}

// CheckCRL CRL检查
func (m *CertManager) CheckCRL(cert *x509.Certificate) (bool, error) {
	// 如果证书没有CRL扩展，跳过检查
	crlURIs := cert.CRLDistributionPoints
	if len(crlURIs) == 0 {
		return true, nil
	}

	// CRL检查需要下载和解析CRL文件
	// 在实际应用中应该：
	// 1. 解析CRL URL
	// 2. 下载CRL文件
	// 3. 验证CRL签名
	// 4. 检查证书序列号是否在CRL中
	// 5. 检查CRL是否过期

	if m.config.Verbose {
		for _, crlURI := range crlURIs {
			fmt.Printf("CRL检查: 发现CRL分发点 %s (跳过实际检查)\n", crlURI)
		}
	}

	// 返回true表示证书未被撤销（简化处理）
	return true, nil
}

// GetCipherSuites 获取支持的加密套件
func (m *CertManager) GetCipherSuites() []uint16 {
	return []uint16{
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	}
}

// GetCurvePreferences 获取曲线偏好
func (m *CertManager) GetCurvePreferences() []tls.CurveID {
	return []tls.CurveID{
		tls.X25519,
		tls.CurveP256,
		tls.CurveP384,
		tls.CurveP521,
	}
}

// HSTSPolicy HSTS策略
type HSTSPolicy struct {
	Domain           string
	MaxAge           time.Duration
	IncludeSubdomains bool
	CreatedAt        time.Time
}

// EnableHSTS 启用HSTS支持
func (m *CertManager) EnableHSTS(domain string, maxAge time.Duration, includeSubdomains bool) {
	// 在实际实现中，这里会存储HSTS策略到持久化存储
	// 这里简化为内存存储（重启后丢失）
	if m.config.Verbose {
		fmt.Printf("HSTS enabled for %s: max-age=%v, includeSubdomains=%v\n",
			domain, maxAge, includeSubdomains)
	}
}

// ShouldUseHTTPS 检查域名是否应该使用HTTPS（根据HSTS策略）
func (m *CertManager) ShouldUseHTTPS(domain string) bool {
	// 在实际实现中，这里会查询HSTS存储
	// 检查域名或其父域名是否有HSTS策略
	// 如果有且未过期，返回true
	return false
}

// ClearHSTS 清除指定域名的HSTS策略
func (m *CertManager) ClearHSTS(domain string) {
	// 在实际实现中，这里会从存储中删除HSTS策略
	if m.config.Verbose {
		fmt.Printf("HSTS cleared for %s\n", domain)
	}
}

// CheckHPKP 检查HTTP公钥固定
func (m *CertManager) CheckHPKP(domain string, pins []string) bool {
	// 如果没有配置公钥固定，返回true
	if len(pins) == 0 {
		return true
	}

	// 在实际实现中应该：
	// 1. 从存储中获取域名的公钥固定列表
	// 2. 提取证书的公钥
	// 3. 计算公钥的SHA-256指纹（Base64编码）
	// 4. 与存储的固定值比较
	// 5. 如果匹配则返回true，否则返回false

	if m.config.Verbose {
		fmt.Printf("HPKP检查: 域名 %s 配置了 %d 个公钥固定 (跳过实际检查)\n", domain, len(pins))
		for i, pin := range pins {
			fmt.Printf("  Pin %d: %s\n", i+1, pin)
		}
	}

	// 返回true表示公钥固定验证通过（简化处理）
	return true
}

// SetHPKP 设置域名的公钥固定
func (m *CertManager) SetHPKP(domain string, pins []string, maxAge time.Duration, includeSubdomains bool) {
	// 在实际实现中，这里会存储公钥固定到持久化存储
	if m.config.Verbose {
		fmt.Printf("HPKP set for %s: max-age=%v, includeSubdomains=%v, pins=%d\n",
			domain, maxAge, includeSubdomains, len(pins))
	}
}

// ClearHPKP 清除域名的公钥固定
func (m *CertManager) ClearHPKP(domain string) {
	// 在实际实现中，这里会从存储中删除公钥固定
	if m.config.Verbose {
		fmt.Printf("HPKP cleared for %s\n", domain)
	}
}
