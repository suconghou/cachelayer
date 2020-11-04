package request

import (
	"crypto/tls"
	"io"
	"net/http"
	"time"
)

var (
	client = &http.Client{
		Timeout: time.Minute * 2,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
)

// Req request return http.Respsone
func Req(target string, method string, body io.ReadCloser, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequest(method, target, body)
	if err != nil {
		return nil, err
	}
	req.Header = headers
	return client.Do(req)
}
