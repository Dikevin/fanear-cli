// Package skills embeds the agent skill shipped with fanear, so the skill
// and the CLI it documents always come from the same version.
package skills

import (
	_ "embed"
	"strings"
)

//go:embed fanear/SKILL.md
var fanearSkill string

// installRef is the install command in SKILL.md's 前提 section, its first
// install command. The file says @latest so it stays usable as read on
// GitHub; installed copies pin it to the release that wrote them.
const installRef = "cmd/fanear@latest"

// Fanear renders SKILL.md for a release such as v0.2.0; "" leaves @latest.
// Later install commands (upgrading after a format change) keep @latest.
func Fanear(version string) string {
	if version == "" {
		return fanearSkill
	}
	return strings.Replace(fanearSkill, installRef, "cmd/fanear@"+version, 1)
}
