package main

import (
	"mygo-player/internal/catalog"
	"path/filepath"
)

func (a *app) brandName() string { return a.sourceDisplayName() }

func (a *app) sourceHas(capability string) bool {
	if a.pluginManager == nil {
		return false
	}
	for _, m := range a.pluginManager.Manifests() {
		if m.ID == a.source {
			return m.Has(capability)
		}
	}
	return false
}

// Each opaque source ID gets its own local favorites and playback history.
func (a *app) store() *catalog.Client {
	if c := a.sourceStores[a.source]; c != nil {
		return c
	}
	return a.client
}
func (a *app) ensureSourceStore(id string) error {
	if id == "" || a.dataDir == "" {
		return nil
	}
	if a.sourceStores == nil {
		a.sourceStores = map[string]*catalog.Client{}
	}
	if a.sourceStores[id] != nil {
		return nil
	}
	c, err := catalog.New(filepath.Join(a.dataDir, "libraries", id))
	if err != nil {
		return err
	}
	a.sourceStores[id] = c
	return nil
}
func (a *app) refreshStore() {
	if a.store() == nil {
		return
	}
	lib, err := a.store().Library()
	if err == nil {
		a.library = lib
	}
}

func (a *app) targetPageSize() int {
	cols := a.layoutCols
	if cols < 1 {
		width := a.gridWidth
		if width <= 0 && a.win != nil {
			w, _ := a.win.Size()
			width = float32(w)
			if !a.portrait {
				width -= musicSidebarWidth
			}
			width -= 56
		}
		cols = gridColumns(width)
	}
	return min(50, max(10, cols*5))
}
func (a *app) pageSizeOr(fallback int) int {
	if a.pageSize > 0 {
		return a.pageSize
	}
	return fallback
}
