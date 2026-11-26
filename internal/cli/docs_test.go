package cli

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Every full `fanear …` example in the docs agents and people read must run
// against the sample data: a renamed command or flag fails here, not in an
// agent's hands.

var (
	codeSpan  = regexp.MustCompile("`([^`]+)`")
	docToken  = regexp.MustCompile(`\[[^\]]*\]|\S+`)
	docValues = map[string]string{
		"日期": "2026-11-24", "<日期>": "2026-11-24", "HH:MM": "10:00",
		"名字": "张三", "<名字>": "张三", "词1": "装修", "词2": "报价",
		"<命令>": "brief", "<目录>": sample,
	}
)

// docExamples returns the `fanear …` commands in a markdown file's inline
// code and code blocks.
func docExamples(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ex []string
	keep := func(s string) {
		s = strings.TrimSpace(strings.ReplaceAll(s, `\|`, "|"))
		if strings.HasPrefix(s, "fanear ") {
			ex = append(ex, s)
		}
	}
	block := false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "```") {
			block = !block
			continue
		}
		if block {
			keep(line)
			continue
		}
		for _, m := range codeSpan.FindAllStringSubmatch(line, -1) {
			keep(m[1])
		}
	}
	return ex
}

// expand turns one documented form into runnable argument lists: optional
// [groups] are filled in, each | alternative is tried, placeholders get
// sample values.
func expand(t *testing.T, example string) [][]string {
	t.Helper()
	toks := docToken.FindAllString(strings.TrimPrefix(example, "fanear"), -1)
	n := 1
	for _, tok := range toks {
		if strings.HasPrefix(tok, "[") {
			n = max(n, len(strings.Split(strings.Trim(tok, "[]"), "|")))
		}
	}
	var out [][]string
	for i := 0; i < n; i++ {
		var args []string
		for _, tok := range toks {
			words := []string{tok}
			if strings.HasPrefix(tok, "[") {
				alts := strings.Split(strings.Trim(tok, "[]"), "|")
				words = strings.Fields(alts[min(i, len(alts)-1)])
			}
			for _, w := range words {
				if v, ok := docValues[w]; ok {
					w = v
				} else if strings.ContainsAny(w, "<>") {
					t.Fatalf("%q: no sample value for placeholder %s; add one to docValues", example, w)
				}
				if w != "…" {
					args = append(args, w)
				}
			}
		}
		out = append(out, args)
	}
	return out
}

func TestDocExamplesRun(t *testing.T) {
	documented := map[string]bool{}
	for _, doc := range []string{"../../skills/fanear/SKILL.md", "../../README.md"} {
		examples := docExamples(t, doc)
		if len(examples) < 10 {
			t.Fatalf("%s: found only %d examples, the extractor is probably broken", doc, len(examples))
		}
		for _, ex := range examples {
			for _, args := range expand(t, ex) {
				if len(args) > 0 && doc == "../../README.md" {
					documented[args[0]] = true
				}
				r := runWith(t, nil, "", append(args, "--root", sample)...)
				if r.code != 0 {
					t.Errorf("%s: `%s` (ran fanear %s) exited %d: %s", doc, ex, strings.Join(args, " "), r.code, r.stderr)
				}
			}
		}
	}
	for _, c := range commands() {
		if !documented[c.name] {
			t.Errorf("README.md has no `fanear %s` example", c.name)
		}
	}
}
