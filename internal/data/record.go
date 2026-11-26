package data

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// DayRecord is 「M月D日 饭耳记录.json」 (schema_version 2). Only the fields
// fanear uses are decoded.
type DayRecord struct {
	SchemaVersion int    `json:"schema_version"`
	TimeZone      string `json:"timeZone"`
	Day           struct {
		Date          string         `json:"date"`
		Conversations []Conversation `json:"conversations"`
		Records       []Record       `json:"records"`
	} `json:"day"`
}

// Conversation is one conversation with its sentences.
type Conversation struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Summary   string     `json:"summary"`
	Start     string     `json:"start"`
	End       string     `json:"end"`
	Sentences []Sentence `json:"sentences"`
}

// Sentence is one transcribed sentence. SpeakerName already carries
// 「（推测）」 when SpeakerInferred is set.
type Sentence struct {
	ID              string `json:"id"`
	Start           string `json:"start"`
	End             string `json:"end"`
	Text            string `json:"text"`
	SpeakerName     string `json:"speakerName"`
	IsMe            bool   `json:"isMe"`
	SpeakerInferred bool   `json:"speakerInferred"`
	Highlighted     bool   `json:"highlighted"`
	Noise           bool   `json:"noise"`
	AsrLoop         bool   `json:"asrLoop"` // speech recognition got stuck repeating a phrase
	ChunkID         string `json:"chunkID"`
}

// DisplayText is the sentence as 饭耳记录.md shows it: trimmed, and a stuck
// recognition loop summarised as 「这里转文字卡住了，「X」重复了 N 次」.
func (s Sentence) DisplayText() string {
	t := strings.TrimSpace(s.Text)
	if !s.AsrLoop {
		return t
	}
	counts := map[string]int{}
	best, n := "", 0
	for _, part := range strings.FieldsFunc(t, func(r rune) bool { return strings.ContainsRune("，,、。 ", r) }) {
		counts[part]++
		if counts[part] > n {
			best, n = part, counts[part]
		}
	}
	if n < 3 {
		return t
	}
	return fmt.Sprintf("这里转文字卡住了，「%s」重复了 %d 次", best, n)
}

// Record is one entry of day.records; kind is chunkSummaries, periodCards,
// episodeNotes or commitments.
type Record struct {
	ID            string      `json:"id"`
	Kind          string      `json:"kind"`
	Title         string      `json:"title"`
	Text          string      `json:"text"`
	At            string      `json:"at"`
	Commitment    *Commitment `json:"commitment"`
	TodoState     string      `json:"todoState"`
	TodoDoneBasis *DoneBasis  `json:"todoDoneBasis"`
	TodoGrouping  *struct {
		Matter string `json:"matter"`
		Status string `json:"status"`
	} `json:"todoGrouping"`
}

// Commitment is a todo card as FanEar keeps it.
type Commitment struct {
	ID               string `json:"id"`
	Kind             string `json:"kind"` // mine: I promised; theirs: someone else did
	Summary          string `json:"summary"`
	Quote            string `json:"quote"`
	SaidTo           string `json:"saidTo"`
	Counterparty     string `json:"counterparty"`
	DueText          string `json:"dueText"`
	DueDate          string `json:"dueDate"`
	Note             string `json:"note"`
	ChunkID          string `json:"chunkID"`
	SourceRecordedAt string `json:"sourceRecordedAt"`
	Status           struct {
		Kind string `json:"kind"`
	} `json:"status"`
}

// DoneBasis records who marked a todo done: by is user, said or inferred.
type DoneBasis struct {
	By      string `json:"by"`
	Source  string `json:"source"`
	At      string `json:"at"`
	Quote   string `json:"quote"`
	QuoteAt string `json:"quoteAt"`
	Reason  string `json:"reason"`
}

// TodoStateLabel words a todo state the way 「饭耳记录.md」 does.
func TodoStateLabel(state string, basis *DoneBasis) string {
	switch state {
	case "open":
		return "已进待办"
	case "waiting":
		return "等你确认"
	case "done":
		if basis != nil {
			switch basis.By {
			case "user":
				return "已做完（自己点的）"
			case "said":
				return "已做完（小耳听到原话）"
			case "inferred":
				return "已做完（小耳推测）"
			}
		}
		return "已做完"
	case "hidden":
		return "没进待办"
	case "ended":
		return "已结束"
	}
	return state
}

// QuoteSentence finds the sentence a todo quotes. FanEar stores the quote's
// time truncated to the second, and someone else may have spoken a moment
// earlier, so it prefers the sentence whose text matches the quote and only
// then the one closest in time (within a few seconds, in the same chunk).
func (r *DayRecord) QuoteSentence(chunkID, ts, quote string) (Sentence, bool) {
	want, ok := ParseTS(ts)
	if !ok {
		return Sentence{}, false
	}
	q := normText(quote)
	var best Sentence
	bestScore, bestDist, found := 3, time.Duration(0), false
	for _, c := range r.Day.Conversations {
		for _, s := range c.Sentences {
			if chunkID != "" && s.ChunkID != chunkID {
				continue
			}
			st, ok := ParseTS(s.Start)
			if !ok {
				continue
			}
			dist := st.Sub(want)
			if dist < 0 {
				dist = -dist
			}
			if dist > 5*time.Second {
				continue
			}
			t := normText(s.Text)
			score := 2
			switch {
			case q != "" && t == q:
				score = 0
			case q != "" && t != "" && (strings.Contains(t, q) || strings.Contains(q, t)):
				score = 1
			}
			if !found || score < bestScore || (score == bestScore && dist < bestDist) {
				best, bestScore, bestDist, found = s, score, dist, true
			}
		}
	}
	return best, found
}

// normText drops punctuation and spaces so a quote matches its sentence.
func normText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// Matches reports whether text contains any (or, with all, every) keyword,
// ignoring letter case.
func Matches(text string, kws []string, all bool) bool {
	t := strings.ToLower(text)
	for _, k := range kws {
		hit := strings.Contains(t, strings.ToLower(k))
		if all && !hit {
			return false
		}
		if !all && hit {
			return true
		}
	}
	return all && len(kws) > 0
}
