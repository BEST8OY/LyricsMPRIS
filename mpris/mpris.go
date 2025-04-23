//go:build linux
// +build linux

package mpris

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
)

// TrackMetadata holds basic song info
type TrackMetadata struct {
	Title  string
	Artist string
	Album  string
}

// getActivePlayer returns the first available MPRIS player name
func getActivePlayer(conn *dbus.Conn) (string, error) {
	var names []string
	err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
	if err != nil {
		return "", err
	}
	for _, name := range names {
		if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			return name, nil
		}
	}
	return "", fmt.Errorf("no MPRIS player found")
}

// GetMetadata tries to fetch metadata from the first available MPRIS player
func GetMetadata(ctx context.Context) (*TrackMetadata, float64, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, 0, err
	}
	defer conn.Close()

	playerName, err := getActivePlayer(conn)
	if err != nil {
		return nil, 0, err
	}

	obj := conn.Object(playerName, "/org/mpris/MediaPlayer2")
	variant, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.Metadata")
	if err != nil {
		return nil, 0, err
	}
	metadata, ok := variant.Value().(map[string]dbus.Variant)
	if !ok {
		return nil, 0, fmt.Errorf("metadata type assertion failed for %s", playerName)
	}
	title, _ := metadata["xesam:title"].Value().(string)
	// If title is empty, try to extract from URL
	if title == "" {
		if u, ok := metadata["xesam:url"].Value().(string); ok {
			parsed, err := url.Parse(u)
			if err == nil {
				title = strings.TrimSuffix(filepath.Base(parsed.Path), filepath.Ext(parsed.Path))
			}
		}
	}
	var artist string
	if arr, ok := metadata["xesam:artist"].Value().([]string); ok && len(arr) > 0 {
		artist = arr[0]
	} else if arr, ok := metadata["xesam:artist"].Value().([]interface{}); ok && len(arr) > 0 {
		if s, ok := arr[0].(string); ok {
			artist = s
		}
	} else if s, ok := metadata["xesam:artist"].Value().(string); ok {
		artist = s
	}
	album, _ := metadata["xesam:album"].Value().(string)
	lengthMicros := uint64(0)
	if v, ok := metadata["mpris:length"]; ok {
		lengthMicros, _ = v.Value().(uint64)
	}
	duration := float64(lengthMicros) / 1e6 // convert microseconds to seconds
	if title != "" && artist != "" && album != "" && duration > 0 {
		return &TrackMetadata{Title: title, Artist: artist, Album: album}, duration, nil
	}
	return nil, 0, fmt.Errorf("no valid title/artist/album/duration for %s", playerName)
}

// GetPositionAndStatus fetches the current playback position (seconds) and playback status (Playing/Paused)
func GetPositionAndStatus() (float64, string, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return 0, "", err
	}
	defer conn.Close()

	playerName, err := getActivePlayer(conn)
	if err != nil {
		return 0, "", err
	}

	obj := conn.Object(playerName, "/org/mpris/MediaPlayer2")
	posVar, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.Position")
	if err != nil {
		return 0, "", err
	}
	pos, ok := posVar.Value().(int64)
	if !ok {
		return 0, "", fmt.Errorf("position type assertion failed")
	}
	statusVar, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.PlaybackStatus")
	if err != nil {
		return 0, "", err
	}
	status, ok := statusVar.Value().(string)
	if !ok {
		return 0, "", fmt.Errorf("status type assertion failed")
	}
	return float64(pos) / 1e6, status, nil
}
