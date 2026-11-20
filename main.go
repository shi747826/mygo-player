package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/plugins/automation"
	"github.com/egoist/mygo/ui"
	"mygo-player/internal/catalog"
	"mygo-player/internal/plugins"
)

func main() {
	dir := flag.String("data", "", "data directory (default: user config/mygo-player)")
	ffmpeg := flag.String("ffmpeg", "", "FFmpeg executable; also MYGO_PLAYER_FFMPEG")
	sourceID := flag.String("source", "", "source plugin ID to select on launch")
	query := flag.String("query", "", "search on launch")
	smoke := flag.Bool("smoke", false, "play first search result; verify decoded playback and seek, then exit")
	media := flag.String("media", "", "open a local video for player validation")
	smokeNext := flag.Bool("smoke-next", false, "also validate automatic next episode (requires -smoke)")
	flag.Parse()
	mygo.Updater.SetSource(plugins.PlayerReleaseSource("https://github.com/shi747826/mygo-player"))
	if *dir == "" {
		base, e := os.UserConfigDir()
		if e != nil {
			log.Fatal(e)
		}
		*dir = filepath.Join(base, "mygo-player")
	}
	client, err := catalog.New(*dir)
	if err != nil {
		log.Fatal(err)
	}
	path := *ffmpeg
	if path == "" {
		if exe, e := os.Executable(); e == nil {
			name := "ffmpeg"
			if runtime.GOOS == "windows" {
				name = "ffmpeg.exe"
			}
			p := filepath.Join(filepath.Dir(exe), name)
			if _, e = os.Stat(p); e == nil {
				path = p
			}
		}
	}
	if path != "" {
		_ = os.Setenv("MYGO_PLAYER_FFMPEG", path)
	}
	app := newApp(client, nil)
	app.dataDir = *dir
	pm, err := plugins.LoadManager(*dir)
	if err != nil {
		log.Fatal(err)
	}
	defer pm.Close()
	app.pluginManager = pm
	app.pluginClients = map[string]*plugins.Client{}
	for _, m := range pm.Manifests() {
		if pm.Enabled(m.ID) {
			source, err := pm.Open(app.ctx, m.ID)
			if err != nil {
				log.Printf("start plugin %s: %v", m.ID, err)
				app.pluginMessage = "插件启动失败：" + m.Name + " · " + err.Error()
				continue
			}
			app.pluginClients[m.ID] = source
		}
	}
	for _, m := range pm.Manifests() {
		if app.pluginClients[m.ID] != nil && (*sourceID == "" || m.ID == *sourceID) {
			app.source = m.ID
			break
		}
	}
	if *sourceID != "" && app.source == "" {
		log.Fatalf("source plugin %q is unavailable or disabled", *sourceID)
	}
	if err := app.ensureSourceStore(app.source); err != nil {
		log.Fatal(err)
	}
	app.refreshStore()
	app.applyPreferences(loadPreferences(*dir))
	app.smoke = *smoke
	app.smokeNext = *smokeNext
	mygo.App.WhenReady(func() {
		app.win = mygo.NewWindow(mygo.WindowOptions{Title: "MyGo Player", TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 40, TrafficLightPosition: &mygo.Point{X: 14, Y: 12}, Width: 1280, Height: 820, MinWidth: 900, MinHeight: 620, Content: ui.View(app.view)})
		app.win.SetWindowButtonsTheme(mygo.ThemeLight)
		app.win.OnClose(func(_ *mygo.CloseEvent) { app.shutdown() })
		app.win.OnClosed(func() { mygo.App.Quit() })
		mygo.App.Focus()
		app.win.Focus()
		app.restoreAccount()
		if *media != "" {
			app.openLocal(*media)
		} else if *query != "" {
			app.query = *query
			app.loadPage("搜索", false)
		} else if app.source == "" {
			app.loadPage("插件", false)
		} else {
			app.loadPage("推荐", false)
		}
		go app.checkPlayerUpdateOnLaunch()
		if *smoke {
			go func() {
				timer := time.NewTimer(300 * time.Second)
				defer timer.Stop()
				select {
				case <-app.ctx.Done():
					return
				case <-timer.C:
					app.win.Update(func() { app.failSmoke("timed out") })
				}
			}()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
	app.shutdown()
	if *smoke {
		if !app.smokePassed {
			fmt.Fprintln(os.Stderr, "SMOKE FAILED")
			os.Exit(1)
		}
		fmt.Println("SMOKE PASS: WebView video decoded, playback advanced, seek acknowledged")
	}
}
