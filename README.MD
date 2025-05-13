## a cache layer proxy 

split request to chunks and proxy each chunk with cache


并发控制
代理控制

多路并写

存储，请求分离

请求时range切片10M，底层存储是切片存储1M

上层切片是1M的倍数

上层请求时无需对齐，底层自动对齐，底层采用256kb对齐，最后一个分片可能不足256kb

如果确认没有到达文件尽头，可能需要对用户请求的end改写，使其256kb对齐

流式响应，使用io.Reader接口封装缓存逻辑


colordiff store/store.go ../gateway/store/store.go
colordiff vhost/vhost.go ../cproxy/vhost/vhost.go
colordiff pool/pool.go ../cproxy/pool/pool.go
