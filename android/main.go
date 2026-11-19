package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"mygo-player/android/internal/core"
	"mygo-player/android/native/localplayback"
)

//go:embed frontend
var assets embed.FS

type Mobile struct{}

// PreparePlayback configures the trusted Android WebView for local media. It is
// called from the loaded app page, so Android already knows the current origin.
func (Mobile) PreparePlayback() (bool, error) {
	if runtime.GOOS != "android" {
		return true, nil
	}
	n, err := localplayback.EnableLocalPlayback()
	return n > 0, err
}

// SetVideoFullscreen rotates the existing WebView without replacing its player.
func (Mobile) SetVideoFullscreen(enabled bool) {
	orientation := mygo.OrientationAny
	if enabled {
		orientation = mygo.OrientationLandscape
	}
	mygo.SetOrientations(orientation)
	mygo.SystemBars.SetHidden(enabled)
}

func main() {
	data := flag.String("data", "", "override private data directory for development")
	flag.Parse()
	mygo.App.SetName("MyGo Player")
	mygo.Bind(Mobile{})
	var service *core.Service
	mygo.App.WhenReady(func() {
		dir := *data
		if dir == "" {
			var err error
			dir, err = mygo.App.Path(mygo.PathUserData)
			if err != nil {
				log.Print(err)
				mygo.App.Quit()
				return
			}
		}
		var err error
		service, err = core.New(dir)
		if err != nil {
			log.Printf("Unable to open mobile data: %v", err)
			mygo.App.Quit()
			return
		}
		if runtime.GOOS == "android" {
			// Android can decode HEIF covers that the Go side cannot.
			service.SetCoverConverter(localplayback.ConvertCover)
		}
		frontend, _ := fs.Sub(assets, "frontend")
		index, err := fs.ReadFile(frontend, "index.html")
		if err != nil {
			log.Printf("Unable to load frontend: %v", err)
			mygo.App.Quit()
			return
		}
		token, _ := json.Marshal(map[string]string{"token": service.Token})
		injection := []byte("<script>window.__MYGO_MOBILE__=" + string(token) + ";</script>")
		index = bytes.Replace(index, []byte("<head>"), append([]byte("<head>"), injection...), 1)
		files := http.FileServer(http.FS(frontend))
		if err := mygo.Protocol.HandleFunc("mygo", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				service.ServeHTTP(w, r)
				return
			}
			w.Header().Set("X-Content-Type-Options", "nosniff")
			if r.URL.Path == "/" || r.URL.Path == "/index.html" {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("Cache-Control", "no-store")
				_, _ = w.Write(index)
				return
			}
			files.ServeHTTP(w, r)
		}); err != nil {
			log.Print(err)
			mygo.App.Quit()
			return
		}
		// The Android WebView leaves room for the display cutout. Paint that
		// native area black so it blends with landscape video letterboxing.
		win := mygo.NewWindow(mygo.WindowOptions{Title: "MyGo Player", Width: 390, Height: 844, MinWidth: 320, MinHeight: 480, URL: "/", BackgroundColor: "#000000"})
		win.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
			// Keep remote pages out of the app WebView and its native bindings.
			if !strings.HasPrefix(e.URL, "mygo://localhost/") && !strings.HasPrefix(e.URL, "https://mygo.localhost/") {
				e.PreventDefault()
			}
		})
		mygo.Power.OnSuspend(func() { go func() { _, _ = win.Page().Eval(`window.dispatchEvent(new Event('mygo:suspend'))`) }() })
		win.OnClosed(func() { service.Close() })
	})
	mygo.App.OnWindowAllClosed(func() { mygo.App.Quit() })
	if err := mygo.App.Run(); err != nil {
		fmt.Println(err)
	}
	if service != nil {
		service.Close()
	}
}
