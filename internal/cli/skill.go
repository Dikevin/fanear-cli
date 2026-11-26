package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Dikevin/fanear-cli/internal/source"
	"github.com/Dikevin/fanear-cli/skills"
)

func cmdSkill(e Env, args []string) error {
	f := newFlags(e, "skill", "skill [install [--dir 目录] [--claude]]   （不带 install 就打印 SKILL.md）")
	dir := f.String("dir", "", "装到哪个 skills 目录（默认 ~/.agents/skills，codex 和 agy 读这里）")
	claude := f.Bool("claude", false, "再在 ~/.claude/skills 里建一个指向它的链接，给 Claude Code 用")
	pos, err := f.parse(args)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		_, err := io.WriteString(e.Stdout, skills.Fanear(skillPin()))
		return err
	}
	if pos[0] != "install" {
		ue := usageError{fmt.Sprintf("skill 没有 %q 这个子命令", pos[0]), closest(pos[0], []string{"install"})}
		if ue.suggestion == "" {
			ue.msg += "，只有 install"
		}
		return ue
	}
	if len(pos) > 1 {
		return usageError{msg: "用法：fanear skill install [--dir 目录] [--claude]"}
	}
	base := *dir
	if base == "" {
		base = filepath.Join(e.Home, ".agents", "skills")
	}
	target, err := filepath.Abs(filepath.Join(base, "fanear"))
	if err != nil {
		return err
	}
	if err := writeSkill(e, target); err != nil {
		return err
	}
	fmt.Fprintf(e.Stdout, "已装好：%s（%s）\n", filepath.Join(target, "SKILL.md"), displayVersion())
	if target != defaultSkillDir(e) {
		// Remember it so later versions keep this copy current too.
		opts := source.Options{Getenv: e.Getenv, Home: e.Home, GOOS: e.GOOS}
		if err := source.RecordSkillDir(opts, target); err != nil {
			fmt.Fprintf(e.Stderr, "fanear: 没能记下 %s，以后升级不会自动更新它：%v\n", target, err)
		}
	}
	link := filepath.Join(e.Home, ".claude", "skills", "fanear")
	if !*claude {
		fmt.Fprintf(e.Stdout, "Claude Code 读 ~/.claude/skills；还没装的话加 --claude\n")
		return nil
	}
	fi, err := os.Lstat(link)
	switch {
	case err == nil && fi.Mode()&os.ModeSymlink != 0:
		cur, _ := os.Readlink(link)
		if cur == target {
			fmt.Fprintf(e.Stdout, "Claude Code 的链接已经在：%s\n", link)
			return nil
		}
		if err := os.Remove(link); err != nil {
			return err
		}
		if err := os.Symlink(target, link); err != nil {
			return err
		}
		fmt.Fprintf(e.Stdout, "已把 Claude Code 的链接从 %s 改指向 %s\n", cur, target)
		return nil
	case err == nil:
		if !readInstalled(link).Ours {
			return fmt.Errorf("%s 已经存在，不是 fanear 的 skill，没动它", link)
		}
		// A copy (for example where links are not allowed): refresh it.
		if err := writeSkill(e, link); err != nil {
			return err
		}
		fmt.Fprintf(e.Stdout, "Claude Code 那边是一份副本，已更新：%s\n", link)
		return nil
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	// Windows only allows links with developer mode or admin rights; a copy
	// works everywhere and is kept current like any other installed copy.
	if e.GOOS == "windows" || os.Symlink(target, link) != nil {
		if err := writeSkill(e, link); err != nil {
			return err
		}
		fmt.Fprintf(e.Stdout, "这个系统建不了链接，给 Claude Code 放了一份副本：%s（升级 fanear 后会自动更新）\n", link)
		return nil
	}
	fmt.Fprintf(e.Stdout, "已给 Claude Code 建链接：%s → %s\n", link, target)
	return nil
}

func defaultSkillDir(e Env) string {
	return filepath.Join(e.Home, ".agents", "skills", "fanear")
}
