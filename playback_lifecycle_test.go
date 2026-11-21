package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"mygo-player/internal/catalog"
	"mygo-player/pluginapi"
)

// releaseRecorder is a source stub that only records Release calls; resolving
// media is not needed for lifecycle checks.
type releaseRecorder struct {
	mu       sync.Mutex
	released []string
	called   chan string
}

func (r *releaseRecorder) record(id string) {
	r.mu.Lock()
	r.released = append(r.released, id)
	r.mu.Unlock()
	if r.called != nil {
		select {
		case r.called <- id:
		default:
		}
	}
}

func (r *releaseRecorder) Discover(context.Context, string, string) (pluginapi.Page, error) {
	return pluginapi.Page{}, nil
}
func (r *releaseRecorder) Search(context.Context, string, string) (pluginapi.Page, error) {
	return pluginapi.Page{}, nil
}
func (r *releaseRecorder) Rank(context.Context) (pluginapi.Page, error) {
	return pluginapi.Page{}, nil
}
func (r *releaseRecorder) Detail(context.Context, string) (pluginapi.Series, []pluginapi.Episode, error) {
	return pluginapi.Series{}, nil, nil
}
func (r *releaseRecorder) Seasons(context.Context, string) ([]pluginapi.Series, error) {
	return nil, nil
}
func (r *releaseRecorder) Covers(context.Context, []string) (map[string]string, error) {
	return nil, nil
}
func (r *releaseRecorder) ResetDevice(context.Context) error { return nil }
func (r *releaseRecorder) Calendar(context.Context, int) (map[string]pluginapi.UpcomingEntry, error) {
	return nil, nil
}
func (r *releaseRecorder) Resolve(context.Context, string, string) (pluginapi.Media, error) {
	return pluginapi.Media{}, nil
}
func (r *releaseRecorder) Danmaku(context.Context, string, string) ([]pluginapi.Cue, error) {
	return nil, nil
}
func (r *releaseRecorder) Release(_ context.Context, id string) error {
	r.record(id)
	return nil
}

func newLifecycleApp(t *testing.T) *app {
	t.Helper()
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	t.Cleanup(a.cancel)
	return a
}

// A prepared episode can be closed by the stale-callback path and the media
// owner at the same time; the source session must only be released once.
func TestPreparedEpisodeReleaseIsIdempotent(t *testing.T) {
	source := &releaseRecorder{called: make(chan string, 4)}
	prep := &preparedEpisode{media: pluginapi.Media{SessionID: "session-1"}, source: source}
	prep.close()
	prep.close()
	prep.close()
	select {
	case id := <-source.called:
		if id != "session-1" {
			t.Fatalf("released %q", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Release was not called")
	}
	time.Sleep(50 * time.Millisecond)
	source.mu.Lock()
	defer source.mu.Unlock()
	if len(source.released) != 1 {
		t.Fatalf("released %v, want exactly once", source.released)
	}
}

// A resolve callback from a superseded episode must release its media rather
// than adopt it into the player.
func TestStaleAdoptReleasesWithoutOpening(t *testing.T) {
	a := newLifecycleApp(t)
	source := &releaseRecorder{called: make(chan string, 1)}
	a.playID = 7
	prep := &preparedEpisode{media: pluginapi.Media{SessionID: "old", URL: "https://example.invalid/old.mp4"}, source: source}
	a.adoptMedia(6, prep, 0, true)
	select {
	case <-source.called:
	case <-time.After(2 * time.Second):
		t.Fatal("stale media session was not released")
	}
	if a.player != nil {
		t.Fatal("stale media must not create a player")
	}
}

// Leaving the player invalidates resolve callbacks that are still in flight and
// clears the switch bookkeeping so a late callback cannot revive old state.
func TestStopPlayerInvalidatesPendingEpisode(t *testing.T) {
	a := newLifecycleApp(t)
	a.playID = 3
	a.awaitingMedia = true
	a.switchIssued = true
	a.switchingEpisode = true
	a.stopPlayer()
	if a.playID <= 3 {
		t.Fatalf("playID=%d, want it bumped", a.playID)
	}
	if a.awaitingMedia || a.switchIssued || a.switchingEpisode {
		t.Fatalf("stale switch state survived: awaiting=%v issued=%v switching=%v", a.awaitingMedia, a.switchIssued, a.switchingEpisode)
	}
}

// While the next episode is resolving no new media generation exists yet, so
// playback state from the old video must not be consumed.
func TestAdvancePlaybackWaitsForUnissuedSwitch(t *testing.T) {
	a := newLifecycleApp(t)
	a.awaitingMedia = true
	if a.switchIssued {
		t.Fatal("test precondition")
	}
	a.advancePlayback()
	if !a.awaitingMedia {
		t.Fatal("advancePlayback consumed state before a switch was issued")
	}
}
