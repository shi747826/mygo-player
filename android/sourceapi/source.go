// Package sourceapi provides a portable, in-process boundary for mobile sources.
// Implementations remain in independent repositories and register at build time.
package sourceapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"
)

type Source interface {
	Call(context.Context, string, json.RawMessage) (json.RawMessage, error)
	Close() error
}
type Factory func(context.Context, string) (Source, error)
type Navigation struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
type Manifest struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	Capabilities []string        `json:"capabilities"`
	Navigation   []Navigation    `json:"navigation"`
	Filters      json.RawMessage `json:"filters,omitempty"`
}
type Registration struct {
	Manifest Manifest
	Open     Factory
}

var registry struct {
	sync.RWMutex
	entries []Registration
}
var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Register is called by the generated build-local registration file. The host
// repository never imports a specific source implementation.
func Register(raw string, factory Factory) {
	var m Manifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		panic(fmt.Errorf("mobile source manifest: %w", err))
	}
	if !validID.MatchString(m.ID) || m.Name == "" || factory == nil {
		panic(errors.New("invalid mobile source registration"))
	}
	registry.Lock()
	defer registry.Unlock()
	for _, r := range registry.entries {
		if r.Manifest.ID == m.ID {
			panic("duplicate mobile source registration")
		}
	}
	registry.entries = append(registry.entries, Registration{m, factory})
}
func Registered() []Registration {
	registry.RLock()
	defer registry.RUnlock()
	out := append([]Registration(nil), registry.entries...)
	sort.Slice(out, func(i, j int) bool { return out[i].Manifest.ID < out[j].Manifest.ID })
	return out
}
