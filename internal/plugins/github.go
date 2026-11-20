package plugins

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// InstallFrom accepts an existing local package path or an HTTPS GitHub release link.
func (m *Manager) InstallFrom(ctx context.Context, input string) (Manifest, error) {
	return m.InstallFromWithProgress(ctx, input, nil)
}

// InstallFromWithProgress reports GitHub stages synchronously on the calling
// goroutine. The callback should return promptly and marshal UI updates as needed.
// Stages contain no credentials or user-supplied URLs. Local installs emit none.
func (m *Manager) InstallFromWithProgress(ctx context.Context, input string, progress func(string)) (Manifest, error) {
	input = strings.TrimSpace(input)
	if strings.Contains(input, "://") || strings.HasPrefix(input, "github.com/") {
		return m.InstallGitHubWithProgress(ctx, input, progress)
	}
	if err := ctx.Err(); err != nil {
		return Manifest{}, err
	}
	return m.Install(input)
}

// InstallGitHub installs a published ZIP binary for the current operating system
// and architecture. It never downloads source archives or runs a build.
func (m *Manager) InstallGitHub(ctx context.Context, input string) (Manifest, error) {
	return m.InstallGitHubWithProgress(ctx, input, nil)
}

// InstallGitHubWithProgress is InstallGitHub with the stage callback described
// by InstallFromWithProgress. Stages are github:credentials, github:release,
// github:download, github:install, and github:complete (only after success).
func (m *Manager) InstallGitHubWithProgress(ctx context.Context, input string, progress func(string)) (Manifest, error) {
	return newGitHubInstaller().installWithProgress(ctx, m, input, progress)
}

type githubLink struct{ owner, repo, tag, asset string }

var githubComponent = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func parseGitHubLink(input string) (githubLink, error) {
	var out githubLink
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "github.com/") {
		input = "https://" + input
	}
	u, err := url.Parse(input)
	invalid := func() (githubLink, error) {
		return out, errors.New("plugin: expected an HTTPS github.com repository or release URL (without query or fragment)")
	}
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" {
		return invalid()
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/"), "/")
	if len(parts) < 2 {
		return invalid()
	}
	for _, p := range parts {
		if !githubComponent.MatchString(p) || p == "." || p == ".." {
			return invalid()
		}
	}
	out.owner, out.repo = parts[0], strings.TrimSuffix(parts[1], ".git")
	if out.repo == "" || out.repo == "." || out.repo == ".." {
		return invalid()
	}
	switch {
	case len(parts) == 2:
	case len(parts) == 4 && parts[2] == "releases" && parts[3] == "latest":
	case len(parts) == 5 && parts[2] == "releases" && parts[3] == "tag":
		out.tag = parts[4]
	case len(parts) == 6 && parts[2] == "releases" && parts[3] == "latest" && parts[4] == "download":
		out.asset = parts[5]
	case len(parts) == 6 && parts[2] == "releases" && parts[3] == "download":
		out.tag, out.asset = parts[4], parts[5]
	default:
		return invalid()
	}
	return out, nil
}

type githubAsset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}
type githubRelease struct {
	Assets []githubAsset `json:"assets"`
}
type githubInstaller struct {
	client       *http.Client
	apiBase      string
	credential   func(context.Context) string
	goos, goarch string
	maxBytes     int64
	timeout      time.Duration
}

func newGitHubInstaller() githubInstaller {
	return githubInstaller{client: newGitHubHTTPClient(), apiBase: "https://api.github.com", credential: githubCredential, goos: runtime.GOOS, goarch: runtime.GOARCH, maxBytes: MaxPackageSize, timeout: 5 * time.Minute}
}
func githubCredential(ctx context.Context) string {
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if token := strings.TrimSpace(os.Getenv(key)); token != "" {
			return token
		}
	}
	// Output only is used deliberately: gh diagnostics must never expose credentials.
	home, _ := os.UserHomeDir()
	gh := githubCLIPath(home)
	if gh == "" {
		return ""
	}
	raw, err := githubCommand(ctx, githubCommandTimeout, gh, "auth", "token", "--hostname", "github.com")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// Finder's PATH commonly omits both user-local and Homebrew executables.
func githubCLIPath(home string) string {
	if path, err := exec.LookPath("gh"); err == nil {
		return path
	}
	candidates := []string{"/opt/homebrew/bin/gh", "/usr/local/bin/gh"}
	if home != "" {
		candidates = append([]string{filepath.Join(home, ".local", "bin", "gh")}, candidates...)
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.Mode().IsRegular() && st.Mode().Perm()&0111 != 0 {
			return candidate
		}
	}
	return ""
}

func platformAsset(name, goos, goarch string) bool {
	if !strings.HasSuffix(strings.ToLower(name), ".zip") {
		return false
	}
	osAliases := map[string]string{"darwin": "darwin", "macos": "darwin", "osx": "darwin", "linux": "linux", "windows": "windows", "win": "windows", "win32": "windows", "freebsd": "freebsd"}
	archAliases := map[string]string{"arm64": "arm64", "aarch64": "arm64", "amd64": "amd64", "x64": "amd64", "x86_64": "amd64", "386": "386", "i386": "386", "i686": "386", "arm": "arm", "armv7": "arm"}
	// Preserve x86_64 before splitting underscores.
	normalized := strings.ReplaceAll(strings.ToLower(name), "x86_64", "amd64")
	words := strings.FieldsFunc(normalized, func(r rune) bool { return r == '-' || r == '_' || r == '.' })
	hasOS, hasArch := false, false
	for _, word := range words {
		if value, ok := osAliases[word]; ok {
			if value != goos {
				return false
			}
			hasOS = true
		}
		if value, ok := archAliases[word]; ok {
			if value != goarch {
				return false
			}
			hasArch = true
		}
	}
	return hasOS && hasArch
}
func (g githubInstaller) request(ctx context.Context, endpoint, token, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, errors.New("plugin: invalid GitHub API endpoint")
	}
	base, _ := url.Parse(g.apiBase)
	if req.URL.Scheme != base.Scheme || req.URL.Host != base.Host || !strings.HasPrefix(req.URL.Path, "/repos/") {
		return nil, errors.New("plugin: untrusted GitHub API asset endpoint")
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "mygo-player")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := *g.client
	previous := client.CheckRedirect
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		// Strip first, including same-host redirects; a signed release CDN needs no token.
		next.Header.Del("Authorization")
		defer next.Header.Del("Authorization")
		if len(via) >= 10 {
			return errors.New("plugin: too many GitHub redirects")
		}
		if next.URL.Scheme != "https" && !(base.Scheme == "http" && next.URL.Scheme == "http") {
			return errors.New("plugin: insecure GitHub download redirect")
		}
		if previous != nil {
			return previous(next, via)
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var timeout interface{ Timeout() bool }
		if errors.As(err, &timeout) && timeout.Timeout() {
			return nil, fmt.Errorf("plugin: GitHub connection or response timed out: %w", context.DeadlineExceeded)
		}
		return nil, errors.New("plugin: GitHub request failed")
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		switch resp.StatusCode {
		case 401, 403:
			return nil, fmt.Errorf("plugin: GitHub HTTP %d: authentication or access denied; set GH_TOKEN/GITHUB_TOKEN or install GitHub CLI and run gh auth login", resp.StatusCode)
		case 404:
			return nil, errors.New("plugin: GitHub HTTP 404: repository or published release not found; private repositories require GH_TOKEN/GITHUB_TOKEN or GitHub CLI (gh auth login); a published platform ZIP is required, source builds are unsupported")
		default:
			return nil, fmt.Errorf("plugin: GitHub HTTP %d", resp.StatusCode)
		}
	}
	return resp, nil
}
func (g githubInstaller) install(ctx context.Context, m *Manager, input string) (Manifest, error) {
	return g.installWithProgress(ctx, m, input, nil)
}

func (g githubInstaller) installWithProgress(ctx context.Context, m *Manager, input string, progress func(string)) (Manifest, error) {
	link, err := parseGitHubLink(input)
	if err != nil {
		return Manifest{}, err
	}
	return g.withPackage(ctx, link, false, progress, func(ctx context.Context, path string) (Manifest, error) {
		return m.installPackage(ctx, path, link.updateSource())
	})
}

func (g githubInstaller) withPackage(ctx context.Context, link githubLink, preferAsset bool, progress func(string), apply func(context.Context, string) (Manifest, error)) (Manifest, error) {
	var zero Manifest
	report := func(stage string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if progress != nil {
			progress(stage)
		}
		return ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()
	if err := report("github:credentials"); err != nil {
		return zero, err
	}
	token := g.credential(ctx)
	if err := report("github:release"); err != nil {
		return zero, err
	}
	endpoint := strings.TrimSuffix(g.apiBase, "/") + "/repos/" + link.owner + "/" + link.repo + "/releases/"
	if link.tag == "" {
		endpoint += "latest"
	} else {
		endpoint += "tags/" + url.PathEscape(link.tag)
	}
	resp, err := g.request(ctx, endpoint, token, "application/vnd.github+json")
	if err != nil {
		return zero, err
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	resp.Body.Close()
	if err != nil {
		return zero, fmt.Errorf("plugin: read GitHub release: %w", err)
	}
	if len(raw) > 2<<20 {
		return zero, errors.New("plugin: GitHub release response too large")
	}
	var release githubRelease
	if err = json.Unmarshal(raw, &release); err != nil {
		return zero, errors.New("plugin: invalid GitHub release response")
	}
	if preferAsset && link.asset != "" {
		found := false
		for _, asset := range release.Assets {
			if asset.Name == link.asset {
				found = true
				break
			}
		}
		if !found {
			link.asset = ""
		}
	}
	var selected *githubAsset
	for i := range release.Assets {
		a := &release.Assets[i]
		if link.asset != "" && a.Name != link.asset {
			continue
		}
		if !platformAsset(a.Name, g.goos, g.goarch) {
			continue
		}
		if selected != nil {
			return zero, fmt.Errorf("plugin: multiple ZIP assets match %s/%s; use an explicit release asset URL", g.goos, g.goarch)
		}
		selected = a
	}
	if selected == nil {
		return zero, fmt.Errorf("plugin: release has no compatible ZIP binary for %s/%s (asset names must include platform and architecture); source builds are unsupported", g.goos, g.goarch)
	}
	if selected.Size < 0 || selected.Size > g.maxBytes {
		return zero, errors.New("plugin: GitHub ZIP exceeds 256 MiB size limit")
	}
	// Use the authenticated asset API, never browser_download_url (private repos need it).
	assetURL, parseErr := url.Parse(selected.URL)
	expectedPath := "/repos/" + link.owner + "/" + link.repo + "/releases/assets/"
	if parseErr != nil || assetURL.RawQuery != "" || assetURL.Fragment != "" || assetURL.User != nil || !strings.HasPrefix(assetURL.Path, expectedPath) || !regexp.MustCompile(`^[0-9]+$`).MatchString(strings.TrimPrefix(assetURL.Path, expectedPath)) {
		return zero, errors.New("plugin: invalid GitHub release asset API URL")
	}
	if err := report("github:download"); err != nil {
		return zero, err
	}
	resp, err = g.request(ctx, selected.URL, token, "application/octet-stream")
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	if resp.ContentLength > g.maxBytes {
		return zero, errors.New("plugin: GitHub ZIP exceeds 256 MiB size limit")
	}
	f, err := os.CreateTemp("", "mygo-github-*.zip")
	if err != nil {
		return zero, err
	}
	path := f.Name()
	// path is the exact absolute name returned by CreateTemp, owned by this call.
	defer os.Remove(path)
	n, err := io.Copy(f, io.LimitReader(resp.Body, g.maxBytes+1))
	closeErr := f.Close()
	if err != nil {
		return zero, fmt.Errorf("plugin: download GitHub ZIP: %w", err)
	}
	if closeErr != nil {
		return zero, closeErr
	}
	if n > g.maxBytes {
		return zero, errors.New("plugin: GitHub ZIP exceeds 256 MiB size limit")
	}
	if selected.Size > 0 && n != selected.Size {
		return zero, errors.New("plugin: GitHub ZIP download size mismatch")
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		return zero, errors.New("plugin: GitHub asset is not a valid ZIP package")
	}
	z.Close()
	if err := report("github:install"); err != nil {
		return zero, err
	}
	manifest, err := apply(ctx, path)
	if err == nil && progress != nil {
		// Installation has committed: completion must still be reported if the
		// caller cancels immediately after the last pre-install check.
		progress("github:complete")
	}
	return manifest, err
}
