package lyrics

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/best8oy/LyricsMPRIS/logutil"
)

type LrcLibAPIResponse struct {
	ID           int     `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

type LrcLibLyricLine struct {
	Time  float64
	Words string
}

type LrcLibLyric struct {
	Lines []LrcLibLyricLine
}

func parseSyncedLyrics(synced string) []LrcLibLyricLine {
	var lines []LrcLibLyricLine
	for _, line := range strings.Split(synced, "\n") {
		if len(line) < 1 {
			continue
		}
		if line[0] != '[' {
			continue
		}
		endIdx := strings.Index(line, "]")
		if endIdx < 0 {
			continue
		}
		timestamp := line[1:endIdx]
		words := strings.TrimSpace(line[endIdx+1:])
		if words == "" {
			continue
		}
		var min, sec, centi float64
		fmt.Sscanf(timestamp, "%02f:%02f.%02f", &min, &sec, &centi)
		timeVal := min*60 + sec + centi/100
		lines = append(lines, LrcLibLyricLine{Time: timeVal, Words: words})
	}
	return lines
}

func normalizeQuotes(s string) string {
	s = strings.ReplaceAll(s, "’", "'")
	s = strings.ReplaceAll(s, "‘", "'")
	s = strings.ReplaceAll(s, "“", "\"")
	s = strings.ReplaceAll(s, "”", "\"")
	return s
}

func FetchLyrics(title, artist, album string, duration float64) (*LrcLibLyric, error) {
	title = normalizeQuotes(title)
	artist = normalizeQuotes(artist)
	album = normalizeQuotes(album)
	client := &http.Client{Timeout: 10 * time.Second}

	// Try exact match first
	url := fmt.Sprintf("https://lrclib.net/api/get?track_name=%s&artist_name=%s&album_name=%s&duration=%.0f",
		urlQueryEscape(title), urlQueryEscape(artist), urlQueryEscape(album), duration)
	logutil.LogVerbose("[lrclib] Querying: %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logutil.LogVerbose("[lrclib] NewRequest error: %v", err)
		return nil, err
	}
	req.Header.Set("User-Agent", "LyricsMPRIS/1.0 (https://github.com/best8oy/LyricsMPRIS)")

	resp, err := client.Do(req)
	if err != nil {
		logutil.LogVerbose("[lrclib] HTTP error: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 || resp.StatusCode == 400 {
		logutil.LogVerbose("[lrclib] Not found for %s - %s, falling back to search", artist, title)
		// fallback to search
		return fetchLyricsBySearch(client, title, artist, album)
	}
	if resp.StatusCode != 200 {
		logutil.LogVerbose("[lrclib] Non-200 status: %d", resp.StatusCode)
		return nil, nil
	}

	lyricBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logutil.LogVerbose("[lrclib] ReadAll error: %v", err)
		return nil, err
	}

	var apiResp LrcLibAPIResponse
	err = json.Unmarshal(lyricBytes, &apiResp)
	if err != nil {
		logutil.LogVerbose("[lrclib] JSON unmarshal error: %v", err)
		return nil, err
	}

	if apiResp.SyncedLyrics == "" {
		logutil.LogVerbose("[lrclib] No synced lyrics available for %s - %s", artist, title)
		return nil, nil
	}

	lines := parseSyncedLyrics(apiResp.SyncedLyrics)
	if len(lines) == 0 {
		logutil.LogVerbose("[lrclib] No valid lyric lines parsed for %s - %s", artist, title)
		return nil, nil
	}
	return &LrcLibLyric{Lines: lines}, nil
}

func fetchLyricsBySearch(client *http.Client, title, artist, album string) (*LrcLibLyric, error) {
	q := strings.TrimSpace(artist + " " + title)
	url := fmt.Sprintf("https://lrclib.net/api/search?q=%s", urlQueryEscape(q))
	logutil.LogVerbose("[lrclib] Fallback search: %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logutil.LogVerbose("[lrclib] NewRequest error: %v", err)
		return nil, err
	}
	req.Header.Set("User-Agent", "LyricsMPRIS/1.0 (https://github.com/best8oy/LyricsMPRIS)")

	resp, err := client.Do(req)
	if err != nil {
		logutil.LogVerbose("[lrclib] HTTP error: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logutil.LogVerbose("[lrclib] Search non-200 status: %d", resp.StatusCode)
		return nil, nil
	}

	lyricBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logutil.LogVerbose("[lrclib] Search ReadAll error: %v", err)
		return nil, err
	}

	var results []LrcLibAPIResponse
	err = json.Unmarshal(lyricBytes, &results)
	if err != nil {
		logutil.LogVerbose("[lrclib] Search JSON unmarshal error: %v", err)
		return nil, err
	}
	for _, apiResp := range results {
		if apiResp.SyncedLyrics != "" {
			lines := parseSyncedLyrics(apiResp.SyncedLyrics)
			if len(lines) > 0 {
				return &LrcLibLyric{Lines: lines}, nil
			}
		}
	}
	return nil, nil
}

func urlQueryEscape(s string) string {
	return url.QueryEscape(s)
}
