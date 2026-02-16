package skill

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	markerBeginPrefix = "<!-- BEGIN SYNAPSE SKILL"
	markerEnd         = "<!-- END SYNAPSE SKILL -->"
)

// markerBegin returns the opening marker with version info.
func markerBegin(version string) string {
	return fmt.Sprintf("<!-- BEGIN SYNAPSE SKILL v%s -->", version)
}

// installAgentsSection installs or updates the Synapse section in an AGENTS.md file.
// If the file doesn't exist, it creates it. If markers exist, the section is replaced.
// Otherwise the section is appended. Writes use a temp file + rename for atomicity.
func installAgentsSection(targetFile, version string) error {
	section, err := buildAgentsSection(version)
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	dir := filepath.Dir(targetFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	existing, err := os.ReadFile(targetFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", targetFile, err)
	}

	var newContent string
	if os.IsNotExist(err) || len(existing) == 0 {
		// New file — just the section
		newContent = section
	} else {
		content := string(existing)
		if beginIdx, endIdx := findMarkers(content); beginIdx >= 0 {
			// Replace existing section
			newContent = content[:beginIdx] + section + content[endIdx:]
		} else {
			// Append section
			newContent = strings.TrimRight(content, "\n") + "\n\n" + section
		}
	}

	return atomicWrite(targetFile, []byte(newContent))
}

// uninstallAgentsSection removes the Synapse section from an AGENTS.md file.
// Returns true if the section was found and removed.
func uninstallAgentsSection(targetFile string) (bool, error) {
	content, err := os.ReadFile(targetFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading %s: %w", targetFile, err)
	}

	text := string(content)
	beginIdx, endIdx := findMarkers(text)
	if beginIdx < 0 {
		return false, nil
	}

	// Remove the section and clean up extra blank lines
	before := strings.TrimRight(text[:beginIdx], "\n")
	after := strings.TrimLeft(text[endIdx:], "\n")

	var newContent string
	if before == "" && after == "" {
		// File is now empty — remove it
		if err := os.Remove(targetFile); err != nil && !os.IsNotExist(err) {
			return false, fmt.Errorf("removing empty %s: %w", targetFile, err)
		}
		return true, nil
	} else if before == "" {
		newContent = after + "\n"
	} else if after == "" {
		newContent = before + "\n"
	} else {
		newContent = before + "\n\n" + after + "\n"
	}

	if err := atomicWrite(targetFile, []byte(newContent)); err != nil {
		return false, err
	}
	return true, nil
}

// buildAgentsSection reads the embedded template and wraps it with markers.
func buildAgentsSection(version string) (string, error) {
	data, err := fs.ReadFile(skillData, "skilldata/AGENTS_SECTION.md")
	if err != nil {
		return "", fmt.Errorf("reading embedded AGENTS_SECTION.md: %w", err)
	}

	body := injectVersion(string(data), version)
	return markerBegin(version) + "\n" + body + "\n" + markerEnd + "\n", nil
}

// findMarkers returns the byte offsets of the begin and end markers in content.
// beginIdx is the start of the begin marker line; endIdx is one past the end marker line.
// Returns (-1, -1) if not found.
func findMarkers(content string) (beginIdx, endIdx int) {
	beginIdx = -1
	endIdx = -1

	// Find begin marker
	idx := strings.Index(content, markerBeginPrefix)
	if idx < 0 {
		return
	}
	// Walk back to start of line
	lineStart := idx
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	beginIdx = lineStart

	// Find end marker after begin
	endMarkerIdx := strings.Index(content[idx:], markerEnd)
	if endMarkerIdx < 0 {
		beginIdx = -1
		return
	}

	// endIdx is after the end marker line
	endPos := idx + endMarkerIdx + len(markerEnd)
	if endPos < len(content) && content[endPos] == '\n' {
		endPos++
	}
	endIdx = endPos

	return
}

// atomicWrite writes data to a temp file and renames it to the target.
func atomicWrite(targetFile string, data []byte) error {
	dir := filepath.Dir(targetFile)
	tmp, err := os.CreateTemp(dir, ".synapse-skill-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpName, targetFile); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming %s to %s: %w", tmpName, targetFile, err)
	}

	return nil
}
