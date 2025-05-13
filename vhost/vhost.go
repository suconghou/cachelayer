package vhost

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type vhost struct {
	Prefix      string `json:"prefix"`
	Suffix      string `json:"suffix"`
	KeyWord     string `json:"keyword"`
	Replace     string `json:"replace"`
	Target      string `json:"target"`
	Host        string `json:"host"`
	WithQuery   bool   `json:"withQuery"`
	StrictCache bool   `json:"strictCache"`
	CacheSec    uint32 `json:"cachesec"`
	Timeout     uint32 `json:"timeout"`
	MaxRedirect uint32 `json:"maxredirect"`
	client      *http.Client
}

var (
	vhosts = []*vhost{}
	dialer = &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
)

func hasPort(s string) bool { return strings.LastIndex(s, ":") > strings.LastIndex(s, "]") }

// Load config file
func Load(file string) error {
	var bs, err = os.ReadFile(file)
	if err != nil {
		return err
	}
	var config []*vhost
	err = json.Unmarshal(bs, &config)
	if err != nil {
		return err
	}
	for _, item := range config {
		if item.Timeout <= 0 {
			item.Timeout = 60
		}
		if item.MaxRedirect <= 0 {
			item.MaxRedirect = 3
		}
		var (
			match = ""
			host  = item.Host
		)
		if item.Host != "" {
			u, err := url.Parse(item.Target)
			if err != nil {
				return err
			}
			var originPort = u.Port()
			if originPort == "" {
				if u.Scheme == "https" {
					originPort = "443"
				} else {
					originPort = "80"
				}
			}
			if !hasPort(item.Host) {
				host = item.Host + ":" + originPort
			}
			match = u.Hostname() + ":" + originPort
		}
		item.client = client(item.Timeout, item.MaxRedirect, match, host)
	}
	vhosts = config
	return nil
}

// Parse got real target by parse vhost
func Parse(target string) (string, bool, bool, *http.Client, uint32) {
	for _, item := range vhosts {
		if strings.HasPrefix(target, item.Prefix) && strings.HasSuffix(target, item.Suffix) && strings.Contains(target, item.KeyWord) {
			if len(item.KeyWord) > 0 {
				target = strings.Replace(target, item.KeyWord, item.Replace, 1)
			}
			return item.Target + target, item.WithQuery, item.StrictCache, item.client, item.CacheSec
		}
	}
	return "", false, false, nil, 0
}

func client(timeout uint32, maxredirect uint32, match string, host string) *http.Client {
	var dialcontext = dialer.DialContext
	if match != "" && host != "" {
		dialcontext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == match {
				addr = host
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}
	return &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialcontext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= int(maxredirect) {
				return fmt.Errorf("stopped after %d redirects", maxredirect)
			}
			req.Header.Del("Referer")
			return nil
		},
	}
}
