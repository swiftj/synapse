// Package storage provides persistence for Synapse data.
package storage

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitIntegration handles Git-related operations for Synapse.
type GitIntegration struct {
	repoRoot string
}

// DetectGitRepo checks if we're inside a Git repository and returns
// the repository root path, or empty string if not in a repo.
func DetectGitRepo() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	root := strings.TrimSpace(string(out))
	// Resolve symlinks for consistent path comparison (e.g., /tmp -> /private/tmp on macOS)
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		return resolved
	}
	return root
}

// NewGitIntegration creates a Git integration helper.
// Returns nil if not in a Git repository.
func NewGitIntegration() *GitIntegration {
	root := DetectGitRepo()
	if root == "" {
		return nil
	}
	return &GitIntegration{repoRoot: root}
}

// AddToGitignore appends an entry to .gitignore if not already present.
// Creates .gitignore if it doesn't exist.
// Returns (added, error) where added is true if the entry was actually written.
func (g *GitIntegration) AddToGitignore(entry string) (bool, error) {
	gitignorePath := filepath.Join(g.repoRoot, ".gitignore")

	// Check if entry already exists
	if g.gitignoreContains(gitignorePath, entry) {
		return false, nil // Already present, no action needed
	}

	// Check if file exists and get its content to see if it ends with newline
	prefix := ""
	if content, err := os.ReadFile(gitignorePath); err == nil && len(content) > 0 {
		if content[len(content)-1] != '\n' {
			prefix = "\n"
		}
	}

	// Append entry to .gitignore
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.WriteString(prefix + entry + "\n")
	if err != nil {
		return false, err
	}

	return true, nil
}

// StageFile runs git add on the specified path (relative to repo root).
func (g *GitIntegration) StageFile(relativePath string) error {
	cmd := exec.Command("git", "add", relativePath)
	cmd.Dir = g.repoRoot
	return cmd.Run()
}

// gitignoreContains checks if .gitignore already contains the entry.
func (g *GitIntegration) gitignoreContains(path, entry string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == entry {
			return true
		}
	}
	return false
}

// RepoRoot returns the Git repository root path.
func (g *GitIntegration) RepoRoot() string {
	return g.repoRoot
}
