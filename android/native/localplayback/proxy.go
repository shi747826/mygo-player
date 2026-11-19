package localplayback

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// NewSystemProxy creates a proxy selector that follows Android's current settings.
func NewSystemProxy() func(*http.Request) (*url.URL, error) {
	controller := sync.OnceValues(NewController)
	return func(req *http.Request) (*url.URL, error) {
		host := strings.ToLower(strings.TrimSuffix(req.URL.Hostname(), "."))
		if host == "localhost" || strings.HasSuffix(host, ".localhost") {
			return nil, nil
		}
		if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsUnspecified()) {
			return nil, nil
		}
		native, err := controller()
		if err != nil {
			return nil, err
		}
		target := *req.URL
		target.User = nil
		target.Fragment = ""
		raw, err := native.SystemProxy(target.String())
		if err != nil || raw == "" {
			return nil, err
		}
		proxy, err := url.Parse(raw)
		if err != nil || proxy.Hostname() == "" || proxy.Port() == "" || (proxy.Scheme != "http" && proxy.Scheme != "socks5") {
			return nil, fmt.Errorf("invalid Android system proxy address")
		}
		return proxy, nil
	}
}
