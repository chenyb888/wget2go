package cli

import (
	"fmt"
	"strings"

	"github.com/example/wget2go/internal/config"
	"github.com/example/wget2go/internal/core/types"
	"github.com/spf13/cobra"
)

// CLI 命令行界面
type CLI struct {
	rootCmd    *cobra.Command
	configMgr  *config.ConfigManager
	config     *types.Config
	urls       []string
}

// NewCLI 创建命令行界面
func NewCLI() *CLI {
	cli := &CLI{
		configMgr: config.NewConfigManager(),
	}

	cli.rootCmd = &cobra.Command{
		Use:   "wget2go [URL...]",
		Short: "wget2go - Go语言实现的多线程下载工具",
		Long: `wget2go是一个用Go语言重写的wget2，提供了现代化的多线程下载工具。
支持HTTP/1.1、HTTP/2、HTTPS等协议，具有高性能和完整的安全支持。`,
		Args: cobra.MinimumNArgs(0),
		RunE: cli.run,
	}

	cli.setupFlags()
	return cli
}

// setupFlags 设置命令行标志
func (cli *CLI) setupFlags() {
	cmd := cli.rootCmd

	// 版本标志
	cmd.Flags().BoolP("version", "V", false, "显示版本信息")
	
	// 基本选项
	cmd.Flags().StringP("output", "o", "", "写入文档到FILE")
	cmd.Flags().StringP("output-document", "O", "", "将所有内容写入FILE")
	cmd.Flags().BoolP("continue", "c", false, "断点续传")
	cmd.Flags().BoolP("quiet", "q", false, "安静模式（不输出信息）")
	cmd.Flags().BoolP("verbose", "v", false, "详细输出模式")

	// 下载选项
	cmd.Flags().String("chunk-size", "1M", "分片大小（如1M、10M）")
	cmd.Flags().Int("max-threads", 5, "最大并发线程数")
	cmd.Flags().String("limit-rate", "0", "限制下载速度（如100K、1M）")
	cmd.Flags().String("timeout", "30s", "超时时间")

	// HTTP选项
	cmd.Flags().String("user-agent", "", "设置User-Agent")
	cmd.Flags().String("referer", "", "设置Referer")
	cmd.Flags().StringArrayP("header", "H", []string{}, "添加HTTP头")
	cmd.Flags().String("cookie", "", "设置Cookie")
	cmd.Flags().Int("max-redirects", 10, "最大重定向次数")
	cmd.Flags().Bool("follow-redirects", true, "跟随重定向")
	cmd.Flags().Bool("insecure", false, "允许不安全的SSL连接")

	// 递归下载选项
	cmd.Flags().BoolP("recursive", "r", false, "递归下载")
	cmd.Flags().IntP("level", "l", 5, "最大递归深度")
	cmd.Flags().BoolP("convert-links", "k", false, "转换链接用于本地浏览")
	cmd.Flags().BoolP("page-requisites", "p", false, "下载页面所需的所有文件")

	// 其他选项
	cmd.Flags().Bool("progress", true, "显示进度条")
	cmd.Flags().Bool("metalink", false, "使用Metalink")
	cmd.Flags().Bool("robots-txt", true, "尊重robots.txt")

	// 隐藏的帮助标志
	cmd.Flags().BoolP("help", "h", false, "显示帮助信息")
}

// Execute 执行命令行
func (cli *CLI) Execute() error {
	return cli.rootCmd.Execute()
}

// run 运行命令
func (cli *CLI) run(cmd *cobra.Command, args []string) error {
	// 检查版本标志
	if version, _ := cmd.Flags().GetBool("version"); version {
		cli.ShowVersion()
		return nil
	}

	// 解析配置
	if err := cli.parseConfig(cmd); err != nil {
		return err
	}

	// 获取URL参数
	cli.urls = args

	// 如果没有URL，显示帮助
	if len(cli.urls) == 0 {
		cmd.Help()
		return nil
	}

	// 验证URL
	if err := cli.validateURLs(); err != nil {
		return err
	}

	// 显示配置信息
	if cli.config.Verbose {
		cli.showConfig()
	}

	// 开始下载
	return cli.startDownload()
}

// parseConfig 解析配置
func (cli *CLI) parseConfig(cmd *cobra.Command) error {
	// 绑定命令行标志到viper
	if err := cli.bindFlags(cmd); err != nil {
		return err
	}

	// 解析配置
	config, err := cli.configMgr.Parse()
	if err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	cli.config = config
	return nil
}

// bindFlags 绑定命令行标志
func (cli *CLI) bindFlags(cmd *cobra.Command) error {
	// 获取所有标志
	flags := cmd.Flags()

	// 绑定每个标志到viper
	flagNames := []string{
		"version", "output", "output-document", "continue", "quiet", "verbose",
		"chunk-size", "max-threads", "limit-rate", "timeout",
		"user-agent", "referer", "header", "cookie", "max-redirects",
		"follow-redirects", "insecure",
		"recursive", "level", "convert-links", "page-requisites",
		"progress", "metalink", "robots-txt",
	}

	for _, name := range flagNames {
		if flag := flags.Lookup(name); flag != nil {
			if err := cli.configMgr.GetViper().BindPFlag(name, flag); err != nil {
				return fmt.Errorf("绑定标志 %s 失败: %w", name, err)
			}
		}
	}

	return nil
}

// validateURLs 验证URL
func (cli *CLI) validateURLs() error {
	for _, url := range cli.urls {
		if !isValidURL(url) {
			return fmt.Errorf("无效的URL: %s", url)
		}
	}
	return nil
}

// isValidURL 检查URL是否有效
func isValidURL(urlStr string) bool {
	// 简单验证，实际应该使用更严格的验证
	return strings.HasPrefix(urlStr, "http://") ||
		strings.HasPrefix(urlStr, "https://") ||
		strings.HasPrefix(urlStr, "ftp://")
}

// showConfig 显示配置信息
func (cli *CLI) showConfig() {
	fmt.Println("=== 配置信息 ===")
	fmt.Printf("输出文件: %s\n", cli.config.OutputFile)
	fmt.Printf("分片大小: %d bytes\n", cli.config.ChunkSize)
	fmt.Printf("最大线程数: %d\n", cli.config.MaxThreads)
	fmt.Printf("超时时间: %v\n", cli.config.Timeout)
	fmt.Printf("User-Agent: %s\n", cli.config.UserAgent)
	fmt.Printf("递归下载: %v\n", cli.config.Recursive)
	fmt.Printf("递归深度: %d\n", cli.config.RecursiveLevel)
	fmt.Printf("跟随重定向: %v\n", cli.config.FollowRedirects)
	fmt.Printf("显示进度: %v\n", cli.config.Progress)
	fmt.Println("================")
}

// startDownload 开始下载
func (cli *CLI) startDownload() error {
	fmt.Printf("开始下载 %d 个文件...\n", len(cli.urls))

	for i, url := range cli.urls {
		fmt.Printf("\n[%d/%d] 下载: %s\n", i+1, len(cli.urls), url)
		
		// 这里应该调用下载器
		// 简化版本，只显示信息
		if err := cli.downloadFile(url); err != nil {
			fmt.Printf("下载失败: %v\n", err)
			return err
		}
		
		fmt.Printf("下载完成: %s\n", url)
	}

	fmt.Println("\n所有下载完成!")
	return nil
}

// downloadFile 下载单个文件（简化版）
func (cli *CLI) downloadFile(url string) error {
	// 在实际实现中，这里会调用下载器
	// 简化版本只模拟下载
	
	if cli.config.Progress {
		// 模拟进度显示
		for i := 0; i <= 100; i += 10 {
			fmt.Printf("\r进度: %d%%", i)
			// 模拟下载延迟
			// time.Sleep(100 * time.Millisecond)
		}
		fmt.Println()
	}
	
	return nil
}

// GetConfig 获取配置
func (cli *CLI) GetConfig() *types.Config {
	return cli.config
}

// GetURLs 获取URL列表
func (cli *CLI) GetURLs() []string {
	return cli.urls
}

// ShowHelp 显示帮助信息
func (cli *CLI) ShowHelp() {
	cli.rootCmd.Help()
}

// ShowVersion 显示版本信息
func (cli *CLI) ShowVersion() {
	fmt.Println("wget2go v1.0.0")
	fmt.Println("Go语言实现的多线程下载工具")
	fmt.Println("Copyright (c) 2025 wget2go Contributors")
}
