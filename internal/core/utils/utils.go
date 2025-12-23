package utils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// FileExists 检查文件是否存在
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// IsDir 检查是否为目录
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// EnsureDir 确保目录存在
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// GetFileSize 获取文件大小
func GetFileSize(filename string) (int64, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// ParseSize 解析大小字符串（如1M、10K、2G）
func ParseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, nil
	}

	sizeStr = strings.ToUpper(sizeStr)
	
	// 匹配数字和单位
	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([KMGTP]?B?)?$`)
	matches := re.FindStringSubmatch(sizeStr)
	
	if len(matches) < 2 {
		return 0, fmt.Errorf("无效的大小格式: %s", sizeStr)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("解析数值失败: %w", err)
	}

	var multiplier float64 = 1
	if len(matches) > 2 && matches[2] != "" {
		unit := matches[2]
		switch {
		case strings.HasPrefix(unit, "K"):
			multiplier = 1024
		case strings.HasPrefix(unit, "M"):
			multiplier = 1024 * 1024
		case strings.HasPrefix(unit, "G"):
			multiplier = 1024 * 1024 * 1024
		case strings.HasPrefix(unit, "T"):
			multiplier = 1024 * 1024 * 1024 * 1024
		case strings.HasPrefix(unit, "P"):
			multiplier = 1024 * 1024 * 1024 * 1024 * 1024
		}
	}

	return int64(value * multiplier), nil
}

// FormatSize 格式化大小为可读字符串
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	
	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// FormatSpeed 格式化速度
func FormatSpeed(bytesPerSecond int64) string {
	return FormatSize(bytesPerSecond) + "/s"
}

// FormatDuration 格式化持续时间
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, minutes)
}

// CalculateETA 计算预计完成时间
func CalculateETA(total, downloaded, speed int64) time.Duration {
	if speed <= 0 {
		return 0
	}
	
	remaining := total - downloaded
	seconds := float64(remaining) / float64(speed)
	return time.Duration(seconds * float64(time.Second))
}

// CalculateMD5 计算MD5哈希
func CalculateMD5(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CalculateSHA1 计算SHA1哈希
func CalculateSHA1(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CalculateSHA256 计算SHA256哈希
func CalculateSHA256(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// SafeFileName 创建安全的文件名
func SafeFileName(filename string) string {
	// 移除非法字符
	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	filename = re.ReplaceAllString(filename, "_")
	
	// 限制长度
	if len(filename) > 255 {
		filename = filename[:255]
	}
	
	return filename
}

// GetUniqueFileName 获取唯一的文件名
func GetUniqueFileName(filename string) string {
	if !FileExists(filename) {
		return filename
	}

	ext := filepath.Ext(filename)
	base := filename[:len(filename)-len(ext)]
	
	for i := 1; ; i++ {
		newFilename := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if !FileExists(newFilename) {
			return newFilename
		}
	}
}

// CopyFile 复制文件
func CopyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// MoveFile 移动文件
func MoveFile(src, dst string) error {
	// 尝试直接重命名
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// 如果跨设备，先复制再删除
	if err := CopyFile(src, dst); err != nil {
		return err
	}
	
	return os.Remove(src)
}

// HumanReadableTime 人类可读的时间格式
func HumanReadableTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "刚刚"
	case diff < time.Hour:
		minutes := int(diff.Minutes())
		return fmt.Sprintf("%d分钟前", minutes)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("%d小时前", hours)
	case diff < 30*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d天前", days)
	default:
		return t.Format("2006-01-02")
	}
}