package skill

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Install installs the skill for the given agent at the specified level.
// The version string is injected into the skill content.
// Install is idempotent — calling it twice overwrites cleanly.
func Install(agentName string, level Level, version string) error {
	cfg, ok := GetAgent(agentName)
	if !ok {
		return fmt.Errorf("unknown agent: %s (available: %s)", agentName, strings.Join(AgentNames(), ", "))
	}

	target := TargetPath(cfg, level)

	switch cfg.Format {
	case FormatSkillDir:
		return installSkillDir(target, version)
	case FormatAgentsMD:
		return installAgentsSection(target, version)
	default:
		return fmt.Errorf("unsupported format: %s", cfg.Format)
	}
}

// Update re-installs the skill, overwriting the existing installation.
func Update(agentName string, level Level, version string) error {
	return Install(agentName, level, version)
}

// UpdateAll re-installs the skill for all agents that are currently installed.
func UpdateAll(version string) ([]string, error) {
	var updated []string
	for _, name := range AgentNames() {
		for _, level := range []Level{LevelUser, LevelProject} {
			if IsInstalled(name, level) {
				if err := Install(name, level, version); err != nil {
					return updated, fmt.Errorf("updating %s (%s): %w", name, level, err)
				}
				updated = append(updated, fmt.Sprintf("%s (%s)", name, level))
			}
		}
	}
	return updated, nil
}

// installSkillDir creates the skill directory with SKILL.md and references/.
func installSkillDir(targetDir, version string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", targetDir, err)
	}

	// Write SKILL.md
	content, err := fs.ReadFile(skillData, "skilldata/SKILL.md")
	if err != nil {
		return fmt.Errorf("reading embedded SKILL.md: %w", err)
	}
	content = []byte(injectVersion(string(content), version))

	if err := os.WriteFile(filepath.Join(targetDir, "SKILL.md"), content, 0o644); err != nil {
		return fmt.Errorf("writing SKILL.md: %w", err)
	}

	// Copy references/ directory
	refsDir := filepath.Join(targetDir, "references")
	if err := os.MkdirAll(refsDir, 0o755); err != nil {
		return fmt.Errorf("creating references directory: %w", err)
	}

	entries, err := fs.ReadDir(skillData, "skilldata/references")
	if err != nil {
		return fmt.Errorf("reading embedded references: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := fs.ReadFile(skillData, "skilldata/references/"+entry.Name())
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", entry.Name(), err)
		}
		dest := filepath.Join(refsDir, entry.Name())
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// injectVersion replaces {{VERSION}} placeholders with the actual version.
func injectVersion(content, version string) string {
	return strings.ReplaceAll(content, "{{VERSION}}", version)
}
