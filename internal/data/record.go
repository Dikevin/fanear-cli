package data

import (
	"strings"
	"time"
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
	ChunkID         string `json:"chunkID"`
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

// SpeakerAt finds who said the sentence starting at ts in the given chunk,
// for naming who made a promise.
func (r *DayRecord) SpeakerAt(chunkID, ts string) string {
	want, ok := ParseTS(ts)
	if !ok {
		return ""
	}
	for _, c := range r.Day.Conversations {
		for _, s := range c.Sentences {
			if chunkID != "" && s.ChunkID != chunkID {
				continue
			}
			st, ok := ParseTS(s.Start)
			if !ok {
				continue
			}
			if d := st.Sub(want); d > -1500*time.Millisecond && d < 1500*time.Millisecond {
				return s.SpeakerName
			}
		}
	}
	return ""
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
