package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/example/wget2go/internal/core/types"
	"github.com/example/wget2go/internal/core/utils"
	"github.com/spf13/viper"
)

// ConfigManager 配置管理器
type ConfigManager struct {
	config *types.Config
	viper  *viper.Viper
}

// NewConfigManager 创建配置管理器
func NewConfigManager() *ConfigManager {
	v := viper.New()
	
	// 设置默认值
	setDefaults(v)
	
	// 读取配置文件
	loadConfigFile(v)
	
	// 绑定环境变量
	bindEnvVars(v)
	
	return &ConfigManager{
		config: &types.Config{},
		viper:  v,
	}
}

// GetViper 获取viper实例（用于CLI绑定）
func (cm *ConfigManager) GetViper() *viper.Viper {
	return cm.viper
}

// setDefaults 设置默认值
func setDefaults(v *viper.Viper) {
	v.SetDefault("output_file", "")
	v.SetDefault("output_document", "")
	v.SetDefault("continue", false)
	v.SetDefault("chunk_size", "1M")
	v.SetDefault("max_threads", 5)
	v.SetDefault("limit_rate", "0")
	v.SetDefault("timeout", "30s")
	v.SetDefault("user_agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	v.SetDefault("referer", "")
	v.SetDefault("recursive", false)
	v.SetDefault("recursive_level", 5)
	v.SetDefault("convert_links", false)
	v.SetDefault("page_requisites", false)
	v.SetDefault("max_redirects", 10)
	v.SetDefault("follow_redirects", true)
	v.SetDefault("insecure", false)
	v.SetDefault("quiet", false)
	v.SetDefault("verbose", false)
	v.SetDefault("progress", true)
	v.SetDefault("metalink", false)
	v.SetDefault("robots_txt", true)
}

// loadConfigFile 加载配置文件
func loadConfigFile(v *viper.Viper) {
	// 设置配置文件搜索路径
	home, err := os.UserHomeDir()
	if err == nil {
		v.AddConfigPath(filepath.Join(home, ".config", "wget2go"))
		v.AddConfigPath(home)
	}
	v.AddConfigPath(".")
	
	// 设置配置文件名
	v.SetConfigName(".wget2go")
	v.SetConfigType("yaml")
	
	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		// 配置文件不存在是正常的
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Printf("警告: 读取配置文件失败: %v\n", err)
		}
	}
}

// bindEnvVars 绑定环境变量
func bindEnvVars(v *viper.Viper) {
	v.BindEnv("output_file", "WGET2GO_OUTPUT")
	v.BindEnv("user_agent", "WGET2GO_USER_AGENT")
	v.BindEnv("timeout", "WGET2GO_TIMEOUT")
	v.BindEnv("max_threads", "WGET2GO_MAX_THREADS")
	v.BindEnv("limit_rate", "WGET2GO_LIMIT_RATE")
}

// Parse 解析配置
func (cm *ConfigManager) Parse() (*types.Config, error) {
	// 解析chunk size
	chunkSizeStr := cm.viper.GetString("chunk_size")
	chunkSize, err := parseSize(chunkSizeStr)
	if err != nil {
		return nil, fmt.Errorf("解析chunk_size失败: %w", err)
	}

	// 解析限速
	limitRateStr := cm.viper.GetString("limit_rate")
	limitRate, err := parseSize(limitRateStr)
	if err != nil {
		return nil, fmt.Errorf("解析limit_rate失败: %w", err)
	}

	// 解析超时
	timeoutStr := cm.viper.GetString("timeout")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("解析timeout失败: %w", err)
	}

	// 构建配置
	cm.config = &types.Config{
		OutputFile:      cm.viper.GetString("output_file"),
		OutputDocument:  cm.viper.GetString("output_document"),
		Continue:        cm.viper.GetBool("continue"),
		ChunkSize:       chunkSize,
		MaxThreads:      cm.viper.GetInt("max_threads"),
		LimitRate:       limitRate,
		Timeout:         timeout,
		UserAgent:       cm.viper.GetString("user_agent"),
		Referer:         cm.viper.GetString("referer"),
		Headers:         parseHeaders(cm.viper.GetStringSlice("header")),
		Cookies:         parseCookies(cm.viper.GetString("cookie")),
		Recursive:       cm.viper.GetBool("recursive"),
		RecursiveLevel:  cm.viper.GetInt("recursive_level"),
		ConvertLinks:    cm.viper.GetBool("convert_links"),
		PageRequisites:  cm.viper.GetBool("page_requisites"),
		MaxRedirects:    cm.viper.GetInt("max_redirects"),
		FollowRedirects: cm.viper.GetBool("follow_redirects"),
		Insecure:        cm.viper.GetBool("insecure"),
		Quiet:           cm.viper.GetBool("quiet"),
		Verbose:         cm.viper.GetBool("verbose"),
		Progress:        cm.viper.GetBool("progress"),
		Metalink:        cm.viper.GetBool("metalink"),
		RobotsTxt:       cm.viper.GetBool("robots_txt"),
	}

	return cm.config, nil
}

// parseSize 解析大小字符串
func parseSize(sizeStr string) (int64, error) {
	return utils.ParseSize(sizeStr)
}

// parseHeaders 解析HTTP头部
func parseHeaders(headerStrs []string) map[string]string {
	headers := make(map[string]string)
	
	for _, headerStr := range headerStrs {
		parts := splitHeader(headerStr)
		if len(parts) == 2 {
			headers[parts[0]] = parts[1]
		}
	}
	
	return headers
}

// parseCookies 解析Cookie
func parseCookies(cookieStr string) map[string]string {
	cookies := make(map[string]string)
	
	if cookieStr == "" {
		return cookies
	}
	
	// 解析格式如 "name1=value1; name2=value2"
	cookieParts := splitCookies(cookieStr)
	for _, cookiePart := range cookieParts {
		parts := splitCookie(cookiePart)
		if len(parts) == 2 {
			cookies[parts[0]] = parts[1]
		}
	}
	
	return cookies
}

// splitHeader 分割头部字符串
func splitHeader(headerStr string) []string {
	// 格式: "Header: Value"
	idx := indexOf(headerStr, ':')
	if idx == -1 {
		return nil
	}
	
	key := trim(headerStr[:idx])
	value := trim(headerStr[idx+1:])
	
	return []string{key, value}
}

// splitCookies 分割Cookie字符串
func splitCookies(cookieStr string) []string {
	// 按分号分割
	return splitBy(cookieStr, ';')
}

// splitCookie 分割单个Cookie
func splitCookie(cookieStr string) []string {
	// 格式: "name=value"
	idx := indexOf(cookieStr, '=')
	if idx == -1 {
		return nil
	}
	
	key := trim(cookieStr[:idx])
	value := trim(cookieStr[idx+1:])
	
	return []string{key, value}
}

// splitBy 按分隔符分割字符串
func splitBy(s string, sep byte) []string {
	var parts []string
	var start int
	
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			part := trim(s[start:i])
			if part != "" {
				parts = append(parts, part)
			}
			start = i + 1
		}
	}
	
	// 添加最后一部分
	if start < len(s) {
		part := trim(s[start:])
		if part != "" {
			parts = append(parts, part)
		}
	}
	
	return parts
}

// indexOf 查找字符位置
func indexOf(s string, ch byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ch {
			return i
		}
	}
	return -1
}

// trim 去除空白字符
func trim(s string) string {
	// 去除前后空白
	start := 0
	end := len(s)
	
	for start < end && isSpace(s[start]) {
		start++
	}
	
	for end > start && isSpace(s[end-1]) {
		end--
	}
	
	return s[start:end]
}

// isSpace 判断是否为空白字符
func isSpace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

// SaveConfig 保存配置到文件
func (cm *ConfigManager) SaveConfig() error {
	configPath := getConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	
	return cm.viper.WriteConfigAs(configPath)
}

// getConfigPath 获取配置文件路径
func getConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".wget2go.yaml"
	}
	
	return filepath.Join(home, ".config", "wget2go", "config.yaml")
}

// GetConfig 获取当前配置
func (cm *ConfigManager) GetConfig() *types.Config {
	return cm.config
}

// UpdateConfig 更新配置
func (cm *ConfigManager) UpdateConfig(updates map[string]interface{}) {
	for key, value := range updates {
		cm.viper.Set(key, value)
	}
	
	// 重新解析配置
	cm.Parse()
}