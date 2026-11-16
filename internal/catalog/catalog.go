// Package catalog owns source-neutral local favorites and playback history.
// Remote requests belong to independently installed source plugins.
package catalog

import (
	"errors"
	"fmt"
	"mygo-player/pluginapi"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ReleaseState = pluginapi.ReleaseState

const (
	ReleaseUnknown   = pluginapi.ReleaseUnknown
	ReleaseUpcoming  = pluginapi.ReleaseUpcoming
	ReleasePublished = pluginapi.ReleasePublished
)

type Series = pluginapi.Series
type Episode = pluginapi.Episode
type Page = pluginapi.Page
type UpcomingEntry = pluginapi.UpcomingEntry

type Progress struct {
	Series    Series    `json:"series"`
	Episode   Episode   `json:"episode"`
	Position  float64   `json:"position"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Library struct {
	Favorites map[string]Series   `json:"favorites"`
	History   map[string]Progress `json:"history"`
}
type Client struct {
	mu      sync.Mutex
	path    string
	library Library
}

func New(dataDir string) (*Client, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, errors.New("data directory is required")
	}
	c := &Client{path: filepath.Join(dataDir, "library.json")}
	if err := c.open(); err != nil {
		return nil, err
	}
	return c, nil
}
func localError(message string, cause error) error { return fmt.Errorf("%s: %w", message, cause) }
