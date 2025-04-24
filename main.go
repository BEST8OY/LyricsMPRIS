package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/best8oy/LyricsMPRIS/pool"
	"github.com/best8oy/LyricsMPRIS/ui"
)

// Config holds application settings.
type Config struct {
	displayMode    string
	pollIntervalMs int
}

func main() {
	pipe := flag.Bool("pipe", false, "Pipe current lyric line to stdout (default is modern UI)")
	pollMs := flag.Int("poll", 1000, "Lyric poll interval in milliseconds")
	flag.Parse()

	cfg := Config{
		displayMode:    "modern",
		pollIntervalMs: *pollMs,
	}
	if *pipe {
		cfg.displayMode = "pipe"
	}

	pollInterval := time.Duration(cfg.pollIntervalMs) * time.Millisecond
	updateCh := make(chan pool.Update, 10)

	if cfg.displayMode == "pipe" {
		go pool.Listen(context.Background(), updateCh, pollInterval)
		ui.PipeModeContext(context.Background(), pollInterval)
		return
	}

	go pool.Listen(context.Background(), updateCh, pollInterval)
	err := ui.TerminalLyricsContextWithChannel(context.Background(), updateCh)
	if err != nil {
		os.Exit(1)
	}
}
