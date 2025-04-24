package main

import (
	"context"
	"flag"
	"time"

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
	pollMs := flag.Int("poll", 2000, "Lyric poll interval in milliseconds")
	flag.Parse()

	cfg := Config{
		displayMode:    "modern",
		pollIntervalMs: *pollMs,
	}
	if *pipe {
		cfg.displayMode = "pipe"
	}

	pollInterval := time.Duration(cfg.pollIntervalMs) * time.Millisecond

	// Fetch metadata and position from MPRIS
	ctx := context.Background()
	meta, _, err := mpris.GetMetadata(ctx)
	if err != nil || meta == nil {
		// If no track is playing, wait for the next track (handled by UI/pool)
		meta = &mpris.TrackMetadata{}
	}
	pos, _, err := mpris.GetPositionAndStatus(ctx)
	if err != nil {
		pos = 0.0
	}

	if cfg.displayMode == "pipe" {
		ui.DisplayLyricsContext(ctx, "pipe", *meta, pos, pollInterval)
		return
	}

	ui.DisplayLyricsContext(ctx, "modern", *meta, pos, pollInterval)
}
