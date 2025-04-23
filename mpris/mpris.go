//go:build linux
// +build linux

package mpris

import (
	"context"
	"errors"
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

// ListPlayers returns all available MPRIS player names for diagnostics.
func ListPlayers() ([]string, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}
	defer conn.Close()
	var names []string
	err = conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
	if err != nil {
		return nil, fmt.Errorf("failed to list D-Bus names: %w", err)
	}
	var players []string
	for _, name := range names {
		if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			players = append(players, name)
		}
	}
	return players, nil
}

// getActivePlayer returns only playerctld if available, otherwise error.
func getActivePlayer(conn *dbus.Conn) (string, error) {
	var names []string
	err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
	if err != nil {
		return "", fmt.Errorf("failed to list D-Bus names: %w", err)
	}
	for _, name := range names {
		if name == "org.mpris.MediaPlayer2.playerctld" {
			return name, nil
		}
	}
	return "", errors.New("playerctld (org.mpris.MediaPlayer2.playerctld) not found on the session bus")
}

// GetMetadata fetches metadata from the first available MPRIS player
func GetMetadata(ctx context.Context) (*TrackMetadata, float64, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to connect to session bus: %w", err)
	}
	defer conn.Close()

	playerName, err := getActivePlayer(conn)
	if err != nil {
		return nil, 0, err
	}

	obj := conn.Object(playerName, "/org/mpris/MediaPlayer2")
	variant, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.Metadata")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get metadata property: %w", err)
	}
	metadata, ok := variant.Value().(map[string]dbus.Variant)
	if !ok {
		return nil, 0, fmt.Errorf("metadata type assertion failed for %s", playerName)
	}
	title := getString(metadata, "xesam:title")
	if title == "" {
		if u := getString(metadata, "xesam:url"); u != "" {
			parsed, err := url.Parse(u)
			if err == nil {
				title = strings.TrimSuffix(filepath.Base(parsed.Path), filepath.Ext(parsed.Path))
			}
		}
	}
	artist := getFirstString(metadata, "xesam:artist")
	album := getString(metadata, "xesam:album")
	lengthMicros := getUint64(metadata, "mpris:length")
	duration := float64(lengthMicros) / 1e6 // microseconds to seconds
	if title != "" && artist != "" && album != "" && duration > 0 {
		return &TrackMetadata{Title: title, Artist: artist, Album: album}, duration, nil
	}
	return nil, 0, fmt.Errorf("no valid title/artist/album/duration for %s", playerName)
}

// GetPositionAndStatus fetches the current playback position (seconds) and playback status (Playing/Paused)
func GetPositionAndStatus(ctx context.Context) (float64, string, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return 0, "", fmt.Errorf("failed to connect to session bus: %w", err)
	}
	defer conn.Close()

	playerName, err := getActivePlayer(conn)
	if err != nil {
		return 0, "", err
	}

	obj := conn.Object(playerName, "/org/mpris/MediaPlayer2")
	posVar, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.Position")
	if err != nil {
		return 0, "", fmt.Errorf("failed to get position property: %w", err)
	}
	pos, ok := posVar.Value().(int64)
	if !ok {
		return 0, "", fmt.Errorf("position type assertion failed")
	}
	statusVar, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.PlaybackStatus")
	if err != nil {
		return 0, "", fmt.Errorf("failed to get playback status property: %w", err)
	}
	status, ok := statusVar.Value().(string)
	if !ok {
		return 0, "", fmt.Errorf("status type assertion failed")
	}
	return float64(pos) / 1e6, status, nil
}

// WatchAndHandleEvents listens for MPRIS property changes and invokes the callback on track/position changes.
func WatchAndHandleEvents(ctx context.Context, onTrackChange func(meta TrackMetadata, pos float64), onSeek func(meta TrackMetadata, pos float64)) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	err = conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	)
	if err != nil {
		return err
	}

	signalCh := make(chan *dbus.Signal, 10)
	conn.Signal(signalCh)

	var lastTrack TrackMetadata

	// Initial fetch
	meta, pos, err := GetMetadata(ctx)
	if err == nil {
		lastTrack = *meta
		onTrackChange(*meta, pos)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case sig := <-signalCh:
			if sig == nil || len(sig.Body) < 2 {
				continue
			}
			iface, ok := sig.Body[0].(string)
			if !ok || iface != "org.mpris.MediaPlayer2.Player" {
				continue
			}
			changed, ok := sig.Body[1].(map[string]dbus.Variant)
			if !ok {
				continue
			}
			if _, ok := changed["Metadata"]; ok {
				meta, pos, err := GetMetadata(ctx)
				if err == nil && (meta.Title != lastTrack.Title || meta.Artist != lastTrack.Artist || meta.Album != lastTrack.Album) {
					lastTrack = *meta
					onTrackChange(*meta, pos)
				}
			}
			if _, ok := changed["Position"]; ok {
				posVar := changed["Position"]
				pos, _ := posVar.Value().(int64)
				sec := float64(pos) / 1e6
				onSeek(lastTrack, sec)
			}
		}
	}
}

// getString safely extracts a string from metadata
func getString(metadata map[string]dbus.Variant, key string) string {
	if v, ok := metadata[key]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

// getFirstString extracts the first string from a string array or interface array
func getFirstString(metadata map[string]dbus.Variant, key string) string {
	if v, ok := metadata[key]; ok {
		switch val := v.Value().(type) {
		case []string:
			if len(val) > 0 {
				return val[0]
			}
		case []interface{}:
			if len(val) > 0 {
				if s, ok := val[0].(string); ok {
					return s
				}
			}
		case string:
			return val
		}
	}
	return ""
}

// getUint64 safely extracts a uint64 from metadata
func getUint64(metadata map[string]dbus.Variant, key string) uint64 {
	if v, ok := metadata[key]; ok {
		switch val := v.Value().(type) {
		case uint64:
			return val
		case int64:
			if val >= 0 {
				return uint64(val)
			}
		}
	}
	return 0
}
