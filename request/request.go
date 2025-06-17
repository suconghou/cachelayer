package request

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/suconghou/cachelayer/layer"
	"github.com/suconghou/cachelayer/pool"
	"github.com/suconghou/cachelayer/util"
)

var (
	bufferPool   = pool.NewBufferPool(1048576, 5242880)
	HttpProvider = NewLockGeter()
)

const (
	ps = 262144
	cr = "Content-Range"
	cl = "Content-Length"
	rr = "Range"
)

type buffer struct {
	*bytes.Buffer
}

func (b *buffer) Close() error {
	b.Reset()
	bufferPool.Put(b.Buffer)
	return nil
}

type LockGeter struct {
}

func NewLockGeter() *LockGeter {
	return &LockGeter{}
}

// 此处我们需要确认目标是否支持range，及其大小
func (l *LockGeter) Get(url string, reqHeaders http.Header, client *http.Client, ttl int64) (io.ReadCloser, int, http.Header, error) {
	var (
		cacheKey     = util.Md5([]byte(url))
		cacheKeyMeta = bytes.Join([][]byte{cacheKey, []byte("0")}, []byte(":"))
		start, end   = util.GetRange(reqHeaders.Get(rr))
		minfo, err   = layer.LoadMeta(cacheKeyMeta)
	)
	if err != nil {
		res, code, h, ll, err := part1(url, reqHeaders.Clone(), client)
		if err != nil {
			return res, code, h, err
		}
		if ll < 1 || code != http.StatusPartialContent { // 不支持range，直接返回响应体
			return res, code, h, nil
		}
		if ll < ps { // 文件太小, 我们检查，用户是否请求了range，把内容切割出来
			b, err := ReadBytes(res, ll)
			if err != nil { // 读取body时发生错误，有可能超时，或者http协议不规范，响应头与响应体字节数不一致
				return b, code, h, err
			}
			if (start > 0 || end > 0) && start < ll { // 有请求range，需要切割响应体
				if end == 0 || end >= ll {
					end = ll - 1
				}
				bb := bytes.NewBuffer(b.Bytes()[start:end])
				h.Set(cl, strconv.Itoa(bb.Len()))
				h.Set(cr, fmt.Sprintf("bytes %d-%d/%d", start, end, ll))
				return &buffer{bb}, code, h, nil
			}
			h.Set(cl, strconv.Itoa(b.Len()))
			h.Del(cr)
			return b, http.StatusOK, h, nil
		}
		// 否则，支持range，文件大小也符合
		b, err := ReadBytes(res, ps)
		if err != nil { // 应该读取 262144 字节，可能网络超时，或者http协议不规范，读取的响应体比预期大
			return b, code, h, err
		}
		if err = layer.CacheSet(bytes.Join([][]byte{cacheKey, []byte("1")}, []byte(":")), b.Bytes(), ttl); err != nil {
			return b, code, h, err // 写盘错误
		}
		if minfo, err = layer.SetMeta(cacheKeyMeta, ll, h, ttl); err != nil { // 存储或序列化失败
			return b, code, h, err
		}
		if start >= ll || end >= ll {
			h.Set(cl, "0")
			h.Del(cr)
			return &buffer{bytes.NewBuffer([]byte(""))}, http.StatusRequestedRangeNotSatisfiable, h, nil
		}
	}
	var statusCode = http.StatusPartialContent
	if start >= minfo.Length || end >= minfo.Length {
		return &buffer{bytes.NewBuffer([]byte(""))}, http.StatusRequestedRangeNotSatisfiable, minfo.Header, nil
	} else if start < 1 && end < 1 {
		statusCode = http.StatusOK
	}
	data := layer.NewCacheLayer(Get, url, cacheKey, start, end, reqHeaders, client, minfo.Length, ttl)
	return data, statusCode, minfo.Header, err
}

func Get(target string, reqHeaders http.Header, client *http.Client) (io.ReadCloser, int, http.Header, error) {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header = reqHeaders
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	if resp.StatusCode/100 != 2 {
		resp.Body.Close()
		return nil, resp.StatusCode, resp.Header, fmt.Errorf("%s %s : %s", resp.Request.Method, resp.Request.URL, resp.Status)
	}
	return resp.Body, resp.StatusCode, resp.Header, nil
}

// 返回的buf被Close后自动回收
func GetBytes(target string, reqHeaders http.Header, client *http.Client, max int64) (*buffer, int, http.Header, error) {
	resp, code, h, err := Get(target, reqHeaders, client)
	if err != nil {
		return nil, code, h, err
	}
	b, err := ReadBytes(resp, max)
	return b, code, h, err
}

func ReadBytes(r io.ReadCloser, max int64) (*buffer, error) {
	var buf = bufferPool.Get(65536)
	buf.Reset()
	defer r.Close()
	if _, err := buf.ReadFrom(http.MaxBytesReader(nil, r, max)); err != nil {
		buf.Reset()
		bufferPool.Put(buf)
		return nil, err
	}
	return &buffer{buf}, nil
}

// 传入的http.Header必须是clone后的，修改不会干扰源数据,请求前256kb数据
func part1(url string, reqHeaders http.Header, client *http.Client) (io.ReadCloser, int, http.Header, int64, error) {
	reqHeaders.Set(rr, "bytes=0-262143")
	b, code, h, err := Get(url, reqHeaders, client)
	if err != nil {
		return nil, code, h, 0, err
	}
	l := util.GetLen(h.Get(cr))
	return b, code, h, l, nil
}
