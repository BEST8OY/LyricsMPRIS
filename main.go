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

	// Fetch metadata and position from MPRIS (placeholder, replace with actual implementation)
	meta := /* fetch metadata */ mpris.TrackMetadata{}
	pos := 0.0 // fetch current position if needed

	if cfg.displayMode == "pipe" {
		ui.DisplayLyricsContext(context.Background(), "pipe", meta, pos, pollInterval)
		return
	}

	ui.DisplayLyricsContext(context.Background(), "modern", meta, pos, pollInterval)
}
