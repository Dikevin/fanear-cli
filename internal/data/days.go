// Package data reads the files FanEar writes into each day folder:
// 记录/年/月/日/{M月D日 饭耳总结.md/.json, 饭耳记录.md/.json, 健康.json, 足迹.json, 录音清单.json, 录音/}.
// The format is documented in the README.md FanEar writes next to them.
package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Day is one day folder.
type Day struct {
	Date time.Time // midnight UTC of the calendar day
	Dir  string
}

// Files lists what a day folder holds; empty strings mean absent.
type Files struct {
	SummaryMD  string `json:"summaryMd,omitempty"`
	Summary    string `json:"summary,omitempty"`
	RecordMD   string `json:"recordMd,omitempty"`
	Record     string `json:"record,omitempty"`
	Health     string `json:"health,omitempty"`
	Footprint  string `json:"footprint,omitempty"`
	Manifest   string `json:"manifest,omitempty"`
	Recordings int    `json:"recordings"`
}

var numRe = regexp.MustCompile(`^\d+$`)

// ListDays returns the day folders under the records folder, oldest first.
func ListDays(records string) ([]Day, error) {
	var days []Day
	years, err := os.ReadDir(records)
	if err != nil {
		return nil, err
	}
	for _, y := range years {
		if !y.IsDir() || !numRe.MatchString(y.Name()) || len(y.Name()) != 4 {
			continue
		}
		yn, _ := strconv.Atoi(y.Name())
		months, _ := os.ReadDir(filepath.Join(records, y.Name()))
		for _, m := range months {
			mn, err := strconv.Atoi(m.Name())
			if !m.IsDir() || err != nil || mn < 1 || mn > 12 {
				continue
			}
			ds, _ := os.ReadDir(filepath.Join(records, y.Name(), m.Name()))
			for _, d := range ds {
				dn, err := strconv.Atoi(d.Name())
				if !d.IsDir() || err != nil || dn < 1 || dn > 31 {
					continue
				}
				days = append(days, Day{
					Date: time.Date(yn, time.Month(mn), dn, 0, 0, 0, 0, time.UTC),
					Dir:  filepath.Join(records, y.Name(), m.Name(), d.Name()),
				})
			}
		}
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Date.Before(days[j].Date) })
	return days, nil
}

// FindDay returns the folder for date, or an error naming the nearest days.
func FindDay(days []Day, date time.Time) (Day, error) {
	for _, d := range days {
		if d.Date.Equal(date) {
			return d, nil
		}
	}
	if len(days) == 0 {
		return Day{}, errors.New("数据目录里还没有任何一天的记录")
	}
	return Day{}, fmt.Errorf("%s 没有记录（有记录的是 %s ～ %s，共 %d 天）", FormatDate(date),
		FormatDate(days[0].Date), FormatDate(days[len(days)-1].Date), len(days))
}

// FilterDays keeps days within [from, to]; zero times mean unbounded.
func FilterDays(days []Day, from, to time.Time) []Day {
	var out []Day
	for _, d := range days {
		if !from.IsZero() && d.Date.Before(from) {
			continue
		}
		if !to.IsZero() && d.Date.After(to) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// DayFiles lists the files of a day folder by their suffix, since the
// "M月D日" prefix follows the phone's locale.
func DayFiles(d Day) Files {
	var f Files
	ents, _ := os.ReadDir(d.Dir)
	for _, e := range ents {
		n := e.Name()
		p := filepath.Join(d.Dir, n)
		switch {
		case e.IsDir() && n == "录音":
			recs, _ := os.ReadDir(p)
			for _, r := range recs {
				if !r.IsDir() && !strings.HasPrefix(r.Name(), ".") {
					f.Recordings++
				}
			}
		case strings.HasSuffix(n, "饭耳总结.md"):
			f.SummaryMD = p
		case strings.HasSuffix(n, "饭耳总结.json"):
			f.Summary = p
		case strings.HasSuffix(n, "饭耳记录.md"):
			f.RecordMD = p
		case strings.HasSuffix(n, "饭耳记录.json"):
			f.Record = p
		case strings.HasSuffix(n, "健康.json"):
			f.Health = p
		case strings.HasSuffix(n, "足迹.json"):
			f.Footprint = p
		case n == "录音清单.json":
			f.Manifest = p
		}
	}
	return f
}

// ReadJSON decodes a FanEar file, wording errors for a reader who may be
// looking at a format FanEar has since changed.
func ReadJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("没有 %s", filepath.Base(path))
		}
		return fmt.Errorf("读不了 %s：%w", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("解析 %s 失败（饭耳可能改了格式）：%w", filepath.Base(path), err)
	}
	return nil
}

var weekdays = [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

// FormatDate renders a calendar day as 2026-11-24.
func FormatDate(t time.Time) string { return t.Format("2006-01-02") }

// FormatDateCN renders a calendar day as 11月24日 周一.
func FormatDateCN(t time.Time) string {
	return fmt.Sprintf("%d月%d日 %s", t.Month(), t.Day(), weekdays[t.Weekday()])
}
