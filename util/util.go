package util

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"
)

var (
	// Log print to stdout
	Log = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
	// 此值，别处需要只读，只在此处修改
	tt   = time.Now()
	T    = tt.UnixMilli() // 时间戳毫秒值， 我们有一个地方总体维护时间戳，对精度要求不高的可以使用这个，减少系统调用
	NOW  = tt.Unix()      // 转化为秒级时间戳，只需要秒级精度的可以使用这个
	tick = make(chan int64, 5)
	cr   = regexp.MustCompile(`\d+/(\d+)`)
	rq   = regexp.MustCompile(`(\d+)-(\d+)?$`)
)

// JSONPut resp json
func JSONPut(w http.ResponseWriter, v any) (int, error) {
	bs, err := json.Marshal(v)
	if err != nil {
		return 0, err
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	return w.Write(bs)
}

func TimerInit() {
	tick <- T
	go func() {
		for tt = range time.Tick(time.Millisecond * 500) {
			T = tt.UnixMilli()
			NOW = tt.Unix()
			select {
			case tick <- T:
			default:
			}
		}
	}()
}

func GetLen(r string) int64 {
	arr := cr.FindStringSubmatch(r)
	if len(arr) < 2 {
		return 0
	}
	l, _ := strconv.ParseInt(arr[1], 10, 64)
	return l
}

func GetRange(r string) (int64, int64) {
	var start, end int64
	arr := rq.FindStringSubmatch(r)
	if len(arr) < 2 {
		return start, end
	}
	if len(arr) > 2 {
		end, _ = strconv.ParseInt(arr[2], 10, 64)
	}
	start, _ = strconv.ParseInt(arr[1], 10, 64)
	return start, end
}

func Md5(b []byte) []byte {
	sum := md5.Sum(b)
	dst := make([]byte, hex.EncodedLen(len(sum)))
	hex.Encode(dst, sum[:])
	return dst
}
