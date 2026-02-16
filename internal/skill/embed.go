package skill

import "embed"

//go:embed skilldata/SKILL.md skilldata/AGENTS_SECTION.md skilldata/references/*
var skillData embed.FS
