package main

import (
	"errors"
	"testing"

	"mygo-player/internal/catalog"
)

// 失败后停止自动重试，避免停在底部时反复请求同一页。
func TestPrefetchPausedAfterFailure(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.hasMore = true
	a.cursor = "2"
	a.prefetchPaused = true
	a.prefetch()
	if a.prefetching {
		t.Fatal("paused prefetch must not start a request")
	}
}

// 重新加载列表会清掉暂停状态，让用户刷新后能继续翻页。
func TestLoadPageClearsPrefetchPause(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.prefetchPaused = true
	a.page = "推荐"
	a.loadPage("推荐", false)
	if a.prefetchPaused {
		t.Fatal("reload must clear prefetch pause")
	}
	a.listCancel()
}

func TestMergeSeriesDropsDuplicates(t *testing.T) {
	old := []catalog.Series{{ID: "a"}, {ID: "b"}}
	more := []catalog.Series{{ID: "b"}, {ID: "c"}, {ID: ""}}
	got := mergeSeries(old, more)
	if len(got) != 3 || got[0].ID != "a" || got[1].ID != "b" || got[2].ID != "c" {
		t.Fatalf("merge = %+v", got)
	}
}

var _ = errors.Is
