package ui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/best8oy/LyricsMPRIS/lyrics"
	"github.com/best8oy/LyricsMPRIS/mpris"
	tea "github.com/charmbracelet/bubbletea"
	gloss "github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
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

// global terminal size state
var termWidth, termHeight int

func init() {
	updateTerminalSize()
	go watchTerminalResize()
}

func updateTerminalSize() {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err == nil {
		termWidth = w
		termHeight = h
	}
}

func watchTerminalResize() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGWINCH)
	for range sigs {
		updateTerminalSize()
	}
}

// DisplayLyricsContext handles lyric fetching and UI display for a given track and position.
func DisplayLyricsContext(ctx context.Context, mode string, meta mpris.TrackMetadata, pos float64) (*lyrics.Lyric, error) {
	lyric, err := lyrics.FetchLyrics(meta.Title, meta.Artist, meta.Album, pos)
	if err != nil || lyric == nil || len(lyric.Lines) == 0 {
		return nil, err
	}
	if mode == "pipe" {
		PipeModeContext(ctx, lyric, pos)
	} else if mode == "bubbletea" {
		BubbleTeaModeContext(ctx, lyric, pos)
	} else {
		ModernModeContext(ctx, lyric, pos)
	}
	return lyric, nil
}

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

	for {
		select {
		case <-ctx.Done():
			fmt.Print(ansiReset)
			return
		default:
		}

		// Always get the latest terminal size
		termWidth := getTerminalWidth()
		termHeight := getTerminalHeight()

		// Clean UI if no lyrics
		if lyric == nil || len(lyric.Lines) == 0 {
			fmt.Print(ansiClear + ansiHome)
			msg := centerText("No lyrics found", termWidth)
			padTop := (termHeight - 1) / 2
			for i := 0; i < padTop; i++ {
				fmt.Println()
			}
			fmt.Printf("%s%s%s%s\n", ansiBold, ansiCyan, msg, ansiReset)
			time.Sleep(200 * time.Millisecond)
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
				fmt.Printf("%s%s%s%s\n", ansiBold, ansiCyan, centered, ansiReset)
			} else if i < cur {
				fmt.Printf("%s%s%s%s%s\n", ansiFaint, ansiItalic, centered, ansiReset, ansiReset)
			} else {
				fmt.Printf("%s\n", centered)
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func BubbleTeaModeContext(ctx context.Context, lyric *lyrics.Lyric, _ float64) {
	if lyric == nil || len(lyric.Lines) == 0 {
		fmt.Println("No lyrics found")
		return
	}
	p := tea.NewProgram(newLyricModel(lyric), tea.WithContext(ctx))
	_ = p.Start()
}

type lyricModel struct {
	lyric *lyrics.Lyric
	cur   int
	w, h  int
}

func newLyricModel(lyric *lyrics.Lyric) *lyricModel {
	return &lyricModel{lyric: lyric, cur: 0, w: 80, h: 24}
}

func (m *lyricModel) Init() tea.Cmd {
	return nil
}

func (m *lyricModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.cur > 0 {
				m.cur--
			}
		case "down":
			if m.cur < len(m.lyric.Lines)-1 {
				m.cur++
			}
		}
	}
	return m, nil
}

func (m *lyricModel) View() string {
	if m.lyric == nil || len(m.lyric.Lines) == 0 {
		return gloss.NewStyle().Align(gloss.Center).Width(m.w).Render("No lyrics found")
	}
	windowSize := 9
	from := m.cur - windowSize/2
	if from < 0 {
		from = 0
	}
	to := from + windowSize
	if to > len(m.lyric.Lines) {
		to = len(m.lyric.Lines)
	}
	lines := make([]string, 0, to-from)
	for i := from; i < to; i++ {
		line := m.lyric.Lines[i].Text
		style := gloss.NewStyle().Width(m.w).Align(gloss.Center)
		if i == m.cur {
			style = style.Bold(true).Foreground(gloss.Color("36"))
		} else if i < m.cur {
			style = style.Faint(true).Italic(true)
		}
		lines = append(lines, style.Render(line))
	}
	return gloss.JoinVertical(gloss.Center, lines...)
}

func getTerminalWidth() int {
	if termWidth < 40 {
		return 80
	}
	return termWidth
}

func getTerminalHeight() int {
	if termHeight < 10 {
		return 24
	}
	return termHeight
}

func centerText(s string, width int) string {
	pad := width - runewidth.StringWidth(s)
	if pad <= 0 {
		return s
	}
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}
