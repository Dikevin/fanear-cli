// Package cli implements the fanear commands.
package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/Dikevin/fanear-cli/internal/data"
	"github.com/Dikevin/fanear-cli/internal/source"
)

// Env is what the commands read from and write to; tests replace it.
type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	Getenv func(string) string
	Home   string
	GOOS   string
	Now    func() time.Time
}

// Version is set with -ldflags "-X github.com/Dikevin/fanear-cli/internal/cli.Version=…".
var Version = ""

// Main runs fanear with the process environment.
func Main() int {
	home, _ := os.UserHomeDir()
	return Run(os.Args[1:], Env{
		Stdout: os.Stdout, Stderr: os.Stderr, Getenv: os.Getenv,
		Home: home, GOOS: runtime.GOOS, Now: time.Now,
	})
}

type command struct {
	name    string
	summary string
	run     func(e Env, args []string) error
}

func commands() []command {
	return []command{
		{"where", "数据目录在哪、怎么找到的、有哪些天；--set 记住目录", cmdWhere},
		{"days", "列出有记录的日期和各自的文件", cmdDays},
		{"brief", "某天的总结（饭耳总结.md 原文）", cmdBrief},
		{"transcript", "某天的逐句转写，可按时间段截取", cmdTranscript},
		{"search", "跨天搜原句", cmdSearch},
		{"todos", "待办，以及做完的依据", cmdTodos},
		{"health", "某天的健康数据汇总", cmdHealth},
		{"places", "某天去过哪、停了多久", cmdPlaces},
		{"memory", "小耳的记忆：关于你、人物和话题档案", cmdMemory},
		{"favorites", "收藏的对话", cmdFavorites},
		{"skill", "安装给 agent 用的 skill", cmdSkill},
		{"version", "版本", cmdVersion},
	}
}

// usageError marks a mistake in how fanear was called (exit status 2).
type usageError struct{ msg string }

func (u usageError) Error() string { return u.msg }

// Run executes one fanear command and returns the exit status.
func Run(args []string, e Env) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		if len(args) > 1 {
			args = []string{args[1], "-h"}
		} else {
			printHelp(e.Stdout)
			return 0
		}
	}
	if args[0] == "--version" || args[0] == "-v" {
		args[0] = "version"
	}
	for _, c := range commands() {
		if c.name != args[0] {
			continue
		}
		err := c.run(e, args[1:])
		switch {
		case err == nil:
			return 0
		case errors.Is(err, flag.ErrHelp):
			return 0
		}
		fmt.Fprintln(e.Stderr, "fanear:", err)
		var ue usageError
		if errors.As(err, &ue) {
			return 2
		}
		return 1
	}
	fmt.Fprintf(e.Stderr, "fanear: 没有 %q 这个命令\n\n", args[0])
	printHelp(e.Stderr)
	return 2
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, `fanear：读饭耳（FanEar）写到保存位置里的记录。非官方工具，只读，不联网。

用法：fanear <命令> [参数]

命令：
`)
	for _, c := range commands() {
		fmt.Fprintf(w, "  %-11s %s\n", c.name, c.summary)
	}
	fmt.Fprint(w, `
通用参数：
  --root 目录   饭耳的保存位置（也可用环境变量 FANEAR_ROOT 或 fanear where --set）
  --json        输出 JSON

日期写法：今天、昨天、前天、2026-11-24、11-24、11月24日
每个命令的参数：fanear <命令> -h
`)
}

// cmdFlags is a FlagSet with the flags every data command takes.
type cmdFlags struct {
	*flag.FlagSet
	root   *string
	asJSON *bool
}

func newFlags(e Env, name, usage string) *cmdFlags {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(e.Stderr)
	f := &cmdFlags{FlagSet: fs}
	f.root = fs.String("root", "", "饭耳的保存位置（可以指到它上面任意一层）")
	f.asJSON = fs.Bool("json", false, "输出 JSON")
	fs.Usage = func() {
		fmt.Fprintf(e.Stderr, "用法：fanear %s\n\n参数：\n", usage)
		fs.PrintDefaults()
	}
	return f
}

// parse accepts flags before, between and after positional arguments.
func (f *cmdFlags) parse(args []string) ([]string, error) {
	var pos []string
	for {
		if err := f.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil, err
			}
			return nil, usageError{err.Error()}
		}
		rest := f.Args()
		if len(rest) == 0 {
			return pos, nil
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
}

func (f *cmdFlags) locate(e Env) (source.Root, error) {
	return source.Locate(source.Options{Flag: *f.root, Getenv: e.Getenv, Home: e.Home, GOOS: e.GOOS})
}

// openDays locates the data and lists its days.
func (f *cmdFlags) openDays(e Env) (source.Root, []data.Day, error) {
	root, err := f.locate(e)
	if err != nil {
		return root, nil, err
	}
	days, err := data.ListDays(root.Records)
	if err != nil {
		return root, nil, err
	}
	if len(days) == 0 {
		return root, nil, fmt.Errorf("%s 里还没有任何一天的记录", root.Records)
	}
	return root, days, nil
}

// pickDay resolves an optional date argument; without one it takes the
// latest day that satisfies has.
func pickDay(e Env, days []data.Day, pos []string, has func(data.Files) bool) (data.Day, data.Files, error) {
	if len(pos) > 1 {
		return data.Day{}, data.Files{}, usageError{"只能给一个日期"}
	}
	if len(pos) == 1 {
		date, err := data.ParseDate(pos[0], e.Now())
		if err != nil {
			return data.Day{}, data.Files{}, usageError{err.Error()}
		}
		d, err := data.FindDay(days, date)
		if err != nil {
			return d, data.Files{}, err
		}
		return d, data.DayFiles(d), nil
	}
	for i := len(days) - 1; i >= 0; i-- {
		if f := data.DayFiles(days[i]); has(f) {
			return days[i], f, nil
		}
	}
	return days[len(days)-1], data.DayFiles(days[len(days)-1]), nil
}

// dateRange parses --from/--to dates; empty means unbounded.
func dateRange(e Env, from, to string) (time.Time, time.Time, error) {
	var a, b time.Time
	var err error
	if from != "" {
		if a, err = data.ParseDate(from, e.Now()); err != nil {
			return a, b, usageError{err.Error()}
		}
	}
	if to != "" {
		if b, err = data.ParseDate(to, e.Now()); err != nil {
			return a, b, usageError{err.Error()}
		}
	}
	return a, b, nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// out buffers text output; call flush once at the end.
type out struct{ *bufio.Writer }

func newOut(w io.Writer) out { return out{bufio.NewWriter(w)} }

func (o out) f(format string, a ...any) { fmt.Fprintf(o, format, a...) }

func cmdVersion(e Env, args []string) error {
	v := Version
	if v == "" {
		v = "(devel)"
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
			v = bi.Main.Version
		}
	}
	fmt.Fprintln(e.Stdout, "fanear", v)
	return nil
}

func cmdWhere(e Env, args []string) error {
	f := newFlags(e, "where", "where [--all] [--set 目录]")
	all := f.Bool("all", false, "列出自动查找到的所有位置")
	set := f.String("set", "", "记住这个目录，以后默认用它")
	if _, err := f.parse(args); err != nil {
		return err
	}
	opts := source.Options{Flag: *f.root, Getenv: e.Getenv, Home: e.Home, GOOS: e.GOOS}
	if *set != "" {
		rec, cfg, err := source.SaveRoot(opts, *set)
		if err != nil {
			return err
		}
		if *f.asJSON {
			return writeJSON(e.Stdout, map[string]string{"records": rec, "config": cfg})
		}
		fmt.Fprintf(e.Stdout, "已记住：%s\n记录目录：%s\n配置文件：%s\n", *set, rec, cfg)
		return nil
	}
	if *all {
		cands := source.Discover(opts)
		if *f.asJSON {
			if cands == nil {
				cands = []source.Candidate{}
			}
			return writeJSON(e.Stdout, cands)
		}
		if len(cands) == 0 {
			fmt.Fprintln(e.Stdout, "自动查找的位置里都没有饭耳的记录。")
		}
		for _, c := range cands {
			fmt.Fprintf(e.Stdout, "%s\t%s\n", c.Label, c.Records)
		}
		return nil
	}
	root, err := f.locate(e)
	if err != nil {
		return err
	}
	days, _ := data.ListDays(root.Records)
	info := struct {
		source.Root
		Config string `json:"config"`
		Days   int    `json:"days"`
		First  string `json:"first,omitempty"`
		Last   string `json:"last,omitempty"`
	}{Root: root, Config: source.ConfigPath(opts), Days: len(days)}
	if len(days) > 0 {
		info.First, info.Last = data.FormatDate(days[0].Date), data.FormatDate(days[len(days)-1].Date)
	}
	if *f.asJSON {
		return writeJSON(e.Stdout, info)
	}
	o := newOut(e.Stdout)
	o.f("记录目录：%s\n", root.Records)
	o.f("怎么找到的：%s\n", root.How)
	if root.Restore != "" {
		o.f("资料备份：%s（memory、favorites 读这里）\n", root.Restore)
	} else {
		o.f("资料备份：没有（这个保存位置没有“换手机找回资料”的备份，memory、favorites 用不了）\n")
	}
	if len(days) > 0 {
		o.f("有记录的日期：%s ～ %s，共 %d 天\n", info.First, info.Last, len(days))
	} else {
		o.f("有记录的日期：还没有\n")
	}
	return o.Flush()
}

func cmdDays(e Env, args []string) error {
	f := newFlags(e, "days", "days [--from 日期] [--to 日期]")
	from := f.String("from", "", "从这天起")
	to := f.String("to", "", "到这天止")
	if _, err := f.parse(args); err != nil {
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
	type row struct {
		Date string `json:"date"`
		Dir  string `json:"dir"`
		data.Files
	}
	var rows []row
	for _, d := range days {
		rows = append(rows, row{data.FormatDate(d.Date), d.Dir, data.DayFiles(d)})
	}
	if *f.asJSON {
		if rows == nil {
			rows = []row{}
		}
		return writeJSON(e.Stdout, rows)
	}
	o := newOut(e.Stdout)
	for i, r := range rows {
		var have []string
		for _, x := range []struct {
			ok   bool
			name string
		}{{r.Summary != "" || r.SummaryMD != "", "总结"}, {r.Record != "" || r.RecordMD != "", "记录"}, {r.Health != "", "健康"}, {r.Footprint != "", "足迹"}} {
			if x.ok {
				have = append(have, x.name)
			}
		}
		if r.Recordings > 0 {
			have = append(have, fmt.Sprintf("录音×%d", r.Recordings))
		}
		o.f("%s  %s  %s\n", r.Date, data.FormatDateCN(days[i].Date), strings.Join(have, " "))
	}
	return o.Flush()
}

func cmdBrief(e Env, args []string) error {
	f := newFlags(e, "brief", "brief [日期]   （不给日期就是最近有总结的一天）")
	pos, err := f.parse(args)
	if err != nil {
		return err
	}
	_, days, err := f.openDays(e)
	if err != nil {
		return err
	}
	d, files, err := pickDay(e, days, pos, func(x data.Files) bool { return x.SummaryMD != "" || x.Summary != "" })
	if err != nil {
		return err
	}
	path := files.SummaryMD
	if *f.asJSON {
		path = files.Summary
	}
	if path == "" {
		return fmt.Errorf("%s 还没有饭耳总结（小耳一般当晚写好，没写成的第二天补）；逐句内容用 fanear transcript %s", data.FormatDate(d.Date), data.FormatDate(d.Date))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if *f.asJSON && !json.Valid(b) {
		return fmt.Errorf("%s 不是合法 JSON", path)
	}
	_, err = e.Stdout.Write(b)
	return err
}

// sortedKeys returns map keys in order, for stable output.
func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
