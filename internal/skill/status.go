package skill

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// InstallationInfo describes the installation state for an agent at a level.
type InstallationInfo struct {
	Agent     string
	Level     Level
	Installed bool
	Version   string // empty if not installed or version not detected
	Path      string
}

// IsInstalled checks whether the skill is installed for the given agent and level.
func IsInstalled(agentName string, level Level) bool {
	cfg, ok := GetAgent(agentName)
	if !ok {
		return false
	}

	target := TargetPath(cfg, level)

	switch cfg.Format {
	case FormatSkillDir:
		_, err := os.Stat(target + "/SKILL.md")
		return err == nil
	case FormatAgentsMD:
		content, err := os.ReadFile(target)
		if err != nil {
			return false
		}
		beginIdx, _ := findMarkers(string(content))
		return beginIdx >= 0
	}
	return false
}

// InstalledVersion extracts the version from an installed skill.
// Returns empty string if not installed or version cannot be determined.
func InstalledVersion(agentName string, level Level) string {
	cfg, ok := GetAgent(agentName)
	if !ok {
		return ""
	}

	target := TargetPath(cfg, level)

	switch cfg.Format {
	case FormatSkillDir:
		content, err := os.ReadFile(target + "/SKILL.md")
		if err != nil {
			return ""
		}
		return extractVersionFromContent(string(content))
	case FormatAgentsMD:
		content, err := os.ReadFile(target)
		if err != nil {
			return ""
		}
		return extractVersionFromMarker(string(content))
	}
	return ""
}

// List returns installation info for all agents across both levels.
func List() []InstallationInfo {
	var infos []InstallationInfo
	for _, name := range AgentNames() {
		cfg, _ := GetAgent(name)
		for _, level := range []Level{LevelUser, LevelProject} {
			target := TargetPath(cfg, level)
			installed := IsInstalled(name, level)
			ver := ""
			if installed {
				ver = InstalledVersion(name, level)
			}
			infos = append(infos, InstallationInfo{
				Agent:     name,
				Level:     level,
				Installed: installed,
				Version:   ver,
				Path:      target,
			})
		}
	}
	return infos
}

// ShowSkillContent returns the raw embedded SKILL.md content with version injected.
func ShowSkillContent(version string) (string, error) {
	data, err := fs.ReadFile(skillData, "skilldata/SKILL.md")
	if err != nil {
		return "", fmt.Errorf("reading embedded SKILL.md: %w", err)
	}
	return injectVersion(string(data), version), nil
}

// extractVersionFromContent extracts the version from YAML frontmatter.
// Looks for a line like:  version: "1.0.4"
func extractVersionFromContent(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "version:") {
			val := strings.TrimPrefix(trimmed, "version:")
			val = strings.TrimSpace(val)
			val = strings.Trim(val, "\"'")
			return val
		}
	}
	return ""
}

// extractVersionFromMarker extracts the version from the AGENTS.md begin marker.
// Looks for: <!-- BEGIN SYNAPSE SKILL v1.0.4 -->
func extractVersionFromMarker(content string) string {
	idx := strings.Index(content, markerBeginPrefix)
	if idx < 0 {
		return ""
	}
	// Find the end of the marker line
	rest := content[idx+len(markerBeginPrefix):]
	endIdx := strings.Index(rest, "-->")
	if endIdx < 0 {
		return ""
	}
	segment := strings.TrimSpace(rest[:endIdx])
	// segment should be like "v1.0.4"
	if strings.HasPrefix(segment, "v") {
		return segment[1:]
	}
	return segment
}
