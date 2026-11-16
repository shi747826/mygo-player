package main

import (
	"testing"

	"mygo-player/internal/catalog"
)

// 没有手动选过画质时，默认用分辨率最高的一档。
func TestHighestQualityDefault(t *testing.T) {
	choices := []string{"自动", "720P 标清", "1080P 高清", "480P 流畅"}
	if got := highestQuality(choices, ""); got != "1080P 高清" {
		t.Fatalf("default quality = %q want the highest definition", got)
	}
	// 片源实际选中的档位优先，即使顺序不同。
	if got := highestQuality(choices, "720P 标清"); got != "720P 标清" {
		t.Fatalf("chosen stream ignored: %q", got)
	}
	if got := highestQuality([]string{"自动"}, ""); got != "自动" {
		t.Fatalf("automatic fallback = %q", got)
	}
	if got := highestQuality(nil, ""); got != "自动" {
		t.Fatalf("empty choices fallback = %q", got)
	}
}

// 用户手动选的画质会保存，下次启动沿用；没选过则不写死档位。
func TestQualityPreferencePersistence(t *testing.T) {
	client, err := catalog.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(client, nil)
	defer a.cancel()
	a.dataDir = t.TempDir()

	if got := a.currentPreferences().Quality; got != "" {
		t.Fatalf("unselected quality should not persist, got %q", got)
	}
	a.quality = "720P 标清"
	a.qualityChosen = true
	saved := a.currentPreferences()
	if saved.Quality != "720P 标清" {
		t.Fatalf("saved quality = %q", saved.Quality)
	}
	if err := savePreferences(a.dataDir, saved); err != nil {
		t.Fatal(err)
	}
	p := loadPreferences(a.dataDir)
	if p.Quality != "720P 标清" {
		t.Fatalf("loaded quality = %q", p.Quality)
	}
	b := newApp(client, nil)
	defer b.cancel()
	b.applyPreferences(p)
	if b.quality != "720P 标清" || !b.qualityChosen {
		t.Fatalf("restored quality = %q chosen=%v", b.quality, b.qualityChosen)
	}
}
