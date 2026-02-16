package skill

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testVersion = "1.2.3"

// overrideAgentPaths temporarily overrides agent path functions for testing.
// Returns a cleanup function.
func overrideAgentPaths(t *testing.T, userDir, projectDir string) {
	t.Helper()
	origAgents := make(map[string]AgentConfig)
	for k, v := range agents {
		origAgents[k] = v
	}
	t.Cleanup(func() {
		for k, v := range origAgents {
			agents[k] = v
		}
	})

	for name, cfg := range agents {
		cfg := cfg // capture
		switch cfg.Format {
		case FormatSkillDir:
			localUser := userDir
			localProject := projectDir
			cfg.UserPath = func() string {
				return filepath.Join(localUser, cfg.Name)
			}
			cfg.ProjectPath = func() string {
				return filepath.Join(localProject, cfg.Name)
			}
		case FormatAgentsMD:
			localUser := userDir
			localProject := projectDir
			cfg.UserPath = func() string {
				return filepath.Join(localUser, "AGENTS.md")
			}
			cfg.ProjectPath = func() string {
				return filepath.Join(localProject, "AGENTS.md")
			}
		}
		agents[name] = cfg
	}
}

func TestEmbeddedFilesExist(t *testing.T) {
	tests := []struct {
		path string
	}{
		{"skilldata/SKILL.md"},
		{"skilldata/AGENTS_SECTION.md"},
		{"skilldata/references/workflows.md"},
		{"skilldata/references/tool-reference.md"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			data, err := fs.ReadFile(skillData, tt.path)
			if err != nil {
				t.Fatalf("embedded file not found: %s: %v", tt.path, err)
			}
			if len(data) == 0 {
				t.Fatalf("embedded file is empty: %s", tt.path)
			}
		})
	}
}

func TestSkillMDHasVersionPlaceholder(t *testing.T) {
	data, err := fs.ReadFile(skillData, "skilldata/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "{{VERSION}}") {
		t.Error("SKILL.md should contain {{VERSION}} placeholder")
	}
}

func TestAgentsMDHasVersionPlaceholder(t *testing.T) {
	data, err := fs.ReadFile(skillData, "skilldata/AGENTS_SECTION.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "{{VERSION}}") {
		t.Error("AGENTS_SECTION.md should contain {{VERSION}} placeholder")
	}
}

func TestInjectVersion(t *testing.T) {
	input := "version: {{VERSION}} and also {{VERSION}}"
	got := injectVersion(input, "2.0.0")
	want := "version: 2.0.0 and also 2.0.0"
	if got != want {
		t.Errorf("injectVersion = %q, want %q", got, want)
	}
}

func TestAgentNames(t *testing.T) {
	names := AgentNames()
	if len(names) != 5 {
		t.Errorf("expected 5 agents, got %d: %v", len(names), names)
	}
	// Verify sorted
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("agent names not sorted: %v", names)
			break
		}
	}
}

func TestGetAgent(t *testing.T) {
	for _, name := range AgentNames() {
		cfg, ok := GetAgent(name)
		if !ok {
			t.Errorf("GetAgent(%q) returned false", name)
		}
		if cfg.Name != name {
			t.Errorf("GetAgent(%q).Name = %q", name, cfg.Name)
		}
		if cfg.DisplayName == "" {
			t.Errorf("GetAgent(%q).DisplayName is empty", name)
		}
	}

	_, ok := GetAgent("nonexistent")
	if ok {
		t.Error("GetAgent(nonexistent) should return false")
	}
}

func TestInstallSkillDir(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(userDir, 0o755)
	os.MkdirAll(projectDir, 0o755)
	overrideAgentPaths(t, userDir, projectDir)

	// Install claude-code at user level
	if err := Install("claude-code", LevelUser, testVersion); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	cfg, _ := GetAgent("claude-code")
	target := cfg.UserPath()

	// Check SKILL.md exists
	skillPath := filepath.Join(target, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("SKILL.md not found: %v", err)
	}
	content := string(data)

	// Version should be injected
	if strings.Contains(content, "{{VERSION}}") {
		t.Error("SKILL.md still contains {{VERSION}} placeholder")
	}
	if !strings.Contains(content, testVersion) {
		t.Errorf("SKILL.md does not contain version %s", testVersion)
	}

	// Check references/ exist
	refsDir := filepath.Join(target, "references")
	entries, err := os.ReadDir(refsDir)
	if err != nil {
		t.Fatalf("references/ not found: %v", err)
	}
	if len(entries) < 2 {
		t.Errorf("expected at least 2 reference files, got %d", len(entries))
	}
}

func TestInstallAgentsMD(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(userDir, 0o755)
	os.MkdirAll(projectDir, 0o755)
	overrideAgentPaths(t, userDir, projectDir)

	// Install codex at project level
	if err := Install("codex", LevelProject, testVersion); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	cfg, _ := GetAgent("codex")
	target := cfg.ProjectPath()

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("AGENTS.md not found: %v", err)
	}
	content := string(data)

	// Check markers
	if !strings.Contains(content, markerBeginPrefix) {
		t.Error("AGENTS.md missing begin marker")
	}
	if !strings.Contains(content, markerEnd) {
		t.Error("AGENTS.md missing end marker")
	}
	if !strings.Contains(content, testVersion) {
		t.Error("AGENTS.md missing version")
	}
}

func TestAgentsMDPreservesExistingContent(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(userDir, 0o755)
	os.MkdirAll(projectDir, 0o755)
	overrideAgentPaths(t, userDir, projectDir)

	cfg, _ := GetAgent("codex")
	target := cfg.ProjectPath()

	// Write pre-existing content
	existing := "# My Project Agents\n\nSome existing content here.\n"
	os.WriteFile(target, []byte(existing), 0o644)

	// Install
	if err := Install("codex", LevelProject, testVersion); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Existing content preserved
	if !strings.Contains(content, "# My Project Agents") {
		t.Error("existing content was not preserved")
	}
	if !strings.Contains(content, "Some existing content here.") {
		t.Error("existing content was not preserved")
	}

	// Synapse section added
	if !strings.Contains(content, markerBeginPrefix) {
		t.Error("synapse section not added")
	}
}

func TestAgentsMDReplacesSectionOnReinstall(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(userDir, 0o755)
	os.MkdirAll(projectDir, 0o755)
	overrideAgentPaths(t, userDir, projectDir)

	// Install version 1
	if err := Install("codex", LevelProject, "1.0.0"); err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	// Install version 2 (should replace, not duplicate)
	if err := Install("codex", LevelProject, "2.0.0"); err != nil {
		t.Fatalf("second install failed: %v", err)
	}

	cfg, _ := GetAgent("codex")
	data, err := os.ReadFile(cfg.ProjectPath())
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Should have v2 marker, not v1
	if strings.Contains(content, "v1.0.0") {
		t.Error("old version marker still present")
	}
	if !strings.Contains(content, "v2.0.0") {
		t.Error("new version marker not present")
	}

	// Should only have one begin marker
	count := strings.Count(content, markerBeginPrefix)
	if count != 1 {
		t.Errorf("expected 1 begin marker, got %d", count)
	}
}

func TestUninstallSkillDir(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(userDir, 0o755)
	os.MkdirAll(projectDir, 0o755)
	overrideAgentPaths(t, userDir, projectDir)

	// Install then uninstall
	if err := Install("gemini-cli", LevelUser, testVersion); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if err := Uninstall("gemini-cli", LevelUser); err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}

	cfg, _ := GetAgent("gemini-cli")
	if _, err := os.Stat(cfg.UserPath()); !os.IsNotExist(err) {
		t.Error("skill directory should have been removed")
	}
}

func TestUninstallAgentsMD(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(userDir, 0o755)
	os.MkdirAll(projectDir, 0o755)
	overrideAgentPaths(t, userDir, projectDir)

	// Write pre-existing content + install
	cfg, _ := GetAgent("codex")
	target := cfg.ProjectPath()
	existing := "# My Agents\n\nOther stuff.\n"
	os.WriteFile(target, []byte(existing), 0o644)

	if err := Install("codex", LevelProject, testVersion); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Uninstall
	if err := Uninstall("codex", LevelProject); err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}

	// File should still exist with original content
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal("AGENTS.md should still exist after removing synapse section")
	}
	content := string(data)
	if strings.Contains(content, markerBeginPrefix) {
		t.Error("synapse markers should have been removed")
	}
	if !strings.Contains(content, "# My Agents") {
		t.Error("existing content should be preserved")
	}
}

func TestUninstallAgentsMDRemovesEmptyFile(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(userDir, 0o755)
	os.MkdirAll(projectDir, 0o755)
	overrideAgentPaths(t, userDir, projectDir)

	// Install (creates file with only synapse content)
	if err := Install("codex", LevelProject, testVersion); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Uninstall should remove the file entirely
	if err := Uninstall("codex", LevelProject); err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}

	cfg, _ := GetAgent("codex")
	if _, err := os.Stat(cfg.ProjectPath()); !os.IsNotExist(err) {
		t.Error("empty AGENTS.md should have been removed")
	}
}

func TestUninstallNotInstalled(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(userDir, 0o755)
	os.MkdirAll(projectDir, 0o755)
	overrideAgentPaths(t, userDir, projectDir)

	err := Uninstall("claude-code", LevelUser)
	if err == nil {
		t.Error("expected error when uninstalling non-installed skill")
	}
}

func TestIsInstalled(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(userDir, 0o755)
	os.MkdirAll(projectDir, 0o755)
	overrideAgentPaths(t, userDir, projectDir)

	// Not installed initially
	if IsInstalled("claude-code", LevelUser) {
		t.Error("should not be installed initially")
	}

	// Install
	if err := Install("claude-code", LevelUser, testVersion); err != nil {
		t.Fatal(err)
	}

	if !IsInstalled("claude-code", LevelUser) {
		t.Error("should be installed after Install")
	}

	// Not installed at project level
	if IsInstalled("claude-code", LevelProject) {
		t.Error("should not be installed at project level")
	}
}

func TestInstalledVersion(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(userDir, 0o755)
	os.MkdirAll(projectDir, 0o755)
	overrideAgentPaths(t, userDir, projectDir)

	// SkillDir format
	if err := Install("claude-code", LevelUser, testVersion); err != nil {
		t.Fatal(err)
	}
	ver := InstalledVersion("claude-code", LevelUser)
	if ver != testVersion {
		t.Errorf("InstalledVersion = %q, want %q", ver, testVersion)
	}

	// AgentsMD format
	if err := Install("codex", LevelProject, "3.0.0"); err != nil {
		t.Fatal(err)
	}
	ver = InstalledVersion("codex", LevelProject)
	if ver != "3.0.0" {
		t.Errorf("InstalledVersion = %q, want %q", ver, "3.0.0")
	}
}

func TestList(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(userDir, 0o755)
	os.MkdirAll(projectDir, 0o755)
	overrideAgentPaths(t, userDir, projectDir)

	// Install two agents at different levels
	Install("claude-code", LevelUser, testVersion)
	Install("codex", LevelProject, testVersion)

	infos := List()

	// Should have entries for all agents x both levels
	if len(infos) != len(AgentNames())*2 {
		t.Errorf("expected %d entries, got %d", len(AgentNames())*2, len(infos))
	}

	// Find claude-code user entry
	var claudeUser *InstallationInfo
	for i := range infos {
		if infos[i].Agent == "claude-code" && infos[i].Level == LevelUser {
			claudeUser = &infos[i]
			break
		}
	}
	if claudeUser == nil {
		t.Fatal("claude-code user entry not found")
	}
	if !claudeUser.Installed {
		t.Error("claude-code user should be installed")
	}
	if claudeUser.Version != testVersion {
		t.Errorf("claude-code version = %q, want %q", claudeUser.Version, testVersion)
	}
}

func TestInstallUnknownAgent(t *testing.T) {
	err := Install("unknown-agent", LevelProject, testVersion)
	if err == nil {
		t.Error("expected error for unknown agent")
	}
	if !strings.Contains(err.Error(), "unknown agent") {
		t.Errorf("error should mention 'unknown agent', got: %v", err)
	}
}

func TestUninstallUnknownAgent(t *testing.T) {
	err := Uninstall("unknown-agent", LevelProject)
	if err == nil {
		t.Error("expected error for unknown agent")
	}
}

func TestInstallIdempotent(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(userDir, 0o755)
	os.MkdirAll(projectDir, 0o755)
	overrideAgentPaths(t, userDir, projectDir)

	// Install twice — should not error
	if err := Install("opencode", LevelUser, testVersion); err != nil {
		t.Fatal(err)
	}
	if err := Install("opencode", LevelUser, testVersion); err != nil {
		t.Fatalf("second install should not fail: %v", err)
	}

	// Should still be correctly installed
	if !IsInstalled("opencode", LevelUser) {
		t.Error("should be installed after double-install")
	}
}

func TestShowSkillContent(t *testing.T) {
	content, err := ShowSkillContent("9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "9.9.9") {
		t.Error("ShowSkillContent should inject version")
	}
	if strings.Contains(content, "{{VERSION}}") {
		t.Error("ShowSkillContent should replace all {{VERSION}} placeholders")
	}
}

func TestUpdateAll(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(userDir, 0o755)
	os.MkdirAll(projectDir, 0o755)
	overrideAgentPaths(t, userDir, projectDir)

	// Install a couple
	Install("claude-code", LevelUser, "1.0.0")
	Install("codex", LevelProject, "1.0.0")

	// Update all
	updated, err := UpdateAll("2.0.0")
	if err != nil {
		t.Fatal(err)
	}

	if len(updated) != 2 {
		t.Errorf("expected 2 updated, got %d: %v", len(updated), updated)
	}

	// Verify versions
	ver := InstalledVersion("claude-code", LevelUser)
	if ver != "2.0.0" {
		t.Errorf("claude-code version = %q after update", ver)
	}

	ver = InstalledVersion("codex", LevelProject)
	if ver != "2.0.0" {
		t.Errorf("codex version = %q after update", ver)
	}
}

func TestExtractVersionFromContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "quoted version",
			content: "---\nname: test\n  version: \"1.0.4\"\n---",
			want:    "1.0.4",
		},
		{
			name:    "unquoted version",
			content: "version: 2.0.0\n",
			want:    "2.0.0",
		},
		{
			name:    "no version",
			content: "name: test\n",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractVersionFromContent(tt.content)
			if got != tt.want {
				t.Errorf("extractVersionFromContent = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractVersionFromMarker(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "standard marker",
			content: "<!-- BEGIN SYNAPSE SKILL v1.0.4 -->\nstuff\n<!-- END SYNAPSE SKILL -->",
			want:    "1.0.4",
		},
		{
			name:    "no marker",
			content: "just some content",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractVersionFromMarker(tt.content)
			if got != tt.want {
				t.Errorf("extractVersionFromMarker = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindMarkers(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantBegin int
		wantEnd   int
	}{
		{
			name:      "no markers",
			content:   "just text",
			wantBegin: -1,
			wantEnd:   -1,
		},
		{
			name:      "markers at start",
			content:   "<!-- BEGIN SYNAPSE SKILL v1.0 -->\nbody\n<!-- END SYNAPSE SKILL -->\n",
			wantBegin: 0,
			wantEnd:   -1, // will compute
		},
		{
			name:      "markers with prefix content",
			content:   "stuff\n<!-- BEGIN SYNAPSE SKILL v1.0 -->\nbody\n<!-- END SYNAPSE SKILL -->\nmore",
			wantBegin: 6, // after "stuff\n"
			wantEnd:   -1,
		},
		{
			name:      "begin without end",
			content:   "<!-- BEGIN SYNAPSE SKILL v1.0 -->\nbody",
			wantBegin: -1,
			wantEnd:   -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			begin, end := findMarkers(tt.content)
			if tt.wantBegin >= 0 {
				if begin != tt.wantBegin {
					t.Errorf("beginIdx = %d, want %d", begin, tt.wantBegin)
				}
				if end < 0 {
					t.Error("endIdx should be >= 0 when beginIdx is found")
				}
			} else {
				if begin != -1 {
					t.Errorf("beginIdx = %d, want -1", begin)
				}
			}
		})
	}
}

func TestAllAgentFormats(t *testing.T) {
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(userDir, 0o755)
	os.MkdirAll(projectDir, 0o755)
	overrideAgentPaths(t, userDir, projectDir)

	// Install every agent at user level
	for _, name := range AgentNames() {
		t.Run(name, func(t *testing.T) {
			if err := Install(name, LevelUser, testVersion); err != nil {
				t.Fatalf("Install(%s) failed: %v", name, err)
			}
			if !IsInstalled(name, LevelUser) {
				t.Errorf("%s should be installed", name)
			}
			ver := InstalledVersion(name, LevelUser)
			if ver != testVersion {
				t.Errorf("%s version = %q, want %q", name, ver, testVersion)
			}
		})
	}
}
