package data

import (
	"math"
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	cases := map[string]string{
		"今天": "2026-11-26", "yesterday": "2026-11-25", "前天": "2026-11-24",
		"2026-11-24": "2026-11-24", "2026/1/5": "2026-01-05", "20261124": "2026-11-24",
		"11-24": "2026-11-24", "11/24": "2026-11-24", "11月24日": "2026-11-24", "2025年3月1号": "2025-03-01",
		"12-01": "2025-12-01", // a day later this year means last year
	}
	for in, want := range cases {
		got, err := ParseDate(in, now)
		if err != nil || FormatDate(got) != want {
			t.Errorf("ParseDate(%q) = %s, %v; want %s", in, FormatDate(got), err, want)
		}
	}
	for _, bad := range []string{"", "明年", "2-30", "13-01"} {
		if _, err := ParseDate(bad, now); err == nil {
			t.Errorf("ParseDate(%q) should fail", bad)
		}
	}
}

func TestParseClock(t *testing.T) {
	cases := map[string]time.Duration{
		"14": 14 * time.Hour, "14:05": 14*time.Hour + 5*time.Minute, "9:05:30": 9*time.Hour + 5*time.Minute + 30*time.Second,
		"14点": 14 * time.Hour, "14点05": 14*time.Hour + 5*time.Minute, "14：05": 14*time.Hour + 5*time.Minute,
	}
	for in, want := range cases {
		if got, err := ParseClock(in); err != nil || got != want {
			t.Errorf("ParseClock(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "25:00", "aa", "1:2:3:4"} {
		if _, err := ParseClock(bad); err == nil {
			t.Errorf("ParseClock(%q) should fail", bad)
		}
	}
}

func TestClockKeepsWrittenOffset(t *testing.T) {
	if got := Clock("2026-11-25T23:59:51.000+08:00"); got != "23:59:51" {
		t.Fatal(got)
	}
}

func TestWGS84ToGCJ02(t *testing.T) {
	// Reference pair used by common eviltransform implementations.
	lat, lon := WGS84ToGCJ02(31.1774276, 121.5272106)
	if math.Abs(lat-31.17530398364597) > 1e-6 || math.Abs(lon-121.531541859215) > 1e-6 {
		t.Errorf("Shanghai: got %.8f,%.8f", lat, lon)
	}
	if lat, lon := WGS84ToGCJ02(48.8584, 2.2945); lat != 48.8584 || lon != 2.2945 {
		t.Errorf("outside China must stay put: %v,%v", lat, lon)
	}
}

func TestTodoStateLabel(t *testing.T) {
	cases := []struct {
		state string
		by    string
		want  string
	}{
		{"open", "", "已进待办"}, {"waiting", "", "等你确认"}, {"hidden", "", "没进待办"},
		{"done", "user", "已做完（自己点的）"}, {"done", "said", "已做完（小耳听到原话）"}, {"done", "inferred", "已做完（小耳推测）"},
	}
	for _, c := range cases {
		var b *DoneBasis
		if c.by != "" {
			b = &DoneBasis{By: c.by}
		}
		if got := TodoStateLabel(c.state, b); got != c.want {
			t.Errorf("%s/%s = %s", c.state, c.by, got)
		}
	}
}

func TestMatches(t *testing.T) {
	if !Matches("Hello 世界", []string{"hello"}, false) || Matches("abc", []string{"x", "a"}, true) || !Matches("abc", []string{"x", "a"}, false) {
		t.Fatal("Matches")
	}
}
