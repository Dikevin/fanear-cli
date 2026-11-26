package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/Dikevin/fanear-cli/internal/source"
	"github.com/Dikevin/fanear-cli/skills"
)

// markerFile sits next to an installed SKILL.md and says which fanear
// version wrote it.
const markerFile = ".installed.json"

type marker struct {
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	InstalledAt string `json:"installedAt"`
}

// programVersion is the version of this binary: the -ldflags value, else
// the version the Go toolchain stamped (a tag, a pseudo-version, or
// v0.1.0+dirty for a modified checkout), else "".
func programVersion() string {
	if Version != "" {
		return Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return ""
}

// displayVersion is programVersion for people: "(devel)" when unversioned.
func displayVersion() string {
	if v := programVersion(); v != "" {
		return v
	}
	return "(devel)"
}

// skillPin is the version SKILL.md tells agents to install: this release,
// or "" (rendered as @latest) for any build that is not one.
func skillPin() string {
	if v := programVersion(); isRelease(v) {
		return v
	}
	return ""
}

var releaseRe = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

// isRelease reports whether v is a tagged release such as v0.2.0, as
// opposed to a pseudo-version from an untagged commit.
func isRelease(v string) bool { return releaseRe.MatchString(v) }

// olderThan reports whether release a predates release b. An empty or
// unparsable a (a copy from before markers existed) counts as older.
func olderThan(a, b string) bool {
	ma, mb := releaseRe.FindStringSubmatch(a), releaseRe.FindStringSubmatch(b)
	if mb == nil {
		return false
	}
	if ma == nil {
		return true
	}
	for i := 1; i <= 3; i++ {
		x, _ := strconv.Atoi(ma[i])
		y, _ := strconv.Atoi(mb[i])
		if x != y {
			return x < y
		}
	}
	return false
}

// installedSkill describes a skill folder fanear may have written.
type installedSkill struct {
	Dir     string
	Exists  bool
	Ours    bool   // written by fanear (marker, or a SKILL.md named fanear)
	Version string // from the marker; "" when written before markers existed
}

func readInstalled(dir string) installedSkill {
	s := installedSkill{Dir: dir}
	b, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return s
	}
	s.Exists = true
	var m marker
	if mb, err := os.ReadFile(filepath.Join(dir, markerFile)); err == nil && json.Unmarshal(mb, &m) == nil && m.Tool == "fanear" {
		s.Ours, s.Version = true, m.Version
		return s
	}
	s.Ours = strings.Contains(string(b), "\nname: fanear\n")
	return s
}

// writeSkill writes this version's SKILL.md and its marker into dir.
func writeSkill(e Env, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skills.Fanear(skillPin())), 0o644); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(marker{Tool: "fanear", Version: displayVersion(), InstalledAt: e.Now().Format(time.RFC3339)}, "", "  ")
	return os.WriteFile(filepath.Join(dir, markerFile), append(b, '\n'), 0o644)
}

// skillDirs lists the folders fanear keeps up to date: the default
// ~/.agents/skills/fanear, a real (not linked) ~/.claude/skills/fanear, and
// every folder `fanear skill install --dir` recorded.
func skillDirs(e Env) []string {
	opts := source.Options{Getenv: e.Getenv, Home: e.Home, GOOS: e.GOOS}
	dirs := []string{defaultSkillDir(e)}
	claude := filepath.Join(e.Home, ".claude", "skills", "fanear")
	if fi, err := os.Lstat(claude); err == nil && fi.Mode()&os.ModeSymlink == 0 {
		dirs = append(dirs, claude)
	}
	seen := map[string]bool{}
	var out []string
	for _, d := range append(dirs, source.SkillDirs(opts)...) {
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out
}

// syncSkills refreshes installed skill copies older than this release, so
// agents never read instructions for a CLI that has moved on. Development
// builds leave them alone. Failures are reported but never stop a command.
func syncSkills(e Env) {
	cur := programVersion()
	if !isRelease(cur) {
		return
	}
	for _, dir := range skillDirs(e) {
		s := readInstalled(dir)
		if !s.Exists || !s.Ours || !olderThan(s.Version, cur) {
			continue
		}
		from := s.Version
		if from == "" {
			from = "旧版"
		}
		if err := writeSkill(e, dir); err != nil {
			// Sandboxed agent shells often cannot write here; one run from a
			// normal terminal fixes it.
			fmt.Fprintf(e.Stderr, "fanear: 没能把 %s 的 skill 从 %s 更新到 %s（%v）；在终端里运行一次 fanear version 即可\n", tilde(e, dir), from, cur, err)
			continue
		}
		fmt.Fprintf(e.Stderr, "fanear: 已把 %s 的 skill 从 %s 更新到 %s\n", tilde(e, dir), from, cur)
	}
}

// tilde shortens a path under the home folder to ~/….
func tilde(e Env, p string) string {
	if rel, err := filepath.Rel(e.Home, p); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.Join("~", rel)
	}
	return p
}
