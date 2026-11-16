package main

import (
	"errors"
	"mygo-player/internal/catalog"
	"mygo-player/internal/plugins"
	"mygo-player/pluginapi"
	"testing"
)

func TestBrandNameFollowsArbitraryManifest(t *testing.T) {
	a := &app{}
	attachUITestPlugin(t, a, plugins.Manifest{ID: "third-source", Name: "第三个来源", Version: "1", Capabilities: []string{"detail", "resolve"}})
	if a.brandName() != "第三个来源" {
		t.Fatal(a.brandName())
	}
	a.source = "missing"
	if a.brandName() != "选择来源" {
		t.Fatal(a.brandName())
	}
}
func TestStoreIsolatedByOpaquePluginID(t *testing.T) {
	base, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := &app{client: base, dataDir: t.TempDir()}
	for _, id := range []string{"third-source", "fourth-source"} {
		if err := a.ensureSourceStore(id); err != nil {
			t.Fatal(err)
		}
		a.source = id
		if _, err := a.store().ToggleFavorite(catalog.Series{ID: "same", Title: id}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"third-source", "fourth-source"} {
		a.source = id
		lib, err := a.store().Library()
		if err != nil || lib.Favorites["same"].Title != id {
			t.Fatalf("source=%s lib=%+v err=%v", id, lib, err)
		}
	}
	a.source = "uninitialized"
	if a.store() != base {
		t.Fatal("offline fallback missing")
	}
}
func TestFriendlyPluginErrors(t *testing.T) {
	for _, err := range []error{pluginapi.ErrTransport, pluginapi.ErrUnsupported, pluginapi.ErrContentUnavailable, errors.New("boom")} {
		if friendly(err) == "" {
			t.Fatal("missing neutral error")
		}
	}
}
