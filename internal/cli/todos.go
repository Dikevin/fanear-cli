package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Dikevin/fanear-cli/internal/data"
)

type todoOut struct {
	Date         string     `json:"date"`
	ID           string     `json:"id"`
	State        string     `json:"state"`
	StateLabel   string     `json:"stateLabel"`
	Kind         string     `json:"kind"` // mine or theirs
	Who          string     `json:"who"`
	Counterparty string     `json:"counterparty,omitempty"`
	Summary      string     `json:"summary"`
	Due          string     `json:"due,omitempty"`
	DueDate      string     `json:"dueDate,omitempty"`
	SaidAt       string     `json:"saidAt,omitempty"`
	Quote        string     `json:"quote,omitempty"`
	QuoteSpeaker string     `json:"quoteSpeaker,omitempty"`
	Note         string     `json:"note,omitempty"`
	Matter       string     `json:"matter,omitempty"`
	MatterStatus string     `json:"matterStatus,omitempty"`
	Done         *doneBasis `json:"done,omitempty"`
	at           string
}

type doneBasis struct {
	By      string `json:"by"` // user, said or inferred
	At      string `json:"at,omitempty"`
	Quote   string `json:"quote,omitempty"`
	QuoteAt string `json:"quoteAt,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func cmdTodos(e Env, args []string) error {
	f := newFlags(e, "todos", "todos [--from 日期] [--to 日期] [--open | --done | --all] [--mine | --theirs] [关键词]")
	from := f.String("from", "", "从这天起")
	to := f.String("to", "", "到这天止")
	open := f.Bool("open", false, "只看没做完的（已进待办、等你确认）")
	done := f.Bool("done", false, "只看做完的")
	all := f.Bool("all", false, "也列出饭耳没放进待办、已结束的")
	mine := f.Bool("mine", false, "只看我答应的")
	theirs := f.Bool("theirs", false, "只看别人答应的")
	kws, err := f.parse(args)
	if err != nil {
		return err
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
	keepState := func(s string) bool {
		switch {
		case *open:
			return s == "open" || s == "waiting"
		case *done:
			return s == "done"
		case *all:
			return true
		}
		return s == "open" || s == "waiting" || s == "done"
	}
	var todos []todoOut
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
		var day []todoOut
		for _, r := range rec.Day.Records {
			c := r.Commitment
			if c == nil || !keepState(r.TodoState) {
				continue
			}
			if (*mine && c.Kind != "mine") || (*theirs && c.Kind != "theirs") {
				continue
			}
			if len(kws) > 0 && !data.Matches(c.Summary+"\n"+c.Quote+"\n"+c.Counterparty+"\n"+r.Text, kws, false) {
				continue
			}
			t := todoOut{
				Date: data.FormatDate(d.Date), ID: c.ID, State: r.TodoState,
				StateLabel: data.TodoStateLabel(r.TodoState, r.TodoDoneBasis),
				Kind:       c.Kind, Counterparty: c.Counterparty, Summary: c.Summary,
				Due: c.DueText, DueDate: dateOnly(c.DueDate), SaidAt: c.SourceRecordedAt,
				Quote: c.Quote, QuoteSpeaker: rec.SpeakerAt(c.ChunkID, c.SourceRecordedAt), Note: c.Note, at: r.At,
			}
			if t.Summary == "" {
				t.Summary = r.Title
			}
			t.Who = whoPromised(t)
			if g := r.TodoGrouping; g != nil {
				t.Matter, t.MatterStatus = g.Matter, g.Status
			}
			if bs := r.TodoDoneBasis; bs != nil && r.TodoState == "done" {
				t.Done = &doneBasis{By: bs.By, At: bs.At, Quote: bs.Quote, QuoteAt: bs.QuoteAt, Reason: bs.Reason}
			}
			day = append(day, t)
		}
		sort.SliceStable(day, func(i, j int) bool { return day[i].at < day[j].at })
		todos = append(todos, day...)
	}
	if *f.asJSON {
		if todos == nil {
			todos = []todoOut{}
		}
		return writeJSON(e.Stdout, todos)
	}
	o := newOut(e.Stdout)
	cur := ""
	for _, t := range todos {
		if t.Date != cur {
			cur = t.Date
			date, _ := data.ParseDate(t.Date, e.Now())
			o.f("\n# %s\n\n", data.FormatDateCN(date))
		}
		meta := []string{t.Who}
		if due := dueText(t); due != "" {
			meta = append(meta, due)
		}
		o.f("- [%s] %s · %s\n", t.StateLabel, t.Summary, strings.Join(meta, " · "))
		if t.Quote != "" {
			sp := t.QuoteSpeaker
			if sp == "" {
				sp = "原话"
			}
			o.f("  > [%s] %s：%s\n", data.Clock(t.SaidAt), sp, t.Quote)
		}
		if t.Done != nil {
			if t.Done.Quote != "" {
				o.f("  做完的依据：[%s] %s\n", data.Clock(t.Done.QuoteAt), t.Done.Quote)
			}
			if t.Done.Reason != "" {
				o.f("  小耳的理由：%s\n", t.Done.Reason)
			}
		}
		if t.Matter != "" {
			o.f("  事情：%s", t.Matter)
			if t.MatterStatus != "" {
				o.f("（%s）", t.MatterStatus)
			}
			o.f("\n")
		}
	}
	if len(todos) == 0 {
		o.f("没有符合条件的待办（看了 %d 天）。\n", len(days))
	} else {
		o.f("\n（%d 条）\n", len(todos))
	}
	return o.Flush()
}

func whoPromised(t todoOut) string {
	if t.Kind == "mine" {
		if t.Counterparty != "" {
			return "我答应 " + t.Counterparty
		}
		return "我答应的"
	}
	if sp := t.QuoteSpeaker; sp != "" && !strings.HasPrefix(sp, "我") {
		if last := sp[len(sp)-1]; last >= '0' && last <= '9' {
			return sp + " 答应的"
		}
		return sp + "答应的"
	}
	return "别人答应的"
}

func dueText(t todoOut) string {
	switch {
	case t.Due != "" && t.DueDate != "":
		return fmt.Sprintf("截止 %s（%s）", t.Due, t.DueDate)
	case t.Due != "":
		return "截止 " + t.Due
	case t.DueDate != "":
		return "截止 " + t.DueDate
	}
	return ""
}

func dateOnly(ts string) string {
	if t, ok := data.ParseTS(ts); ok {
		return t.Format("2006-01-02")
	}
	return ts
}
