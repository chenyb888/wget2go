# wget2go 使用示例

## 基本用法

### 下载单个文件
```bash
wget2go https://example.com/file.zip
```

### 指定输出文件名
```bash
wget2go -o myfile.zip https://example.com/file.zip
```

### 断点续传
```bash
wget2go -c https://example.com/large-file.iso
```

## 多线程下载

### 启用分片下载（默认1M分片）
```bash
wget2go --chunk-size=1M https://example.com/large-file.iso
```

### 指定分片大小和线程数
```bash
wget2go --chunk-size=10M --max-threads=8 https://example.com/large-file.iso
```

### 限速下载
```bash
wget2go --limit-rate=1M https://example.com/file.zip
```

## HTTP选项

### 自定义User-Agent
```bash
wget2go --user-agent="MyCustomAgent/1.0" https://example.com/
```

### 添加HTTP头
```bash
wget2go -H "Accept: application/json" -H "Authorization: Bearer token" https://api.example.com/data
```

### 设置Cookie
```bash
wget2go --cookie="session=abc123; user=john" https://example.com/dashboard
```

### 设置Referer
```bash
wget2go --referer="https://example.com/" https://example.com/download
```

## 递归下载

### 递归下载网站
```bash
wget2go -r https://example.com/
```

### 限制递归深度
```bash
wget2go -r -l 2 https://example.com/
```

### 下载页面所需的所有文件
```bash
wget2go -p https://example.com/page.html
```

### 转换链接用于本地浏览
```bash
wget2go -r -k https://example.com/
```

## 批量下载

### 从文件读取URL列表
```bash
wget2go -i urls.txt
```

urls.txt 内容：
```
https://example.com/file1.zip
https://example.com/file2.zip
https://example.com/file3.zip
```

### 使用通配符
```bash
wget2go https://example.com/files/file{1..10}.zip
```

## 高级用法

### 安静模式（不显示进度）
```bash
wget2go -q https://example.com/file.zip
```

### 详细模式（显示详细信息）
```bash
wget2go -v https://example.com/file.zip
```

### 不验证SSL证书（仅用于测试）
```bash
wget2go --insecure https://self-signed.example.com/
```

### 不跟随重定向
```bash
wget2go --no-follow-redirects https://example.com/redirect
```

## 配置文件示例

创建 `~/.config/wget2go/config.yaml`：

```yaml
# 全局配置
chunk_size: "10M"
max_threads: 8
timeout: "60s"
user_agent: "MyApp/1.0"
progress: true
```

## 环境变量

### 设置默认选项
```bash
export WGET2GO_MAX_THREADS=10
export WGET2GO_LIMIT_RATE="2M"
export WGET2GO_USER_AGENT="MyDownloader/1.0"
```

### 使用环境变量运行
```bash
WGET2GO_MAX_THREADS=10 wget2go https://example.com/large-file.iso
```

## 实际场景示例

### 下载Linux发行版ISO
```bash
wget2go --chunk-size=100M --max-threads=10 \
  https://releases.ubuntu.com/22.04/ubuntu-22.04.3-desktop-amd64.iso
```

### 镜像小型网站
```bash
wget2go -r -l 3 -k -p \
  --user-agent="Mozilla/5.0" \
  https://small-website.example.com/
```

### 批量下载图片
```bash
# 创建URL列表
for i in {1..100}; do
  echo "https://example.com/images/img_$i.jpg"
done > images.txt

# 批量下载
wget2go -i images.txt --max-threads=5
```

### 限速下载（避免占用所有带宽）
```bash
wget2go --limit-rate=500K \
  https://example.com/large-file.zip
```

## 故障排除

### 查看详细错误信息
```bash
wget2go -v https://example.com/nonexistent
```

### 测试连接
```bash
# 先测试HEAD请求
curl -I https://example.com/file.zip

# 然后使用wget2go下载
wget2go https://example.com/file.zip
```

### 处理重定向问题
```bash
# 查看重定向链
curl -L -I https://example.com/redirect

# 使用wget2go下载，限制重定向次数
wget2go --max-redirects=5 https://example.com/redirect
```

## 性能优化建议

1. **调整分片大小**：根据网络状况调整
   - 高速网络：10M-100M
   - 低速网络：1M-10M

2. **调整线程数**：根据服务器限制调整
   - 默认：5个线程
   - 高速服务器：10-20个线程
   - 限制严格的服务器：2-3个线程

3. **使用限速**：避免影响其他网络活动
   - 家庭网络：限制为总带宽的80%
   - 共享网络：限制为公平份额

4. **调整超时**：根据网络稳定性调整
   - 稳定网络：30秒
   - 不稳定网络：60-120秒