package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlayerReleaseSource(t *testing.T) {
	const prefix = "/repos/owner/player/releases/"
	const target = "update-windows-amd64"
	var base string
	var wrongVersion bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("missing GitHub authorization")
		}
		switch r.URL.Path {
		case prefix + "latest":
			fmt.Fprintf(w, `{"tag_name":"v1.2.3","assets":[{"name":%q,"url":%q},{"name":%q,"url":%q,"size":7}]}`, target+".json", base+prefix+"assets/1", target+".tar.gz", base+prefix+"assets/2")
		case prefix + "assets/1":
			version := "1.2.3"
			if wrongVersion {
				version = "1.2.4"
			}
			fmt.Fprintf(w, `{"version":%q,"url":%q,"size":7,"signature":"fixture-signature"}`, version, target+".tar.gz")
		case prefix + "assets/2":
			if r.Header.Get("Accept") != "application/octet-stream" {
				t.Error("missing binary asset accept header")
			}
			fmt.Fprint(w, "archive")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	base = srv.URL
	source := PlayerReleaseSource("https://github.com/owner/player")
	source.github.apiBase, source.github.client = base, srv.Client()
	source.github.goos, source.github.goarch = "windows", "amd64"
	source.github.credential = func(_ context.Context) string { return "fixture-token" }
	manifestBody, err := source.Manifest(t.Context(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer manifestBody.Close()
	var manifest struct{ Version, URL string }
	if err = json.NewDecoder(manifestBody).Decode(&manifest); err != nil || manifest.Version != "1.2.3" || manifest.URL != base+prefix+"assets/2" {
		t.Fatalf("manifest = %+v, %v", manifest, err)
	}
	body, err := source.Open(t.Context(), manifest.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil || string(data) != "archive" {
		t.Fatalf("archive = %q, %v", data, err)
	}
	if _, err := source.Open(t.Context(), strings.Replace(manifest.URL, "/owner/", "/other/", 1)); err == nil {
		t.Fatal("accepted an asset in another repository")
	}
	wrongVersion = true
	if _, err := source.Manifest(t.Context(), false); err == nil {
		t.Fatal("accepted a mismatched release version")
	}
}
