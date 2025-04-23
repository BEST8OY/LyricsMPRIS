package main

import (
	"context"
	"flag"
	"sync"

	"github.com/best8oy/LyricsMPRIS/lyrics"
	"github.com/best8oy/LyricsMPRIS/mpris"
	"github.com/best8oy/LyricsMPRIS/ui"
)

// Config holds application settings.
type Config struct {
	displayMode    string
	pollIntervalMs int
}

func main() {
	pipe := flag.Bool("pipe", false, "Pipe current lyric line to stdout (default is modern UI)")
	pollMs := flag.Int("poll", 200, "Lyric poll interval in milliseconds")
	flag.Parse()

	cfg := Config{
		displayMode:    "modern",
		pollIntervalMs: *pollMs,
	}
	if *pipe {
		cfg.displayMode = "pipe"
	}

	var mu sync.Mutex
	var cancel context.CancelFunc
	var wg sync.WaitGroup
	var lastLyric *lyrics.Lyric

	handleTrack := func(meta mpris.TrackMetadata, pos float64) {
		mu.Lock()
		if cancel != nil {
			cancel()
			wg.Wait()
		}
		ctx, c := context.WithCancel(context.Background())
		cancel = c
		wg.Add(1)
		mu.Unlock()
		go func() {
			defer wg.Done()
			lyric, _ := lyrics.FetchLyrics(meta.Title, meta.Artist, meta.Album, pos)
			mu.Lock()
			lastLyric = lyric
			mu.Unlock()
			if lyric == nil || len(lyric.Lines) == 0 {
				return
			}
			if cfg.displayMode == "pipe" {
				ui.PipeModeContext(ctx)
			} else {
				ui.TerminalLyricsContext(ctx)
			}
		}()
	}

	handleSeek := func(meta mpris.TrackMetadata, pos float64) {
		mu.Lock()
		lyric := lastLyric
		mu.Unlock()
		if lyric == nil {
			return
		}
		mu.Lock()
		if cancel != nil {
			cancel()
			wg.Wait()
		}
		ctx, c := context.WithCancel(context.Background())
		cancel = c
		wg.Add(1)
		mu.Unlock()
		go func() {
			defer wg.Done()
			if cfg.displayMode == "pipe" {
				ui.PipeModeContext(ctx)
			} else {
				ui.TerminalLyricsContext(ctx)
			}
		}()
	}

	ctx := context.Background()
	_ = mpris.WatchAndHandleEvents(ctx, handleTrack, handleSeek)
}
