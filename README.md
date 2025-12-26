# wget2go - Modern Multi-threaded Download Tool in Go

wget2go is a wget2 rewrite in Go, providing a modern multi-threaded download tool with support for HTTP/1.1, HTTP/2, HTTPS, and more.

## Features

- 🚀 **High-performance multi-threaded downloads**: True concurrent downloads using Go's goroutines
- 🔒 **Complete security support**: TLS 1.2/1.3, HSTS, certificate verification
- 📦 **Multiple protocol support**: HTTP/1.1, HTTP/2, HTTPS
- 🎯 **Intelligent chunked downloads**: Automatic file chunking for large files with parallel multi-threaded downloads
- 📄 **Format support**: Metalink, Cookie, compression formats (gzip, brotli, etc.)
- 🖥️ **Cross-platform**: Full support for Windows, Linux, macOS
- 📊 **Progress display**: Real-time download progress and speed display

## Installation

### Build from source
```bash
git clone https://github.com/chenyb888/wget2go.git
cd wget2go
go build -o wget2go ./cmd/wget2go
```

### Using go install
```bash
go install github.com/chenyb888/wget2go/cmd/wget2go@latest
```

### Download pre-built binaries

Pre-built binaries are available for the following platforms:

| Platform | Architecture | Binary |
|----------|-------------|--------|
| macOS | AMD64 | wget2go-darwin-amd64 |
| macOS | ARM64 | wget2go-darwin-arm64 |
| Linux | 386 | wget2go-linux-386 |
| Linux | AMD64 | wget2go-linux-amd64 |
| Linux | ARM | wget2go-linux-arm |
| Linux | ARM64 | wget2go-linux-arm64 |
| Windows | 386 | wget2go-windows-386.exe |
| Windows | AMD64 | wget2go-windows-amd64.exe |
| Windows | ARM64 | wget2go-windows-arm64.exe |

## Usage Examples

### Basic download
```bash
wget2go https://example.com/file.zip
```

### Multi-threaded download for large files
```bash
wget2go --max-threads 15 https://example.com/largefile.tar.gz
```

### Resume interrupted download
```bash
wget2go -c https://example.com/file.zip
```

### Recursive website download
```bash
wget2go --recursive --convert-links https://example.com/
```

### Using Metalink
```bash
wget2go --metalink https://example.com/file.meta4
```

### Set custom User-Agent
```bash
wget2go --user-agent "MyCustomAgent/1.0" https://example.com/file.zip
```

### Download with custom headers
```bash
wget2go --header "Authorization: Bearer token" https://example.com/file.zip
```

### Download through proxy
```bash
wget2go --http-proxy http://127.0.0.1:8080 https://example.com/file.zip
```

## Command-Line Options

### Version & Help
- `-V, --version` : Display version information
- `-h, --help` : Display help information

### Basic Options
- `-o, --output FILE` : Write documents to FILE
- `-O, --output-document FILE` : Write all content to FILE
- `-c, --continue` : Resume interrupted download
- `-q, --quiet` : Quiet mode (no output)
- `-v, --verbose` : Verbose output mode

### Download Options
- `--chunk-size=SIZE` : Chunk size (e.g., 1M, 10M)
- `--max-threads=N` : Maximum number of concurrent threads (default: 5)
- `--limit-rate=RATE` : Limit download speed (e.g., 100K, 1M)
- `--timeout=DURATION` : Timeout duration (default: 30s)

### HTTP Options
- `--user-agent=STRING` : Set User-Agent
- `--referer=URL` : Set Referer
- `-H, --header=HEADER` : Add HTTP header (can be used multiple times)
- `--cookie=COOKIE` : Set Cookie
- `--max-redirects=N` : Maximum number of redirects (default: 10)
- `--follow-redirects` : Follow redirects (default: true)
- `--insecure` : Allow insecure SSL connections

### Proxy Options
- `--http-proxy=URL` : Set HTTP proxy (format: http://host:port or http://user:pass@host:port)
- `--https-proxy=URL` : Set HTTPS proxy
- `--no-proxy=LIST` : List of hosts that don't need proxy (comma-separated)
- `--proxy` : Enable/disable proxy support (default: true)
- `--proxy-user=USERNAME` : Proxy authentication username
- `--proxy-password=PASSWORD` : Proxy authentication password

### Recursive Download Options
- `-r, --recursive` : Recursive download
- `-l, --level=N` : Maximum recursion depth (default: 5)
- `-k, --convert-links` : Convert links for local browsing
- `-p, --page-requisites` : Download all files required by the page

### Other Options
- `--progress` : Show progress bar (default: true)
- `--metalink` : Use Metalink
- `--robots-txt` : Respect robots.txt (default: true)

## Project Structure

```
wget2go/
├── cmd/wget2go/          # Main program entry
├── internal/             # Internal packages (not exposed)
│   ├── core/             # Core library
│   ├── downloader/       # Download manager
│   ├── config/           # Configuration management
│   └── cli/              # Command-line interface
├── pkg/                  # Reusable packages
│   ├── metalink/         # Metalink support
│   └── progress/         # Progress display
├── test/                 # Test files
└── docs/                 # Documentation
```

## Development

### Run tests
```bash
go test ./...
```

### Format code
```bash
go fmt ./...
```

### Build for all platforms
```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o bin/wget2go-linux-amd64 ./cmd/wget2go

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o bin/wget2go-linux-arm64 ./cmd/wget2go

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -o bin/wget2go-darwin-amd64 ./cmd/wget2go

# macOS ARM64
GOOS=darwin GOARCH=arm64 go build -o bin/wget2go-darwin-arm64 ./cmd/wget2go

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -o bin/wget2go-windows-amd64.exe ./cmd/wget2go
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Issues and Pull Requests are welcome!

## Acknowledgments

- Thanks to the GNU wget2 project for inspiration
- Thanks to all contributors to Go open-source projects