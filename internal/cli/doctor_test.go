package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDoctorHealthy(t *testing.T) {
	needsUnix(t)
	setVersion(t, "v0.2.0")
	home := t.TempDir()
	must(t, runWith(t, nil, home, "skill", "install", "--claude"))
	r := runWith(t, nil, home, "doctor", "--root", sample)
	must(t, r,
		"✓ 版本         v0.2.0\n",
		"✓ 数据目录     ",
		"✓ 记录         2 天（2026-11-24 ～ 2026-11-25），最近一天是昨天\n",
		"✓ 格式         11月25日 饭耳记录.json 能读\n",
		"✓ 资料备份     找到了",
		"✓ skill        ~/.agents/skills/fanear 是 v0.2.0，和程序一致\n",
		"✓ Claude Code  ~/.claude/skills/fanear → ~/.agents/skills/fanear\n",
		"结论：一切正常\n")
}

func TestDoctorProblems(t *testing.T) {
	needsUnix(t)
	setVersion(t, "v0.2.0")
	home := t.TempDir()
	// Claude Code is installed but its fanear link points at an unmanaged copy.
	stray := filepath.Join(t.TempDir(), "fanear")
	writeFile(t, filepath.Join(stray, "SKILL.md"), "---\nname: fanear\n---\n")
	link := filepath.Join(home, ".claude", "skills", "fanear")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(stray, link); err != nil {
		t.Fatal(err)
	}
	r := runWith(t, nil, home, "doctor", "--root", sample)
	if r.code != 1 {
		t.Fatalf("want exit 1, got %d:\n%s", r.code, r.stdout)
	}
	for _, want := range []string{
		"! skill        ~/.agents/skills/fanear 还没装；运行 fanear skill install --claude\n",
		"✗ Claude Code  ~/.claude/skills/fanear 指向了别处（" + stray + "），那份不会跟着程序更新；运行 fanear skill install --claude 可修\n",
		"结论：1 个问题，1 个提醒\n",
	} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("output lacks %q:\n%s", want, r.stdout)
		}
	}
	// The suggested fix works.
	must(t, runWith(t, nil, home, "skill", "install", "--claude"))
	must(t, runWith(t, nil, home, "doctor", "--root", sample), "结论：一切正常")

	// No data at all.
	r = runWith(t, nil, home, "doctor", "--root", t.TempDir(), "--json")
	var out struct {
		OK     bool
		Checks []check
	}
	if r.code != 1 || json.Unmarshal([]byte(r.stdout), &out) != nil || out.OK {
		t.Fatalf("exit %d: %s", r.code, r.stdout)
	}
	if c := out.Checks[1]; c.Name != "root" || c.Status != "fail" || !strings.Contains(c.Fix, "fanear where --set") {
		t.Fatalf("root check %+v", c)
	}
}

func TestDoctorStaleRecords(t *testing.T) {
	var out bytes.Buffer
	e := Env{
		Stdout: &out, Stderr: &bytes.Buffer{}, Home: t.TempDir(), GOOS: "linux",
		Getenv: func(string) string { return "" },
		Now:    func() time.Time { return time.Date(2026, 12, 1, 9, 0, 0, 0, time.Local) },
	}
	if code := Run([]string{"doctor", "--root", sample}, e); code != 0 {
		t.Fatalf("a stale day is a warning, not a failure; exit %d:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "! 记录         2 天（2026-11-24 ～ 2026-11-25），最近一天是 11月25日 周三，已经 6 天没更新；看看手机上饭耳有没有同步\n") {
		t.Fatalf("output:\n%s", out.String())
	}
}
