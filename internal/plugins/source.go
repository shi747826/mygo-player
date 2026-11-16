// Package plugins discovers and runs independent executable source plugins.
package plugins

import (
	"context"
	"mygo-player/pluginapi"
)

const ProtocolVersion = pluginapi.ProtocolVersion
const MaxMessageSize = pluginapi.MaxMessageSize

type Series = pluginapi.Series
type Episode = pluginapi.Episode
type Page = pluginapi.Page
type UpcomingEntry = pluginapi.UpcomingEntry
type DiscoverOptions = pluginapi.DiscoverOptions
type SearchOptions = pluginapi.SearchOptions
type params = pluginapi.Params
type Source interface {
	Discover(context.Context, string, string) (Page, error)
	Search(context.Context, string, string) (Page, error)
	Rank(context.Context) (Page, error)
	Detail(context.Context, string) (Series, []Episode, error)
	Seasons(context.Context, string) ([]Series, error)
	Covers(context.Context, []string) (map[string]string, error)
	ResetDevice(context.Context) error
	Calendar(context.Context, int) (map[string]UpcomingEntry, error)
	Resolve(context.Context, string, string) (pluginapi.Media, error)
	Release(context.Context, string) error
	Danmaku(context.Context, string, string) ([]pluginapi.Cue, error)
}

func (c *Client) Discover(ctx context.Context, channel, cursor string) (Page, error) {
	return c.DiscoverWithOptions(ctx, DiscoverOptions{Channel: channel, Cursor: cursor})
}
func (c *Client) DiscoverWithOptions(ctx context.Context, o DiscoverOptions) (out Page, err error) {
	err = c.call(ctx, "Discover", params{DiscoverOptions: o}, &out)
	return
}
func (c *Client) Search(ctx context.Context, query, cursor string) (Page, error) {
	return c.SearchWithOptions(ctx, SearchOptions{Query: query, Cursor: cursor})
}
func (c *Client) SearchWithOptions(ctx context.Context, o SearchOptions) (out Page, err error) {
	err = c.call(ctx, "Search", params{Search: o}, &out)
	return
}
func (c *Client) Rank(ctx context.Context) (out Page, err error) {
	err = c.call(ctx, "Rank", params{}, &out)
	return
}
func (c *Client) Detail(ctx context.Context, id string) (Series, []Episode, error) {
	var out pluginapi.DetailResult
	err := c.call(ctx, "Detail", params{ID: id}, &out)
	return out.Series, out.Episodes, err
}
func (c *Client) Seasons(ctx context.Context, id string) (out []Series, err error) {
	err = c.call(ctx, "Seasons", params{ID: id}, &out)
	return
}
func (c *Client) Covers(ctx context.Context, ids []string) (out map[string]string, err error) {
	err = c.call(ctx, "Covers", params{IDs: ids}, &out)
	return
}
func (c *Client) ResetDevice(ctx context.Context) error {
	return c.call(ctx, "ResetDevice", params{}, nil)
}
func (c *Client) Calendar(ctx context.Context, days int) (out map[string]UpcomingEntry, err error) {
	err = c.call(ctx, "Calendar", params{Days: days}, &out)
	return
}
func (c *Client) Resolve(ctx context.Context, id, quality string) (out pluginapi.Media, err error) {
	err = c.call(ctx, "Resolve", params{ID: id, Quality: quality}, &out)
	return
}
func (c *Client) Release(ctx context.Context, sessionID string) error {
	return c.call(ctx, "Release", params{SessionID: sessionID}, nil)
}
func (c *Client) Danmaku(ctx context.Context, seriesID, episodeID string) (out []pluginapi.Cue, err error) {
	err = c.call(ctx, "Danmaku", params{SeriesID: seriesID, ID: episodeID}, &out)
	return
}

var _ Source = (*Client)(nil)
