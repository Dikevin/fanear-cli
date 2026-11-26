package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/Dikevin/fanear-cli/internal/data"
)

// sentenceOut is one sentence in JSON output.
type sentenceOut struct {
	Date              string `json:"date"`
	Time              string `json:"time"`
	Start             string `json:"start"`
	Speaker           string `json:"speaker"`
	IsMe              bool   `json:"isMe"`
	SpeakerInferred   bool   `json:"speakerInferred,omitempty"`
	Highlighted       bool   `json:"highlighted,omitempty"`
	Text              string `json:"text"`
	ConversationID    string `json:"conversationId"`
	ConversationTitle string `json:"conversationTitle"`
	Match             bool   `json:"match,omitempty"`
	Context           bool   `json:"context,omitempty"`
}

func toOut(date time.Time, c data.Conversation, s data.Sentence) sentenceOut {
	return sentenceOut{
		Date: data.FormatDate(date), Time: data.Clock(s.Start), Start: s.Start,
		Speaker: s.SpeakerName, IsMe: s.IsMe, SpeakerInferred: s.SpeakerInferred, Highlighted: s.Highlighted,
		Text: s.Text, ConversationID: c.ID, ConversationTitle: c.Title,
	}
}

func sentenceLine(s data.Sentence) string {
	star := ""
	if s.Highlighted {
		star = "★ "
	}
	return fmt.Sprintf("%s[%s] %s：%s", star, data.Clock(s.Start), s.SpeakerName, s.Text)
}

func convHeader(c data.Conversation) string {
	return fmt.Sprintf("%s–%s %s", data.HM(c.Start), data.HM(c.End), c.Title)
}

// speakerFilter keeps sentences by me or by a named speaker.
type speakerFilter struct {
	me      bool
	speaker string
	noise   bool
}

func (sf speakerFilter) keep(s data.Sentence) bool {
	if s.Noise && !sf.noise {
		return false
	}
	if sf.me && !s.IsMe {
		return false
	}
	if sf.speaker != "" && !strings.Contains(s.SpeakerName, sf.speaker) {
		return false
	}
	return true
}

func cmdTranscript(e Env, args []string) error {
	f := newFlags(e, "transcript", "transcript [日期] [--from 14:00] [--to 15:30] [--me] [--speaker 名字]")
	from := f.String("from", "", "从这个时刻起，如 14:00")
	to := f.String("to", "", "到这个时刻止，如 15:30")
	me := f.Bool("me", false, "只看我说的")
	speaker := f.String("speaker", "", "只看名字里带这几个字的说话人")
	noise := f.Bool("noise", false, "也显示饭耳标成噪声的句子")
	limit := f.Int("limit", 400, "最多输出多少句，0 表示不限")
	pos, err := f.parse(args)
	if err != nil {
		return err
	}
	var lo, hi time.Duration = -1, -1
	if *from != "" {
		if lo, err = data.ParseClock(*from); err != nil {
			return usageError{err.Error()}
		}
	}
	if *to != "" {
		if hi, err = data.ParseClock(*to); err != nil {
			return usageError{err.Error()}
		}
	}
	_, days, err := f.openDays(e)
	if err != nil {
		return err
	}
	d, files, err := pickDay(e, days, pos, func(x data.Files) bool { return x.Record != "" })
	if err != nil {
		return err
	}
	if files.Record == "" {
		return fmt.Errorf("%s 没有饭耳记录", data.FormatDate(d.Date))
	}
	var rec data.DayRecord
	if err := data.ReadJSON(files.Record, &rec); err != nil {
		return err
	}
	sf := speakerFilter{me: *me, speaker: *speaker, noise: *noise}
	inWindow := func(s data.Sentence) bool {
		t, ok := data.ParseTS(s.Start)
		if !ok {
			return lo < 0 && hi < 0
		}
		off := data.SinceMidnight(t)
		return (lo < 0 || off >= lo) && (hi < 0 || off < hi)
	}
	var js []sentenceOut
	o := newOut(e.Stdout)
	n, total := 0, 0
	for _, c := range rec.Day.Conversations {
		header := false
		for _, s := range c.Sentences {
			if !sf.keep(s) || !inWindow(s) {
				continue
			}
			total++
			if *limit > 0 && n >= *limit {
				continue
			}
			n++
			if *f.asJSON {
				js = append(js, toOut(d.Date, c, s))
				continue
			}
			if !header {
				o.f("\n## %s\n", convHeader(c))
				if c.Summary != "" {
					o.f("%s\n", c.Summary)
				}
				o.f("\n")
				header = true
			}
			o.f("%s\n", sentenceLine(s))
		}
	}
	if *f.asJSON {
		if js == nil {
			js = []sentenceOut{}
		}
		return writeJSON(e.Stdout, map[string]any{"date": data.FormatDate(d.Date), "timeZone": rec.TimeZone, "total": total, "sentences": js})
	}
	if total == 0 {
		o.f("%s 这个范围里没有句子。\n", data.FormatDate(d.Date))
	} else {
		o.f("\n（%s，%d 句", data.FormatDateCN(d.Date), total)
		if n < total {
			o.f("，只显示前 %d 句；用 --from/--to 缩小时间段，或 --limit 0 全部输出", n)
		}
		o.f("）\n")
	}
	return o.Flush()
}

func cmdSearch(e Env, args []string) error {
	f := newFlags(e, "search", "search <关键词>... [--from 日期] [--to 日期] [--and] [--me] [--speaker 名字] [--context N]")
	from := f.String("from", "", "从这天起")
	to := f.String("to", "", "到这天止")
	and := f.Bool("and", false, "几个关键词都要出现（默认出现任意一个就算）")
	me := f.Bool("me", false, "只搜我说的")
	speaker := f.String("speaker", "", "只搜名字里带这几个字的说话人")
	ctxN := f.Int("context", 0, "每个命中前后各带几句")
	noise := f.Bool("noise", false, "也搜饭耳标成噪声的句子")
	limit := f.Int("limit", 100, "最多输出多少条命中，0 表示不限")
	kws, err := f.parse(args)
	if err != nil {
		return err
	}
	if len(kws) == 0 {
		return usageError{"要给至少一个关键词，比如 fanear search 装修 报价"}
	}
	a, b, err := dateRange(e, *from, *to)
	if err != nil {
		return err
	}
	_, days, err := f.openDays(e)
	if err != nil {
		return err
	}
	days = data.FilterDays(days, a, b)
	sf := speakerFilter{me: *me, speaker: *speaker, noise: *noise}
	o := newOut(e.Stdout)
	var js []sentenceOut
	hits, shown := 0, 0
	for i := len(days) - 1; i >= 0; i-- {
		d := days[i]
		files := data.DayFiles(d)
		if files.Record == "" {
			continue
		}
		var rec data.DayRecord
		if err := data.ReadJSON(files.Record, &rec); err != nil {
			return err
		}
		dayHeader := false
		// Latest conversation first, so --limit keeps the most recent hits.
		for ci := len(rec.Day.Conversations) - 1; ci >= 0; ci-- {
			c := rec.Day.Conversations[ci]
			var idx []int
			for j, s := range c.Sentences {
				if sf.keep(s) && data.Matches(s.Text, kws, *and) {
					idx = append(idx, j)
				}
			}
			if len(idx) == 0 {
				continue
			}
			hits += len(idx)
			if *limit > 0 && shown >= *limit {
				continue
			}
			if *limit > 0 && shown+len(idx) > *limit {
				idx = idx[:*limit-shown]
			}
			shown += len(idx)
			// Expand to context and keep sentence order.
			keep := map[int]bool{}
			match := map[int]bool{}
			for _, j := range idx {
				match[j] = true
				for k := j - *ctxN; k <= j+*ctxN; k++ {
					if k >= 0 && k < len(c.Sentences) && (!c.Sentences[k].Noise || *noise) {
						keep[k] = true
					}
				}
			}
			if !*f.asJSON {
				if !dayHeader {
					o.f("\n# %s\n", data.FormatDateCN(d.Date))
					dayHeader = true
				}
				o.f("\n## %s\n", convHeader(c))
			}
			prev := -2
			for j := range c.Sentences {
				if !keep[j] {
					continue
				}
				s := c.Sentences[j]
				if *f.asJSON {
					so := toOut(d.Date, c, s)
					so.Match, so.Context = match[j], !match[j]
					js = append(js, so)
					continue
				}
				if *ctxN > 0 && prev >= 0 && j != prev+1 {
					o.f("  ……\n")
				}
				mark := "  "
				if match[j] && *ctxN > 0 {
					mark = "→ "
				}
				o.f("%s%s\n", mark, sentenceLine(s))
				prev = j
			}
		}
	}
	if *f.asJSON {
		if js == nil {
			js = []sentenceOut{}
		}
		return writeJSON(e.Stdout, map[string]any{"keywords": kws, "hits": hits, "shown": shown, "sentences": js})
	}
	if hits == 0 {
		o.f("没搜到：%s（搜了 %d 天）。可以换个说法，转写可能有错字。\n", strings.Join(kws, " / "), len(days))
	} else {
		o.f("\n（%d 条命中", hits)
		if shown < hits {
			o.f("，只显示最近的 %d 条；用 --from/--to 缩小日期，或 --limit 0 全部输出", shown)
		}
		o.f("）\n")
	}
	return o.Flush()
}
