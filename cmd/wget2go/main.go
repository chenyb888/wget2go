package main

import (
	"fmt"
	"os"

	"github.com/example/wget2go/internal/cli"
)

func main() {
	// 创建CLI实例
	app := cli.NewCLI()

	// 执行命令
	if err := app.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}