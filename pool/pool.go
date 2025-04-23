package pool

import (
	"context"
	"time"

	"github.com/best8oy/LyricsMPRIS/lyrics"
	"github.com/best8oy/LyricsMPRIS/mpris"
)

// Update represents the state of the lyrics and player.
type Update struct {
	Lines   []lyrics.LyricLine
	Index   int
	Playing bool
	Err     error
}

type playerState struct {
	Title    string
	Artist   string
	Album    string
	Playing  bool
	Position float64
	Err      error
}

// Listen polls for player and lyrics updates and writes them to the channel.
func Listen(ctx context.Context, ch chan Update, pollInterval time.Duration) {
	stateCh := make(chan playerState)
	go listenPlayer(ctx, stateCh, pollInterval)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var (
		state      playerState
		index      int
		lines      []lyrics.LyricLine
		lastUpdate time.Time
	)

	for {
		changed := false

		select {
		case <-ctx.Done():
			return
		case newState := <-stateCh:
			lastUpdate = time.Now()
			if newState.Title != state.Title || newState.Artist != state.Artist || newState.Album != state.Album {
				changed = true
				if newState.Title != "" && newState.Artist != "" {
					lyric, err := lyrics.FetchLyrics(newState.Title, newState.Artist, newState.Album, newState.Position)
					if err != nil {
						state.Err = err
						lines = nil
					} else if lyric != nil {
						lines = lyric.Lines
						state.Err = nil
					}
				} else {
					lines = nil
				}
				index = 0
			}
			if newState.Playing != state.Playing {
				changed = true
			}
			state = newState
		case <-ticker.C:
			if !state.Playing || len(lines) == 0 {
				break
			}
			now := time.Now()
			state.Position += now.Sub(lastUpdate).Seconds()
			lastUpdate = now
		}

		newIndex := getIndex(state.Position, index, lines)
		if newIndex != index {
			changed = true
			index = newIndex
		}

		if changed {
			ch <- Update{
				Lines:   lines,
				Index:   index,
				Playing: state.Playing,
				Err:     state.Err,
			}
		}
	}
}

func listenPlayer(ctx context.Context, ch chan playerState, interval time.Duration) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		meta, _, err := mpris.GetMetadata(ctx)
		pos, status, err2 := mpris.GetPositionAndStatus(ctx)
		st := playerState{Err: err}
		if err == nil && meta != nil && err2 == nil {
			st.Title = meta.Title
			st.Artist = meta.Artist
			st.Album = meta.Album
			st.Playing = status == "Playing"
			st.Position = pos
		}
		ch <- st
		time.Sleep(interval)
	}
}

// getIndex returns the index of the current lyric line based on position.
func getIndex(position float64, curIndex int, lines []lyrics.LyricLine) int {
	if len(lines) <= 1 {
		return 0
	}
	if position >= lines[curIndex].Time {
		for i := curIndex + 1; i < len(lines); i++ {
			if position < lines[i].Time {
				return i - 1
			}
		}
		return len(lines) - 1
	}
	for i := curIndex; i > 0; i-- {
		if position > lines[i].Time {
			return i
		}
	}
	return 0
}
