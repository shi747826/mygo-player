package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Windows has no HEIF decoder in WebView2, so covers arrive as HEIC and must
// be converted before the UI can decode them.
func TestDecodeCoverBytesConvertsHEIC(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("generating a HEIC fixture needs sips")
	}
	dir := t.TempDir()
	png := filepath.Join(dir, "source.png")
	heic := filepath.Join(dir, "cover.heic")
	data, err := os.ReadFile("packaging/app-icon.png")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(png, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = exec.Command("sips", "-s", "format", "heic", png, "--out", heic).Run(); err != nil {
		t.Skipf("cannot build a HEIC fixture: %v", err)
	}
	raw, err := os.ReadFile(heic)
	if err != nil {
		t.Fatal(err)
	}
	// Any converter must return a format the UI can decode.
	converted, err := decodeCoverBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(converted[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("unexpected converted header %q", converted[:8])
	}
}

func TestFFmpegCoverFallback(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("generating a HEIC fixture needs sips")
	}
	dir := t.TempDir()
	png := filepath.Join(dir, "source.png")
	heic := filepath.Join(dir, "cover.heic")
	data, err := os.ReadFile("packaging/app-icon.png")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(png, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = exec.Command("sips", "-s", "format", "heic", png, "--out", heic).Run(); err != nil {
		t.Skipf("cannot build a HEIC fixture: %v", err)
	}
	raw, err := os.ReadFile(heic)
	if err != nil {
		t.Fatal(err)
	}
	ffmpeg := os.Getenv("MYGO_PLAYER_FFMPEG")
	if ffmpeg == "" {
		ffmpeg, _ = exec.LookPath("ffmpeg")
	}
	if ffmpeg == "" {
		t.Skip("ffmpeg is unavailable")
	}
	converted, err := convertCoverWithFFmpeg(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(converted[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("unexpected converted header %q", converted[:8])
	}
}
