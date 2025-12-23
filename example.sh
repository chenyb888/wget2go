#!/bin/bash

echo "=== wget2go 示例演示 ==="
echo ""

# 显示版本信息
echo "1. 显示版本信息:"
./wget2go --version
echo ""

# 显示帮助信息
echo "2. 显示帮助信息:"
./wget2go --help | head -20
echo ""

# 测试基本下载（使用本地测试服务器）
echo "3. 测试基本命令行解析:"
echo "模拟下载命令: ./wget2go --chunk-size=1M --max-threads=5 https://example.com/test.txt"
echo ""

# 创建测试配置文件
echo "4. 创建测试配置文件:"
cat > test_config.yaml << 'EOF'
# 测试配置文件
chunk_size: "2M"
max_threads: 8
timeout: "60s"
user_agent: "wget2go-test/1.0"
progress: true
EOF
echo "配置文件已创建: test_config.yaml"
echo ""

# 显示项目结构
echo "5. 项目结构:"
find . -type f -name "*.go" | head -10 | sed 's/^/  /'
echo "..."

# 运行测试
echo "6. 运行单元测试:"
go test ./test -v
echo ""

# 构建其他平台
echo "7. 交叉编译示例:"
echo "Linux:   GOOS=linux GOARCH=amd64 go build -o wget2go-linux ./cmd/wget2go"
echo "Windows: GOOS=windows GOARCH=amd64 go build -o wget2go.exe ./cmd/wget2go"
echo "macOS:   GOOS=darwin GOARCH=arm64 go build -o wget2go-macos ./cmd/wget2go"
echo ""

echo "=== 演示完成 ==="
echo ""
echo "使用示例:"
echo "  ./wget2go --chunk-size=10M --max-threads=8 https://example.com/large-file.zip"
echo "  ./wget2go -r -l 2 https://example.com/"
echo "  ./wget2go --limit-rate=1M https://example.com/file.zip"