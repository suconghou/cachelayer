package layer

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/suconghou/cachelayer/multio"
	"github.com/suconghou/cachelayer/util"
)

const (
	// ChunkSize 定义了缓存分片的细粒度大小，固定大小 256KB
	ChunkSize = 256 * 1024
)

// getter 是执行实际HTTP请求的函数签名
type getter func(string, http.Header) (io.ReadCloser, int, http.Header, error)

// cacheLayer 实现了 io.ReadCloser 接口
type cacheLayer struct {
	target     string
	getter     getter
	store      CacheStore // 用于生成分片缓存键的基础Key
	start      int64
	end        int64
	reqHeaders http.Header
	length     int64
	ttl        int64

	reader io.ReadCloser // 内部使用的拼接读取器
	once   sync.Once     // 保证读取器只构建一次
	err    error         // 存储构建读取器时发生的错误
}

type cacheKVItem struct {
	load   func() (io.Reader, error)
	reader io.Reader // 内部使用的拼接读取器
	once   sync.Once // 保证读取器只构建一次
	err    error     // 存储构建读取器时发生的错误
}

func (c *cacheKVItem) Read(p []byte) (int, error) {
	c.once.Do(func() {
		c.reader, c.err = c.load()
	})
	if c.err != nil {
		return 0, c.err
	}
	return c.reader.Read(p)
}

// lazyDownloader 是一个下载任务的占位符，实现了 io.ReadCloser
type lazyDownloader struct {
	layer     *cacheLayer // 引用父级以访问 getter, storage 等
	startByte int64
	endByte   int64

	reader io.ReadCloser // 实际的下载通道 (cachingTeeReader)
	once   sync.Once
	err    error
}

// Read 在首次被调用时，才真正触发下载
func (l *lazyDownloader) Read(p []byte) (int, error) {
	l.once.Do(func() {
		// 1. 准备 HTTP 请求
		headers := l.layer.reqHeaders.Clone()
		headers.Set("Range", fmt.Sprintf("bytes=%d-%d", l.startByte, l.endByte))
		// 2. 执行下载
		res, statusCode, _, err := l.layer.getter(l.layer.target, headers)
		if err != nil {
			l.err = err
			return
		}
		if statusCode != http.StatusOK && statusCode != http.StatusPartialContent {
			l.err = fmt.Errorf("bad status code: %d", statusCode)
			res.Close()
			return
		}
		// 3. 将响应体包装成 cachingTeeReader
		l.reader = &cachingTeeReader{
			source:            res,
			store:             l.layer.store,
			ttl:               l.layer.ttl,
			currentChunkIndex: l.startByte / ChunkSize, // 计算起始分片索引
			buffer:            new(bytes.Buffer),
		}
	})
	if l.err != nil {
		return 0, l.err
	}
	return l.reader.Read(p)
}

// Close 确保底层的 reader 被关闭
func (l *lazyDownloader) Close() error {
	if l.reader != nil {
		return l.reader.Close()
	}
	return nil
}

// cachingTeeReader 是 "边下边存" 的智能通道，实现了 io.ReadCloser
type cachingTeeReader struct {
	source io.ReadCloser // 原始 HTTP 响应体
	store  CacheStore
	ttl    int64

	currentChunkIndex int64         // 当前正在填充的分片索引
	buffer            *bytes.Buffer // 当前分片的缓冲
}

func (r *cachingTeeReader) Read(p []byte) (n int, err error) {
	// 从原始数据源读取
	n, err = r.source.Read(p)
	if n > 0 {
		// Tee: 将读到的数据也写入我们自己的缓冲
		r.buffer.Write(p[:n])
		// 检查缓冲是否达到了一个或多个分片的大小
		for r.buffer.Len() >= ChunkSize {
			chunkData := r.buffer.Next(ChunkSize)
			// 存储完整的分片
			chunkKey := []byte(strconv.FormatInt(r.currentChunkIndex, 10))
			if err = r.store.Set(chunkKey, chunkData, r.ttl); err != nil {
				util.Log.Print(err)
			}
			// 移至下一个分片
			r.currentChunkIndex++
		}
	}
	return n, err
}

func (r *cachingTeeReader) Close() error {
	// 冲洗(Flush)最后一个不完整的分片
	if r.buffer.Len() > 0 {
		chunkKey := []byte(strconv.FormatInt(r.currentChunkIndex, 10))
		// 这里不需要复制，因为是最后一次写入
		if err := r.store.Set(chunkKey, r.buffer.Bytes(), r.ttl); err != nil {
			util.Log.Print(err)
		}
	}
	return r.source.Close()
}

func (c *cacheLayer) Read(p []byte) (int, error) {
	c.once.Do(func() { // 使用 sync.Once 确保 buildReader 方法只被执行一次
		c.reader, c.err = c.buildReader()
	})
	if c.err != nil {
		return 0, c.err
	}
	return c.reader.Read(p)
}

func (c *cacheLayer) Close() error {
	if c.reader != nil {
		return c.reader.Close()
	}
	return nil
}

func (c *cacheLayer) buildReader() (io.ReadCloser, error) {
	var (
		readers    []io.Reader
		startChunk = c.start / ChunkSize
		endChunk   = c.end / ChunkSize
	)
	util.Log.Printf("%d-%d %d-%d", c.start, c.end, startChunk, endChunk)
	// 遍历所有需要的分片
	for i := startChunk; i <= endChunk; {
		chunkKey := []byte(strconv.FormatInt(i, 10))
		if c.store.Has(chunkKey) {
			util.Log.Printf("缓存命中 chunkKey: %d", i)
			readers = append(readers, &cacheKVItem{load: func() (io.Reader, error) {
				b, err := c.store.Get(chunkKey)
				return bytes.NewReader(b), err
			}})
			i++
		} else {
			util.Log.Printf("无缓存 chunkKey: %d", i)
			missingEndChunk := i
			for j := i + 1; j <= endChunk; j++ {
				chunkKey = []byte(strconv.FormatInt(j, 10))
				if c.store.Has(chunkKey) {
					break
				}
				util.Log.Printf("无缓存 chunkKey: %d", j)
				missingEndChunk = j
			}
			downloadStart := i * ChunkSize
			downloadEnd := (missingEndChunk+1)*ChunkSize - 1
			if downloadEnd >= c.length {
				downloadEnd = c.length - 1
			}
			downloader := &lazyDownloader{
				layer:     c, // 传递对 cacheLayer 的引用
				startByte: downloadStart,
				endByte:   downloadEnd,
			}
			util.Log.Printf("下载 lazyDownloader chunkKey: %d-%d", downloadStart, downloadEnd)
			readers = append(readers, downloader)
			// 跳过整个缺失的区块
			i = missingEndChunk + 1
		}
	}
	multiReader := multio.MultiReadReader(readers...)
	startOffsetInChunk := c.start % ChunkSize
	if startOffsetInChunk > 0 {
		if _, err := io.CopyN(io.Discard, multiReader, startOffsetInChunk); err != nil {
			return nil, fmt.Errorf("failed to seek to start offset: %w", err)
		}
	}
	totalReadSize := c.end - c.start + 1
	finalReader := io.LimitReader(multiReader, totalReadSize)
	return multio.FuncCloser(finalReader, multiReader.Close), nil
}

// 传入的getter在非200区间时也自动抛出错误
func NewCacheLayer(gt getter, target string, cstore CacheStore, start, end int64, reqHeaders http.Header, length, ttl int64) io.ReadCloser {
	if end <= 0 || end > length-1 {
		end = length - 1
	}
	if start > end {
		start = end
	}
	l := &cacheLayer{
		getter:     gt,
		target:     target,
		store:      cstore,
		start:      start,
		end:        end,
		reqHeaders: reqHeaders,
		length:     length,
		ttl:        ttl,
	}
	return l
}
