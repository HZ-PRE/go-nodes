浏览器访问 http://localhost:8080

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