package cachelayer

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"sync"

	"github.com/suconghou/cachelayer/request"
	"github.com/suconghou/cachelayer/store"
)

var (
	kv sync.Map
)

// CacheLayer for cache
type CacheLayer struct {
	target string
	r      io.Reader
}

type cacheItem struct {
	target string
	left   int64
	right  int64
	header http.Header
}

type cacheLazyReader struct {
	r   *cacheItem
	res io.Reader
}

func (l *cacheLazyReader) Read(p []byte) (int, error) {
	if l.res == nil {
		res, err := cacherequest(l.r)
		if err != nil {
			return 0, err
		}
		l.res = res
	}
	return l.res.Read(p)
}

func (c *CacheLayer) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

// NewLayer for cache layer
func NewLayer(target string, start int64, end int64) (*CacheLayer, int64, error) {
	var r, size, err = pre(target, 1024*1024, start, end)
	if err != nil {
		return nil, size, err
	}
	return &CacheLayer{
		target,
		r,
	}, size, nil
}

func pre(urlStr string, max int64, start int64, end int64) (io.Reader, int64, error) {
	var parts, size, err = cacheItemParts(urlStr, max, start, end)
	if err != nil {
		return nil, size, err
	}
	return newCacheReadConcater(parts), size, nil
}

func newCacheReadConcater(items []*cacheItem) io.Reader {
	var buffers = []io.Reader{}
	for _, t := range items {
		buffers = append(buffers, &cacheLazyReader{
			r: t,
		})
	}
	return io.MultiReader(buffers...)
}

func cacherequest(item *cacheItem) (io.Reader, error) {
	var data = store.GetCache(item.target)
	if data == nil {
		var res, err = request.Req(item.target, http.MethodGet, nil, item.header)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		data, err = io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		if err = store.SetCache(item.target, data); err != nil {
			return nil, err
		}
	}
	var bs []byte
	if item.left > 0 && item.right > 0 {
		bs = data[item.left:item.right]
	} else if item.left > 0 {
		bs = data[item.left:]
	} else if item.right > 0 {
		bs = data[:item.right]
	} else {
		bs = data
	}
	return bytes.NewReader(bs), nil
}

// from , to 根据请求range解析得来,按照range规范,按照规范,浏览器发出的to值,最大应为size-1
// 对客户端的响应大小应为 to-from+1
func cacheItemParts(urlStr string, itemLen int64, from int64, to int64) ([]*cacheItem, int64, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, 0, err
	}
	var query = u.Query()
	filesize, err := getReqLen(urlStr)
	if err != nil {
		return nil, 0, err
	}
	if filesize <= itemLen {
		return nil, filesize, fmt.Errorf("too small to support")
	}
	if to <= 0 || to >= filesize {
		to = filesize - 1
	}
	if from >= filesize || from > to {
		return nil, filesize, fmt.Errorf("error from-to")
	}
	var (
		items = []*cacheItem{}
		left  int64
		right int64
		start = (from / itemLen) * itemLen
		end   = ((to / itemLen) + 1) * itemLen
		i     = 0
		last  bool
	)
	if end > filesize {
		end = filesize
	}
	// start,end 是字节对齐的,end值被修正时也可能是文件大小
	for {
		offset := start + itemLen - 1
		if offset >= end-1 {
			offset = end - 1
			last = true
		}
		if i == 0 {
			left = from - start
		} else {
			left = 0
		}
		if last {
			right = (offset - start + 1) - (end - to) + 1
		} else {
			right = 0
		}
		rr := fmt.Sprintf("%d-%d", start, offset)
		query.Set("range", rr)
		u.RawQuery = query.Encode()
		items = append(items, &cacheItem{
			u.String(),
			left,
			right,
			http.Header{
				"Range": []string{fmt.Sprintf("bytes=%s", rr)},
			},
		})
		i++
		start = offset + 1
		if last {
			break
		}
	}
	return items, filesize, nil
}

func getReqLen(urlStr string) (int64, error) {
	v, exist := kv.Load(urlStr)
	if exist {
		val, ok := v.(int64)
		if !ok {
			return 0, fmt.Errorf("cache val error")
		}
		return val, nil
	}
	resp, err := request.Req(urlStr, http.MethodGet, nil, http.Header{"range": []string{"bytes=0-1"}})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.ContentLength != 2 {
		return resp.ContentLength, fmt.Errorf("not support")
	}
	var (
		filesize    int64
		cr          = resp.Header.Get("Content-Range")
		rangeResReg = regexp.MustCompile(`\d+/(\d+)`)
	)
	if rangeResReg.MatchString(cr) {
		matches := rangeResReg.FindStringSubmatch(cr)
		filesize, _ = strconv.ParseInt(matches[1], 10, 64)
	}
	kv.Store(urlStr, filesize)
	return filesize, nil
}
