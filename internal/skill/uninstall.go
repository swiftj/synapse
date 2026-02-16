package skill

import (
	"fmt"
	"os"
	"strings"
)

// Uninstall removes the skill for the given agent at the specified level.
func Uninstall(agentName string, level Level) error {
	cfg, ok := GetAgent(agentName)
	if !ok {
		return fmt.Errorf("unknown agent: %s (available: %s)", agentName, strings.Join(AgentNames(), ", "))
	}

	target := TargetPath(cfg, level)

	switch cfg.Format {
	case FormatSkillDir:
		return uninstallSkillDir(target)
	case FormatAgentsMD:
		removed, err := uninstallAgentsSection(target)
		if err != nil {
			return err
		}
		if !removed {
			return fmt.Errorf("synapse skill not found in %s", target)
		}
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", cfg.Format)
	}
}

// uninstallSkillDir removes the skill directory entirely.
func uninstallSkillDir(targetDir string) error {
	info, err := os.Stat(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("skill not installed at %s", targetDir)
		}
		return fmt.Errorf("checking %s: %w", targetDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("expected directory at %s", targetDir)
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("removing %s: %w", targetDir, err)
	}
	return nil
}
