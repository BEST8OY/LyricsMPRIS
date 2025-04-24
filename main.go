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

	ctx := context.Background()
	// Always start the UI, even if no song is playing yet
	meta := &mpris.TrackMetadata{}
	pos := 0.0
	// Try to get current metadata/position, but ignore errors and let UI handle waiting
	if m, _, err := mpris.GetMetadata(ctx); err == nil && m != nil {
		meta = m
	}
	if p, _, err := mpris.GetPositionAndStatus(ctx); err == nil {
		pos = p
	}

	if cfg.displayMode == "pipe" {
		ui.DisplayLyricsContext(ctx, "pipe", *meta, pos, pollInterval)
		return
	}

	ui.DisplayLyricsContext(ctx, "modern", *meta, pos, pollInterval)
}
