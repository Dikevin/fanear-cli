package data

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	isoRe     = regexp.MustCompile(`^(\d{4})[-/.](\d{1,2})[-/.](\d{1,2})$`)
	compactRe = regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})$`)
	shortRe   = regexp.MustCompile(`^(\d{1,2})[-/.](\d{1,2})$`)
	cnRe      = regexp.MustCompile(`^(?:(\d{4})年)?(\d{1,2})月(\d{1,2})[日号]?$`)
)

// ParseDate reads a calendar day: 今天/昨天/前天 (today/yesterday),
// 2026-11-24, 20261124, 11-24, 11/24 or 11月24日. A day without a year that
// would fall in the future is taken from last year.
func ParseDate(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	switch strings.ToLower(s) {
	case "今天", "today":
		return today, nil
	case "昨天", "yesterday":
		return today.AddDate(0, 0, -1), nil
	case "前天":
		return today.AddDate(0, 0, -2), nil
	}
	var y, m, d int
	if g := isoRe.FindStringSubmatch(s); g != nil {
		y, m, d = atoi(g[1]), atoi(g[2]), atoi(g[3])
	} else if g := compactRe.FindStringSubmatch(s); g != nil {
		y, m, d = atoi(g[1]), atoi(g[2]), atoi(g[3])
	} else if g := cnRe.FindStringSubmatch(s); g != nil {
		y, m, d = atoi(g[1]), atoi(g[2]), atoi(g[3])
	} else if g := shortRe.FindStringSubmatch(s); g != nil {
		m, d = atoi(g[1]), atoi(g[2])
	} else {
		return time.Time{}, fmt.Errorf("看不懂日期 %q（可以写 今天、昨天、2026-11-24、11-24、11月24日）", s)
	}
	guessYear := y == 0
	if guessYear {
		y = today.Year()
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if t.Month() != time.Month(m) || t.Day() != d {
		return time.Time{}, fmt.Errorf("没有这一天：%q", s)
	}
	if guessYear && t.After(today) {
		t = t.AddDate(-1, 0, 0)
	}
	return t, nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// ParseClock reads a time of day (14, 14:05, 14:05:30, 14点, 14点05) and
// returns the offset from midnight.
func ParseClock(s string) (time.Duration, error) {
	orig := s
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("点", ":", "分", ":", "秒", "", "：", ":").Replace(s)
	s = strings.TrimSuffix(s, ":")
	parts := strings.Split(s, ":")
	if len(parts) > 3 || s == "" {
		return 0, fmt.Errorf("看不懂时刻 %q（可以写 14:05 或 14:05:30）", orig)
	}
	var vals [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("看不懂时刻 %q（可以写 14:05 或 14:05:30）", orig)
		}
		vals[i] = n
	}
	if vals[0] > 24 || vals[1] > 59 || vals[2] > 59 {
		return 0, fmt.Errorf("时刻超出范围：%q", orig)
	}
	return time.Duration(vals[0])*time.Hour + time.Duration(vals[1])*time.Minute + time.Duration(vals[2])*time.Second, nil
}

// ParseTS parses FanEar's timestamps (RFC 3339 with milliseconds and offset).
func ParseTS(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, s)
	return t, err == nil
}

// Clock renders a timestamp as HH:MM:SS in the offset it was written with.
func Clock(s string) string {
	if t, ok := ParseTS(s); ok {
		return t.Format("15:04:05")
	}
	return s
}

// HM renders a timestamp as HH:MM in the offset it was written with.
func HM(s string) string {
	if t, ok := ParseTS(s); ok {
		return t.Format("15:04")
	}
	return s
}

// SinceMidnight is the offset of a timestamp from midnight in its own offset.
func SinceMidnight(t time.Time) time.Duration {
	y, m, d := t.Date()
	return t.Sub(time.Date(y, m, d, 0, 0, 0, 0, t.Location()))
}

// HumanDuration renders 1h23m as 1小时23分.
func HumanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%d小时%d分", h, m)
	case h > 0:
		return fmt.Sprintf("%d小时", h)
	case m > 0:
		return fmt.Sprintf("%d分钟", m)
	}
	return fmt.Sprintf("%d秒", int(d.Seconds()))
}
