package converter

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/example/wget2go/internal/core/types"
)

// Converter 链接转换器
type Converter struct {
	conversions map[string]*types.Conversion
	baseDir     string
	backup      bool
}

// NewConverter 创建链接转换器
func NewConverter(baseDir string, backup bool) *Converter {
	return &Converter{
		conversions: make(map[string]*types.Conversion),
		baseDir:     baseDir,
		backup:      backup,
	}
}

// AddConversion 添加待转换的文件
func (c *Converter) AddConversion(filename, baseURL string, result *types.ParsedResult) {
	c.conversions[filename] = &types.Conversion{
		Filename: filename,
		BaseURL:  baseURL,
		Result:   result,
	}
}

// ConvertAll 转换所有文件中的链接
func (c *Converter) ConvertAll() error {
	for filename, conversion := range c.conversions {
		if err := c.ConvertFile(filename, conversion); err != nil {
			return fmt.Errorf("转换文件 %s 失败: %w", filename, err)
		}
	}
	return nil
}

// ConvertFile 转换单个文件中的链接
func (c *Converter) ConvertFile(filename string, conversion *types.Conversion) error {
	// 读取文件内容
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 备份原文件
	if c.backup {
		backupFile := filename + ".orig"
		if err := os.WriteFile(backupFile, data, 0644); err != nil {
			return fmt.Errorf("备份文件失败: %w", err)
		}
	}

	// 转换链接
	converted := c.convertLinks(data, filename, conversion)

	// 写入转换后的内容
	if err := os.WriteFile(filename, converted, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	conversion.Converted = true
	return nil
}

// convertLinks 转换链接
func (c *Converter) convertLinks(data []byte, filename string, conversion *types.Conversion) []byte {
	var buf bytes.Buffer
	lastPos := 0

	for _, parsedURL := range conversion.Result.URLs {
		// 在原始数据中查找URL的位置
		pos := bytes.Index(data[lastPos:], []byte(parsedURL.URL))
		if pos == -1 {
			continue
		}

		// 写入URL之前的内容
		buf.Write(data[lastPos : lastPos+pos])

		// 计算相对路径
		relPath := c.getRelativePath(filename, parsedURL.URL)

		// 写入转换后的URL
		buf.WriteString(relPath)

		lastPos += pos + len(parsedURL.URL)
	}

	// 写入剩余内容
	buf.Write(data[lastPos:])

	return buf.Bytes()
}

// getRelativePath 计算相对路径
func (c *Converter) getRelativePath(fromFile, targetURL string) string {
	// 解析目标URL获取路径
	targetPath := c.getURLPath(targetURL)
	if targetPath == "" {
		return targetURL
	}

	// 获取源文件的目录
	fromDir := filepath.Dir(fromFile)

	// 计算相对路径
	relPath, err := filepath.Rel(fromDir, targetPath)
	if err != nil {
		return targetURL
	}

	// 使用正斜杠（Web标准）
	relPath = filepath.ToSlash(relPath)

	return relPath
}

// getURLPath 从URL获取本地文件路径
func (c *Converter) getURLPath(urlStr string) string {
	// 移除协议部分
	if idx := strings.Index(urlStr, "://"); idx != -1 {
		urlStr = urlStr[idx+3:]
	}

	// 移除主机名部分
	if idx := strings.Index(urlStr, "/"); idx != -1 {
		urlStr = urlStr[idx:]
	}

	// 移除查询参数和锚点
	if idx := strings.Index(urlStr, "?"); idx != -1 {
		urlStr = urlStr[:idx]
	}
	if idx := strings.Index(urlStr, "#"); idx != -1 {
		urlStr = urlStr[:idx]
	}

	// 拼接baseDir
	if c.baseDir != "" {
		return filepath.Join(c.baseDir, filepath.FromSlash(urlStr))
	}

	return filepath.FromSlash(urlStr)
}

// ConvertLinksWhole 转换完整链接（包括路径）
func (c *Converter) ConvertLinksWhole(filename string, conversion *types.Conversion) error {
	return c.ConvertFile(filename, conversion)
}

// ConvertLinksFileOnly 仅转换文件名部分
func (c *Converter) ConvertLinksFileOnly(filename string, conversion *types.Conversion) error {
	// 读取文件内容
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 备份原文件
	if c.backup {
		backupFile := filename + ".orig"
		if err := os.WriteFile(backupFile, data, 0644); err != nil {
			return fmt.Errorf("备份文件失败: %w", err)
		}
	}

	// 转换链接（仅文件名）
	converted := c.convertLinksFileOnly(data, conversion)

	// 写入转换后的内容
	if err := os.WriteFile(filename, converted, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	conversion.Converted = true
	return nil
}

// convertLinksFileOnly 仅转换文件名
func (c *Converter) convertLinksFileOnly(data []byte, conversion *types.Conversion) []byte {
	var buf bytes.Buffer
	lastPos := 0

	for _, parsedURL := range conversion.Result.URLs {
		// 在原始数据中查找URL的位置
		pos := bytes.Index(data[lastPos:], []byte(parsedURL.URL))
		if pos == -1 {
			continue
		}

		// 写入URL之前的内容
		buf.Write(data[lastPos : lastPos+pos])

		// 仅提取文件名
		filename := filepath.Base(c.getURLPath(parsedURL.URL))

		// 写入转换后的文件名
		buf.WriteString(filename)

		lastPos += pos + len(parsedURL.URL)
	}

	// 写入剩余内容
	buf.Write(data[lastPos:])

	return buf.Bytes()
}

// GetConversionCount 获取待转换文件数量
func (c *Converter) GetConversionCount() int {
	return len(c.conversions)
}

// Clear 清空转换列表
func (c *Converter) Clear() {
	c.conversions = make(map[string]*types.Conversion)
}

// HasConversion 检查是否有待转换的文件
func (c *Converter) HasConversion(filename string) bool {
	_, ok := c.conversions[filename]
	return ok
}

// GetConversion 获取转换信息
func (c *Converter) GetConversion(filename string) *types.Conversion {
	return c.conversions[filename]
}

// RemoveConversion 移除转换任务
func (c *Converter) RemoveConversion(filename string) {
	delete(c.conversions, filename)
}

// SetBaseDir 设置基础目录
func (c *Converter) SetBaseDir(dir string) {
	c.baseDir = dir
}

// GetBaseDir 获取基础目录
func (c *Converter) GetBaseDir() string {
	return c.baseDir
}

// SetBackup 设置是否备份原文件
func (c *Converter) SetBackup(backup bool) {
	c.backup = backup
}

// GetBackup 获取备份设置
func (c *Converter) GetBackup() bool {
	return c.backup
}

// AddConversionFromReader 从Reader添加转换任务
func (c *Converter) AddConversionFromReader(filename, baseURL string, reader io.Reader) error {
	_, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("读取数据失败: %w", err)
	}

	// 这里需要解析数据，暂时简化处理
	result := &types.ParsedResult{
		URLs:     make([]*types.ParsedURL, 0),
		Follow:   true,
		Encoding: "utf-8",
		Links:    make(map[string]string),
	}

	c.AddConversion(filename, baseURL, result)
	return nil
}

// GetUnconvertedFiles 获取未转换的文件列表
func (c *Converter) GetUnconvertedFiles() []string {
	var files []string
	for filename, conversion := range c.conversions {
		if !conversion.Converted {
			files = append(files, filename)
		}
	}
	return files
}

// GetConvertedFiles 获取已转换的文件列表
func (c *Converter) GetConvertedFiles() []string {
	var files []string
	for filename, conversion := range c.conversions {
		if conversion.Converted {
			files = append(files, filename)
		}
	}
	return files
}

// ConvertFileWithEncoding 使用指定编码转换文件
func (c *Converter) ConvertFileWithEncoding(filename string, conversion *types.Conversion, encoding string) error {
	// 简化处理，直接调用ConvertFile
	return c.ConvertFile(filename, conversion)
}

// RestoreBackup 恢复备份文件
func (c *Converter) RestoreBackup(filename string) error {
	backupFile := filename + ".orig"
	data, err := os.ReadFile(backupFile)
	if err != nil {
		return fmt.Errorf("读取备份文件失败: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("恢复文件失败: %w", err)
	}

	// 删除备份文件
	os.Remove(backupFile)

	return nil
}

// RestoreAllBackups 恢复所有备份文件
func (c *Converter) RestoreAllBackups() error {
	for filename := range c.conversions {
		if err := c.RestoreBackup(filename); err != nil {
			return err
		}
	}
	return nil
}

// CleanBackups 清理所有备份文件
func (c *Converter) CleanBackups() error {
	for filename := range c.conversions {
		backupFile := filename + ".orig"
		if err := os.Remove(backupFile); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}