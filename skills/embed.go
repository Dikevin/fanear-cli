// Package skills embeds the agent skill shipped with fanear, so the skill
// and the CLI it documents always come from the same version.
package skills

import (
	_ "embed"
	"strings"
)

// fanearSkill is skills/fanear/SKILL.md; {{version}} marks where the
// install command pins the release that wrote it.
//
//go:embed fanear/SKILL.md
var fanearSkill string

// Fanear renders SKILL.md for a fanear version. Development builds, which
// have no release to pin, point at @latest.
func Fanear(version string) string {
	if version == "" {
		version = "latest"
	}
	return strings.ReplaceAll(fanearSkill, "{{version}}", version)
}
