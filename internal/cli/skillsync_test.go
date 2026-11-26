package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dikevin/fanear-cli/skills"
)

func setVersion(t *testing.T, v string) {
	t.Helper()
	old := Version
	Version = v
	t.Cleanup(func() { Version = old })
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func markerVersion(t *testing.T, dir string) string {
	t.Helper()
	var m marker
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(dir, markerFile))), &m); err != nil {
		t.Fatal(err)
	}
	return m.Version
}

func TestSkillRender(t *testing.T) {
	s := skills.Fanear("v0.2.0")
	if !strings.Contains(s, "cmd/fanear@v0.2.0") || strings.Contains(s, "{{") {
		t.Fatalf("version not rendered:\n%s", s)
	}
	if !strings.Contains(skills.Fanear(""), "cmd/fanear@latest") {
		t.Fatal("development builds should point at @latest")
	}
	// A build from a modified checkout is not something go install can fetch.
	setVersion(t, "v0.1.0+dirty")
	if r := runWith(t, nil, "", "skill"); !strings.Contains(r.stdout, "cmd/fanear@latest") {
		t.Fatal("a non-release build should point at @latest")
	}
}

func TestSkillInstallWritesMarker(t *testing.T) {
	setVersion(t, "v0.2.0")
	home := t.TempDir()
	must(t, runWith(t, nil, home, "skill", "install"), "（v0.2.0）")
	dir := filepath.Join(home, ".agents", "skills", "fanear")
	if v := markerVersion(t, dir); v != "v0.2.0" {
		t.Fatalf("marker version %q", v)
	}
	if !strings.Contains(readFile(t, filepath.Join(dir, "SKILL.md")), "@v0.2.0") {
		t.Fatal("installed SKILL.md not rendered for the version")
	}
}

func TestSkillSync(t *testing.T) {
	home := t.TempDir()
	def := filepath.Join(home, ".agents", "skills", "fanear")
	other := filepath.Join(t.TempDir(), "skills")

	// v0.1.0 left a copy without a marker; a later install used --dir.
	writeFile(t, filepath.Join(def, "SKILL.md"), "---\nname: fanear\ndescription: old\n---\n")
	setVersion(t, "v0.1.5")
	must(t, runWith(t, nil, home, "skill", "install", "--dir", other))

	setVersion(t, "v0.2.0")
	r := runWith(t, nil, home, "days", "--root", sample)
	must(t, r, "2026-11-24")
	for _, want := range []string{
		"fanear: 已把 ~/.agents/skills/fanear 的 skill 从 旧版 更新到 v0.2.0\n",
		"的 skill 从 v0.1.5 更新到 v0.2.0\n",
	} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, r.stderr)
		}
	}
	for _, dir := range []string{def, filepath.Join(other, "fanear")} {
		if v := markerVersion(t, dir); v != "v0.2.0" {
			t.Errorf("%s marker %q", dir, v)
		}
	}

	// Up to date now: nothing to say.
	if r := runWith(t, nil, home, "days", "--root", sample); r.stderr != "" {
		t.Fatalf("second run should be quiet: %q", r.stderr)
	}

	// An older binary leaves a newer copy alone.
	setVersion(t, "v0.1.9")
	if r := runWith(t, nil, home, "days", "--root", sample); r.stderr != "" || markerVersion(t, def) != "v0.2.0" {
		t.Fatalf("older binary touched a newer skill: %q", r.stderr)
	}

	// Development builds never rewrite installed skills.
	setVersion(t, "")
	writeFile(t, filepath.Join(def, markerFile), `{"tool":"fanear","version":"v0.0.1"}`)
	if r := runWith(t, nil, home, "days", "--root", sample); r.stderr != "" || markerVersion(t, def) != "v0.0.1" {
		t.Fatalf("development build touched the skill: %q", r.stderr)
	}
}

func TestSkillSyncLeavesOthersAlone(t *testing.T) {
	setVersion(t, "v0.2.0")
	home := t.TempDir()
	def := filepath.Join(home, ".agents", "skills", "fanear")
	writeFile(t, filepath.Join(def, "SKILL.md"), "---\nname: something-else\n---\n")
	// A real (not linked) Claude copy written by fanear is refreshed too.
	claude := filepath.Join(home, ".claude", "skills", "fanear")
	writeFile(t, filepath.Join(claude, "SKILL.md"), "---\nname: fanear\n---\n")
	writeFile(t, filepath.Join(claude, markerFile), `{"tool":"fanear","version":"v0.1.0"}`)

	r := runWith(t, nil, home, "version")
	must(t, r, "fanear v0.2.0")
	if !strings.Contains(r.stderr, "~/.claude/skills/fanear 的 skill 从 v0.1.0 更新到 v0.2.0") || strings.Contains(r.stderr, ".agents") {
		t.Fatalf("stderr %q", r.stderr)
	}
	if readFile(t, filepath.Join(def, "SKILL.md")) != "---\nname: something-else\n---\n" {
		t.Fatal("a skill fanear did not write was overwritten")
	}
}

func TestSkillClaudeRelink(t *testing.T) {
	home := t.TempDir()
	link := filepath.Join(home, ".claude", "skills", "fanear")
	old := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(old, link); err != nil {
		t.Fatal(err)
	}
	must(t, runWith(t, nil, home, "skill", "install", "--claude"), "已把 Claude Code 的链接从 "+old+" 改指向")
	if cur, _ := os.Readlink(link); cur != filepath.Join(home, ".agents", "skills", "fanear") {
		t.Fatalf("link points at %s", cur)
	}

	// A real folder that is not fanear's is never replaced.
	home2 := t.TempDir()
	writeFile(t, filepath.Join(home2, ".claude", "skills", "fanear", "SKILL.md"), "---\nname: mine\n---\n")
	if r := runWith(t, nil, home2, "skill", "install", "--claude"); r.code != 1 || !strings.Contains(r.stderr, "不是 fanear 的 skill，没动它") {
		t.Fatalf("exit %d %q", r.code, r.stderr)
	}
}
