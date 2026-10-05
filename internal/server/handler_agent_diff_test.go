package server

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/sync"
)

func TestComputeAgentTargetDiff_MissingInTarget(t *testing.T) {
	targetDir := t.TempDir()

	agents := []resource.DiscoveredResource{
		{FlatName: "helper.md", AbsPath: "/src/helper.md", RelPath: "helper.md"},
	}

	items := computeAgentTargetDiff("claude", targetDir, config.ResourceTargetConfig{}, agents)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Action != "link" {
		t.Errorf("expected action 'link', got %q", items[0].Action)
	}
	if items[0].Kind != "agent" {
		t.Errorf("expected kind 'agent', got %q", items[0].Kind)
	}
}

func TestComputeAgentTargetDiff_OrphanSymlink(t *testing.T) {
	targetDir := t.TempDir()
	os.Symlink("/nonexistent/old.md", filepath.Join(targetDir, "orphan.md"))

	items := computeAgentTargetDiff("claude", targetDir, config.ResourceTargetConfig{}, nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Action != "prune" {
		t.Errorf("expected action 'prune', got %q", items[0].Action)
	}
}

func TestComputeAgentTargetDiff_LocalFile(t *testing.T) {
	targetDir := t.TempDir()
	os.WriteFile(filepath.Join(targetDir, "local.md"), []byte("# Local"), 0644)

	items := computeAgentTargetDiff("claude", targetDir, config.ResourceTargetConfig{}, nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Action != "local" {
		t.Errorf("expected action 'local', got %q", items[0].Action)
	}
}

func TestComputeAgentTargetDiff_SymlinkPointsElsewhere(t *testing.T) {
	sourceDir := t.TempDir()
	otherDir := t.TempDir()
	targetDir := t.TempDir()

	srcFile := filepath.Join(sourceDir, "agent.md")
	os.WriteFile(srcFile, []byte("# Agent"), 0644)
	otherFile := filepath.Join(otherDir, "agent.md")
	os.WriteFile(otherFile, []byte("# Other"), 0644)

	// Symlink points to otherFile, not srcFile
	os.Symlink(otherFile, filepath.Join(targetDir, "agent.md"))

	agents := []resource.DiscoveredResource{
		{FlatName: "agent.md", AbsPath: srcFile, RelPath: "agent.md"},
	}

	items := computeAgentTargetDiff("claude", targetDir, config.ResourceTargetConfig{}, agents)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Action != "update" {
		t.Errorf("expected action 'update', got %q", items[0].Action)
	}
	if items[0].Reason != "symlink points elsewhere" {
		t.Errorf("expected reason 'symlink points elsewhere', got %q", items[0].Reason)
	}
}

func TestComputeAgentTargetDiff_InSync(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	srcFile := filepath.Join(sourceDir, "agent.md")
	os.WriteFile(srcFile, []byte("# Agent"), 0644)
	os.Symlink(srcFile, filepath.Join(targetDir, "agent.md"))

	agents := []resource.DiscoveredResource{
		{FlatName: "agent.md", AbsPath: srcFile, RelPath: "agent.md"},
	}

	items := computeAgentTargetDiff("claude", targetDir, config.ResourceTargetConfig{}, agents)

	if len(items) != 0 {
		t.Fatalf("expected 0 items (in sync), got %d", len(items))
	}
}

func extensionTargetFixture(t *testing.T) (targetDir string, agents []resource.DiscoveredResource) {
	t.Helper()
	srcDir, targetDir := t.TempDir(), t.TempDir()
	srcFile := filepath.Join(srcDir, "reviewer.md")
	os.WriteFile(srcFile, []byte("# Reviewer"), 0644)
	agents = []resource.DiscoveredResource{{FlatName: "reviewer.md", AbsPath: srcFile, RelPath: "reviewer.md"}}
	spec := &sync.ExtensionSpec{Run: []string{"cat"}, Dir: srcDir, Name: "id", OutputExt: "toml"}
	if _, err := sync.SyncAgentsTransform(agents, srcDir, targetDir, "copy", spec, false, false); err != nil {
		t.Fatal(err)
	}
	return targetDir, agents
}

func TestComputeAgentTargetDiff_ExtensionOutputInSync(t *testing.T) {
	targetDir, agents := extensionTargetFixture(t)

	items := computeAgentTargetDiff("codex", targetDir, config.ResourceTargetConfig{Extension: "x"}, agents)

	if len(items) != 0 {
		t.Fatalf("expected 0 items (converted output in sync), got %+v", items)
	}
}

func TestComputeAgentTargetDiff_ExtensionOutputOutdated(t *testing.T) {
	targetDir, agents := extensionTargetFixture(t)
	os.WriteFile(agents[0].AbsPath, []byte("# Reviewer v2"), 0644)

	items := computeAgentTargetDiff("codex", targetDir, config.ResourceTargetConfig{Extension: "x"}, agents)

	if len(items) != 1 || items[0].Action != "link" {
		t.Fatalf("expected 1 link item, got %+v", items)
	}
}

func TestComputeAgentTargetDiff_ExtensionOrphanOutput(t *testing.T) {
	targetDir, _ := extensionTargetFixture(t)

	items := computeAgentTargetDiff("codex", targetDir, config.ResourceTargetConfig{Extension: "x"}, nil)

	if len(items) != 1 || items[0].Action != "prune" || items[0].Skill != "reviewer.toml" {
		t.Fatalf("expected prune of reviewer.toml, got %+v", items)
	}
}

func TestComputeAgentTargetDiff_ExcludedAgentIsNotPending(t *testing.T) {
	agents := []resource.DiscoveredResource{
		{FlatName: "helper.md", AbsPath: "/src/helper.md", RelPath: "helper.md"},
	}

	items := computeAgentTargetDiff("claude", t.TempDir(), config.ResourceTargetConfig{Exclude: []string{"helper*"}}, agents)

	if len(items) != 0 {
		t.Fatalf("expected 0 items for an excluded agent, got %+v", items)
	}
}
