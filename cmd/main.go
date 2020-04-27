package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/suconghou/cachelayer"
	"github.com/suconghou/cachelayer/store"
)

var (
	// logger write to stdout
	logger = log.New(os.Stdout, "", 0)
)

// 路由定义
type routeInfo struct {
	Reg     *regexp.Regexp
	Handler func(http.ResponseWriter, *http.Request, []string) error
}

var route = []routeInfo{
	{regexp.MustCompile(`^/(https?)/(.+)$`), serveProxy},
}

func serveProxy(w http.ResponseWriter, r *http.Request, match []string) error {
	var (
		ran         = r.URL.Query().Get("range")
		target      = fmt.Sprintf("%s://%s", match[1], match[2])
		rangeReqReg = regexp.MustCompile(`(\d+)-(\d+)?$`)
		start       int64
		end         int64
	)
	if r.Header.Get("range") != "" {
		ran = r.Header.Get("range")
	}
	if ran != "" && rangeReqReg.MatchString(ran) {
		matches := rangeReqReg.FindStringSubmatch(ran)
		start, _ = strconv.ParseInt(matches[1], 10, 64)
		if matches[2] != "" {
			end, _ = strconv.ParseInt(matches[2], 10, 64)
		} else {
			end = 0
		}
	}
	l, size, err := cachelayer.NewLayer(target, start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil
	}
	head := w.Header()
	head.Set("Accept-Ranges", "bytes")
	if end > 0 || start > 0 {
		if end <= 0 || end >= size {
			end = size - 1
		}
		head.Set("Content-Length", fmt.Sprintf("%d", end-start+1))
		head.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		head.Set("Content-Length", fmt.Sprintf("%d", size))
		w.WriteHeader(http.StatusOK)
	}
	_, err = io.Copy(w, l)
	return err
}

func main() {
	var (
		port   = flag.Int("p", 6060, "listen port")
		host   = flag.String("h", "", "bind address")
		dbfile = flag.String("d", "cache.db", "data file path")
	)
	flag.Parse()
	if err := store.Init(*dbfile); err != nil {
		logger.Fatal(err)
	}
	logger.Fatal(serve(*host, *port))
}

func serve(host string, port int) error {
	http.HandleFunc("/", routeMatch)
	logger.Printf("Starting up on port %d", port)
	return http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil)
}

func routeMatch(w http.ResponseWriter, r *http.Request) {
	for _, p := range route {
		if p.Reg.MatchString(r.URL.Path) {
			if err := p.Handler(w, r, p.Reg.FindStringSubmatch(r.URL.Path)); err != nil {
				logger.Print(err)
			}
			return
		}
	}
	fallback(w, r)
}

func fallback(w http.ResponseWriter, r *http.Request) {
	const index = "index.html"
	files := []string{index}
	if r.URL.Path != "/" {
		files = []string{r.URL.Path, path.Join(r.URL.Path, index)}
	}
	if !tryFiles(files, w, r) {
		http.NotFound(w, r)
	}
}

func tryFiles(files []string, w http.ResponseWriter, r *http.Request) bool {
	for _, file := range files {
		realpath := filepath.Join("./public", file)
		if f, err := os.Stat(realpath); err == nil {
			if f.Mode().IsRegular() {
				http.ServeFile(w, r, realpath)
				return true
			}
		}
	}
	return false
}
