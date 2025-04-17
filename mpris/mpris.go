package mpris

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// TrackMetadata holds basic song info
type TrackMetadata struct {
	Title  string
	Artist string
	Album  string
}

// GetMetadata tries to fetch metadata from the MPRIS player, including duration
func GetMetadata(ctx context.Context) (*TrackMetadata, float64, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, 0, err
	}
	defer conn.Close()

	obj := conn.Object("org.mpris.MediaPlayer2.playerctld", "/org/mpris/MediaPlayer2")
	variant, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.Metadata")
	if err != nil {
		return nil, 0, err
	}
	metadata, ok := variant.Value().(map[string]dbus.Variant)
	if !ok {
		return nil, 0, fmt.Errorf("metadata type assertion failed for %s", "org.mpris.MediaPlayer2.playerctld")
	}
	title, _ := metadata["xesam:title"].Value().(string)
	var artist string
	if arr, ok := metadata["xesam:artist"].Value().([]string); ok && len(arr) > 0 {
		artist = arr[0]
	} else if arr, ok := metadata["xesam:artist"].Value().([]interface{}); ok && len(arr) > 0 {
		if s, ok := arr[0].(string); ok {
			artist = s
		}
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
	return nil, 0, fmt.Errorf("no valid title/artist/album/duration for %s", "org.mpris.MediaPlayer2.playerctld")
}

// GetPositionAndStatus fetches the current playback position (seconds) and playback status (Playing/Paused)
func GetPositionAndStatus() (float64, string, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return 0, "", err
	}
	defer conn.Close()

	obj := conn.Object("org.mpris.MediaPlayer2.playerctld", "/org/mpris/MediaPlayer2")
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
