# cachelayer

一个支持 Range 读、读写分离与“边读边缓存”的 HTTP 缓存层（Cache Layer）。

核心目标：
- 作为反向代理/中间层，对上游 HTTP 源站做透明加速；
- 针对大文件使用分块缓存（默认 256KB），支持 Range 请求的按需拉取；
- 提供“读-写分离”：先从缓存读取，缺失部分向上游拉取并写入缓存；
- 尽可能复用上游的 ETag/Last-Modified/Cache-Control 等元信息；
- 在有限内存下进行高效流式转发（避免整文件落内存）。

## 特性
- 分块缓存：每个对象被切分为固定大小块（默认 256KB），块化写入 KV 存储。
- Range 支持：客户端携带 Range 时，仅回源所需区间并按块缓存。
- 边读边缓存（tee）：向客户端回传的同时，将已读满的块写入缓存，尾块在 Close 时落盘。
- 懒下载（lazy download）：仅对缓存缺失的区间回源。
- 元信息管理：缓存对象元数据（大小、过期策略等）。
- TTL 与过期清理：支持 TTL 与后台过期 key 清理（store 层）。

## 架构概览
- layer/：缓存层实现（分块、懒下载、tee 写入）。
- request/：HTTP 获取器，封装 Range/条件请求、Header 管理、错误处理。
- proxy/：对外 HTTP 入口，负责路由、头部转发、缓存控制、响应复制。
- store/：KV 存储接口与实现（含 TTL/扫描/过期清理）。
- vhost/：上游主机/传输层配置（含 Transport）。
- util/、pool/、multio/：通用工具、缓冲池与多路 Close/Flush 辅助。


## 快速开始

### 依赖
- Go 1.22+（建议与 go.mod 保持一致）
- 可选：Docker（用于容器化部署）

### 本地运行

```bash
make build
./cachelayer
```

或直接：

```bash
go run ./...
```

默认会读取当前目录下的 `vhost.json` 作为上游配置。

### Docker 运行

```bash
docker build -t cachelayer:latest .
docker run --rm -p 8080:8080 -v $(pwd)/vhost.json:/data/vhost.json cachelayer:latest
```

## 配置说明

- vhost.json：用于描述上游主机/路由规则/超时等（参见仓库中的示例文件）。
- 参数：
  - `-p`：服务监听端口，默认 6060
  - `-f`：缓存数据文件，默认 `./cache.db`
  - `-c`：配置文件，默认 `./vhost.json`
  - `-h`：监听的地址，默认 0.0.0.0

**信号识别**

- `SIGUSR1`：重新加载配置文件
- `SIGUSR2`：清理过期缓存

## 使用方式

服务作为反向代理对外：

- GET 请求（支持 Range）：
  - 客户端可携带 `Range: bytes=start-end`；
  - 首次命中时将从上游取回并按块写入缓存，同时把响应流式返回；
  - 再次请求命中缓存的块将直接从本地读出，提升响应速度；
  - 返回头部会保留/合成 `Content-Range`、`ETag`、`Last-Modified` 等。


