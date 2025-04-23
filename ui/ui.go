// Package ui provides terminal user interfaces for displaying lyrics in pipe and modern modes.
package ui

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/best8oy/LyricsMPRIS/lyrics"
	"github.com/best8oy/LyricsMPRIS/mpris"
	"github.com/best8oy/LyricsMPRIS/pool"
	tea "github.com/charmbracelet/bubbletea"
	gloss "github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// DisplayLyricsContext handles lyric fetching and UI display for a given track and position.
func DisplayLyricsContext(ctx context.Context, mode string, meta mpris.TrackMetadata, pos float64) (*lyrics.Lyric, error) {
	lyric, err := lyrics.FetchLyrics(meta.Title, meta.Artist, meta.Album, pos)
	if err != nil || lyric == nil || len(lyric.Lines) == 0 {
		return nil, err
	}
	if mode == "pipe" {
		PipeModeContext(ctx)
	} else {
		TerminalLyricsContext(ctx)
	}
	return lyric, nil
}

// PipeModeContext prints lyrics line-by-line to stdout for pipe mode.
func PipeModeContext(ctx context.Context) {
	ch := make(chan pool.Update)
	go pool.Listen(ctx, ch, 200*time.Millisecond)
	lastLineIdx := -1
	printed := make(map[int]bool)
	for {
		select {
		case <-ctx.Done():
			return
		case upd := <-ch:
			if upd.Err != nil || len(upd.Lines) == 0 {
				continue
			}
			if upd.Index != lastLineIdx && !printed[upd.Index] {
				fmt.Println(upd.Lines[upd.Index].Text)
				lastLineIdx = upd.Index
				printed[upd.Index] = true
			}
		}
	}
}

// Model is the terminal UI model for displaying lyrics.
type Model struct {
	ch           chan pool.Update
	state        pool.Update
	w, h         int
	styleBefore  gloss.Style
	styleCurrent gloss.Style
	styleAfter   gloss.Style
	hAlignment   gloss.Position
}

func newModel(ch chan pool.Update) *Model {
	m := &Model{ch: ch}
	m.styleBefore = gloss.NewStyle().Faint(true).Italic(true)
	m.styleCurrent = gloss.NewStyle().Bold(true).Foreground(gloss.Color("36"))
	m.styleAfter = gloss.NewStyle()
	m.hAlignment = 0.5 // center
	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(waitForUpdate(m.ch), tea.HideCursor)
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height

	case pool.Update:
		m.state = msg
		if runtime.GOOS == "windows" {
			w, h, err := term.GetSize(int(os.Stdout.Fd()))
			if err == nil {
				m.w, m.h = w, h
			}
		}
		cmd = waitForUpdate(m.ch)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			cmd = tea.Quit
		case "left":
			m.hAlignment -= 0.5
			if m.hAlignment < 0 {
				m.hAlignment = 0
			}
		case "right":
			m.hAlignment += 0.5
			if m.hAlignment > 1 {
				m.hAlignment = 1
			}
		case "up":
			if m.state.Playing && lyrics.Timesynced(m.state.Lines) {
				break
			}
			m.state.Index -= 1
			if m.state.Index < 0 {
				m.state.Index = 0
			}
		case "down":
			if m.state.Playing && lyrics.Timesynced(m.state.Lines) {
				break
			}
			m.state.Index += 1
			if m.state.Index >= len(m.state.Lines) {
				m.state.Index = len(m.state.Lines) - 1
			}
		}
	}
	return m, cmd
}

func (m *Model) View() string {
	if m.w < 1 || m.h < 1 {
		return ""
	}
	if m.state.Err != nil {
		return gloss.PlaceVertical(
			m.h, gloss.Center,
			m.styleCurrent.
				Align(gloss.Center).
				Width(m.w).
				Render(m.state.Err.Error()),
		)
	}
	if len(m.state.Lines) == 0 {
		return ""
	}

	curLine := m.styleCurrent.
		Width(m.w).
		Align(m.hAlignment).
		Render(m.state.Lines[m.state.Index].Text)
	curLines := strings.Split(curLine, "\n")

	curLen := len(curLines)
	beforeLen := (m.h - curLen) / 2
	afterLen := m.h - beforeLen - curLen

	lines := make([]string, beforeLen+curLen+afterLen)

	// fill lines before current
	var filledBefore int
	var beforeIndex = m.state.Index - 1
	for filledBefore < beforeLen {
		index := beforeLen - filledBefore - 1
		if index < 0 || beforeIndex < 0 {
			filledBefore += 1
			continue
		}
		line := m.styleBefore.
			Width(m.w).
			Align(m.hAlignment).
			Render(m.state.Lines[beforeIndex].Text)
		beforeIndex -= 1
		beforeLines := strings.Split(line, "\n")
		for i := len(beforeLines) - 1; i >= 0; i-- {
			lineIndex := index - i
			if lineIndex >= 0 {
				lines[lineIndex] = beforeLines[len(beforeLines)-1-i]
			}
			filledBefore += 1
		}
	}

	// fill current lines
	var curIndex = beforeLen
	for i, line := range curLines {
		index := curIndex + i
		if index >= 0 && index < len(lines) {
			lines[index] = line
		}
	}

	// fill lines after current
	var filledAfter int
	var afterIndex = m.state.Index + 1
	for filledAfter < afterLen {
		index := beforeLen + curLen + filledAfter
		if index >= len(lines) || afterIndex >= len(m.state.Lines) {
			filledAfter += 1
			continue
		}
		line := m.styleAfter.
			Width(m.w).
			Align(m.hAlignment).
			Render(m.state.Lines[afterIndex].Text)
		afterIndex += 1
		afterLines := strings.Split(line, "\n")
		for i, line := range afterLines {
			lineIndex := index + i
			if lineIndex < len(lines) {
				lines[lineIndex] = line
			}
			filledAfter += 1
		}
	}

	return gloss.JoinVertical(m.hAlignment, lines...)
}

func waitForUpdate(ch chan pool.Update) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

// TerminalLyricsUI starts the terminal UI and listens for updates from the pool.
func TerminalLyricsUI(ctx context.Context, pollInterval time.Duration) error {
	ch := make(chan pool.Update)
	go pool.Listen(ctx, ch, pollInterval)
	p := tea.NewProgram(newModel(ch), tea.WithContext(ctx), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// TerminalLyricsContext runs the terminal UI for lyrics display.
func TerminalLyricsContext(ctx context.Context) {
	_ = TerminalLyricsUI(ctx, 200*time.Millisecond)
}
