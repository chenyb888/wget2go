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
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("读取数据失败: %w", err)
	}

	// 根据文件扩展名确定解析器
	var result *types.ParsedResult

	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".html", ".htm":
		// 使用HTML解析器
		result = c.parseHTML(data, baseURL)
	case ".css":
		// 使用CSS解析器
		result = c.parseCSS(data, baseURL)
	case ".js":
		// JavaScript文件可能包含URL，但解析较复杂
		// 这里简化处理，只提取简单的字符串URL
		result = c.parseJS(data, baseURL)
	default:
		// 未知类型，返回空结果
		result = &types.ParsedResult{
			URLs:     make([]*types.ParsedURL, 0),
			Follow:   true,
			Encoding: "utf-8",
			Links:    make(map[string]string),
		}
	}

	c.AddConversion(filename, baseURL, result)
	return nil
}

// parseHTML 解析HTML文件提取URL
func (c *Converter) parseHTML(data []byte, baseURL string) *types.ParsedResult {
	result := &types.ParsedResult{
		URLs:     make([]*types.ParsedURL, 0),
		Follow:   true,
		Encoding: "utf-8",
		Links:    make(map[string]string),
	}

	// 简化的HTML解析，查找常见属性
	// 在实际应用中应该使用完整的HTML解析器如golang.org/x/net/html
	attrs := []string{
		`href="`, `src="`, `srcset="`, `background="`,
		`action="`, `cite="`, `data="`, `poster="`,
	}

	for _, attr := range attrs {
		pos := 0
		for {
			idx := bytes.Index(data[pos:], []byte(attr))
			if idx == -1 {
				break
			}

			start := pos + idx + len(attr)
			// 查找结束引号
			end := bytes.IndexByte(data[start:], '"')
			if end == -1 {
				break
			}

			urlStr := string(data[start : start+end])
			// 过滤空URL和锚点
			if urlStr != "" && !strings.HasPrefix(urlStr, "#") && !strings.HasPrefix(urlStr, "javascript:") {
				result.URLs = append(result.URLs, &types.ParsedURL{
					URL:  urlStr,
					Attr: strings.TrimSuffix(attr, `"`),
					Tag:  "",
				})
				result.Links[urlStr] = urlStr
			}

			pos = start + end + 1
		}
	}

	return result
}

// parseCSS 解析CSS文件提取URL
func (c *Converter) parseCSS(data []byte, baseURL string) *types.ParsedResult {
	result := &types.ParsedResult{
		URLs:     make([]*types.ParsedURL, 0),
		Follow:   true,
		Encoding: "utf-8",
		Links:    make(map[string]string),
	}

	// 查找url()语法
	pos := 0
	for {
		idx := bytes.Index(data[pos:], []byte("url("))
		if idx == -1 {
			break
		}

		start := pos + idx + 4
		// 跳过空白字符
		for start < len(data) && (data[start] == ' ' || data[start] == '\t' || data[start] == '\n' || data[start] == '\r') {
			start++
		}

		if start >= len(data) {
			break
		}

		// 确定引号类型
		var quote byte
		if data[start] == '"' || data[start] == '\'' {
			quote = data[start]
			start++
		}

		// 查找结束位置
		var end int
		if quote != 0 {
			end = bytes.IndexByte(data[start:], quote)
			if end == -1 {
				break
			}
			end += start
		} else {
			end = bytes.IndexAny(data[start:], " )")
			if end == -1 {
				break
			}
			end += start
		}

		urlStr := string(data[start:end])
		// 过滤data: URL
		if !strings.HasPrefix(urlStr, "data:") {
			result.URLs = append(result.URLs, &types.ParsedURL{
				URL:  urlStr,
				Attr: "url",
				Tag:  "",
			})
			result.Links[urlStr] = urlStr
		}

		pos = end + 1
	}

	return result
}

// parseJS 解析JavaScript文件提取URL（简化版）
func (c *Converter) parseJS(data []byte, baseURL string) *types.ParsedResult {
	result := &types.ParsedResult{
		URLs:     make([]*types.ParsedURL, 0),
		Follow:   true,
		Encoding: "utf-8",
		Links:    make(map[string]string),
	}

	// 查找常见的URL模式（简化处理）
	patterns := [][]byte{
		[]byte(`"`), []byte(`'`),
	}

	for _, quote := range patterns {
		pos := 0
		for {
			idx := bytes.Index(data[pos:], quote)
			if idx == -1 {
				break
			}

			start := pos + idx + 1
			end := bytes.Index(data[start:], quote)
			if end == -1 {
				break
			}
			end += start

			urlStr := string(data[start:end])
			// 检查是否是URL
			if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") ||
				strings.HasPrefix(urlStr, "/") || strings.HasPrefix(urlStr, "./") {
				result.URLs = append(result.URLs, &types.ParsedURL{
					URL:  urlStr,
					Attr: "string",
					Tag:  "",
				})
				result.Links[urlStr] = urlStr
			}

			pos = end + 1
		}
	}

	return result
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
	// 读取文件内容
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 如果指定了编码且不是UTF-8，进行编码转换
	var convertedData []byte
	if encoding != "" && !strings.EqualFold(encoding, "utf-8") {
		convertedData, err = c.convertEncoding(data, encoding, "utf-8")
		if err != nil {
			return fmt.Errorf("编码转换失败: %w", err)
		}
	} else {
		convertedData = data
	}

	// 备份原文件
	if c.backup {
		backupFile := filename + ".orig"
		if err := os.WriteFile(backupFile, data, 0644); err != nil {
			return fmt.Errorf("备份文件失败: %w", err)
		}
	}

	// 转换链接
	linksConverted := c.convertLinks(convertedData, filename, conversion)

	// 如果进行了编码转换，需要转换回原始编码
	var finalData []byte
	if encoding != "" && !strings.EqualFold(encoding, "utf-8") {
		finalData, err = c.convertEncoding(linksConverted, "utf-8", encoding)
		if err != nil {
			return fmt.Errorf("编码转换失败: %w", err)
		}
	} else {
		finalData = linksConverted
	}

	// 写入转换后的内容
	if err := os.WriteFile(filename, finalData, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	conversion.Converted = true
	return nil
}

// convertEncoding 转换文本编码
func (c *Converter) convertEncoding(data []byte, fromEncoding, toEncoding string) ([]byte, error) {
	// 在实际应用中，应该使用golang.org/x/text/encoding包
	// 这里简化处理，假设输入是UTF-8兼容的
	// 如果需要完整的编码支持，需要：
	// 1. 根据fromEncoding创建解码器
	// 2. 根据toEncoding创建编码器
	// 3. 进行编码转换

	// 常见编码映射
	encodingMap := map[string]string{
		"utf-8":    "utf-8",
		"utf8":     "utf-8",
		"iso-8859-1": "latin1",
		"latin1":   "latin1",
		"gbk":      "gbk",
		"gb2312":   "gbk",
		"gb18030":  "gb18030",
		"big5":     "big5",
		"shift_jis": "shiftjis",
		"euc-jp":   "eucjp",
		"euc-kr":   "euckr",
	}

	// 标准化编码名称
	from := encodingMap[strings.ToLower(fromEncoding)]
	if from == "" {
		from = strings.ToLower(fromEncoding)
	}
	to := encodingMap[strings.ToLower(toEncoding)]
	if to == "" {
		to = strings.ToLower(toEncoding)
	}

	// 如果源编码和目标编码相同，直接返回
	if strings.EqualFold(from, to) {
		return data, nil
	}

	// 如果是UTF-8到UTF-8，直接返回
	if strings.Contains(from, "utf") && strings.Contains(to, "utf") {
		return data, nil
	}

	// 这里只处理UTF-8兼容的情况
	// 实际应用中需要完整的编码转换支持
	return data, nil
}

// DetectEncoding 检测文件编码
func (c *Converter) DetectEncoding(data []byte) string {
	// 简化的编码检测
	// 在实际应用中应该使用专门的编码检测库

	// 检查BOM标记
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return "utf-8"
	}

	// 检查是否是有效的UTF-8
	if c.isValidUTF8(data) {
		return "utf-8"
	}

	// 默认返回UTF-8
	return "utf-8"
}

// isValidUTF8 检查数据是否是有效的UTF-8
func (c *Converter) isValidUTF8(data []byte) bool {
	// 简化的UTF-8验证
	for i := 0; i < len(data); {
		b := data[i]
		if b < 0x80 {
			// ASCII字符
			i++
		} else if b < 0xC0 {
			// 无效的UTF-8起始字节
			return false
		} else if b < 0xE0 {
			// 2字节序列
			if i+1 >= len(data) {
				return false
			}
			if data[i+1]&0xC0 != 0x80 {
				return false
			}
			i += 2
		} else if b < 0xF0 {
			// 3字节序列
			if i+2 >= len(data) {
				return false
			}
			if data[i+1]&0xC0 != 0x80 || data[i+2]&0xC0 != 0x80 {
				return false
			}
			i += 3
		} else if b < 0xF8 {
			// 4字节序列
			if i+3 >= len(data) {
				return false
			}
			if data[i+1]&0xC0 != 0x80 || data[i+2]&0xC0 != 0x80 || data[i+3]&0xC0 != 0x80 {
				return false
			}
			i += 4
		} else {
			// 无效的UTF-8起始字节
			return false
		}
	}
	return true
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