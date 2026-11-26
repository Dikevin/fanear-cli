package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Dikevin/fanear-cli/internal/data"
)

const upgradeCmd = "GOPROXY=direct go install github.com/Dikevin/fanear-cli/cmd/fanear@latest"

// check is one line of `fanear doctor`.
type check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, warn, fail, info
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
	label  string
}

type doctor struct {
	e      Env
	checks []check
}

func (d *doctor) add(name, label, status, detail, fix string) {
	d.checks = append(d.checks, check{Name: name, Status: status, Detail: detail, Fix: fix, label: label})
}

func cmdDoctor(e Env, args []string) error {
	f := newFlags(e, "doctor", "doctor   （检查数据目录、记录、skill；有问题时退出码 1）")
	pos, err := f.parse(args)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return usageError{msg: "doctor 不带参数"}
	}
	d := &doctor{e: e}
	d.version()
	if root, err := f.locate(e); err != nil {
		d.add("root", "数据目录", "fail", err.Error(), "问清饭耳存在哪（饭耳 › 设置 › 存储），再 fanear where --set <目录>")
	} else {
		d.add("root", "数据目录", "ok", fmt.Sprintf("%s（%s）", tilde(e, root.Records), root.How), "")
		d.records(root.Records)
		if root.Restore != "" {
			d.add("restore", "资料备份", "ok", "找到了，memory、favorites 能用", "")
		} else {
			d.add("restore", "资料备份", "info", "这个保存位置没有（只有 iCloud 和「文件」App 文件夹有），memory、favorites 用不了", "")
		}
	}
	d.skills()
	d.claude()
	return d.report(*f.asJSON)
}

func (d *doctor) version() {
	switch v := programVersion(); {
	case isRelease(v):
		d.add("version", "版本", "ok", v, "")
	default:
		d.add("version", "版本", "info", displayVersion()+"，不是正式版，不会自动更新已装的 skill", "")
	}
}

func (d *doctor) records(dir string) {
	days, err := data.ListDays(dir)
	if err != nil {
		d.add("records", "记录", "fail", err.Error(), "")
		return
	}
	if len(days) == 0 {
		d.add("records", "记录", "fail", "还没有任何一天的记录", "看看手机上饭耳有没有同步到这个保存位置")
		return
	}
	today, _ := data.ParseDate("今天", d.e.Now())
	last := days[len(days)-1].Date
	age := int(today.Sub(last).Hours() / 24)
	span := fmt.Sprintf("%d 天（%s ～ %s）", len(days), data.FormatDate(days[0].Date), data.FormatDate(last))
	switch {
	case age >= 3:
		d.add("records", "记录", "warn", fmt.Sprintf("%s，最近一天是 %s，已经 %d 天没更新", span, data.FormatDateCN(last), age), "看看手机上饭耳有没有同步")
	default:
		d.add("records", "记录", "ok", fmt.Sprintf("%s，最近一天是%s", span, [...]string{"今天", "昨天", "前天"}[max(age, 0)]), "")
	}
	for i := len(days) - 1; i >= 0; i-- {
		path := data.DayFiles(days[i]).Record
		if path == "" {
			continue
		}
		var rec data.DayRecord
		if err := data.ReadJSON(path, &rec); err != nil {
			d.add("format", "格式", "fail", err.Error(), "升级 fanear："+upgradeCmd)
		} else if rec.SchemaVersion != 2 {
			d.add("format", "格式", "warn", fmt.Sprintf("%s 的格式版本是 %d，fanear 按 2 读", filepath.Base(path), rec.SchemaVersion), "升级 fanear："+upgradeCmd)
		} else {
			d.add("format", "格式", "ok", filepath.Base(path)+" 能读", "")
		}
		return
	}
}

func (d *doctor) skills() {
	e, cur := d.e, displayVersion()
	for _, dir := range skillDirs(e) {
		s, at := readInstalled(dir), tilde(e, dir)
		switch {
		case !s.Exists && dir == defaultSkillDir(e):
			d.add("skill", "skill", "warn", at+" 还没装", "运行 fanear skill install --claude")
		case !s.Exists:
			d.add("skill", "skill", "info", "装过的 "+at+" 已经不在了", "")
		case !s.Ours:
			d.add("skill", "skill", "warn", at+" 里不是 fanear 的 skill", "")
		case s.Version == "":
			d.add("skill", "skill", "warn", at+" 是 v0.2.0 以前装的，版本不明", "运行 fanear skill install")
		case s.Version == cur:
			d.add("skill", "skill", "ok", fmt.Sprintf("%s 是 %s，和程序一致", at, cur), "")
		case isRelease(cur) && olderThan(s.Version, cur):
			d.add("skill", "skill", "fail", fmt.Sprintf("%s 是 %s，比程序 %s 旧，没能自动更新", at, s.Version, cur), "运行 fanear skill install")
		case isRelease(cur) && olderThan(cur, s.Version):
			d.add("skill", "skill", "warn", fmt.Sprintf("%s 是 %s，比程序 %s 新", at, s.Version, cur), "升级 fanear："+strings.TrimSuffix(upgradeCmd, "latest")+s.Version)
		default:
			d.add("skill", "skill", "info", fmt.Sprintf("%s 是 %s，程序是 %s", at, s.Version, cur), "想让两边一致就运行 fanear skill install")
		}
	}
}

func (d *doctor) claude() {
	e := d.e
	link := filepath.Join(e.Home, ".claude", "skills", "fanear")
	fi, err := os.Lstat(link)
	if err != nil {
		if _, err := os.Stat(filepath.Join(e.Home, ".claude")); err == nil {
			d.add("claude", "Claude Code", "warn", "~/.claude/skills 里还没有 fanear", "运行 fanear skill install --claude")
		}
		return
	}
	at := tilde(e, link)
	if fi.Mode()&os.ModeSymlink == 0 {
		if readInstalled(link).Ours {
			d.add("claude", "Claude Code", "ok", at+" 是一份副本，会跟着程序版本更新", "")
		} else {
			d.add("claude", "Claude Code", "warn", at+" 不是 fanear 的 skill", "")
		}
		return
	}
	cur, _ := os.Readlink(link)
	if !filepath.IsAbs(cur) {
		cur = filepath.Join(filepath.Dir(link), cur)
	}
	managed := false
	for _, dir := range skillDirs(e) {
		managed = managed || sameDir(dir, cur)
	}
	switch s := readInstalled(cur); {
	case !s.Exists || !s.Ours:
		d.add("claude", "Claude Code", "fail", fmt.Sprintf("%s 指向的 %s 里没有 fanear 的 skill", at, tilde(e, cur)), "运行 fanear skill install --claude 可修")
	case !managed:
		d.add("claude", "Claude Code", "fail", fmt.Sprintf("%s 指向了别处（%s），那份不会跟着程序更新", at, tilde(e, cur)), "运行 fanear skill install --claude 可修")
	default:
		d.add("claude", "Claude Code", "ok", fmt.Sprintf("%s → %s", at, tilde(e, cur)), "")
	}
}

func sameDir(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

func (d *doctor) report(asJSON bool) error {
	fails, warns := 0, 0
	for _, c := range d.checks {
		switch c.Status {
		case "fail":
			fails++
		case "warn":
			warns++
		}
	}
	if asJSON {
		if err := writeJSON(d.e.Stdout, struct {
			OK     bool    `json:"ok"`
			Checks []check `json:"checks"`
		}{fails == 0, d.checks}); err != nil {
			return err
		}
	} else {
		o := newOut(d.e.Stdout)
		mark := map[string]string{"ok": "✓", "warn": "!", "fail": "✗", "info": "•"}
		for _, c := range d.checks {
			line := c.Detail
			if c.Fix != "" {
				line += "；" + c.Fix
			}
			o.f("%s %s %s\n", mark[c.Status], padWidth(c.label, 12), line)
		}
		switch {
		case fails == 0 && warns == 0:
			o.f("结论：一切正常\n")
		case fails == 0:
			o.f("结论：没有问题，%d 个提醒\n", warns)
		case warns == 0:
			o.f("结论：%d 个问题\n", fails)
		default:
			o.f("结论：%d 个问题，%d 个提醒\n", fails, warns)
		}
		if err := o.Flush(); err != nil {
			return err
		}
	}
	if fails > 0 {
		return exitCode(1)
	}
	return nil
}

// padWidth pads s to n terminal columns, counting CJK characters as two.
func padWidth(s string, n int) string {
	w := 0
	for _, r := range s {
		if r >= 0x1100 {
			w += 2
		} else {
			w++
		}
	}
	if w >= n {
		return s + " "
	}
	return s + strings.Repeat(" ", n-w)
}
