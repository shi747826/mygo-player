package main

import (
	"context"
	"mygo-player/internal/catalog"
	"mygo-player/internal/plugins"
	"mygo-player/pluginapi"
)

// sourceAPI snapshots the current plugin. In-flight requests retain this client
// when the user changes sources; generation checks reject stale UI updates.
func (a *app) sourceAPI() plugins.Source {
	a.pluginHostMu.RLock()
	defer a.pluginHostMu.RUnlock()
	if p := a.pluginClients[a.source]; p != nil {
		return p
	}
	return unavailableSource{}
}

func (a *app) sourcePage(ctx context.Context, source plugins.Source, page, query, channel, sort, cursor string, size int) (catalog.Page, error) {
	switch page {
	case "搜索":
		return searchSourcePage(ctx, source, query, cursor, size)
	case "榜单":
		return source.Rank(ctx)
	default:
		if page != "推荐" && channel == "" {
			channel = page
		}
		if p, ok := source.(*plugins.Client); ok {
			return p.DiscoverWithOptions(ctx, plugins.DiscoverOptions{Channel: channel, Sort: sort, Cursor: cursor, PageSize: size})
		}
		return source.Discover(ctx, channel, cursor)
	}
}
func searchSourcePage(ctx context.Context, source plugins.Source, query, cursor string, size int) (catalog.Page, error) {
	fetch := func(cursor string) (catalog.Page, error) {
		if p, ok := source.(*plugins.Client); ok {
			return p.SearchWithOptions(ctx, plugins.SearchOptions{Query: query, Cursor: cursor, PageSize: size})
		}
		return source.Search(ctx, query, cursor)
	}
	page, err := fetch(cursor)
	if err != nil {
		return page, err
	}
	for pulled := 1; cursor == "" && page.HasMore && page.Cursor != "" && len(page.Items) < 18 && pulled < 6; pulled++ {
		if ctx.Err() != nil {
			break
		}
		next, err := fetch(page.Cursor)
		if err != nil {
			break
		}
		previous := page.Cursor
		page.Items = mergeSeries(page.Items, next.Items)
		page.Cursor = next.Cursor
		page.HasMore = next.HasMore
		if page.Cursor == previous {
			break
		}
	}
	var ids []string
	for _, s := range page.Items {
		if s.ID != "" && s.Cover == "" {
			ids = append(ids, s.ID)
		}
	}
	if len(ids) > 0 {
		if covers, err := source.Covers(ctx, ids); err == nil {
			for i := range page.Items {
				if page.Items[i].Cover == "" {
					page.Items[i].Cover = covers[page.Items[i].ID]
				}
			}
		}
	}
	return page, nil
}

// Offline UI tests and the empty-source screen never initialize a source SDK.
type unavailableSource struct{}

func (unavailableSource) Discover(context.Context, string, string) (catalog.Page, error) {
	return catalog.Page{}, pluginapi.ErrUnsupported
}
func (unavailableSource) Search(context.Context, string, string) (catalog.Page, error) {
	return catalog.Page{}, pluginapi.ErrUnsupported
}
func (unavailableSource) Rank(context.Context) (catalog.Page, error) {
	return catalog.Page{}, pluginapi.ErrUnsupported
}
func (unavailableSource) Detail(context.Context, string) (catalog.Series, []catalog.Episode, error) {
	return catalog.Series{}, nil, pluginapi.ErrUnsupported
}
func (unavailableSource) Seasons(context.Context, string) ([]catalog.Series, error) {
	return nil, pluginapi.ErrUnsupported
}
func (unavailableSource) Covers(context.Context, []string) (map[string]string, error) {
	return nil, pluginapi.ErrUnsupported
}
func (unavailableSource) Calendar(context.Context, int) (map[string]catalog.UpcomingEntry, error) {
	return nil, pluginapi.ErrUnsupported
}
func (unavailableSource) ResetDevice(context.Context) error { return pluginapi.ErrUnsupported }
func (unavailableSource) Resolve(context.Context, string, string) (pluginapi.Media, error) {
	return pluginapi.Media{}, pluginapi.ErrUnsupported
}
func (unavailableSource) Release(context.Context, string) error { return nil }
func (unavailableSource) Danmaku(context.Context, string, string) ([]pluginapi.Cue, error) {
	return nil, pluginapi.ErrUnsupported
}
