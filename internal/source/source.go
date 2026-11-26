// Package source finds the folder FanEar writes its daily records into.
//
// FanEar can save to its own iCloud container, any folder picked in the Files
// app, Google Drive, a WebDAV NAS or S3-compatible storage. fanear only reads a
// local directory, so the records are recognised by their content (a 记录
// folder laid out as 年/月/日) rather than by a fixed path.
package source

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Root is a located FanEar data folder.
type Root struct {
	Records string `json:"records"`           // absolute path of the 记录 folder
	How     string `json:"how"`               // how it was found, for humans
	Restore string `json:"restore,omitempty"` // device folder of the restore backup, if any
}

// Candidate is a records folder found by auto-discovery.
type Candidate struct {
	Label   string `json:"label"`
	Records string `json:"records"`
}

// Options carries everything Locate reads from the environment.
type Options struct {
	Flag   string              // value of --root
	Getenv func(string) string // os.Getenv in production
	Home   string
	GOOS   string
}

// ErrNotFound means no records folder could be located.
var ErrNotFound = errors.New("没找到饭耳的数据目录")

// Locate picks the records folder: --root, then FANEAR_ROOT, then the config
// file, then the first auto-discovered candidate.
func Locate(o Options) (Root, error) {
	if o.Flag != "" {
		return fromPath(o, o.Flag, "--root 参数")
	}
	if v := o.Getenv("FANEAR_ROOT"); v != "" {
		return fromPath(o, v, "环境变量 FANEAR_ROOT")
	}
	cfgPath := ConfigPath(o)
	if cfg, err := readConfig(cfgPath); err == nil && cfg.Root != "" {
		return fromPath(o, cfg.Root, "配置文件 "+cfgPath)
	}
	cands := Discover(o)
	if len(cands) == 0 {
		return Root{}, fmt.Errorf("%w：自动查找的位置里都没有。用 --root、环境变量 FANEAR_ROOT 或 `fanear where --set <目录>` 指定饭耳的保存位置", ErrNotFound)
	}
	c := cands[0]
	return Root{Records: c.Records, How: "自动发现：" + c.Label, Restore: FindRestore(c.Records)}, nil
}

func fromPath(o Options, p, how string) (Root, error) {
	rec, err := Resolve(expandHome(p, o.Home))
	if err != nil {
		return Root{}, fmt.Errorf("%s 指定的目录不能用：%w", how, err)
	}
	return Root{Records: rec, How: how, Restore: FindRestore(rec)}, nil
}

// Resolve turns any folder at or above the records folder into the records
// folder itself. It searches a few levels down, so pointing at the storage
// root (for example a Google Drive or NAS share) is enough.
func Resolve(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("打不开 %s：%w", abs, err)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("%s 不是目录", abs)
	}
	if found := findRecords(abs, 4, 3000); len(found) > 0 {
		return found[0], nil
	}
	return "", fmt.Errorf("在 %s 下没找到饭耳的记录（应有 记录/年/月/日 这样的文件夹）", abs)
}

// Discover lists records folders in the places FanEar data usually syncs to
// on this machine, most likely first.
func Discover(o Options) []Candidate {
	if o.GOOS != "darwin" {
		return nil
	}
	h := o.Home
	var out []Candidate
	seen := map[string]bool{}
	add := func(label string, dirs []string) {
		for _, d := range dirs {
			if !seen[d] {
				seen[d] = true
				out = append(out, Candidate{Label: label, Records: d})
			}
		}
	}
	mobile := filepath.Join(h, "Library", "Mobile Documents")
	add("饭耳的 iCloud 目录", findRecords(filepath.Join(mobile, "iCloud~dev~fandao~fanear", "Documents"), 1, 50))
	add("iCloud 云盘", findRecords(filepath.Join(mobile, "com~apple~CloudDocs"), 3, 1500))
	for _, d := range subdirs(filepath.Join(h, "Library", "CloudStorage")) {
		add(cloudLabel(filepath.Base(d)), findRecords(d, 4, 1500))
	}
	add("Dropbox", findRecords(filepath.Join(h, "Dropbox"), 3, 1500))
	for _, d := range subdirs("/Volumes") {
		switch filepath.Base(d) {
		case "Macintosh HD", "Recovery", "Preboot", "VM", "Data", "Update":
			continue
		}
		add("挂载的磁盘或 NAS（"+filepath.Base(d)+"）", findRecords(d, 3, 1500))
	}
	return out
}

func cloudLabel(name string) string {
	switch {
	case strings.HasPrefix(name, "GoogleDrive"):
		return "Google Drive 桌面版"
	case strings.HasPrefix(name, "OneDrive"):
		return "OneDrive 桌面版"
	case strings.HasPrefix(name, "Dropbox"):
		return "Dropbox 桌面版"
	}
	return "云存储（" + name + "）"
}

func subdirs(dir string) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

var (
	yearRe = regexp.MustCompile(`^\d{4}$`)
	mdRe   = regexp.MustCompile(`^\d{1,2}$`)
)

// dayFileSuffixes are file names FanEar writes into every day folder.
var dayFileSuffixes = []string{"饭耳记录.json", "饭耳记录.md", "饭耳总结.json", "饭耳总结.md", "录音清单.json"}

// findRecords walks breadth-first from base up to maxDepth levels and returns
// every records folder, stopping descent at a match. budget caps how many
// folders are listed, so a large NAS share cannot stall the search.
func findRecords(base string, maxDepth, budget int) []string {
	type item struct {
		dir   string
		depth int
	}
	var out []string
	queue := []item{{base, 0}}
	for len(queue) > 0 && budget > 0 {
		it := queue[0]
		queue = queue[1:]
		budget--
		if IsRecords(it.dir) {
			out = append(out, it.dir)
			continue
		}
		if it.depth >= maxDepth {
			continue
		}
		ents, err := os.ReadDir(it.dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			n := e.Name()
			if !e.IsDir() || strings.HasPrefix(n, ".") || n == "Library" || n == "node_modules" {
				continue
			}
			queue = append(queue, item{filepath.Join(it.dir, n), it.depth + 1})
		}
	}
	return out
}

// IsRecords reports whether dir is a FanEar records folder: it holds 年/月/日
// folders and at least one day folder contains a file FanEar writes.
func IsRecords(dir string) bool {
	years, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for i := len(years) - 1; i >= 0; i-- {
		y := years[i]
		if !y.IsDir() || !yearRe.MatchString(y.Name()) {
			continue
		}
		months, _ := os.ReadDir(filepath.Join(dir, y.Name()))
		for j := len(months) - 1; j >= 0; j-- {
			m := months[j]
			if !m.IsDir() || !mdRe.MatchString(m.Name()) {
				continue
			}
			days, _ := os.ReadDir(filepath.Join(dir, y.Name(), m.Name()))
			for k := len(days) - 1; k >= 0; k-- {
				d := days[k]
				if d.IsDir() && mdRe.MatchString(d.Name()) && hasDayFile(filepath.Join(dir, y.Name(), m.Name(), d.Name())) {
					return true
				}
			}
		}
	}
	return false
}

func hasDayFile(dir string) bool {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range ents {
		for _, suf := range dayFileSuffixes {
			if strings.HasSuffix(e.Name(), suf) {
				return true
			}
		}
	}
	return false
}

// FindRestore returns the device folder of FanEar's "换手机找回资料" backup
// next to the records folder, or "" when this storage has none. The backup is
// recognised by its main-phone.json, so a hidden folder name does not matter.
func FindRestore(records string) string {
	parent := filepath.Dir(records)
	for _, base := range []string{parent, filepath.Dir(parent)} {
		ents, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(base, e.Name())
			if _, err := os.Stat(filepath.Join(dir, "main-phone.json")); err != nil {
				continue
			}
			if dev := pickDevice(dir); dev != "" {
				return dev
			}
		}
	}
	return ""
}

// pickDevice chooses the device folder named by main-phone.json, falling back
// to the only (or alphabetically last) folder that has a manifest.
func pickDevice(dir string) string {
	var mp struct {
		DeviceID string `json:"deviceID"`
	}
	if b, err := os.ReadFile(filepath.Join(dir, "main-phone.json")); err == nil && json.Unmarshal(b, &mp) == nil && mp.DeviceID != "" {
		d := filepath.Join(dir, mp.DeviceID)
		if _, err := os.Stat(filepath.Join(d, "manifest.json")); err == nil {
			return d
		}
	}
	var devs []string
	for _, d := range subdirs(dir) {
		if _, err := os.Stat(filepath.Join(d, "manifest.json")); err == nil {
			devs = append(devs, d)
		}
	}
	sort.Strings(devs)
	if len(devs) == 0 {
		return ""
	}
	return devs[len(devs)-1]
}

type config struct {
	Root string `json:"root"`
}

// ConfigPath is where `fanear where --set` remembers the data folder.
func ConfigPath(o Options) string {
	if x := o.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "fanear", "config.json")
	}
	return filepath.Join(o.Home, ".config", "fanear", "config.json")
}

func readConfig(p string) (config, error) {
	var c config
	b, err := os.ReadFile(p)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("配置文件 %s 不是合法 JSON：%w", p, err)
	}
	return c, nil
}

// SaveRoot validates p and stores it in the config file. It returns the
// records folder p resolves to.
func SaveRoot(o Options, p string) (records, cfgPath string, err error) {
	abs, err := filepath.Abs(expandHome(p, o.Home))
	if err != nil {
		return "", "", err
	}
	records, err = Resolve(abs)
	if err != nil {
		return "", "", err
	}
	cfgPath = ConfigPath(o)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return "", "", err
	}
	b, _ := json.MarshalIndent(config{Root: abs}, "", "  ")
	if err := os.WriteFile(cfgPath, append(b, '\n'), 0o644); err != nil {
		return "", "", err
	}
	return records, cfgPath, nil
}

func expandHome(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}
