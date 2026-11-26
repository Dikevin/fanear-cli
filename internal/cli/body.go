package cli

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Dikevin/fanear-cli/internal/data"
)

var unitNames = map[string]string{
	"count": "", "count/min": "次/分", "Cal": "千卡", "kcal": "千卡", "min": "分钟", "km": "公里",
	"ms": "毫秒", "degC": "°C", "dBASPL": "分贝", "km/hr": "公里/时", "cm": "厘米", "m/s": "米/秒",
	"kg": "公斤", "m": "米",
}

func unitName(u string) string {
	if n, ok := unitNames[u]; ok {
		return n
	}
	return u
}

// display scales HealthKit's fractional percentages to 0–100.
func display(v float64, unit string) float64 {
	if unit == "%" {
		return v * 100
	}
	return v
}

func num(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

type seriesOut struct {
	Type        string             `json:"type"`
	Name        string             `json:"name"`
	Unit        string             `json:"unit,omitempty"`
	Kind        string             `json:"kind"`
	Aggregation string             `json:"aggregation,omitempty"`
	Count       int                `json:"count"`
	Sum         *float64           `json:"sum,omitempty"`
	Min         *float64           `json:"min,omitempty"`
	Avg         *float64           `json:"avg,omitempty"`
	Max         *float64           `json:"max,omitempty"`
	Labels      map[string]float64 `json:"labelSeconds,omitempty"`
	Samples     []sampleOut        `json:"samples,omitempty"`
}

type sampleOut struct {
	Start  string  `json:"start"`
	End    string  `json:"end"`
	Value  float64 `json:"value"`
	Label  string  `json:"label,omitempty"`
	Source string  `json:"source,omitempty"`
}

// categoryLabel names a category sample that FanEar left without a label.
func categoryLabel(typ, label string, v float64) string {
	if label != "" {
		return label
	}
	if strings.HasSuffix(typ, "AppleStandHour") {
		if v == 0 {
			return "站了"
		}
		return "没站"
	}
	return num(v)
}

func cmdHealth(e Env, args []string) error {
	f := newFlags(e, "health", "health [日期] [--type 心率]")
	typ := f.String("type", "", "只看这一类（名字或苹果类型标识里带这几个字），并列出每一条")
	limit := f.Int("limit", 100, "--type 时每类最多列多少条，0 表示不限")
	pos, err := f.parse(args)
	if err != nil {
		return err
	}
	_, days, err := f.openDays(e)
	if err != nil {
		return err
	}
	d, files, err := pickDay(e, days, pos, func(x data.Files) bool { return x.Health != "" })
	if err != nil {
		return err
	}
	if files.Health == "" {
		return fmt.Errorf("%s 没有健康数据（要在饭耳 设置 › 存储 里打开「健康数据也存到保存位置」）", data.FormatDate(d.Date))
	}
	var h data.Health
	if err := data.ReadJSON(files.Health, &h); err != nil {
		return err
	}
	var series []seriesOut
	for _, s := range h.Series {
		if *typ != "" && !strings.Contains(s.Name, *typ) && !strings.Contains(strings.ToLower(s.Type), strings.ToLower(*typ)) {
			continue
		}
		so := seriesOut{Type: s.Type, Name: s.Name, Unit: s.Unit, Kind: s.Kind, Aggregation: s.Aggregation, Count: len(s.Samples)}
		if s.Kind == "category" {
			so.Labels = map[string]float64{}
			for _, x := range s.Samples {
				a, ok1 := data.ParseTS(x.Start)
				b, ok2 := data.ParseTS(x.End)
				if ok1 && ok2 {
					label := categoryLabel(s.Type, x.Label, x.Value)
					so.Labels[label] += b.Sub(a).Seconds()
				}
			}
		} else if len(s.Samples) > 0 {
			sum, lo, hi := 0.0, math.Inf(1), math.Inf(-1)
			for _, x := range s.Samples {
				sum += x.Value
				lo, hi = math.Min(lo, x.Value), math.Max(hi, x.Value)
			}
			if s.Aggregation == "cumulative" {
				so.Sum = &sum
			} else {
				avg := sum / float64(len(s.Samples))
				so.Min, so.Avg, so.Max = &lo, &avg, &hi
			}
		}
		if *typ != "" {
			for _, x := range s.Samples {
				so.Samples = append(so.Samples, sampleOut{x.Start, x.End, x.Value, x.Label, x.Source})
			}
		}
		series = append(series, so)
	}
	if *typ != "" && len(series) == 0 {
		var names []string
		for _, s := range h.Series {
			names = append(names, s.Name)
		}
		return fmt.Errorf("这天没有 %q 这类数据，有：%s", *typ, strings.Join(names, "、"))
	}
	if *f.asJSON {
		if series == nil {
			series = []seriesOut{}
		}
		return writeJSON(e.Stdout, map[string]any{
			"date": data.FormatDate(d.Date), "timeZone": h.TimeZone, "series": series,
			"alerts": h.Alerts, "electrocardiograms": h.Electrocardiograms, "moods": len(h.Moods), "unreadTypes": h.UnreadTypes,
		})
	}
	o := newOut(e.Stdout)
	o.f("# %s 健康数据（来自苹果「健康」）\n\n", data.FormatDateCN(d.Date))
	for _, s := range series {
		u := unitName(s.Unit)
		switch {
		case s.Labels != nil:
			o.f("- %s：", s.Name)
			var parts []string
			for _, k := range sortedKeys(s.Labels) {
				parts = append(parts, fmt.Sprintf("%s %s", k, data.HumanDuration(time.Duration(s.Labels[k])*time.Second)))
			}
			o.f("%s\n", strings.Join(parts, "，"))
		case s.Sum != nil:
			o.f("- %s：共 %s%s（%d 条）\n", s.Name, num(display(*s.Sum, s.Unit)), u, s.Count)
		case s.Avg != nil:
			o.f("- %s：平均 %s%s，最低 %s，最高 %s（%d 条）\n", s.Name, num(display(*s.Avg, s.Unit)), u, num(display(*s.Min, s.Unit)), num(display(*s.Max, s.Unit)), s.Count)
		default:
			o.f("- %s：没有数据\n", s.Name)
		}
		for i, x := range s.Samples {
			if *limit > 0 && i >= *limit {
				o.f("  ……还有 %d 条，--limit 0 全部输出，或用 --json 自己算\n", len(s.Samples)-i)
				break
			}
			label := ""
			if x.Label != "" {
				label = " " + x.Label
			}
			o.f("  [%s–%s] %s%s%s · %s\n", data.Clock(x.Start), data.Clock(x.End), num(display(x.Value, s.Unit)), u, label, x.Source)
		}
	}
	if *typ == "" {
		if len(h.Alerts) > 0 {
			o.f("- 手表提醒：%d 次\n", len(h.Alerts))
			for _, a := range h.Alerts {
				o.f("  [%s] %s（阈值 %s 次/分）\n", data.Clock(a.Start), a.Kind, num(a.ThresholdBPM))
			}
		}
		for _, ecg := range h.Electrocardiograms {
			o.f("- 心电图 [%s]：%s，平均心率 %s\n", data.Clock(ecg.Start), ecg.Classification, num(ecg.AverageHeartRate))
		}
		if len(h.Moods) > 0 {
			o.f("- 心情记录：%d 条（细节用 --json）\n", len(h.Moods))
		}
		if len(h.UnreadTypes) > 0 {
			o.f("- 这天没读出来的类型：%s\n", strings.Join(h.UnreadTypes, "、"))
		}
	}
	return o.Flush()
}

type stopOut struct {
	data.Stop
	Duration string `json:"duration"`
}

func cmdPlaces(e Env, args []string) error {
	f := newFlags(e, "places", "places [日期] [--gcj02] [--route]")
	gcj := f.Bool("gcj02", false, "坐标换算成 GCJ-02（高德、腾讯地图用的）")
	route := f.Bool("route", false, "也列出路线上的每个定位点")
	pos, err := f.parse(args)
	if err != nil {
		return err
	}
	_, days, err := f.openDays(e)
	if err != nil {
		return err
	}
	d, files, err := pickDay(e, days, pos, func(x data.Files) bool { return x.Footprint != "" })
	if err != nil {
		return err
	}
	if files.Footprint == "" {
		return fmt.Errorf("%s 没有足迹（要在饭耳里打开「每天的记录带上足迹」）", data.FormatDate(d.Date))
	}
	var fp data.Footprint
	if err := data.ReadJSON(files.Footprint, &fp); err != nil {
		return err
	}
	crs := "WGS-84"
	conv := func(lat, lon float64) (float64, float64) { return lat, lon }
	if *gcj {
		crs = "GCJ-02"
		conv = data.WGS84ToGCJ02
	}
	var stops []stopOut
	for _, s := range fp.Stops {
		s.Latitude, s.Longitude = conv(s.Latitude, s.Longitude)
		so := stopOut{Stop: s}
		if a, ok := data.ParseTS(s.Arrival); ok {
			if b, ok := data.ParseTS(s.Departure); ok {
				so.Duration = data.HumanDuration(b.Sub(a))
			}
		}
		stops = append(stops, so)
	}
	type pointOut struct {
		Time      string  `json:"time"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Speed     float64 `json:"speed"`
	}
	var points []pointOut
	if *route {
		for _, p := range fp.Route {
			lat, lon := conv(p.Latitude, p.Longitude)
			points = append(points, pointOut{p.Time, lat, lon, p.Speed})
		}
	}
	if *f.asJSON {
		if stops == nil {
			stops = []stopOut{}
		}
		res := map[string]any{
			"date": data.FormatDate(d.Date), "coordinateSystem": crs, "distanceMeters": fp.DistanceMeters,
			"movingSeconds": fp.MovingSeconds, "stops": stops, "motions": fp.Motions,
		}
		if *route {
			res["route"] = points
		}
		return writeJSON(e.Stdout, res)
	}
	o := newOut(e.Stdout)
	o.f("# %s 足迹（坐标 %s）\n\n", data.FormatDateCN(d.Date), crs)
	o.f("里程 %s 公里，移动 %s\n\n停留：\n", num(fp.DistanceMeters/1000), data.HumanDuration(time.Duration(fp.MovingSeconds)*time.Second))
	for _, s := range stops {
		name := s.Name
		if name == "" {
			name = fmt.Sprintf("地点%d", s.Place)
		}
		o.f("  - %s–%s（%s）%s  %.5f,%.5f ±%sm\n", data.HM(s.Arrival), data.HM(s.Departure), s.Duration,
			name, s.Latitude, s.Longitude, num(s.AccuracyMeters))
	}
	if len(fp.Motions) > 0 {
		o.f("\n移动：\n")
		for _, m := range fp.Motions {
			dur := ""
			if a, ok := data.ParseTS(m.Start); ok {
				if b, ok := data.ParseTS(m.End); ok {
					dur = "（" + data.HumanDuration(b.Sub(a)) + "）"
				}
			}
			o.f("  %s–%s %s%s\n", data.HM(m.Start), data.HM(m.End), data.MotionLabel(m.Kind), dur)
		}
	}
	if *route {
		o.f("\n路线（%d 个点）：\n", len(points))
		for _, p := range points {
			o.f("  [%s] %.6f,%.6f %s米/秒\n", data.Clock(p.Time), p.Latitude, p.Longitude, num(p.Speed))
		}
	}
	return o.Flush()
}

func cmdMemory(e Env, args []string) error {
	f := newFlags(e, "memory", "memory [名字或关键词] [--list]   （不给参数就是小耳的记忆目录）")
	list := f.Bool("list", false, "列出所有页面")
	pos, err := f.parse(args)
	if err != nil {
		return err
	}
	root, err := f.locate(e)
	if err != nil {
		return err
	}
	if root.Restore == "" {
		return fmt.Errorf("这个保存位置（%s）没有饭耳的“换手机找回资料”备份，读不了小耳的记忆；它只在 iCloud 和「文件」App 里选的文件夹里有", root.Records)
	}
	pages, err := data.LoadMemory(root.Restore)
	if err != nil {
		return err
	}
	q := strings.TrimSpace(strings.Join(pos, " "))
	var hit []data.Page
	switch {
	case *list:
		hit = pages
	case q == "":
		hit = findPages(pages, func(p data.Page) bool { return p.Path == "index.md" })
	case q == "我" || q == "你" || strings.EqualFold(q, "me"):
		hit = findPages(pages, func(p data.Page) bool { return p.Path == "me.md" })
	case strings.EqualFold(q, "log") || q == "整理记录":
		hit = findPages(pages, func(p data.Page) bool { return p.Path == "log.md" })
	default:
		hit = findPages(pages, func(p data.Page) bool { return p.Title == q })
		if len(hit) == 0 {
			hit = findPages(pages, func(p data.Page) bool { return strings.Contains(p.Title, q) })
		}
	}
	if *f.asJSON {
		if len(hit) == 0 && q != "" {
			hit = findPages(pages, func(p data.Page) bool { return strings.Contains(p.Content, q) })
		}
		if *list {
			for i := range hit {
				hit[i].Content = ""
			}
		}
		if hit == nil {
			hit = []data.Page{}
		}
		return writeJSON(e.Stdout, hit)
	}
	o := newOut(e.Stdout)
	switch {
	case *list || len(hit) > 1:
		if len(hit) > 1 && !*list {
			o.f("有 %d 页标题带「%s」，挑一个再查：\n", len(hit), q)
		}
		for _, p := range hit {
			o.f("- [%s] %s（%d 字）\n", p.Kind, p.Title, len([]rune(p.Content)))
		}
	case len(hit) == 1:
		o.f("%s\n", data.CleanPage(hit[0].Content))
	default:
		in := findPages(pages, func(p data.Page) bool { return p.Path != "log.md" && strings.Contains(p.Content, q) })
		if len(in) == 0 {
			o.f("小耳的记忆里没有「%s」。可以再用 fanear search %s 搜原话。\n", q, q)
			break
		}
		o.f("没有叫「%s」的页面，这些页面里提到了：\n", q)
		for _, p := range in {
			o.f("- [%s] %s：%s\n", p.Kind, p.Title, snippet(p.Content, q))
		}
	}
	return o.Flush()
}

func findPages(pages []data.Page, keep func(data.Page) bool) []data.Page {
	var out []data.Page
	for _, p := range pages {
		if keep(p) {
			out = append(out, p)
		}
	}
	return out
}

func snippet(content, q string) string {
	for _, l := range strings.Split(content, "\n") {
		if strings.Contains(l, q) {
			r := []rune(strings.TrimSpace(l))
			if len(r) > 80 {
				r = append(r[:80], '…')
			}
			return string(r)
		}
	}
	return ""
}

func cmdFavorites(e Env, args []string) error {
	f := newFlags(e, "favorites", "favorites [关键词]")
	pos, err := f.parse(args)
	if err != nil {
		return err
	}
	root, err := f.locate(e)
	if err != nil {
		return err
	}
	if root.Restore == "" {
		return fmt.Errorf("这个保存位置（%s）没有饭耳的“换手机找回资料”备份，读不了收藏；它只在 iCloud 和「文件」App 里选的文件夹里有", root.Records)
	}
	favs, err := data.LoadFavorites(root.Restore)
	if err != nil {
		return err
	}
	loc := dataLocation(root.Records)
	var hit []data.Favorite
	for _, fv := range favs {
		if len(pos) == 0 || data.Matches(fv.Title+"\n"+fv.Gist, pos, false) {
			hit = append(hit, fv)
		}
	}
	sort.SliceStable(hit, func(i, j int) bool { return hit[i].Start > hit[j].Start })
	if *f.asJSON {
		if hit == nil {
			hit = []data.Favorite{}
		}
		return writeJSON(e.Stdout, hit)
	}
	o := newOut(e.Stdout)
	for _, fv := range hit {
		a, _ := data.ParseTS(fv.Start)
		b, _ := data.ParseTS(fv.End)
		a, b = a.In(loc), b.In(loc)
		o.f("- %s %s–%s %s\n", a.Format("2006-01-02"), a.Format("15:04"), b.Format("15:04"), fv.Title)
		if fv.Gist != "" {
			o.f("  %s\n", fv.Gist)
		}
	}
	if len(hit) == 0 {
		o.f("没有收藏。\n")
	}
	return o.Flush()
}

// dataLocation is the time zone FanEar wrote the latest summary in, so UTC
// timestamps from the backup print in the user's own clock.
func dataLocation(records string) *time.Location {
	days, _ := data.ListDays(records)
	for i := len(days) - 1; i >= 0; i-- {
		files := data.DayFiles(days[i])
		if files.Summary == "" {
			continue
		}
		var s struct {
			TimeZone string `json:"timeZone"`
		}
		if data.ReadJSON(files.Summary, &s) == nil && s.TimeZone != "" {
			if loc, err := time.LoadLocation(s.TimeZone); err == nil {
				return loc
			}
		}
		break
	}
	return time.Local
}
