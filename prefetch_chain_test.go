package main

import (
	"testing"

	"mygo-player/internal/catalog"
)

// 用户停在同一个位置时，追加完一批就不能再追加下一批。
func TestPrefetchDoesNotChainAtSameScrollPosition(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.hasMore = true
	a.cursor = "2"
	a.listScroll.Y = 1200
	a.prefetch()
	if !a.prefetching || !a.prefetchSeeded {
		t.Fatal("first prefetch at a new position should start")
	}
	// 模拟这一批已落地。
	a.prefetching = false
	a.prefetchY = a.listScroll.Y
	a.prefetch()
	if a.prefetching {
		t.Fatal("unchanged scroll position must not start another batch")
	}
	// 用户继续下滑后允许下一批。
	a.listScroll.Y = 2600
	a.prefetch()
	if !a.prefetching {
		t.Fatal("scrolling further should allow the next batch")
	}
}

func TestReloadResetsPrefetchChain(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.prefetchSeeded = true
	a.prefetchY = 900
	a.loadPage("推荐", false)
	if a.prefetchSeeded || a.prefetchY != 0 {
		t.Fatal("reload must reset the prefetch chain guard")
	}
	a.listCancel()
}
