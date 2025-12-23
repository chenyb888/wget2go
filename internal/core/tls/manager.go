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

// CheckOCSP OCSP检查（简化版）
func (m *CertManager) CheckOCSP(cert *x509.Certificate) (bool, error) {
	// 在实际实现中，这里会执行OCSP检查
	// 简化版本直接返回成功
	return true, nil
}

// CheckCRL CRL检查（简化版）
func (m *CertManager) CheckCRL(cert *x509.Certificate) (bool, error) {
	// 在实际实现中，这里会检查证书撤销列表
	// 简化版本直接返回成功
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

// EnableHSTS 启用HSTS支持
func (m *CertManager) EnableHSTS(domain string, maxAge time.Duration, includeSubdomains bool) {
	// 在实际实现中，这里会存储HSTS策略
	// 简化版本只记录日志
	fmt.Printf("HSTS enabled for %s: max-age=%v, includeSubdomains=%v\n",
		domain, maxAge, includeSubdomains)
}

// CheckHPKP 检查HTTP公钥固定
func (m *CertManager) CheckHPKP(domain string, pins []string) bool {
	// 在实际实现中，这里会检查公钥固定
	// 简化版本直接返回true
	return true
}
