package main

import (
	"context"
	"flag"
	"fmt"
	"sync"

	"github.com/best8oy/LyricsMPRIS/logutil"
	"github.com/best8oy/LyricsMPRIS/lyrics"
	"github.com/best8oy/LyricsMPRIS/mpris"
	"github.com/best8oy/LyricsMPRIS/ui"
	"github.com/godbus/dbus/v5"
)

var verbose bool

func watchMPRISAndDisplayLyrics(mode string) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		fmt.Println("Failed to connect to session bus:", err)
		return
	}
	defer conn.Close()

	// Add a match rule for PropertiesChanged
	err = conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	)
	if err != nil {
		fmt.Println("Failed to add match signal:", err)
		return
	}

	signalCh := make(chan *dbus.Signal, 10)
	conn.Signal(signalCh)

	var lastTrack mpris.TrackMetadata
	var lastLyric *lyrics.Lyric
	var cancel context.CancelFunc
	var wg sync.WaitGroup

	fetchAndDisplay := func(meta mpris.TrackMetadata, pos float64) {
		if cancel != nil {
			cancel()
			wg.Wait()
		}
		ctx, c := context.WithCancel(context.Background())
		cancel = c
		wg.Add(1)
		go func() {
			defer wg.Done()
			lyric, err := lyrics.FetchLyrics(meta.Title, meta.Artist, meta.Album, pos)
			if err != nil || lyric == nil || len(lyric.Lines) == 0 {
				return
			}
			lastLyric = lyric
			if mode == "pipe" {
				ui.PipeModeContext(ctx, lyric, pos)
			} else {
				ui.ModernModeContext(ctx, lyric, pos)
			}
		}()
	}

	// Initial fetch
	meta, pos, err := mpris.GetMetadata(context.Background())
	if err == nil {
		lastTrack = *meta
		fetchAndDisplay(*meta, pos)
	}

	for {
		select {
		case sig := <-signalCh:
			if sig == nil || len(sig.Body) < 2 {
				continue
			}
			iface, ok := sig.Body[0].(string)
			if !ok || iface != "org.mpris.MediaPlayer2.Player" {
				continue
			}
			changed, ok := sig.Body[1].(map[string]dbus.Variant)
			if !ok {
				continue
			}
			// Track change
			if _, ok := changed["Metadata"]; ok {
				meta, pos, err := mpris.GetMetadata(context.Background())
				if err == nil && (meta.Title != lastTrack.Title || meta.Artist != lastTrack.Artist || meta.Album != lastTrack.Album) {
					lastTrack = *meta
					fetchAndDisplay(*meta, pos)
				}
			}
			// Seek/position change
			if _, ok := changed["Position"]; ok {
				posVar := changed["Position"]
				pos, _ := posVar.Value().(int64)
				sec := float64(pos) / 1e6
				if lastLyric != nil {
					fetchAndDisplay(lastTrack, sec)
				}
			}
		}
	}
}

func main() {
	mode := flag.String("mode", "modern", "Mode: 'pipe' for piping current lyric line, 'modern' for modern terminal UI")
	flag.BoolVar(&logutil.Verbose, "verbose", false, "Enable verbose logging")
	flag.Parse()

	watchMPRISAndDisplayLyrics(*mode)
}
