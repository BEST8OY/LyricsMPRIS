package lyrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// LyricLine represents a single line of synced lyrics with its timestamp in seconds.
type LyricLine struct {
	Time float64
	Text string
}

// Lyric holds all parsed lyric lines.
type Lyric struct {
	Lines []LyricLine
}

// lrclibAPIResponse models the response from lrclib.net API.
type lrclibAPIResponse struct {
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

// FetchLyrics queries lrclib.net for synced lyrics, falling back to search if needed.
func FetchLyrics(title, artist, album string, duration float64) (*Lyric, error) {
	title, artist, album = normalizeQuotes(title), normalizeQuotes(artist), normalizeQuotes(album)
	client := &http.Client{Timeout: 10 * time.Second}

	// Try exact match endpoint
	apiURL := fmt.Sprintf("https://lrclib.net/api/get?track_name=%s&artist_name=%s&album_name=%s&duration=%.0f",
		url.QueryEscape(title), url.QueryEscape(artist), url.QueryEscape(album), duration)
	lyric, err := fetchAndParse(client, apiURL)
	if err != nil {
		return nil, err
	}
	if lyric != nil {
		return lyric, nil
	}
	// Fallback to search endpoint
	return fetchLyricsBySearch(client, title, artist)
}

// fetchAndParse performs the HTTP GET and parses the response for synced lyrics.
func fetchAndParse(client *http.Client, apiURL string) (*Lyric, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "LyricsMPRIS/1.0 (https://github.com/best8oy/LyricsMPRIS)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 || resp.StatusCode == 400 {
		return nil, nil // Not found, let caller decide fallback
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("lrclib: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var apiResp lrclibAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}
	if apiResp.SyncedLyrics == "" {
		return nil, nil
	}
	lines := parseSyncedLyrics(apiResp.SyncedLyrics)
	if len(lines) == 0 {
		return nil, errors.New("no valid lyric lines parsed")
	}
	return &Lyric{Lines: lines}, nil
}

// fetchLyricsBySearch tries to find lyrics using the search endpoint.
func fetchLyricsBySearch(client *http.Client, title, artist string) (*Lyric, error) {
	q := strings.TrimSpace(artist + " " + title)
	searchURL := fmt.Sprintf("https://lrclib.net/api/search?q=%s", url.QueryEscape(q))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "LyricsMPRIS/1.0 (https://github.com/best8oy/LyricsMPRIS)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("lrclib search: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var results []lrclibAPIResponse
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, err
	}
	for _, apiResp := range results {
		if apiResp.SyncedLyrics != "" {
			lines := parseSyncedLyrics(apiResp.SyncedLyrics)
			if len(lines) > 0 {
				return &Lyric{Lines: lines}, nil
			}
		}
	}
	return nil, errors.New("no synced lyrics found in search results")
}

// parseSyncedLyrics parses LRC-style synced lyrics into LyricLine slices.
func parseSyncedLyrics(synced string) []LyricLine {
	var lines []LyricLine
	for _, line := range strings.Split(synced, "\n") {
		if !strings.HasPrefix(line, "[") {
			continue
		}
		endIdx := strings.Index(line, "]")
		if endIdx < 0 {
			continue
		}
		timestamp := line[1:endIdx]
		text := strings.TrimSpace(line[endIdx+1:])
		if text == "" {
			continue
		}
		var min, sec, centi float64
		fmt.Sscanf(timestamp, "%02f:%02f.%02f", &min, &sec, &centi)
		timeVal := min*60 + sec + centi/100
		lines = append(lines, LyricLine{Time: timeVal, Text: text})
	}
	return lines
}

// normalizeQuotes replaces curly quotes with straight quotes for better API matching.
func normalizeQuotes(s string) string {
	s = strings.ReplaceAll(s, "’", "'")
	s = strings.ReplaceAll(s, "‘", "'")
	s = strings.ReplaceAll(s, "“", "\"")
	s = strings.ReplaceAll(s, "”", "\"")
	return s
}
