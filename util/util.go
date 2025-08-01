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

	"github.com/suconghou/cachelayer/pool"
)

var (
	// Log print to stdout
	Log        = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
	cr         = regexp.MustCompile(`\d+/(\d+)`)
	rq         = regexp.MustCompile(`(\d+)-(\d+)?$`)
	BufferPool = pool.NewBufferPool(1<<20, 8<<20)
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
