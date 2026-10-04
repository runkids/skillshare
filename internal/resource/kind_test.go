package resource

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/sourcewalk"
)

// --- SkillKind tests ---

func TestSkillKind_Kind(t *testing.T) {
	k := SkillKind{}
	if k.Kind() != "skill" {
		t.Errorf("SkillKind.Kind() = %q, want %q", k.Kind(), "skill")
	}
}

func TestSkillKind_Discover_SkipsTargetDotDirs(t *testing.T) {
	orig := sourcewalk.TargetDotDirs
	sourcewalk.TargetDotDirs = map[string]bool{".claude": true}
	defer func() { sourcewalk.TargetDotDirs = orig }()

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "ffmpeg-skill", ".claude", "skills", "code-review"), 0o755)
	os.WriteFile(filepath.Join(dir, "ffmpeg-skill", "SKILL.md"), []byte("---\nname: ffmpeg-skill\n---\n# FFmpeg"), 0o644)
	os.WriteFile(filepath.Join(dir, "ffmpeg-skill", ".claude", "skills", "code-review", "SKILL.md"), []byte("---\nname: code-review\n---\n# Dev only"), 0o644)

	resources, err := SkillKind{}.Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(resources) != 1 || resources[0].RelPath != "ffmpeg-skill" {
		t.Fatalf("expected only ffmpeg-skill (skipping .claude), got %+v", resources)
	}
}

func TestSkillKind_Discover(t *testing.T) {
	dir := t.TempDir()

	// Create two skills
	os.MkdirAll(filepath.Join(dir, "my-skill"), 0o755)
	os.WriteFile(filepath.Join(dir, "my-skill", "SKILL.md"), []byte("---\nname: my-skill\n---\n# Content"), 0o644)

	os.MkdirAll(filepath.Join(dir, "another"), 0o755)
	os.WriteFile(filepath.Join(dir, "another", "SKILL.md"), []byte("---\nname: another\n---\n# Content"), 0o644)

	// Non-skill directory (no SKILL.md)
	os.MkdirAll(filepath.Join(dir, "not-a-skill"), 0o755)
	os.WriteFile(filepath.Join(dir, "not-a-skill", "README.md"), []byte("# Readme"), 0o644)

	k := SkillKind{}
	resources, err := k.Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	names := map[string]bool{}
	for _, r := range resources {
		names[r.Name] = true
		if r.Kind != "skill" {
			t.Errorf("resource %q has Kind=%q, want %q", r.Name, r.Kind, "skill")
		}
	}

	if !names["my-skill"] {
		t.Error("expected to discover 'my-skill'")
	}
	if !names["another"] {
		t.Error("expected to discover 'another'")
	}
}

func TestSkillKind_Discover_Nested(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "_team", "frontend", "ui"), 0o755)
	os.WriteFile(filepath.Join(dir, "_team", "frontend", "ui", "SKILL.md"), []byte("---\nname: ui\n---\n"), 0o644)

	k := SkillKind{}
	resources, err := k.Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.Name != "ui" {
		t.Errorf("Name = %q, want %q", r.Name, "ui")
	}
	if r.FlatName != "_team__frontend__ui" {
		t.Errorf("FlatName = %q, want %q", r.FlatName, "_team__frontend__ui")
	}
	if !r.IsNested {
		t.Error("expected IsNested=true for nested skill")
	}
	if !r.IsInRepo {
		t.Error("expected IsInRepo=true for _-prefixed dir")
	}
}

func TestSkillKind_ResolveName_FromFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: custom-name\n---\n"), 0o644)

	k := SkillKind{}
	name := k.ResolveName(skillDir)
	if name != "custom-name" {
		t.Errorf("ResolveName = %q, want %q", name, "custom-name")
	}
}

func TestSkillKind_ResolveName_FallbackToDirName(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "fallback-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\n---\n"), 0o644)

	k := SkillKind{}
	name := k.ResolveName(skillDir)
	if name != "fallback-skill" {
		t.Errorf("ResolveName = %q, want %q", name, "fallback-skill")
	}
}

func TestSkillKind_FlatName(t *testing.T) {
	k := SkillKind{}

	tests := []struct {
		relPath string
		want    string
	}{
		{"my-skill", "my-skill"},
		{"_team/frontend/ui", "_team__frontend__ui"},
	}

	for _, tt := range tests {
		got := k.FlatName(tt.relPath)
		if got != tt.want {
			t.Errorf("FlatName(%q) = %q, want %q", tt.relPath, got, tt.want)
		}
	}
}

func TestSkillKind_FeatureGates(t *testing.T) {
	k := SkillKind{}
	if !k.SupportsAudit() {
		t.Error("SkillKind should support audit")
	}
	if !k.SupportsTrack() {
		t.Error("SkillKind should support track")
	}
	if !k.SupportsCollect() {
		t.Error("SkillKind should support collect")
	}
}

// --- AgentKind tests ---

func TestAgentKind_Kind(t *testing.T) {
	k := AgentKind{}
	if k.Kind() != "agent" {
		t.Errorf("AgentKind.Kind() = %q, want %q", k.Kind(), "agent")
	}
}

func TestAgentKind_Discover(t *testing.T) {
	dir := t.TempDir()

	// Create agent files
	os.WriteFile(filepath.Join(dir, "tutor.md"), []byte("# Tutor agent"), 0o644)
	os.WriteFile(filepath.Join(dir, "reviewer.md"), []byte("# Reviewer agent"), 0o644)

	// Conventional excludes should be skipped
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Readme"), 0o644)
	os.WriteFile(filepath.Join(dir, "LICENSE.md"), []byte("# License"), 0o644)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: test\n---\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude config"), 0o644)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Agents config"), 0o644)
	os.WriteFile(filepath.Join(dir, "GEMINI.md"), []byte("# Gemini config"), 0o644)

	// Non-.md files should be skipped
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("key: value"), 0o644)

	// Hidden files should be skipped
	os.WriteFile(filepath.Join(dir, ".hidden.md"), []byte("# Hidden"), 0o644)

	k := AgentKind{}
	resources, err := k.Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d: %v", len(resources), resources)
	}

	names := map[string]bool{}
	for _, r := range resources {
		names[r.Name] = true
		if r.Kind != "agent" {
			t.Errorf("resource %q has Kind=%q, want %q", r.Name, r.Kind, "agent")
		}
	}

	if !names["tutor"] {
		t.Error("expected to discover 'tutor'")
	}
	if !names["reviewer"] {
		t.Error("expected to discover 'reviewer'")
	}
}

func TestAgentKind_Discover_Nested(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "curriculum"), 0o755)
	os.WriteFile(filepath.Join(dir, "curriculum", "math-tutor.md"), []byte("# Math tutor"), 0o644)

	k := AgentKind{}
	resources, err := k.Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.Name != "math-tutor" {
		t.Errorf("Name = %q, want %q", r.Name, "math-tutor")
	}
	if r.RelPath != "curriculum/math-tutor.md" {
		t.Errorf("RelPath = %q, want %q", r.RelPath, "curriculum/math-tutor.md")
	}
	if r.FlatName != "curriculum__math-tutor.md" {
		t.Errorf("FlatName = %q, want %q", r.FlatName, "curriculum__math-tutor.md")
	}
	if !r.IsNested {
		t.Error("expected IsNested=true for nested agent")
	}
}

func TestAgentKind_ResolveName_FromFilename(t *testing.T) {
	dir := t.TempDir()
	agentFile := filepath.Join(dir, "tutor.md")
	os.WriteFile(agentFile, []byte("# Tutor agent"), 0o644)

	k := AgentKind{}
	name := k.ResolveName(agentFile)
	if name != "tutor" {
		t.Errorf("ResolveName = %q, want %q", name, "tutor")
	}
}

func TestAgentKind_ResolveName_FromFrontmatter(t *testing.T) {
	dir := t.TempDir()
	agentFile := filepath.Join(dir, "tutor.md")
	os.WriteFile(agentFile, []byte("---\nname: curriculum-tutor\n---\n# Tutor"), 0o644)

	k := AgentKind{}
	name := k.ResolveName(agentFile)
	if name != "curriculum-tutor" {
		t.Errorf("ResolveName = %q, want %q", name, "curriculum-tutor")
	}
}

func TestAgentKind_FlatName(t *testing.T) {
	k := AgentKind{}

	tests := []struct {
		relPath string
		want    string
	}{
		{"tutor.md", "tutor.md"},
		{"curriculum/math-tutor.md", "curriculum__math-tutor.md"},
		{"a/b/deep.md", "a__b__deep.md"},
	}

	for _, tt := range tests {
		got := k.FlatName(tt.relPath)
		if got != tt.want {
			t.Errorf("FlatName(%q) = %q, want %q", tt.relPath, got, tt.want)
		}
	}
}

func TestAgentKind_FeatureGates(t *testing.T) {
	k := AgentKind{}
	if !k.SupportsAudit() {
		t.Error("AgentKind should support audit")
	}
	if !k.SupportsTrack() {
		t.Error("AgentKind should support track")
	}
	if !k.SupportsCollect() {
		t.Error("AgentKind should support collect")
	}
}

func TestAgentKind_Discover_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	k := AgentKind{}
	resources, err := k.Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestAgentKind_Discover_RespectsAgentignore(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "active.md"), []byte("# Active"), 0o644)
	os.WriteFile(filepath.Join(dir, "ignored.md"), []byte("# Ignored"), 0o644)

	// Create .agentignore
	os.WriteFile(filepath.Join(dir, ".agentignore"), []byte("ignored.md\n"), 0o644)

	k := AgentKind{}
	resources, err := k.Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources (ignored included as disabled), got %d", len(resources))
	}

	// Find each by name and check Disabled flag
	var active, ignored *DiscoveredResource
	for i := range resources {
		switch resources[i].Name {
		case "active":
			active = &resources[i]
		case "ignored":
			ignored = &resources[i]
		}
	}
	if active == nil {
		t.Fatal("active agent not found")
	}
	if active.Disabled {
		t.Error("active agent should not be disabled")
	}
	if ignored == nil {
		t.Fatal("ignored agent not found")
	}
	if !ignored.Disabled {
		t.Error("ignored agent should be disabled")
	}
}

func TestAgentKind_Discover_SkipsGitDir(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, ".git", "config.md"), []byte("# git config"), 0o644)
	os.WriteFile(filepath.Join(dir, "real-agent.md"), []byte("# Agent"), 0o644)

	k := AgentKind{}
	resources, err := k.Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if resources[0].Name != "real-agent" {
		t.Errorf("Name = %q, want %q", resources[0].Name, "real-agent")
	}
}

func TestAgentKind_Discover_TrackedRepoWithAgentsDir(t *testing.T) {
	dir := t.TempDir()

	// Tracked repo WITH agents/ subdir — only agents/ contents should be discovered
	repo := filepath.Join(dir, "_team-agents")
	os.MkdirAll(filepath.Join(repo, ".git"), 0o755)
	os.MkdirAll(filepath.Join(repo, "agents"), 0o755)
	os.MkdirAll(filepath.Join(repo, "docs"), 0o755)
	os.WriteFile(filepath.Join(repo, "CLAUDE.md"), []byte("# Claude config"), 0o644)
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("# Readme"), 0o644)
	os.WriteFile(filepath.Join(repo, "intro.md"), []byte("# Not an agent"), 0o644)
	os.WriteFile(filepath.Join(repo, "agents", "reviewer.md"), []byte("# Reviewer"), 0o644)
	os.WriteFile(filepath.Join(repo, "agents", "tutor.md"), []byte("# Tutor"), 0o644)
	os.WriteFile(filepath.Join(repo, "docs", "guide.md"), []byte("# Guide"), 0o644)

	k := AgentKind{}
	resources, err := k.Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}

	names := map[string]bool{}
	for _, r := range resources {
		names[r.Name] = true
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 agents (only from agents/), got %d: %v", len(resources), names)
	}
	if !names["reviewer"] {
		t.Error("expected to discover 'reviewer' from agents/")
	}
	if !names["tutor"] {
		t.Error("expected to discover 'tutor' from agents/")
	}
}

func TestAgentKind_Discover_TrackedRepoWithoutAgentsDir(t *testing.T) {
	dir := t.TempDir()

	// Tracked repo WITHOUT agents/ subdir — whole repo is agents (minus excludes)
	repo := filepath.Join(dir, "_solo-agents")
	os.MkdirAll(filepath.Join(repo, ".git"), 0o755)
	os.WriteFile(filepath.Join(repo, "CLAUDE.md"), []byte("# Claude config"), 0o644)
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("# Readme"), 0o644)
	os.WriteFile(filepath.Join(repo, "code-reviewer.md"), []byte("# Reviewer"), 0o644)
	os.WriteFile(filepath.Join(repo, "debugging.md"), []byte("# Debugger"), 0o644)

	k := AgentKind{}
	resources, err := k.Discover(dir)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}

	names := map[string]bool{}
	for _, r := range resources {
		names[r.Name] = true
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 agents (CLAUDE.md + README.md excluded), got %d: %v", len(resources), names)
	}
	if !names["code-reviewer"] {
		t.Error("expected to discover 'code-reviewer'")
	}
	if !names["debugging"] {
		t.Error("expected to discover 'debugging'")
	}
	if names["CLAUDE"] {
		t.Error("CLAUDE.md should be excluded as conventional file")
	}
}
