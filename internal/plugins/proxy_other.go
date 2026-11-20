//go:build !windows

package plugins

import (
	"context"
	"net/url"
)

func windowsSystemProxy(context.Context) *url.URL { return nil }
