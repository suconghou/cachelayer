package route

import (
	"net/http"
	"regexp"

	"github.com/suconghou/cachelayer/proxy"
)

// 路由定义
type routeInfo struct {
	Reg     *regexp.Regexp
	Handler func(http.ResponseWriter, *http.Request, []string) error
}

// Route export route list
var Route = []routeInfo{
	{regexp.MustCompile(`^.*$`), proxy.Do},
}
