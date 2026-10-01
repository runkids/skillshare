package sync

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/resource"
)

func TestCheckAgentCollisions_NoCollision(t *testing.T) {
	agents := []resource.DiscoveredResource{
		{FlatName: "tutor.md", RelPath: "tutor.md"},
		{FlatName: "reviewer.md", RelPath: "reviewer.md"},
	}
	collisions := CheckAgentCollisions(agents)
	if len(collisions) != 0 {
		t.Errorf("expected 0 collisions, got %d", len(collisions))
	}
}

func TestCheckAgentCollisions_HasCollision(t *testing.T) {
	agents := []resource.DiscoveredResource{
		{FlatName: "team__helper.md", RelPath: "team/helper.md"},
		{FlatName: "team__helper.md", RelPath: "team__helper.md"},
	}
	collisions := CheckAgentCollisions(agents)
	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d", len(collisions))
	}
	if collisions[0].FlatName != "team__helper.md" {
		t.Errorf("collision FlatName = %q", collisions[0].FlatName)
	}
}

func TestSyncAgentsToTarget_NewLinks(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	// Create agent source files
	os.WriteFile(filepath.Join(sourceDir, "tutor.md"), []byte("# Tutor"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "reviewer.md"), []byte("# Reviewer"), 0644)

	agents := []resource.DiscoveredResource{
		{FlatName: "tutor.md", AbsPath: filepath.Join(sourceDir, "tutor.md")},
		{FlatName: "reviewer.md", AbsPath: filepath.Join(sourceDir, "reviewer.md")},
	}

	result, err := SyncAgentsToTarget(agents, targetDir, false, false)
	if err != nil {
		t.Fatalf("SyncAgentsToTarget: %v", err)
	}

	if len(result.Linked) != 2 {
		t.Errorf("expected 2 linked, got %d", len(result.Linked))
	}

	// Verify symlinks exist
	for _, name := range []string{"tutor.md", "reviewer.md"} {
		linkPath := filepath.Join(targetDir, name)
		info, err := os.Lstat(linkPath)
		if err != nil {
			t.Errorf("expected symlink %s to exist", name)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("expected %s to be a symlink", name)
		}
	}
}

func TestSyncAgentsToTarget_ExistingSymlinkCorrect(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	srcFile := filepath.Join(sourceDir, "tutor.md")
	os.WriteFile(srcFile, []byte("# Tutor"), 0644)

	// Pre-create correct symlink
	os.Symlink(srcFile, filepath.Join(targetDir, "tutor.md"))

	agents := []resource.DiscoveredResource{
		{FlatName: "tutor.md", AbsPath: srcFile},
	}

	result, err := SyncAgentsToTarget(agents, targetDir, false, false)
	if err != nil {
		t.Fatalf("SyncAgentsToTarget: %v", err)
	}

	if len(result.Linked) != 1 {
		t.Errorf("expected 1 linked (existing correct), got %d", len(result.Linked))
	}
	if len(result.Updated) != 0 {
		t.Errorf("expected 0 updated, got %d", len(result.Updated))
	}
}

func TestSyncAgentsToTarget_LocalFileSkipped(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	srcFile := filepath.Join(sourceDir, "tutor.md")
	os.WriteFile(srcFile, []byte("# Tutor source"), 0644)

	// Pre-create local file (not a symlink)
	os.WriteFile(filepath.Join(targetDir, "tutor.md"), []byte("# Local tutor"), 0644)

	agents := []resource.DiscoveredResource{
		{FlatName: "tutor.md", AbsPath: srcFile},
	}

	result, err := SyncAgentsToTarget(agents, targetDir, false, false)
	if err != nil {
		t.Fatalf("SyncAgentsToTarget: %v", err)
	}

	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(result.Skipped))
	}
}

func TestSyncAgentsToTarget_ForceReplacesLocal(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	srcFile := filepath.Join(sourceDir, "tutor.md")
	os.WriteFile(srcFile, []byte("# Tutor source"), 0644)

	os.WriteFile(filepath.Join(targetDir, "tutor.md"), []byte("# Local"), 0644)

	agents := []resource.DiscoveredResource{
		{FlatName: "tutor.md", AbsPath: srcFile},
	}

	result, err := SyncAgentsToTarget(agents, targetDir, false, true)
	if err != nil {
		t.Fatalf("SyncAgentsToTarget: %v", err)
	}

	if len(result.Updated) != 1 {
		t.Errorf("expected 1 updated, got %d", len(result.Updated))
	}

	// Should now be a symlink
	info, _ := os.Lstat(filepath.Join(targetDir, "tutor.md"))
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink after force")
	}
}

func TestPruneOrphanAgentLinks(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	// Create source file and active symlink
	srcFile := filepath.Join(sourceDir, "active.md")
	os.WriteFile(srcFile, []byte("# Active"), 0644)
	os.Symlink(srcFile, filepath.Join(targetDir, "active.md"))

	// Create orphan symlink
	orphanSrc := filepath.Join(sourceDir, "orphan.md")
	os.WriteFile(orphanSrc, []byte("# Orphan"), 0644)
	os.Symlink(orphanSrc, filepath.Join(targetDir, "orphan.md"))

	// Create non-symlink file (should not be removed)
	os.WriteFile(filepath.Join(targetDir, "local.md"), []byte("# Local"), 0644)

	agents := []resource.DiscoveredResource{
		{FlatName: "active.md"},
	}

	removed, err := PruneOrphanAgentLinks(targetDir, sourceDir, agents, false)
	if err != nil {
		t.Fatalf("PruneOrphanAgentLinks: %v", err)
	}

	if len(removed) != 1 {
		t.Fatalf("expected 1 removed, got %d: %v", len(removed), removed)
	}
	if removed[0] != "orphan.md" {
		t.Errorf("expected orphan.md removed, got %q", removed[0])
	}

	// local.md should still exist
	if _, err := os.Stat(filepath.Join(targetDir, "local.md")); err != nil {
		t.Error("local.md should not be removed")
	}
}

// A link the user made to an agent outside the source is not skillshare's to remove.
func TestPruneOrphanAgentLinks_KeepsExternalLink(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	extFile := filepath.Join(t.TempDir(), "mine.md")
	os.WriteFile(extFile, []byte("# Mine"), 0644)
	link := filepath.Join(targetDir, "mine.md")
	os.Symlink(extFile, link)

	removed, err := PruneOrphanAgentLinks(targetDir, sourceDir, nil, false)
	if err != nil {
		t.Fatalf("PruneOrphanAgentLinks: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected external link kept, removed: %v", removed)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Error("external link should not be removed")
	}
}

// A link left behind by an old source location is broken and still pruned.
func TestPruneOrphanAgentLinks_RemovesBrokenExternalLink(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	os.Symlink(filepath.Join(t.TempDir(), "moved.md"), filepath.Join(targetDir, "moved.md"))

	removed, err := PruneOrphanAgentLinks(targetDir, sourceDir, nil, false)
	if err != nil {
		t.Fatalf("PruneOrphanAgentLinks: %v", err)
	}
	if len(removed) != 1 {
		t.Errorf("expected broken link removed, got %v", removed)
	}
}

// Relative links are written from the target's real directory, so an orphan
// under a symlinked target directory must still resolve into the source.
func TestPruneOrphanAgentLinks_RelativeLinkUnderSymlinkedTarget(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "proj", ".skillshare", "agents")
	realDir := filepath.Join(tmp, "home", "dotfiles", "claude") // deeper than the lexical path
	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(filepath.Join(realDir, "agents"), 0755)
	os.Symlink(realDir, filepath.Join(tmp, "proj", ".claude"))
	targetDir := filepath.Join(tmp, "proj", ".claude", "agents")

	srcFile := filepath.Join(sourceDir, "gone.md")
	os.WriteFile(srcFile, []byte("# Gone"), 0644)
	link := filepath.Join(targetDir, "gone.md")
	if err := createLink(link, srcFile, true); err != nil {
		t.Fatal(err)
	}
	// Keep the link live so only path resolution decides, not broken-link cleanup.
	removed, err := PruneOrphanAgentLinks(targetDir, sourceDir, nil, false)
	if err != nil {
		t.Fatalf("PruneOrphanAgentLinks: %v", err)
	}
	if len(removed) != 1 {
		t.Errorf("expected orphan relative link removed, got %v", removed)
	}
}

func TestPruneOrphanAgentLinks_NonExistentDir(t *testing.T) {
	removed, err := PruneOrphanAgentLinks("/nonexistent/path", "", nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(removed))
	}
}

func TestFindLocalAgentsAndPullAgents(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	// Create a local (non-symlink) agent file in target
	os.WriteFile(filepath.Join(targetDir, "new-agent.md"), []byte("# New agent"), 0644)

	// Create a symlink (should be skipped)
	srcFile := filepath.Join(sourceDir, "existing.md")
	os.WriteFile(srcFile, []byte("# Existing"), 0644)
	os.Symlink(srcFile, filepath.Join(targetDir, "existing.md"))

	// Create README (should be skipped)
	os.WriteFile(filepath.Join(targetDir, "README.md"), []byte("# Readme"), 0644)

	// Create non-md file (should be skipped)
	os.WriteFile(filepath.Join(targetDir, "config.yaml"), []byte("key: val"), 0644)

	collectDir := t.TempDir()
	agents, err := FindLocalAgents(targetDir, collectDir)
	if err != nil {
		t.Fatalf("FindLocalAgents: %v", err)
	}

	if len(agents) != 1 {
		t.Fatalf("expected 1 local agent, got %d: %v", len(agents), agents)
	}
	if agents[0].Name != "new-agent.md" {
		t.Errorf("agent name = %q, want %q", agents[0].Name, "new-agent.md")
	}

	result, err := PullAgents(agents, collectDir, PullOptions{})
	if err != nil {
		t.Fatalf("PullAgents: %v", err)
	}
	if len(result.Pulled) != 1 || result.Pulled[0] != "new-agent.md" {
		t.Fatalf("expected pulled=[new-agent.md], got %#v", result)
	}

	// Verify file was copied
	data, err := os.ReadFile(filepath.Join(collectDir, "new-agent.md"))
	if err != nil {
		t.Fatalf("collected file not found: %v", err)
	}
	if string(data) != "# New agent" {
		t.Errorf("collected content = %q", string(data))
	}
}

func TestFindLocalAgents_TargetSymlinkToSource_ReturnsEmpty(t *testing.T) {
	sourceDir := t.TempDir()
	targetParent := t.TempDir()
	targetDir := filepath.Join(targetParent, "agents")

	os.WriteFile(filepath.Join(sourceDir, "tutor.md"), []byte("# Tutor"), 0644)
	os.Symlink(sourceDir, targetDir)

	agents, err := FindLocalAgents(targetDir, sourceDir)
	if err != nil {
		t.Fatalf("FindLocalAgents: %v", err)
	}
	if len(agents) != 0 {
		t.Fatalf("expected 0 local agents, got %d", len(agents))
	}
}

func TestPullAgents_SkipsExistingWithoutForce(t *testing.T) {
	targetDir := t.TempDir()
	collectDir := t.TempDir()

	os.WriteFile(filepath.Join(targetDir, "agent.md"), []byte("# Target"), 0644)
	os.WriteFile(filepath.Join(collectDir, "agent.md"), []byte("# Source"), 0644)

	agents, err := FindLocalAgents(targetDir, collectDir)
	if err != nil {
		t.Fatalf("FindLocalAgents: %v", err)
	}

	result, err := PullAgents(agents, collectDir, PullOptions{})
	if err != nil {
		t.Fatalf("PullAgents: %v", err)
	}
	if len(result.Skipped) != 1 || result.Skipped[0] != "agent.md" {
		t.Fatalf("expected skipped=[agent.md], got %#v", result)
	}

	data, err := os.ReadFile(filepath.Join(collectDir, "agent.md"))
	if err != nil {
		t.Fatalf("read source agent: %v", err)
	}
	if string(data) != "# Source" {
		t.Fatalf("source agent should remain unchanged, got %q", string(data))
	}
}

func TestPullAgents_ForceOverwritesExisting(t *testing.T) {
	targetDir := t.TempDir()
	collectDir := t.TempDir()

	os.WriteFile(filepath.Join(targetDir, "agent.md"), []byte("# Target"), 0644)
	os.WriteFile(filepath.Join(collectDir, "agent.md"), []byte("# Source"), 0644)

	agents, err := FindLocalAgents(targetDir, collectDir)
	if err != nil {
		t.Fatalf("FindLocalAgents: %v", err)
	}

	result, err := PullAgents(agents, collectDir, PullOptions{Force: true})
	if err != nil {
		t.Fatalf("PullAgents: %v", err)
	}
	if len(result.Pulled) != 1 || result.Pulled[0] != "agent.md" {
		t.Fatalf("expected pulled=[agent.md], got %#v", result)
	}

	data, err := os.ReadFile(filepath.Join(collectDir, "agent.md"))
	if err != nil {
		t.Fatalf("read source agent: %v", err)
	}
	if string(data) != "# Target" {
		t.Fatalf("source agent should be overwritten, got %q", string(data))
	}
}

// --- Symlink mode tests ---

func TestSyncAgents_SymlinkMode_NewDir(t *testing.T) {
	sourceDir := t.TempDir()
	os.WriteFile(filepath.Join(sourceDir, "tutor.md"), []byte("# Tutor"), 0644)

	targetDir := filepath.Join(t.TempDir(), "agents")

	result, err := SyncAgents(nil, sourceDir, targetDir, "symlink", false, false)
	if err != nil {
		t.Fatalf("SyncAgents symlink: %v", err)
	}

	if len(result.Linked) != 1 {
		t.Errorf("expected 1 linked, got %d", len(result.Linked))
	}

	// targetDir should be a symlink to sourceDir
	info, err := os.Lstat(targetDir)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected targetDir to be a symlink")
	}
}

func TestSyncAgents_SymlinkMode_AlreadyCorrect(t *testing.T) {
	sourceDir := t.TempDir()
	parentDir := t.TempDir()
	targetDir := filepath.Join(parentDir, "agents")

	os.Symlink(sourceDir, targetDir)

	result, err := SyncAgents(nil, sourceDir, targetDir, "symlink", false, false)
	if err != nil {
		t.Fatalf("SyncAgents symlink: %v", err)
	}

	if len(result.Linked) != 1 {
		t.Errorf("expected 1 linked (already correct), got %d", len(result.Linked))
	}
	if len(result.Updated) != 0 {
		t.Errorf("expected 0 updated, got %d", len(result.Updated))
	}
}

func TestSyncAgents_SymlinkMode_RealDirSkipped(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir() // real directory

	result, err := SyncAgents(nil, sourceDir, targetDir, "symlink", false, false)
	if err != nil {
		t.Fatalf("SyncAgents symlink: %v", err)
	}

	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(result.Skipped))
	}
}

// --- Copy mode tests ---

func TestSyncAgents_CopyMode_NewFiles(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	os.WriteFile(filepath.Join(sourceDir, "tutor.md"), []byte("# Tutor"), 0644)

	agents := []resource.DiscoveredResource{
		{FlatName: "tutor.md", AbsPath: filepath.Join(sourceDir, "tutor.md")},
	}

	result, err := SyncAgents(agents, sourceDir, targetDir, "copy", false, false)
	if err != nil {
		t.Fatalf("SyncAgents copy: %v", err)
	}

	if len(result.Linked) != 1 {
		t.Errorf("expected 1 linked (new copy), got %d", len(result.Linked))
	}

	// Verify it's a real file, not a symlink
	info, _ := os.Lstat(filepath.Join(targetDir, "tutor.md"))
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("copy mode should create real files, not symlinks")
	}

	data, _ := os.ReadFile(filepath.Join(targetDir, "tutor.md"))
	if string(data) != "# Tutor" {
		t.Errorf("content = %q", string(data))
	}
}

func TestSyncAgents_CopyMode_SameContent(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	os.WriteFile(filepath.Join(sourceDir, "tutor.md"), []byte("# Same"), 0644)
	os.WriteFile(filepath.Join(targetDir, "tutor.md"), []byte("# Same"), 0644)

	agents := []resource.DiscoveredResource{
		{FlatName: "tutor.md", AbsPath: filepath.Join(sourceDir, "tutor.md")},
	}

	result, err := SyncAgents(agents, sourceDir, targetDir, "copy", false, false)
	if err != nil {
		t.Fatalf("SyncAgents copy: %v", err)
	}

	if len(result.Linked) != 1 {
		t.Errorf("expected 1 linked (same content), got %d", len(result.Linked))
	}
	if len(result.Updated) != 0 {
		t.Errorf("expected 0 updated, got %d", len(result.Updated))
	}
}

func TestSyncAgents_CopyMode_DifferentContent(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	os.WriteFile(filepath.Join(sourceDir, "tutor.md"), []byte("# New"), 0644)
	os.WriteFile(filepath.Join(targetDir, "tutor.md"), []byte("# Old"), 0644)

	agents := []resource.DiscoveredResource{
		{FlatName: "tutor.md", AbsPath: filepath.Join(sourceDir, "tutor.md")},
	}

	result, err := SyncAgents(agents, sourceDir, targetDir, "copy", false, false)
	if err != nil {
		t.Fatalf("SyncAgents copy: %v", err)
	}

	if len(result.Updated) != 1 {
		t.Errorf("expected 1 updated, got %d", len(result.Updated))
	}

	data, _ := os.ReadFile(filepath.Join(targetDir, "tutor.md"))
	if string(data) != "# New" {
		t.Errorf("content = %q, want %q", string(data), "# New")
	}
}

func TestPruneOrphanAgentCopies(t *testing.T) {
	targetDir := t.TempDir()

	os.WriteFile(filepath.Join(targetDir, "active.md"), []byte("# Active"), 0644)
	os.WriteFile(filepath.Join(targetDir, "orphan.md"), []byte("# Orphan"), 0644)
	os.WriteFile(filepath.Join(targetDir, "README.md"), []byte("# Readme"), 0644) // conventional, skip

	agents := []resource.DiscoveredResource{
		{FlatName: "active.md"},
	}

	removed, err := PruneOrphanAgentCopies(targetDir, agents, "", false)
	if err != nil {
		t.Fatalf("PruneOrphanAgentCopies: %v", err)
	}

	if len(removed) != 1 || removed[0] != "orphan.md" {
		t.Errorf("expected [orphan.md] removed, got %v", removed)
	}

	// README.md should still exist
	if _, err := os.Stat(filepath.Join(targetDir, "README.md")); err != nil {
		t.Error("README.md should not be removed")
	}
}

// --- Dispatch tests ---

func TestSyncAgents_DefaultIsMerge(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	os.WriteFile(filepath.Join(sourceDir, "a.md"), []byte("# A"), 0644)

	agents := []resource.DiscoveredResource{
		{FlatName: "a.md", AbsPath: filepath.Join(sourceDir, "a.md")},
	}

	result, err := SyncAgents(agents, sourceDir, targetDir, "", false, false)
	if err != nil {
		t.Fatalf("SyncAgents default: %v", err)
	}

	if len(result.Linked) != 1 {
		t.Errorf("expected 1 linked, got %d", len(result.Linked))
	}

	// Should be a symlink (merge mode)
	info, _ := os.Lstat(filepath.Join(targetDir, "a.md"))
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("default mode should create symlinks (merge)")
	}
}

func TestSyncAgents_MergeMode_NestedSameBasename_IsStable(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(sourceDir, "team-a"), 0o755); err != nil {
		t.Fatalf("mkdir team-a: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "team-b"), 0o755); err != nil {
		t.Fatalf("mkdir team-b: %v", err)
	}

	teamAPath := filepath.Join(sourceDir, "team-a", "helper.md")
	teamBPath := filepath.Join(sourceDir, "team-b", "helper.md")
	if err := os.WriteFile(teamAPath, []byte("# Team A"), 0o644); err != nil {
		t.Fatalf("write team-a helper: %v", err)
	}
	if err := os.WriteFile(teamBPath, []byte("# Team B"), 0o644); err != nil {
		t.Fatalf("write team-b helper: %v", err)
	}

	agents, err := resource.AgentKind{}.Discover(sourceDir)
	if err != nil {
		t.Fatalf("discover agents: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}

	first, err := SyncAgents(agents, sourceDir, targetDir, "merge", false, false)
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if len(first.Linked) != 2 {
		t.Fatalf("first sync: expected 2 linked, got %d", len(first.Linked))
	}
	if len(first.Updated) != 0 {
		t.Fatalf("first sync: expected 0 updated, got %d", len(first.Updated))
	}

	second, err := SyncAgents(agents, sourceDir, targetDir, "merge", false, false)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if len(second.Linked) != 2 {
		t.Fatalf("second sync: expected 2 linked, got %d", len(second.Linked))
	}
	if len(second.Updated) != 0 {
		t.Fatalf("second sync: expected 0 updated, got %d", len(second.Updated))
	}

	linkA, err := os.Readlink(filepath.Join(targetDir, "team-a__helper.md"))
	if err != nil {
		t.Fatalf("readlink team-a target: %v", err)
	}
	// EvalSymlinks to handle macOS /var → /private/var alias.
	wantA, _ := filepath.EvalSymlinks(teamAPath)
	gotA, _ := filepath.EvalSymlinks(linkA)
	if gotA != wantA {
		t.Fatalf("team-a symlink = %q, want %q", linkA, teamAPath)
	}

	linkB, err := os.Readlink(filepath.Join(targetDir, "team-b__helper.md"))
	if err != nil {
		t.Fatalf("readlink team-b target: %v", err)
	}
	wantB, _ := filepath.EvalSymlinks(teamBPath)
	gotB, _ := filepath.EvalSymlinks(linkB)
	if gotB != wantB {
		t.Fatalf("team-b symlink = %q, want %q", linkB, teamBPath)
	}
}

func TestSyncAgents_MergeMode_ProjectUsesRelativeSymlink(t *testing.T) {
	projectRoot := t.TempDir()
	sourceDir := filepath.Join(projectRoot, ".skillshare", "agents")
	targetDir := filepath.Join(projectRoot, ".claude", "agents")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	srcFile := filepath.Join(sourceDir, "reviewer.md")
	if err := os.WriteFile(srcFile, []byte("# Reviewer"), 0644); err != nil {
		t.Fatal(err)
	}

	agents := []resource.DiscoveredResource{
		{FlatName: "reviewer.md", AbsPath: srcFile},
	}

	if _, err := SyncAgents(agents, sourceDir, targetDir, "merge", false, false, projectRoot); err != nil {
		t.Fatalf("SyncAgents merge: %v", err)
	}

	link, err := os.Readlink(filepath.Join(targetDir, "reviewer.md"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if filepath.IsAbs(link) {
		t.Fatalf("expected project agent symlink to be relative, got %q", link)
	}
}

func TestSyncAgents_SymlinkMode_ProjectUsesRelativeSymlink(t *testing.T) {
	projectRoot := t.TempDir()
	sourceDir := filepath.Join(projectRoot, ".skillshare", "agents")
	targetDir := filepath.Join(projectRoot, ".claude", "agents")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "reviewer.md"), []byte("# Reviewer"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := SyncAgents(nil, sourceDir, targetDir, "symlink", false, false, projectRoot); err != nil {
		t.Fatalf("SyncAgents symlink: %v", err)
	}

	link, err := os.Readlink(targetDir)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if filepath.IsAbs(link) {
		t.Fatalf("expected project agent directory symlink to be relative, got %q", link)
	}
}

func TestSyncAgents_MergeMode_ReformatsProjectAgentSymlink(t *testing.T) {
	projectRoot := t.TempDir()
	sourceDir := filepath.Join(projectRoot, ".skillshare", "agents")
	targetDir := filepath.Join(projectRoot, ".claude", "agents")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	srcFile := filepath.Join(sourceDir, "reviewer.md")
	if err := os.WriteFile(srcFile, []byte("# Reviewer"), 0644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(targetDir, "reviewer.md")
	if err := os.Symlink(srcFile, linkPath); err != nil {
		t.Fatal(err)
	}

	agents := []resource.DiscoveredResource{
		{FlatName: "reviewer.md", AbsPath: srcFile},
	}

	result, err := SyncAgents(agents, sourceDir, targetDir, "merge", false, false, projectRoot)
	if err != nil {
		t.Fatalf("SyncAgents merge: %v", err)
	}
	if len(result.Updated) != 1 {
		t.Fatalf("expected reformat to count as updated, got %#v", result)
	}

	link, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if filepath.IsAbs(link) {
		t.Fatalf("expected reformatted project agent symlink to be relative, got %q", link)
	}
}

// TestSyncAgents_MergeMode_StableWhenSourceResolvesThroughSymlink reproduces the
// macOS /var → /private/var case on any platform: the project paths are expressed
// through a symlinked ancestor, while discovery records the agent's real
// (EvalSymlinks'd) AbsPath. A stable relative link must not be re-counted as
// "updated" on the second sync. Regression test for the agent-sync idempotency
// failure on macOS CI.
func TestSyncAgents_MergeMode_StableWhenSourceResolvesThroughSymlink(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}

	projectRoot := alias
	sourceDir := filepath.Join(alias, ".skillshare", "agents")
	targetDir := filepath.Join(alias, ".claude", "agents")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	srcFile := filepath.Join(sourceDir, "reviewer.md")
	if err := os.WriteFile(srcFile, []byte("# Reviewer"), 0644); err != nil {
		t.Fatal(err)
	}

	// AbsPath mirrors real discovery, which resolves symlinked ancestors.
	realSrc, err := filepath.EvalSymlinks(srcFile)
	if err != nil {
		t.Fatal(err)
	}
	agents := []resource.DiscoveredResource{
		{FlatName: "reviewer.md", AbsPath: realSrc},
	}

	first, err := SyncAgents(agents, sourceDir, targetDir, "merge", false, false, projectRoot)
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if len(first.Updated) != 0 {
		t.Fatalf("first sync should not update, got %#v", first)
	}

	second, err := SyncAgents(agents, sourceDir, targetDir, "merge", false, false, projectRoot)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if len(second.Updated) != 0 {
		t.Fatalf("second sync should be stable (0 updated), got %#v", second)
	}
	if len(second.Linked) != 1 {
		t.Fatalf("second sync should re-link existing agent, got %#v", second)
	}
}

func TestPullAgents_DryRun(t *testing.T) {
	targetDir := t.TempDir()
	os.WriteFile(filepath.Join(targetDir, "agent.md"), []byte("# Agent"), 0644)

	collectDir := t.TempDir()
	agents, err := FindLocalAgents(targetDir, collectDir)
	if err != nil {
		t.Fatalf("FindLocalAgents: %v", err)
	}

	result, err := PullAgents(agents, collectDir, PullOptions{DryRun: true})
	if err != nil {
		t.Fatalf("PullAgents dry-run: %v", err)
	}

	if len(result.Pulled) != 1 {
		t.Fatalf("expected 1 collected in dry-run, got %d", len(result.Pulled))
	}

	// File should NOT exist (dry-run)
	if _, err := os.Stat(filepath.Join(collectDir, "agent.md")); err == nil {
		t.Error("file should not exist in dry-run")
	}
}

// agentTransformFixture creates source agent tutor.md and returns it with an
// uppercasing spec, so any write-through to the source is visible.
func agentTransformFixture(t *testing.T) (string, []resource.DiscoveredResource, *ExtensionSpec) {
	t.Helper()
	sourceDir := t.TempDir()
	src := filepath.Join(sourceDir, "tutor.md")
	if err := os.WriteFile(src, []byte("body"), 0644); err != nil {
		t.Fatal(err)
	}
	agents := []resource.DiscoveredResource{{FlatName: "tutor.md", RelPath: "tutor.md", AbsPath: src}}
	spec := &ExtensionSpec{Run: []string{"tr", "a-z", "A-Z"}, Dir: sourceDir, Name: "upper"}
	return sourceDir, agents, spec
}

func assertSourceUntouched(t *testing.T, sourceDir string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(sourceDir, "tutor.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "body" {
		t.Errorf("source agent overwritten: %q", data)
	}
}

func TestSyncAgentsTransform_ReplacesLeftoverFileSymlink(t *testing.T) {
	sourceDir, agents, spec := agentTransformFixture(t)
	targetDir := t.TempDir()
	// Left over from merge mode.
	if err := os.Symlink(agents[0].AbsPath, filepath.Join(targetDir, "tutor.md")); err != nil {
		t.Fatal(err)
	}

	if _, err := SyncAgentsTransform(agents, sourceDir, targetDir, "", spec, false, false); err != nil {
		t.Fatal(err)
	}
	assertSourceUntouched(t, sourceDir)
}

func TestSyncAgentsTransform_ReplacesLeftoverDirSymlink(t *testing.T) {
	sourceDir, agents, spec := agentTransformFixture(t)
	targetDir := filepath.Join(t.TempDir(), "agents")
	// Left over from symlink mode.
	if err := os.Symlink(sourceDir, targetDir); err != nil {
		t.Fatal(err)
	}

	if _, err := SyncAgentsTransform(agents, sourceDir, targetDir, "", spec, false, false); err != nil {
		t.Fatal(err)
	}
	assertSourceUntouched(t, sourceDir)
}

func TestSyncAgentsTransform_FailureRemovesStaleOutput(t *testing.T) {
	sourceDir, agents, _ := agentTransformFixture(t)
	targetDir := t.TempDir()
	stale := filepath.Join(targetDir, "tutor.md")
	if err := os.WriteFile(stale, []byte("unconverted"), 0644); err != nil {
		t.Fatal(err)
	}
	failing := &ExtensionSpec{Run: []string{"false"}, Dir: sourceDir, Name: "fail"}

	if _, err := SyncAgentsTransform(agents, sourceDir, targetDir, "", failing, false, false); err == nil {
		t.Fatal("expected conversion error")
	}
	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Error("an agent that fails to convert must not keep its old output")
	}
}

func TestPruneOrphanAgentCopies_PrunesTransformedOrphans(t *testing.T) {
	targetDir := t.TempDir()
	orphan := filepath.Join(targetDir, "gone.toml")
	if err := os.WriteFile(orphan, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := PruneOrphanAgentCopies(targetDir, nil, "toml", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("orphan gone.toml should be pruned")
	}
}
