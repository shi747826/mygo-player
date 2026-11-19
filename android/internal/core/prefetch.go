package core

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Playing an episode means resolving it first, and a source may need seconds
// to download and prepare the media. The page asks for the next episode while
// the current one plays, so swiping to it starts immediately; sessions that
// are never used are released after a short while.

const prefetchTTL = 3 * time.Minute

type prefetchEntry struct {
	source    string
	done      chan struct{}
	raw       json.RawMessage
	err       error
	at        time.Time
	sessionID string
}

type prefetchParams struct {
	ID      string `json:"ID"`
	Quality string `json:"quality"`
}

func prefetchKey(source, id string) string { return source + "\x00" + id }

func (s *Service) prefetch(source, id, quality string) {
	if source == "" || id == "" || s.ctx.Err() != nil {
		return
	}
	key := prefetchKey(source, id)
	s.prefetchMu.Lock()
	if _, ok := s.prefetching[key]; ok {
		s.prefetchMu.Unlock()
		return
	}
	if _, ok := s.entries[source]; !ok {
		s.prefetchMu.Unlock()
		return
	}
	entry := &prefetchEntry{source: source, done: make(chan struct{}), at: time.Now()}
	s.prefetching[key] = entry
	s.prefetchMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
		defer cancel()
		params, _ := json.Marshal(prefetchParams{ID: id, Quality: quality})
		raw, err := s.callSource(ctx, source, "Resolve", params)
		s.prefetchMu.Lock()
		entry.raw, entry.err = raw, err
		if err == nil {
			entry.sessionID = sessionOf(raw)
		}
		delete(s.prefetching, key)
		if err != nil || s.ctx.Err() != nil {
			// A failed or abandoned preparation owns no usable session.
			entry.raw = nil
		} else {
			s.ready[key] = entry
		}
		close(entry.done)
		s.prefetchMu.Unlock()
		s.prunePrefetch()
	}()
}

func (s *Service) callSource(ctx context.Context, source, method string, params json.RawMessage) (json.RawMessage, error) {
	s.prefetchMu.Lock()
	e, ok := s.entries[source]
	s.prefetchMu.Unlock()
	if !ok {
		return nil, errors.New("来源不可用")
	}
	return e.source.Call(ctx, method, params)
}

// takePrefetched returns prepared media for a resolve, if one is ready or on
// the way. The entry is consumed either way.
func (s *Service) takePrefetched(ctx context.Context, source string, params json.RawMessage) (json.RawMessage, bool, error) {
	var p prefetchParams
	if err := json.Unmarshal(params, &p); err != nil || p.ID == "" {
		return nil, false, nil
	}
	key := prefetchKey(source, p.ID)
	s.prefetchMu.Lock()
	entry := s.ready[key]
	if entry == nil {
		entry = s.prefetching[key]
	}
	if entry != nil {
		delete(s.ready, key)
	}
	s.prefetchMu.Unlock()
	if entry == nil {
		return nil, false, nil
	}
	select {
	case <-entry.done:
	case <-ctx.Done():
		// The page gave up; the preparation may still finish and be pruned.
		return nil, true, ctx.Err()
	case <-s.ctx.Done():
		return nil, true, errors.New("应用已关闭")
	}
	if entry.err != nil {
		return nil, true, entry.err
	}
	if entry.raw == nil {
		return nil, true, errors.New("播放准备已失效，请重试")
	}
	return entry.raw, true, nil
}

func sessionOf(raw json.RawMessage) string {
	var media struct {
		SessionID string
	}
	if json.Unmarshal(raw, &media) != nil {
		return ""
	}
	return media.SessionID
}

// prunePrefetch releases unused preparations and their media sessions.
func (s *Service) prunePrefetch() {
	now := time.Now()
	var expired []string
	type pending struct{ source, session string }
	var sessions []pending
	s.prefetchMu.Lock()
	for key, entry := range s.ready {
		if now.Sub(entry.at) > prefetchTTL {
			expired = append(expired, key)
			if entry.sessionID != "" {
				sessions = append(sessions, pending{entry.source, entry.sessionID})
			}
		}
	}
	for _, key := range expired {
		delete(s.ready, key)
	}
	s.prefetchMu.Unlock()
	for _, item := range sessions {
		s.releaseSession(item.source, item.session)
	}
}

func (s *Service) releaseSession(source, id string) {
	if id == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	params, _ := json.Marshal(map[string]string{"SessionID": id})
	_, _ = s.callSource(ctx, source, "Release", params)
}

// releasePrefetched drops every prepared media session on shutdown.
func (s *Service) releasePrefetched() {
	s.prefetchMu.Lock()
	type pending struct{ source, session string }
	var sessions []pending
	for key, entry := range s.ready {
		if entry.sessionID != "" {
			sessions = append(sessions, pending{entry.source, entry.sessionID})
		}
		delete(s.ready, key)
	}
	s.prefetchMu.Unlock()
	for _, item := range sessions {
		s.releaseSession(item.source, item.session)
	}
}
