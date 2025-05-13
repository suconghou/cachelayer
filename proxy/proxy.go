package proxy

import (
	"io"
	"net/http"

	"github.com/suconghou/cachelayer/request"
	"github.com/suconghou/cachelayer/vhost"
)

var (
	exposeHeaders = []string{
		"Accept-Ranges",
		"Content-Length",
		"Content-Type",
		"Content-Encoding",
		"Content-Range",
		"Cache-Control",
		"Last-Modified",
		"Etag",
	}
	fwdHeadersBasic = []string{
		"User-Agent",
		"Accept",
		"Accept-Encoding",
		"Accept-Language",
		"Cookie",
		"Range",
		"Referer",
	}
)

func Do(w http.ResponseWriter, r *http.Request, match []string) error {
	url, withQuery, strictCache, client, cacheSec := vhost.Parse(match[0])
	if url == "" {
		http.NotFound(w, r)
		return nil
	}
	if !strictCache && (r.Header.Get("If-Modified-Since") != "" || r.Header.Get("If-None-Match") != "") {
		http.Error(w, "", http.StatusNotModified)
		return nil
	}
	if withQuery && r.URL.RawQuery != "" {
		url = url + "?" + r.URL.RawQuery
	}
	var reqHeaders = copyHeader(r.Header, http.Header{}, fwdHeadersBasic)
	res, statusCode, headers, err := request.HttpProvider.Get(url, reqHeaders, client, int64(cacheSec))
	if res != nil {
		defer res.Close()
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	if r.Method != http.MethodGet {
		return nil
	}
	to := w.Header()
	copyHeader(headers, to, exposeHeaders)
	w.WriteHeader(statusCode)
	_, err = io.Copy(w, res)
	return err
}

func copyHeader(from http.Header, to http.Header, headers []string) http.Header {
	for _, k := range headers {
		if v := from.Get(k); v != "" {
			to.Set(k, v)
		}
	}
	return to
}
