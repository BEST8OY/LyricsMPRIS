package ui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/best8oy/LyricsMPRIS/lyrics"
	"github.com/best8oy/LyricsMPRIS/mpris"
)

// ANSI escape codes for styling (sptlrx style)
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiFaint  = "\033[2m"
	ansiItalic = "\033[3m"
	ansiCyan   = "\033[36m"
	ansiClear  = "\033[2J"
	ansiHome   = "\033[H"
)

func PipeModeContext(ctx context.Context, lyric *lyrics.Lyric, _ float64) {
	lastLineIdx := -1
	printed := make(map[int]bool)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		pos, status, err := mpris.GetPositionAndStatus(ctx)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if status != "Playing" {
			// Wait until playback resumes, checking every 1s for efficiency
			for status != "Playing" {
				select {
				case <-ctx.Done():
					return
				default:
				}
				time.Sleep(1 * time.Second)
				_, status, err = mpris.GetPositionAndStatus(ctx)
				if err != nil {
					time.Sleep(1 * time.Second)
				}
			}
			continue
		}
		lineIdx := -1
		for i, line := range lyric.Lines {
			if pos < line.Time {
				break
			}
			lineIdx = i
		}
		if lineIdx != -1 && lineIdx != lastLineIdx && !printed[lineIdx] {
			fmt.Println(lyric.Lines[lineIdx].Text)
			lastLineIdx = lineIdx
			printed[lineIdx] = true
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func ModernModeContext(ctx context.Context, lyric *lyrics.Lyric, _ float64) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Print(ansiReset)
		os.Exit(0)
	}()

	windowSize := 9 // 4 before, current, 4 after
	lastCurrent := -1

	for {
		select {
		case <-ctx.Done():
			fmt.Print(ansiReset)
			return
		default:
		}

		// Clean UI if no lyrics
		if lyric == nil || len(lyric.Lines) == 0 {
			termWidth := getTerminalWidth()
			termHeight := getTerminalHeight()
			fmt.Print(ansiClear + ansiHome)
			msg := centerText("No lyrics found", termWidth)
			padTop := (termHeight - 1) / 2
			for i := 0; i < padTop; i++ {
				fmt.Println()
			}
			fmt.Printf("%s%s%s%s\n", ansiBold, ansiCyan, msg, ansiReset)
			time.Sleep(1 * time.Second)
			continue
		}

		pos, status, err := mpris.GetPositionAndStatus(ctx)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if status != "Playing" {
			for status != "Playing" {
				select {
				case <-ctx.Done():
					return
				default:
				}
				time.Sleep(1 * time.Second)
				_, status, err = mpris.GetPositionAndStatus(ctx)
				if err != nil {
					time.Sleep(1 * time.Second)
				}
			}
			continue
		}
		cur := 0
		for i, line := range lyric.Lines {
			if pos < line.Time {
				break
			}
			cur = i
		}
		if cur != lastCurrent {
			termWidth := getTerminalWidth()
			termHeight := getTerminalHeight()
			fmt.Print(ansiClear + ansiHome)
			from := cur - windowSize/2
			if from < 0 {
				from = 0
			}
			to := from + windowSize
			if to > len(lyric.Lines) {
				to = len(lyric.Lines)
			}
			linesOnScreen := to - from
			padTop := (termHeight - linesOnScreen) / 2
			if padTop < 0 {
				padTop = 0
			}
			for i := 0; i < padTop; i++ {
				fmt.Println()
			}
			for i := from; i < to; i++ {
				line := lyric.Lines[i].Text
				centered := centerText(line, termWidth)
				if i == cur {
					// sptlrx: current line is cyan and bold, no markers
					fmt.Printf("%s%s%s%s\n", ansiBold, ansiCyan, centered, ansiReset)
				} else if i < cur {
					// sptlrx: previous lines are faint/italic
					fmt.Printf("%s%s%s%s%s\n", ansiFaint, ansiItalic, centered, ansiReset, ansiReset)
				} else {
					// sptlrx: next lines are normal
					fmt.Printf("%s\n", centered)
				}
			}
			lastCurrent = cur
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func getTerminalWidth() int {
	ws, err := getWinsize()
	if err != nil || ws.Col < 40 {
		return 80
	}
	return int(ws.Col)
}

func getTerminalHeight() int {
	ws, err := getWinsize()
	if err != nil || ws.Row < 10 {
		return 24
	}
	return int(ws.Row)
}

type winsize struct {
	Row uint16
	Col uint16
	x   uint16
	y   uint16
}

func getWinsize() (*winsize, error) {
	ws := &winsize{}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if err != 0 {
		return nil, err
	}
	return ws, nil
}

func centerText(s string, width int) string {
	runes := []rune(s)
	pad := width - len(runes)
	if pad <= 0 {
		return s
	}
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}
