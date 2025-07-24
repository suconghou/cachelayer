package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"github.com/suconghou/cachelayer/route"
	"github.com/suconghou/cachelayer/store"
	"github.com/suconghou/cachelayer/util"
	"github.com/suconghou/cachelayer/vhost"
)

func main() {
	var (
		port  = flag.Int("p", 6060, "listen port")
		host  = flag.String("h", "", "bind address")
		cfile = flag.String("c", "vhost.json", "config file path")
		cache = flag.String("f", "cache.db", "cache file")
	)
	flag.Parse()
	if err := store.Init(*cache); err != nil {
		util.Log.Fatal(err)
	}
	go signalListen(*cfile)
	util.Log.Fatal(serve(*host, *port))
}

func serve(host string, port int) error {
	http.HandleFunc("/", routeMatch)
	util.Log.Printf("Starting up on port %d", port)
	return http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil)
}

func routeMatch(w http.ResponseWriter, r *http.Request) {
	for _, p := range route.Route {
		matches := p.Reg.FindStringSubmatch(r.URL.Path)
		if matches != nil {
			if err := p.Handler(w, r, matches); err != nil {
				util.Log.Print(err)
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

func signalListen(cfile string) {
	tick := time.NewTicker(time.Minute * 5)
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGUSR1, syscall.SIGUSR2)
	c <- syscall.SIGUSR1
	for {
		select {
		case <-tick.C:
			if err := store.Expire(); err != nil {
				util.Log.Print(err)
			}
		case s := <-c:
			if s == syscall.SIGUSR2 {
				if err := store.Expire(); err != nil {
					util.Log.Print(err)
				}
			} else {
				if err := vhost.Load(cfile); err != nil {
					util.Log.Print(err)
				}
			}
		}
	}
}
