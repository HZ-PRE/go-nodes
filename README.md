浏览器访问 http://localhost:8080

## Go 1.27

使用 Go 1.27 或更新版本构建。项目已在 Go 1.27.3 上验证，升级依据见 [Go 1.27 发布说明](https://go.dev/doc/go1.27)。

- `encoding/json` 在 1.27 中默认使用新的 v2 引擎，保持原有 JSON 语义。UCloud 成功响应直接从响应体解析，移除完整响应缓冲及字符串转换，并保留 `UseNumber` 的大整数精度。
- HTTP 探测复用共享连接池并启用 HTTP/2。1.27 的 HTTP/1 响应关闭会有限量排空未读内容，使小响应也能复用连接，无需下载整个探测页面。
- 域名回退解析使用 1.27 新增的 `strings.CutLast`，避免 `Split` 切片和 `Join` 拼接。
- 探测任务使用固定 worker 池及 `sync.WaitGroup.Go`（1.25 引入），并发数参考 `GOMAXPROCS` 的运行时 CPU 配额，最少为 4。DNS 失败或没有 IPv4 地址时正常返回失败结果，避免丢失结果导致等待不退出。
- Go 1.27 的内存分配和 DEFLATE 压缩优化由工具链自动提供；实际收益需要结合生产负载测量。

回归测试使用 1.27 新增的 `httptest.NewTestServer` 内存网络及 `synctest.Sleep`，覆盖连接复用、超时、JSON 精度、探测并发和失败路径。测试不需要数据库或云账户：

```powershell
go test ./... -timeout 60s
go test -race ./... -timeout 60s
go vet ./...
```

## 发布

```bash
go mod tidy 
# linux构建
$env:GOOS="linux"  
$env:GOARCH="arm64" #根据环境配置
$env:GOARCH="amd64" #根据环境配置 阿里是
go build -o nodes
go run .
# win构建
$env:GOOS="windows"
$env:GOARCH="amd64" 
go build -o nodes.exe
```

## 运行

```bash
./gotool
```
