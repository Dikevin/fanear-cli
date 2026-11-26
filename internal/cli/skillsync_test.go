package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Dikevin/fanear-cli/skills"
)

// native turns a /-separated expectation into this system's path form.
func native(s string) string { return filepath.FromSlash(s) }

// needsUnix skips tests that create symlinks or rely on Unix permissions,
// which Windows runners do not give ordinary processes.
func needsUnix(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs symlinks and Unix permissions")
	}
}

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
	pre, _, _ := strings.Cut(s, "## 选命令")
	if !strings.Contains(pre, "cmd/fanear@v0.2.0") || strings.Count(s, "@v0.2.0") != 1 {
		t.Fatalf("the 前提 install command should be pinned once:\n%s", s)
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
		native("fanear: 已把 ~/.agents/skills/fanear 的 skill 从 旧版 更新到 v0.2.0\n"),
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
	if !strings.Contains(r.stderr, native("~/.claude/skills/fanear 的 skill 从 v0.1.0 更新到 v0.2.0")) || strings.Contains(r.stderr, ".agents") {
		t.Fatalf("stderr %q", r.stderr)
	}
	if readFile(t, filepath.Join(def, "SKILL.md")) != "---\nname: something-else\n---\n" {
		t.Fatal("a skill fanear did not write was overwritten")
	}
}

func TestSkillClaudeRelink(t *testing.T) {
	needsUnix(t)
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

func TestSkillSyncUnwritable(t *testing.T) {
	needsUnix(t)
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	setVersion(t, "v0.2.0")
	home := t.TempDir()
	def := filepath.Join(home, ".agents", "skills", "fanear")
	writeFile(t, filepath.Join(def, "SKILL.md"), "---\nname: fanear\n---\n")
	writeFile(t, filepath.Join(def, markerFile), `{"tool":"fanear","version":"v0.1.0"}`)
	for _, p := range []string{filepath.Join(def, "SKILL.md"), filepath.Join(def, markerFile)} {
		if err := os.Chmod(p, 0o444); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(def, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(def, 0o755) })

	r := runWith(t, nil, home, "days", "--root", sample)
	must(t, r, "2026-11-24")
	if strings.Count(r.stderr, "\n") != 1 || !strings.Contains(r.stderr, native("没能把 ~/.agents/skills/fanear 的 skill 从 v0.1.0 更新到 v0.2.0")) {
		t.Fatalf("want one stderr line, got %q", r.stderr)
	}
}

func TestSkillClaudeCopyOnWindows(t *testing.T) {
	setVersion(t, "v0.2.0")
	home := t.TempDir()
	var out, errb bytes.Buffer
	e := Env{
		Stdout: &out, Stderr: &errb, Home: home, GOOS: "windows",
		Getenv: func(string) string { return "" },
		Now:    func() time.Time { return time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC) },
	}
	if code := Run([]string{"skill", "install", "--claude"}, e); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "这个系统建不了链接，给 Claude Code 放了一份副本") {
		t.Fatalf("output: %s", out.String())
	}
	claude := filepath.Join(home, ".claude", "skills", "fanear")
	if fi, err := os.Lstat(claude); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("want a real folder at %s: %v", claude, err)
	}
	if v := markerVersion(t, claude); v != "v0.2.0" {
		t.Fatalf("copy marker %q", v)
	}
	// The copy is kept current like any other installed copy.
	writeFile(t, filepath.Join(claude, markerFile), `{"tool":"fanear","version":"v0.1.0"}`)
	out.Reset()
	if code := Run([]string{"doctor", "--root", sample}, e); code != 0 || !strings.Contains(errb.String(), "更新到 v0.2.0") {
		t.Fatalf("exit %d, stderr %s", code, errb.String())
	}
	if !strings.Contains(out.String(), native("✓ Claude Code  ~/.claude/skills/fanear 是一份副本，会跟着程序版本更新")) {
		t.Fatalf("doctor:\n%s", out.String())
	}
}
