package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/egoist/mygo/ui"
)

// cover keeps a small in-memory cache. Metadata and playback never wait for art.
func (a *app) cover(raw string) *ui.Bitmap {
	if raw == "" || a.closed {
		return nil
	}
	if b, ok := a.covers[raw]; ok {
		a.touchCover(raw)
		return b
	}
	if a.coverPending[raw] || len(a.coverPending) >= 160 || a.win == nil {
		return nil
	}
	u, e := url.Parse(raw)
	if e != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil
	}
	a.coverPending[raw] = true
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
		defer cancel()
		select {
		case a.coverSlots <- struct{}{}:
			defer func() { <-a.coverSlots }()
		case <-ctx.Done():
			a.win.Update(func() { delete(a.coverPending, raw) })
			return
		}
		var data []byte
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if e == nil {
			resp, e := http.DefaultClient.Do(req)
			if e == nil {
				if resp.StatusCode == 200 {
					data, _ = io.ReadAll(io.LimitReader(resp.Body, 4<<20))
				}
				resp.Body.Close()
			}
		}
		if dec, err := decodeCoverBytes(data); err == nil {
			data = dec
		}
		a.win.Update(func() {
			if a.closed {
				return
			}
			delete(a.coverPending, raw)
			a.trimCovers()
			var b *ui.Bitmap
			if len(data) > 0 {
				b, _ = ui.DecodeBitmap(data)
			}
			a.covers[raw] = b
			a.touchCover(raw)
		})
	}()
	return nil
}

// decodeCoverBytes converts covers the Go image decoders cannot read
// (notably HEIC, which the CDN serves) into a portable format it can.
// On macOS it uses the system sips tool; elsewhere it returns the data
// unchanged so a generally-supported format still loads, and an
// unsupported one falls back to the placeholder text.
func decodeCoverBytes(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	// Most responses are JPEG/PNG/WebP/GIF; quick-sniff before shelling out.
	kind := http.DetectContentType(data)
	switch kind {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return data, nil
	}
	// HEIC/HEIF are ISO-BMFF: bytes 4..8 spell "ftyp".
	heic := len(data) >= 12 && string(data[4:8]) == "ftyp"
	if !heic && kind != "image/heic" && kind != "image/heif" {
		return data, nil
	}
	if runtime.GOOS != "darwin" {
		return nil, errUnsupportedImage
	}
	sips, err := exec.LookPath("sips")
	if err != nil {
		return nil, errUnsupportedImage
	}
	dir, err := os.MkdirTemp("", "hgcover-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	in := filepath.Join(dir, "cover.heic")
	out := filepath.Join(dir, "cover.png")
	if err := os.WriteFile(in, data, 0600); err != nil {
		return nil, err
	}
	// No user input reaches sips; the fixed -s format png plus temp paths only.
	cmd := exec.Command(sips, "-s", "format", "png", in, "--out", out)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	converted, err := os.ReadFile(out)
	if err != nil {
		return nil, err
	}
	return converted, nil
}

var errUnsupportedImage = &coverError{"unsupported image format"}

type coverError struct{ msg string }

func (e *coverError) Error() string { return e.msg }

// coverCacheLimit 是内存里保留的封面张数上限。
const coverCacheLimit = 600

// trimCovers 超过上限时按最近使用时间淘汰最旧的四分之一。
//
// 以前是整表清空，会让大量卡片同时退回占位图再重新加载，看起来像整页刷新；
// 现在只腾出需要的空间，并且正在看的卡片因为刚被访问过不会被选中。
func (a *app) trimCovers() {
	if len(a.covers) < coverCacheLimit {
		return
	}
	target := coverCacheLimit - coverCacheLimit/4
	type entry struct {
		raw  string
		used int64
	}
	entries := make([]entry, 0, len(a.covers))
	for raw := range a.covers {
		entries = append(entries, entry{raw, a.coverUsed[raw]})
	}
	// 按使用时间从旧到新，删到目标数量为止。
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].used < entries[j-1].used; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
	drop := len(entries) - target
	for i := 0; i < drop; i++ {
		delete(a.covers, entries[i].raw)
		delete(a.coverUsed, entries[i].raw)
	}
}

// touchCover 记下这张封面最近一次被使用的时间，供淘汰时区分新旧。
func (a *app) touchCover(raw string) {
	a.coverSeq++
	a.coverUsed[raw] = a.coverSeq
}
