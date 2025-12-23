# wget2go - Go语言实现的wget2

wget2go是一个用Go语言重写的wget2，提供了现代化的多线程下载工具，支持HTTP/1.1、HTTP/2、HTTPS等协议。

## 特性

- 🚀 **高性能多线程下载**：利用Go的goroutine实现真正的并发下载
- 🔒 **完整的安全支持**：TLS 1.2/1.3、HSTS、证书验证
- 📦 **多种协议支持**：HTTP/1.1、HTTP/2、HTTPS
- 🎯 **智能分片下载**：大文件自动分片，多线程并行下载
- 📄 **格式支持**：Metalink、Cookie、压缩格式（gzip、brotli等）
- 🖥️ **跨平台**：Windows、Linux、macOS全平台支持
- 📊 **进度显示**：实时下载进度和速度显示

## 安装

### 从源码编译
```bash
git clone https://github.com/chenyb888/wget2go.git
cd wget2go
go build -o wget2go ./cmd/wget2go
```

### 使用go install
```bash
go install github.com/chenyb888/wget2go/cmd/wget2go@latest
```

## 使用示例

### 基本下载
```bash
wget2go https://example.com/file.zip
```

### 多线程下载大文件
```bash
wget2go --chunk-size=10M --max-threads=8 https://example.com/large-file.iso
```

### 递归下载网站
```bash
wget2go --recursive --convert-links https://example.com/
```

### 使用Metalink
```bash
wget2go --metalink https://example.com/file.meta4
```

## 命令行选项

### 基本选项
- `-o, --output FILE`：指定输出文件名
- `-O, --output-document FILE`：将所有内容写入单个文件
- `-c, --continue`：断点续传
- `-q, --quiet`：安静模式，不输出信息
- `-v, --verbose`：详细输出模式

### 下载选项
- `--chunk-size=SIZE`：分片大小（如1M、10M）
- `--max-threads=N`：最大并发线程数（默认5）
- `--limit-rate=RATE`：限制下载速度（如100K、1M）
- `--timeout=SECONDS`：超时时间（默认30秒）

### HTTP选项
- `--user-agent=STRING`：设置User-Agent
- `--header=HEADER`：添加HTTP头
- `--cookie=COOKIE`：设置Cookie
- `--referer=URL`：设置Referer

### 递归下载选项
- `-r, --recursive`：递归下载
- `-l, --level=N`：最大递归深度
- `-k, --convert-links`：转换链接用于本地浏览
- `-p, --page-requisites`：下载页面所需的所有文件

## 项目结构

```
wget2go/
├── cmd/wget2go/          # 主程序入口
├── internal/             # 内部包（不对外暴露）
│   ├── core/             # 核心库
│   ├── downloader/       # 下载管理器
│   ├── config/           # 配置管理
│   └── cli/              # 命令行界面
├── pkg/                  # 可复用包
│   ├── metalink/         # Metalink支持
│   └── progress/         # 进度显示
├── test/                 # 测试文件
└── docs/                 # 文档
```

## 开发

### 运行测试
```bash
go test ./...
```

### 代码格式化
```bash
go fmt ./...
```

### 构建所有平台
```bash
./scripts/build-all.sh
```

## 许可证

本项目采用MIT许可证。详见[LICENSE](LICENSE)文件。

## 贡献

欢迎提交Issue和Pull Request！

## 致谢

- 感谢GNU wget2项目的启发
- 感谢所有Go语言开源项目的贡献者
