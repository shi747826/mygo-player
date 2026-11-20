package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
)

// PlayerReleaseSource serves the files from one GitHub release to MyGo's
// signed updater. The archive must belong to the same release as its manifest.
func PlayerReleaseSource(repository string) *playerReleaseSource {
	return &playerReleaseSource{github: newGitHubInstaller(), repository: repository}
}

type playerReleaseSource struct {
	github     githubInstaller
	repository string
}

var playerAssetID = regexp.MustCompile(`^[0-9]+$`)

func (s *playerReleaseSource) Manifest(ctx context.Context, _ bool) (io.ReadCloser, error) {
	link, err := parseGitHubLink(s.repository)
	if err != nil {
		return nil, err
	}
	g := s.github
	prefix := strings.TrimSuffix(g.apiBase, "/") + "/repos/" + link.owner + "/" + link.repo + "/releases/"
	token := g.credential(ctx)
	response, err := g.request(ctx, prefix+"latest", token, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var release struct {
		Tag string `json:"tag_name"`
		githubRelease
	}
	const releaseLimit = 2 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, releaseLimit+1))
	if err != nil {
		return nil, err
	}
	if len(body) > releaseLimit || json.Unmarshal(body, &release) != nil {
		return nil, errors.New("GitHub 返回的播放器发布信息无效")
	}
	stem := "update-" + g.goos + "-" + g.goarch
	var manifestURL, archiveURL string
	var archiveSize int64
	for _, asset := range release.Assets {
		id, ok := strings.CutPrefix(asset.URL, prefix+"assets/")
		if !ok || !playerAssetID.MatchString(id) {
			continue
		}
		switch asset.Name {
		case stem + ".json":
			manifestURL = asset.URL
		case stem + ".tar.gz":
			archiveURL, archiveSize = asset.URL, asset.Size
		}
	}
	if manifestURL == "" || archiveURL == "" {
		return nil, fmt.Errorf("GitHub 发布 %s 缺少适用于当前平台的播放器更新包", release.Tag)
	}
	response, err = g.request(ctx, manifestURL, token, "application/octet-stream")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	const manifestLimit = 1 << 20
	body, err = io.ReadAll(io.LimitReader(response.Body, manifestLimit+1))
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Version   string `json:"version"`
		Notes     string `json:"notes,omitempty"`
		Date      string `json:"date,omitempty"`
		URL       string `json:"url"`
		Size      int64  `json:"size"`
		Signature string `json:"signature"`
	}
	if len(body) > manifestLimit || json.Unmarshal(body, &manifest) != nil ||
		"v"+manifest.Version != release.Tag || manifest.URL != stem+".tar.gz" || manifest.Size != archiveSize || manifest.Size <= 0 || manifest.Signature == "" {
		return nil, errors.New("GitHub 播放器更新清单与发布文件不符")
	}
	manifest.URL = archiveURL
	body, err = json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(string(body))), nil
}

func (s *playerReleaseSource) Open(ctx context.Context, endpoint string) (io.ReadCloser, error) {
	link, err := parseGitHubLink(s.repository)
	if err != nil {
		return nil, err
	}
	prefix := strings.TrimSuffix(s.github.apiBase, "/") + "/repos/" + link.owner + "/" + link.repo + "/releases/assets/"
	id, ok := strings.CutPrefix(endpoint, prefix)
	if !ok || !playerAssetID.MatchString(id) {
		return nil, errors.New("播放器更新文件地址无效")
	}
	response, err := s.github.request(ctx, prefix+url.PathEscape(id), s.github.credential(ctx), "application/octet-stream")
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}
