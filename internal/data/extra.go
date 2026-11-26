package data

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Health is 「M月D日 健康.json」 (schema_version 1), copied from Apple Health.
type Health struct {
	SchemaVersion int    `json:"schema_version"`
	Date          string `json:"date"`
	TimeZone      string `json:"timeZone"`
	Series        []struct {
		Type        string `json:"type"`
		Name        string `json:"name"`
		Kind        string `json:"kind"`        // quantity or category
		Aggregation string `json:"aggregation"` // cumulative or discrete
		Unit        string `json:"unit"`
		Samples     []struct {
			Start  string  `json:"start"`
			End    string  `json:"end"`
			Value  float64 `json:"value"`
			Label  string  `json:"label"`
			Source string  `json:"source"`
		} `json:"samples"`
	} `json:"series"`
	Alerts []struct {
		Kind         string  `json:"kind"`
		Start        string  `json:"start"`
		End          string  `json:"end"`
		ThresholdBPM float64 `json:"thresholdBPM"`
	} `json:"alerts"`
	Moods              []json.RawMessage `json:"moods"`
	Electrocardiograms []struct {
		Start            string  `json:"start"`
		Classification   string  `json:"classification"`
		AverageHeartRate float64 `json:"averageHeartRate"`
	} `json:"electrocardiograms"`
	UnreadTypes []string `json:"unreadTypes"`
}

// Footprint is 「M月D日 足迹.json」 (schema_version 1). Coordinates are WGS-84.
type Footprint struct {
	SchemaVersion  int     `json:"schema_version"`
	Date           string  `json:"date"`
	TimeZone       string  `json:"timeZone"`
	DistanceMeters float64 `json:"distanceMeters"`
	MovingSeconds  float64 `json:"movingSeconds"`
	Stops          []Stop  `json:"stops"`
	Route          []struct {
		Time           string  `json:"time"`
		Latitude       float64 `json:"latitude"`
		Longitude      float64 `json:"longitude"`
		Speed          float64 `json:"speed"`
		AccuracyMeters float64 `json:"accuracyMeters"`
	} `json:"route"`
	Motions []struct {
		Kind  string `json:"kind"`
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"motions"`
}

// Stop is a place stayed at.
type Stop struct {
	Place          int     `json:"place"`
	Name           string  `json:"name"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	AccuracyMeters float64 `json:"accuracyMeters"`
	Arrival        string  `json:"arrival"`
	Departure      string  `json:"departure"`
}

// MotionLabel words Core Motion activity kinds.
func MotionLabel(kind string) string {
	switch kind {
	case "walking":
		return "走路"
	case "running":
		return "跑步"
	case "cycling":
		return "骑车"
	case "automotive":
		return "坐车"
	case "stationary":
		return "静止"
	}
	return kind
}

// WGS84ToGCJ02 converts GPS coordinates to the GCJ-02 datum used by AMap
// and Tencent Maps. Points outside mainland China are returned unchanged.
func WGS84ToGCJ02(lat, lon float64) (float64, float64) {
	if lon < 72.004 || lon > 137.8347 || lat < 0.8293 || lat > 55.8271 {
		return lat, lon
	}
	const a = 6378245.0
	const ee = 0.00669342162296594323
	x, y := lon-105.0, lat-35.0
	dLat := -100.0 + 2.0*x + 3.0*y + 0.2*y*y + 0.1*x*y + 0.2*math.Sqrt(math.Abs(x))
	dLat += (20.0*math.Sin(6.0*x*math.Pi) + 20.0*math.Sin(2.0*x*math.Pi)) * 2.0 / 3.0
	dLat += (20.0*math.Sin(y*math.Pi) + 40.0*math.Sin(y/3.0*math.Pi)) * 2.0 / 3.0
	dLat += (160.0*math.Sin(y/12.0*math.Pi) + 320*math.Sin(y*math.Pi/30.0)) * 2.0 / 3.0
	dLon := 300.0 + x + 2.0*y + 0.1*x*x + 0.1*x*y + 0.1*math.Sqrt(math.Abs(x))
	dLon += (20.0*math.Sin(6.0*x*math.Pi) + 20.0*math.Sin(2.0*x*math.Pi)) * 2.0 / 3.0
	dLon += (20.0*math.Sin(x*math.Pi) + 40.0*math.Sin(x/3.0*math.Pi)) * 2.0 / 3.0
	dLon += (150.0*math.Sin(x/12.0*math.Pi) + 300.0*math.Sin(x/30.0*math.Pi)) * 2.0 / 3.0
	radLat := lat / 180.0 * math.Pi
	magic := math.Sin(radLat)
	magic = 1 - ee*magic*magic
	sqrtMagic := math.Sqrt(magic)
	dLat = (dLat * 180.0) / ((a * (1 - ee)) / (magic * sqrtMagic) * math.Pi)
	dLon = (dLon * 180.0) / (a / sqrtMagic * math.Cos(radLat) * math.Pi)
	return lat + dLat, lon + dLon
}

// Page is one file of 小耳's memory.
type Page struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// LoadMemory reads 小耳's memory pages from the restore backup
// (parts/files/memory/whole.json: {"files": {path: markdown}, "format": 1}).
func LoadMemory(restore string) ([]Page, error) {
	dir := filepath.Join(restore, "parts", "files", "memory")
	var whole struct {
		Format int               `json:"format"`
		Files  map[string]string `json:"files"`
	}
	if err := ReadJSON(filepath.Join(dir, "whole.json"), &whole); err != nil {
		return nil, fmt.Errorf("读不了小耳的记忆（非官方格式，饭耳可能改了）：%w", err)
	}
	var pages []Page
	for p, content := range whole.Files {
		if !strings.HasSuffix(p, ".md") {
			continue
		}
		pages = append(pages, Page{Path: p, Kind: pageKind(p), Title: pageTitle(p, content), Content: content})
	}
	sort.Slice(pages, func(i, j int) bool {
		if pages[i].Kind != pages[j].Kind {
			return kindOrder(pages[i].Kind) < kindOrder(pages[j].Kind)
		}
		return pages[i].Title < pages[j].Title
	})
	return pages, nil
}

func pageKind(p string) string {
	switch {
	case p == "index.md":
		return "目录"
	case p == "me.md":
		return "你"
	case p == "log.md":
		return "整理记录"
	case strings.HasPrefix(p, "people/"):
		return "人物"
	case strings.HasPrefix(p, "topics/"):
		return "话题"
	}
	return "其他"
}

func kindOrder(k string) int {
	return strings.Index("目录你人物话题其他整理记录", k)
}

func pageTitle(p, content string) string {
	for _, l := range strings.SplitN(content, "\n", 3) {
		if strings.HasPrefix(l, "# ") {
			return strings.TrimSpace(l[2:])
		}
	}
	return strings.TrimSuffix(filepath.Base(p), ".md")
}

var pageMetaRe = regexp.MustCompile(`(?m)^<!-- (?:page|night|day)\b.*-->\n?`)

// CleanPage drops FanEar's machine comments from a memory page.
func CleanPage(content string) string {
	return strings.TrimSpace(pageMetaRe.ReplaceAllString(content, ""))
}

// Favorite is one saved conversation.
type Favorite struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	Title           string   `json:"title"`
	Gist            string   `json:"gist"`
	Start           string   `json:"start"`
	End             string   `json:"end"`
	CreatedAt       string   `json:"createdAt"`
	AutoMeeting     bool     `json:"autoMeeting"`
	ConversationIDs []string `json:"conversationIDs"`
}

// LoadFavorites reads favorites from the restore backup. Small lists are a
// single whole.json array; larger ones may be split into _.N.json parts that
// each hold {"items": [...]}.
func LoadFavorites(restore string) ([]Favorite, error) {
	dir := filepath.Join(restore, "parts", "files", "favorites.json")
	var favs []Favorite
	whole := filepath.Join(dir, "whole.json")
	if _, err := os.Stat(whole); err == nil {
		if err := ReadJSON(whole, &favs); err != nil {
			return nil, fmt.Errorf("读不了收藏（非官方格式，饭耳可能改了）：%w", err)
		}
		return favs, nil
	}
	parts, _ := filepath.Glob(filepath.Join(dir, "_.*.json"))
	if len(parts) == 0 {
		return nil, fmt.Errorf("备份里没有收藏（%s 不存在或格式变了）", dir)
	}
	sort.Slice(parts, func(i, j int) bool { return partIndex(parts[i]) < partIndex(parts[j]) })
	for _, p := range parts {
		var part struct {
			Items []Favorite `json:"items"`
		}
		if err := ReadJSON(p, &part); err != nil {
			return nil, fmt.Errorf("读不了收藏（非官方格式，饭耳可能改了）：%w", err)
		}
		favs = append(favs, part.Items...)
	}
	return favs, nil
}

func partIndex(p string) int {
	n, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), "_."), ".json"))
	return n
}
