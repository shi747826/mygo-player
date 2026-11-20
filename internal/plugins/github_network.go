package plugins

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http/httpproxy"
)

const githubCommandTimeout = 2 * time.Second

// githubCommand bounds both process execution and pipe draining: killing a
// command alone does not close stdout inherited by one of its descendants.
func githubCommand(ctx context.Context, timeout time.Duration, path string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	configureBackgroundCommand(cmd)
	cmd.WaitDelay = 100 * time.Millisecond
	raw, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return raw, err
}

func newGitHubHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.ResponseHeaderTimeout = 15 * time.Second
	transport.ExpectContinueTimeout = time.Second
	transport.Proxy = githubProxy
	return &http.Client{Transport: transport}
}

var githubProxyCache struct {
	sync.Mutex
	at     time.Time
	values map[string]string
}

func githubProxy(req *http.Request) (*url.URL, error) {
	host := req.URL.Hostname()
	ip := net.ParseIP(host)
	if strings.EqualFold(host, "localhost") || (ip != nil && (ip.IsLoopback() || ip.IsUnspecified())) {
		return nil, nil
	}
	// Read the environment per request instead of using ProxyFromEnvironment's
	// process-wide snapshot. Explicit shell configuration remains authoritative.
	env := httpproxy.FromEnvironment()
	if (req.URL.Scheme == "https" && env.HTTPSProxy != "") || (req.URL.Scheme == "http" && env.HTTPProxy != "") {
		return env.ProxyFunc()(req.URL)
	}
	// Apply NO_PROXY to system proxies too, using the standard matcher.
	env.HTTPProxy, env.HTTPSProxy = "http://proxy.invalid", "http://proxy.invalid"
	if proxy, err := env.ProxyFunc()(req.URL); err != nil || proxy == nil {
		return nil, err
	}
	if runtime.GOOS == "darwin" {
		return githubMacProxy(req.URL.Scheme, githubSystemProxyValues(req.Context())), nil
	}
	return nil, nil
}

func githubSystemProxyValues(ctx context.Context) map[string]string {
	githubProxyCache.Lock()
	if !githubProxyCache.at.IsZero() && time.Since(githubProxyCache.at) < 5*time.Second {
		values := githubProxyCache.values
		githubProxyCache.Unlock()
		return values
	}
	githubProxyCache.Unlock()
	// Do not hold the cache lock across a subprocess; cancellation of another
	// install must not wait for the current reader.
	raw, err := githubCommand(ctx, githubCommandTimeout, "/usr/sbin/scutil", "--proxy")
	if ctx.Err() != nil {
		return nil
	}
	var values map[string]string
	if err == nil {
		values = githubParseProxyValues(string(raw))
	}
	githubProxyCache.Lock()
	githubProxyCache.at, githubProxyCache.values = time.Now(), values
	githubProxyCache.Unlock()
	return values
}

func githubParseProxyValues(raw string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), " : ")
		if ok {
			values[key] = strings.TrimSpace(value)
		}
	}
	return values
}

func githubMacProxy(scheme string, values map[string]string) *url.URL {
	prefix := "HTTP"
	if scheme == "https" {
		prefix = "HTTPS"
	} else if scheme != "http" {
		return nil
	}
	if proxy := githubMacProxyAddress(prefix, "http", values); proxy != nil {
		return proxy
	}
	return githubMacProxyAddress("SOCKS", "socks5", values)
}

func githubMacProxyAddress(prefix, scheme string, values map[string]string) *url.URL {
	if values[prefix+"Enable"] != "1" {
		return nil
	}
	host := values[prefix+"Proxy"]
	port, err := strconv.Atoi(values[prefix+"Port"])
	if host == "" || strings.ContainsAny(host, "/@?# \t\r\n") || err != nil || port < 1 || port > 65535 {
		return nil
	}
	return &url.URL{Scheme: scheme, Host: net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(port))}
}
