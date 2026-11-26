package cli

import (
	"encoding/json"
	"testing"
)

func TestClosest(t *testing.T) {
	names := []string{"where", "days", "brief", "transcript", "search", "todos", "doctor"}
	for word, want := range map[string]string{
		"todo": "todos", "trnascript": "transcript", "tran": "transcript", "serch": "search",
		"x": "", "zzzzzz": "", "": "",
	} {
		if got := closest(word, names); got != want {
			t.Errorf("closest(%q) = %q, want %q", word, got, want)
		}
	}
}

func TestSuggestions(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"todo"}, "fanear: 没有 \"todo\" 这个命令，你是不是想输入 todos？\n"},
		{[]string{"xyzzy"}, "fanear: 没有 \"xyzzy\" 这个命令；命令有：where、days、brief、transcript、search、todos、health、places、memory、favorites、skill、doctor、version\n"},
		{[]string{"help", "serch"}, "fanear: 没有 \"serch\" 这个命令，你是不是想输入 search？\n"},
		{[]string{"search", "装修", "--form", "11-24"}, "fanear: 没有 --form 这个参数，你是不是想输入 --from？\n"},
		{[]string{"days", "--from"}, "fanear: --from 后面要跟一个值（fanear days -h 看所有参数）\n"},
		{[]string{"skill", "instal"}, "fanear: skill 没有 \"instal\" 这个子命令，你是不是想输入 install？\n"},
	}
	for _, c := range cases {
		r := runWith(t, nil, "", c.args...)
		if r.code != 2 || r.stderr != c.want || r.stdout != "" {
			t.Errorf("%v: exit %d, stderr %q, stdout %q; want stderr %q", c.args, r.code, r.stderr, r.stdout, c.want)
		}
	}
}

func TestJSONErrors(t *testing.T) {
	type jsonErr struct {
		OK         bool   `json:"ok"`
		Error      string `json:"error"`
		Suggestion string `json:"suggestion"`
	}
	r := runWith(t, nil, "", "todo", "--json")
	var e jsonErr
	if r.code != 2 || json.Unmarshal([]byte(r.stderr), &e) != nil || e.OK || e.Error != "没有 \"todo\" 这个命令" || e.Suggestion != "todos" {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	r = run(t, "search", "装修", "--form", "11-24", "--json")
	e = jsonErr{}
	if r.code != 2 || json.Unmarshal([]byte(r.stderr), &e) != nil || e.Error != "没有 --form 这个参数" || e.Suggestion != "--from" {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	// Failures that are not typos keep exit 1 and carry no suggestion.
	r = run(t, "brief", "2020-01-01", "--json")
	e = jsonErr{}
	if r.code != 1 || json.Unmarshal([]byte(r.stderr), &e) != nil || e.Error == "" || e.Suggestion != "" {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
}
