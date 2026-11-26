// Package skills embeds the agent skill shipped with fanear, so the skill
// and the CLI it documents always come from the same version.
package skills

import _ "embed"

// FanearSkill is skills/fanear/SKILL.md.
//
//go:embed fanear/SKILL.md
var FanearSkill string
