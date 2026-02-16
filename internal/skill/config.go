// Package skill manages agentic skill installation for AI coding agents.
//
// It supports the Agentic Skills specification (agentskills.io) by providing
// SKILL.md files and AGENTS.md sections that teach agents how to use Synapse.
package skill

import (
	"os"
	"path/filepath"
	"sort"
)

// SkillName is the directory name used for skill installations.
const SkillName = "synapse-skill"

// Format describes how a skill is delivered to an agent.
type Format string

const (
	// FormatSkillDir installs a SKILL.md file plus references/ into a directory.
	FormatSkillDir Format = "skill.md"
	// FormatAgentsMD appends a marked section to an AGENTS.md file.
	FormatAgentsMD Format = "agents.md"
)

// Level indicates whether the skill is installed per-user or per-project.
type Level string

const (
	LevelUser    Level = "user"
	LevelProject Level = "project"
)

// AgentConfig describes where and how to install a skill for a given agent.
type AgentConfig struct {
	Name        string
	DisplayName string
	Format      Format
	// UserPath returns the absolute path for a user-level installation.
	// For FormatSkillDir this is the skill directory; for FormatAgentsMD this is the AGENTS.md file.
	UserPath func() string
	// ProjectPath returns the path relative to the project root.
	ProjectPath func() string
}

// homeDir returns the user's home directory, falling back to $HOME.
func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.Getenv("HOME")
}

// agents is the canonical registry of supported agents.
var agents = map[string]AgentConfig{
	"claude-code": {
		Name:        "claude-code",
		DisplayName: "Claude Code",
		Format:      FormatSkillDir,
		UserPath: func() string {
			return filepath.Join(homeDir(), ".claude", "skills", SkillName)
		},
		ProjectPath: func() string {
			return filepath.Join(".claude", "skills", SkillName)
		},
	},
	"gemini-cli": {
		Name:        "gemini-cli",
		DisplayName: "Gemini CLI",
		Format:      FormatSkillDir,
		UserPath: func() string {
			return filepath.Join(homeDir(), ".gemini", "skills", SkillName)
		},
		ProjectPath: func() string {
			return filepath.Join(".gemini", "skills", SkillName)
		},
	},
	"codex": {
		Name:        "codex",
		DisplayName: "Codex",
		Format:      FormatAgentsMD,
		UserPath: func() string {
			return filepath.Join(homeDir(), ".codex", "AGENTS.md")
		},
		ProjectPath: func() string {
			return "AGENTS.md"
		},
	},
	"antigravity": {
		Name:        "antigravity",
		DisplayName: "Antigravity",
		Format:      FormatSkillDir,
		UserPath: func() string {
			return filepath.Join(homeDir(), ".gemini", "antigravity", "skills", SkillName)
		},
		ProjectPath: func() string {
			return filepath.Join(".agent", "skills", SkillName)
		},
	},
	"opencode": {
		Name:        "opencode",
		DisplayName: "OpenCode",
		Format:      FormatSkillDir,
		UserPath: func() string {
			return filepath.Join(homeDir(), ".config", "opencode", "skills", SkillName)
		},
		ProjectPath: func() string {
			return filepath.Join(".opencode", "skills", SkillName)
		},
	},
}

// GetAgent returns the configuration for a named agent, or false if unknown.
func GetAgent(name string) (AgentConfig, bool) {
	cfg, ok := agents[name]
	return cfg, ok
}

// AgentNames returns sorted agent names.
func AgentNames() []string {
	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TargetPath returns the installation path for an agent at the given level.
// For project-level installs, relative paths are resolved against the current working directory.
func TargetPath(cfg AgentConfig, level Level) string {
	if level == LevelUser {
		return cfg.UserPath()
	}
	p := cfg.ProjectPath()
	if filepath.IsAbs(p) {
		return p
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return filepath.Join(cwd, p)
}
