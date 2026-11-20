package plugins

import (
	"fmt"
	"regexp"
	"strings"
)

// A leading v and abbreviated versions such as 1 or 1.2 are accepted for
// existing manifests. Compare prereleases by SemVer precedence and ignore build metadata.
var pluginVersionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)(?:\.(0|[1-9][0-9]*))?(?:\.(0|[1-9][0-9]*))?(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

var numericIdentifier = regexp.MustCompile(`^[0-9]+$`)

func parsePluginVersion(version string) ([]string, []string, error) {
	parts := pluginVersionPattern.FindStringSubmatch(strings.TrimSpace(version))
	if parts == nil {
		return nil, nil, fmt.Errorf("plugin: cannot compare version %q", version)
	}
	core := parts[1:4]
	for i, n := range core {
		if n == "" {
			core[i] = "0"
		}
	}
	var prerelease []string
	if parts[4] != "" {
		prerelease = strings.Split(parts[4], ".")
		for _, n := range prerelease {
			if len(n) > 1 && n[0] == '0' && numericIdentifier.MatchString(n) {
				return nil, nil, fmt.Errorf("plugin: cannot compare version %q", version)
			}
		}
	}
	return core, prerelease, nil
}

func compareVersionNumber(a, b string) int {
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(a, b)
}

func comparePluginVersions(a, b string) (int, error) {
	ac, ap, err := parsePluginVersion(a)
	if err != nil {
		return 0, err
	}
	bc, bp, err := parsePluginVersion(b)
	if err != nil {
		return 0, err
	}
	for i, n := range ac {
		if cmp := compareVersionNumber(n, bc[i]); cmp != 0 {
			return cmp, nil
		}
	}
	if len(ap) == 0 || len(bp) == 0 {
		return len(bp) - len(ap), nil
	}
	for i := range min(len(ap), len(bp)) {
		x, y := ap[i], bp[i]
		xn, yn := numericIdentifier.MatchString(x), numericIdentifier.MatchString(y)
		cmp := strings.Compare(x, y)
		switch {
		case xn && yn:
			cmp = compareVersionNumber(x, y)
		case xn:
			cmp = -1
		case yn:
			cmp = 1
		}
		if cmp != 0 {
			return cmp, nil
		}
	}
	return len(ap) - len(bp), nil
}
