package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

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
		_, err := e.Stdout.Write([]byte(skills.FanearSkill))
		return err
	}
	if pos[0] != "install" || len(pos) > 1 {
		return usageError{"用法：fanear skill install [--dir 目录] [--claude]"}
	}
	base := *dir
	if base == "" {
		base = filepath.Join(e.Home, ".agents", "skills")
	}
	target := filepath.Join(base, "fanear")
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte(skills.FanearSkill), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(e.Stdout, "已装好：%s\n", filepath.Join(target, "SKILL.md"))
	link := filepath.Join(e.Home, ".claude", "skills", "fanear")
	if !*claude {
		fmt.Fprintf(e.Stdout, "Claude Code 读 ~/.claude/skills；还没链接的话加 --claude，或自己运行：ln -s %s %s\n", target, link)
		return nil
	}
	if cur, err := os.Readlink(link); err == nil {
		if cur == target {
			fmt.Fprintf(e.Stdout, "Claude Code 的链接已经在：%s\n", link)
			return nil
		}
		return fmt.Errorf("%s 已经指向 %s，没动它", link, cur)
	} else if _, statErr := os.Lstat(link); statErr == nil {
		return fmt.Errorf("%s 已经存在且不是链接，没动它", link)
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return statErr
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	if err := os.Symlink(target, link); err != nil {
		return err
	}
	fmt.Fprintf(e.Stdout, "已给 Claude Code 建链接：%s → %s\n", link, target)
	return nil
}
