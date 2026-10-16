// Package benchpress carries the documents the binary ships with.
package benchpress

import _ "embed"

// Skill is how an agent writes and keeps a workbench; init installs it.
//
//go:embed skills/benchpress/SKILL.md
var Skill string
