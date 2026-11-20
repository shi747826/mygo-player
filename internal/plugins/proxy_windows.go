//go:build windows

package plugins

import (
	"context"
	"net/url"
	"strings"
)

// windowsSystemProxy returns the proxy that Windows applications use for
// HTTPS, or nil when the system proxy is off. GitHub's asset CDN is often
// unreachable without it on networks that require a local proxy.
func windowsSystemProxy(ctx context.Context) *url.URL {
	raw, err := githubCommand(ctx, githubCommandTimeout, "reg", "query",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`)
	if err != nil {
		return nil
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		switch fields[0] {
		case "ProxyEnable", "ProxyServer":
			values[fields[0]] = fields[len(fields)-1]
		}
	}
	if values["ProxyEnable"] != "0x1" && values["ProxyEnable"] != "1" {
		return nil
	}
	server := values["ProxyServer"]
	if server == "" {
		return nil
	}
	// ProxyServer may name one proxy or list them per protocol.
	host := server
	if strings.Contains(server, "=") {
		host = ""
		for _, entry := range strings.Split(server, ";") {
			protocol, address, ok := strings.Cut(entry, "=")
			if ok && (protocol == "https" || (host == "" && protocol == "http")) {
				host = address
				if protocol == "https" {
					break
				}
			}
		}
	}
	host = strings.TrimSpace(host)
	if host == "" || strings.ContainsAny(host, "/@?# \t\r\n") {
		return nil
	}
	// A proxy without a port uses the protocol default.
	if !strings.Contains(host, ":") {
		host += ":8080"
	}
	return &url.URL{Scheme: "http", Host: host}
}
