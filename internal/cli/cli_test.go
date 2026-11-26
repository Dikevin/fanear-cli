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

var sample, _ = filepath.Abs("../../testdata/sample")

type result struct {
	code   int
	stdout string
	stderr string
}

func runWith(t *testing.T, env map[string]string, home string, args ...string) result {
	t.Helper()
	var out, errb bytes.Buffer
	if home == "" {
		home = t.TempDir()
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	e := Env{
		Stdout: &out, Stderr: &errb, Home: home, GOOS: "linux",
		Getenv: func(k string) string { return env[k] },
		Now:    func() time.Time { return time.Date(2026, 11, 26, 10, 0, 0, 0, loc) },
	}
	code := Run(args, e)
	return result{code, out.String(), errb.String()}
}

// run uses the sample data through --root.
func run(t *testing.T, args ...string) result {
	t.Helper()
	return runWith(t, nil, "", append(args, "--root", sample)...)
}

func must(t *testing.T, r result, wants ...string) {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("exit %d, stderr: %s", r.code, r.stderr)
	}
	for _, w := range wants {
		if !strings.Contains(r.stdout, w) {
			t.Errorf("output lacks %q:\n%s", w, r.stdout)
		}
	}
}

func lacks(t *testing.T, r result, nots ...string) {
	t.Helper()
	for _, n := range nots {
		if strings.Contains(r.stdout, n) {
			t.Errorf("output should not contain %q:\n%s", n, r.stdout)
		}
	}
}

func TestWhere(t *testing.T) {
	r := run(t, "where")
	must(t, r, filepath.Join(sample, "Documents", "记录"), "--root 参数", "TEST-DEVICE", "2026-11-24 ～ 2026-11-25，共 2 天")

	var info map[string]any
	r = run(t, "where", "--json")
	must(t, r)
	if err := json.Unmarshal([]byte(r.stdout), &info); err != nil || info["days"].(float64) != 2 {
		t.Fatalf("bad json %v: %s", err, r.stdout)
	}
}

func TestRootFromEnvAndConfig(t *testing.T) {
	r := runWith(t, map[string]string{"FANEAR_ROOT": filepath.Join(sample, "Documents", "记录")}, "", "where")
	must(t, r, "环境变量 FANEAR_ROOT")

	home := t.TempDir()
	r = runWith(t, nil, home, "where", "--set", sample)
	must(t, r, "已记住")
	r = runWith(t, nil, home, "days")
	must(t, r, "2026-11-24", "2026-11-25")

	r = runWith(t, nil, t.TempDir(), "days")
	if r.code != 1 || !strings.Contains(r.stderr, "没找到饭耳的数据目录") {
		t.Fatalf("want not-found error, got %d %q", r.code, r.stderr)
	}
}

func TestDays(t *testing.T) {
	must(t, run(t, "days"), "2026-11-24  11月24日 周二  总结 记录 健康 足迹 录音×1", "2026-11-25  11月25日 周三  总结 记录")
	r := run(t, "days", "--from", "11-25")
	lacks(t, r, "2026-11-24")
}

func TestBrief(t *testing.T) {
	must(t, run(t, "brief", "11-24"), "比较装修报价")
	must(t, run(t, "brief"), "给张三发了报价单") // latest day with a summary
	r := run(t, "brief", "--json", "2026-11-24")
	must(t, r)
	if !json.Valid([]byte(r.stdout)) {
		t.Fatal("brief --json is not JSON")
	}
	r = run(t, "brief", "--json", "11-25")
	if r.code != 1 || !strings.Contains(r.stderr, "还没有饭耳总结") {
		t.Fatalf("want missing-summary error, got %d %q", r.code, r.stderr)
	}
	r = run(t, "brief", "12-31")
	if r.code != 1 || !strings.Contains(r.stderr, "没有记录") {
		t.Fatalf("want no-record error, got %d %q", r.code, r.stderr)
	}
}

func TestTranscript(t *testing.T) {
	r := run(t, "transcript", "11-24", "--from", "10:00", "--to", "10:01")
	must(t, r, "## 10:00–10:02 和张三聊装修", "[10:00:10] 张三：那你明天把报价单发给我。", "★ [10:00:20] 我：行，我明天发你。")
	lacks(t, r, "嗡嗡嗡", "样板间", "中午吃面")

	must(t, run(t, "transcript", "11-24", "--noise"), "嗡嗡嗡", "李四（推测）：我下周去看样板间。")
	r = run(t, "transcript", "11-24", "--me")
	lacks(t, r, "张三：")
	r = run(t, "transcript", "11-24", "--limit", "1")
	must(t, r, "只显示前 1 句")

	var js struct {
		Total     int
		Sentences []sentenceOut
	}
	r = run(t, "transcript", "11-24", "--speaker", "李四", "--json")
	must(t, r)
	if err := json.Unmarshal([]byte(r.stdout), &js); err != nil || js.Total != 1 || !js.Sentences[0].SpeakerInferred {
		t.Fatalf("bad json %v: %s", err, r.stdout)
	}
}

func TestSearch(t *testing.T) {
	r := run(t, "search", "报价单")
	must(t, r, "# 11月25日 周三", "# 11月24日 周二", "（2 条命中）")
	if strings.Index(r.stdout, "11月25日") > strings.Index(r.stdout, "11月24日") {
		t.Errorf("newest day should come first:\n%s", r.stdout)
	}
	r = run(t, "search", "报价", "样板间", "--from", "11-24", "--to", "11-24", "--context", "1", "--noise")
	must(t, r, "→ [10:00:05]", "李四（推测）")
	r = run(t, "search", "报价", "明天", "--and")
	must(t, r, "（1 条命中）")
	r = run(t, "search", "火星")
	must(t, r, "没搜到")
	r = run(t, "search")
	if r.code != 2 {
		t.Fatalf("want usage error, got %d", r.code)
	}
}

func TestTodos(t *testing.T) {
	r := run(t, "todos")
	must(t, r,
		"- [已做完（小耳听到原话）] 把装修报价单发给张三 · 我答应 张三 · 截止 明天（2026-11-25）",
		"> [10:00:20] 我：行，我明天发你。",
		"做完的依据：[09:00:00] 报价单发你微信了。",
		"事情：装修（等张三回复）",
		"[已进待办] 下周去看样板间 · 李四（推测）答应的",
		"[等你确认] 给王五回电话",
		"（3 条）")
	lacks(t, r, "中午吃面")
	must(t, run(t, "todos", "--all"), "[没进待办] 中午吃面", "（4 条）")
	r = run(t, "todos", "--open", "--mine")
	must(t, r, "给王五回电话", "（1 条）")
	must(t, run(t, "todos", "样板间"), "（1 条）")

	var js []todoOut
	r = run(t, "todos", "--done", "--json")
	must(t, r)
	if err := json.Unmarshal([]byte(r.stdout), &js); err != nil || len(js) != 1 || js[0].Done.By != "said" {
		t.Fatalf("bad json %v: %s", err, r.stdout)
	}
}

func TestHealth(t *testing.T) {
	r := run(t, "health", "11-24")
	must(t, r, "心率：平均 75次/分，最低 60，最高 90（2 条）", "血氧：平均 97%", "步数：共 2000（2 条）",
		"睡眠：核心睡眠 1小时30分，深睡 45分钟", "站立小时：站了 1小时")
	r = run(t, "health", "11-24", "--type", "心率", "--limit", "1")
	must(t, r, "[10:00:00–10:00:00] 60次/分 · Apple Watch", "还有 1 条")
	r = run(t, "health", "11-24", "--type", "体重")
	if r.code != 1 || !strings.Contains(r.stderr, "有：心率") {
		t.Fatalf("want unknown-type error, got %d %q", r.code, r.stderr)
	}
	r = run(t, "health", "11-25")
	if r.code != 1 || !strings.Contains(r.stderr, "没有健康数据") {
		t.Fatalf("want missing-health error, got %d %q", r.code, r.stderr)
	}
}

func TestPlaces(t *testing.T) {
	r := run(t, "places", "11-24")
	must(t, r, "坐标 WGS-84", "里程 5.23 公里，移动 30分钟", "00:00–09:30（9小时30分）家  39.90000,116.40000 ±10m", "09:30–10:00 坐车（30分钟）")
	r = run(t, "places", "11-24", "--gcj02", "--route")
	must(t, r, "坐标 GCJ-02", "路线（1 个点）")
	lacks(t, r, "39.90000,116.40000")
}

func TestMemoryAndFavorites(t *testing.T) {
	must(t, run(t, "memory"), "# 饭耳眼中的你")
	r := run(t, "memory", "张三")
	must(t, r, "# 张三", "装修顾问")
	lacks(t, r, "<!-- page")
	must(t, run(t, "memory", "张"), "有 2 页标题带「张」")
	must(t, run(t, "memory", "我"), "最近在比较装修报价")
	must(t, run(t, "memory", "样板间"), "没有叫「样板间」的页面", "[话题] 家居与装修")
	must(t, run(t, "memory", "--list"), "[人物] 张小三")
	must(t, run(t, "favorites"), "- 2026-11-24 10:00–10:02 和张三聊装修")

	// A storage without the restore backup says so instead of guessing.
	r = runWith(t, nil, "", "memory", "--root", filepath.Join(sample, "Documents", "记录", "2026"))
	if r.code != 1 {
		t.Fatalf("want error for a root without records, got %d", r.code)
	}
	tmp := t.TempDir()
	if err := os.CopyFS(filepath.Join(tmp, "饭耳"), os.DirFS(filepath.Join(sample, "Documents"))); err != nil {
		t.Fatal(err)
	}
	r = runWith(t, nil, "", "favorites", "--root", tmp)
	if r.code != 1 || !strings.Contains(r.stderr, "没有饭耳的“换手机找回资料”备份") {
		t.Fatalf("want no-backup error, got %d %q", r.code, r.stderr)
	}
}

func TestSkill(t *testing.T) {
	r := runWith(t, nil, "", "skill")
	must(t, r, "name: fanear")

	home := t.TempDir()
	r = runWith(t, nil, home, "skill", "install", "--claude")
	must(t, r, "已装好", "已给 Claude Code 建链接")
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "fanear", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	r = runWith(t, nil, home, "skill", "install", "--claude")
	must(t, r, "链接已经在")
}

func TestUsage(t *testing.T) {
	r := runWith(t, nil, "")
	must(t, r, "fanear <命令>", "transcript")
	r = runWith(t, nil, "", "nope")
	if r.code != 2 {
		t.Fatalf("want 2, got %d", r.code)
	}
	r = run(t, "transcript", "--from", "25:00")
	if r.code != 2 {
		t.Fatalf("want 2, got %d", r.code)
	}
	r = run(t, "days", "--bogus")
	if r.code != 2 {
		t.Fatalf("want 2, got %d", r.code)
	}
	r = runWith(t, nil, "", "brief", "-h")
	if r.code != 0 || !strings.Contains(r.stderr, "用法：fanear brief") {
		t.Fatalf("want help, got %d %q", r.code, r.stderr)
	}
	must(t, runWith(t, nil, "", "version"), "fanear ")
}
