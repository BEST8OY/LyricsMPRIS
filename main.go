package main

import (
	"context"
	"flag"
	"sync"

	"github.com/best8oy/LyricsMPRIS/lyrics"
	"github.com/best8oy/LyricsMPRIS/mpris"
	"github.com/best8oy/LyricsMPRIS/ui"
)

func main() {
	mode := flag.String("mode", "modern", "Mode: 'pipe' for piping current lyric line, 'modern' for modern terminal UI")
	flag.Parse()

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
			lyric, _ := ui.DisplayLyricsContext(ctx, *mode, meta, pos)
			mu.Lock()
			lastLyric = lyric
			mu.Unlock()
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
			if *mode == "pipe" {
				ui.PipeModeContext(ctx, lyric, pos)
			} else {
				ui.ModernModeContext(ctx, lyric, pos)
			}
		}()
	}

	ctx := context.Background()
	_ = mpris.WatchAndHandleEvents(ctx, handleTrack, handleSeek)
}
