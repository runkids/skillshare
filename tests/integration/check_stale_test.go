//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// TestCheck_Stale_DeletedSubdir verifies that when a skill's subdir is deleted
// from the upstream repo, check reports "stale" instead of "update_available".
func TestCheck_Stale_DeletedSubdir(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Create bare remote with two skill subdirs
	remoteRepo := filepath.Join(sb.Root, "stale-registry.git")
	workClone := filepath.Join(sb.Root, "stale-work")
	gitInit(t, remoteRepo, true)
	gitClone(t, remoteRepo, workClone)

	os.MkdirAll(filepath.Join(workClone, "skills", "alive"), 0755)
	os.WriteFile(filepath.Join(workClone, "skills", "alive", "SKILL.md"), []byte("# Alive"), 0644)
	os.MkdirAll(filepath.Join(workClone, "skills", "doomed"), 0755)
	os.WriteFile(filepath.Join(workClone, "skills", "doomed", "SKILL.md"), []byte("# Doomed"), 0644)
	gitAddCommit(t, workClone, "add both skills")
	gitPush(t, workClone)

	commitHash := gitRevParse(t, workClone, "HEAD")
	aliveTreeHash := gitRevParse(t, workClone, "HEAD:skills/alive")
	doomedTreeHash := gitRevParse(t, workClone, "HEAD:skills/doomed")

	// Install both skills locally with tree hash metadata
	aliveDir := sb.CreateSkill("alive", map[string]string{"SKILL.md": "# Alive"})
	writeMetaWithTreeHash(t, aliveDir, "file://"+remoteRepo, commitHash, aliveTreeHash, "skills/alive")

	doomedDir := sb.CreateSkill("doomed", map[string]string{"SKILL.md": "# Doomed"})
	writeMetaWithTreeHash(t, doomedDir, "file://"+remoteRepo, commitHash, doomedTreeHash, "skills/doomed")

	// Delete doomed from remote, push new commit
	os.RemoveAll(filepath.Join(workClone, "skills", "doomed"))
	gitAddCommit(t, workClone, "remove doomed skill")
	gitPush(t, workClone)

	// Check via JSON
	result := sb.RunCLI("check", "--json")
	result.AssertSuccess(t)

	var output checkOutput
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, result.Stdout)
	}

	if len(output.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(output.Skills))
	}

	statusMap := make(map[string]string)
	for _, s := range output.Skills {
		statusMap[s.Name] = s.Status
	}

	if statusMap["alive"] != "up_to_date" {
		t.Errorf("expected alive to be up_to_date, got %q", statusMap["alive"])
	}
	if statusMap["doomed"] != "stale" {
		t.Errorf("expected doomed to be stale, got %q", statusMap["doomed"])
	}
}

// TestCheck_Stale_WarningInTextOutput verifies the human-readable output
// includes a stale warning with --prune hint.
func TestCheck_Stale_WarningInTextOutput(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	remoteRepo := filepath.Join(sb.Root, "stale-text.git")
	workClone := filepath.Join(sb.Root, "stale-text-work")
	gitInit(t, remoteRepo, true)
	gitClone(t, remoteRepo, workClone)

	os.MkdirAll(filepath.Join(workClone, "skills", "gone-skill"), 0755)
	os.WriteFile(filepath.Join(workClone, "skills", "gone-skill", "SKILL.md"), []byte("# Gone"), 0644)
	gitAddCommit(t, workClone, "add skill")
	gitPush(t, workClone)

	commitHash := gitRevParse(t, workClone, "HEAD")
	treeHash := gitRevParse(t, workClone, "HEAD:skills/gone-skill")

	skillDir := sb.CreateSkill("gone-skill", map[string]string{"SKILL.md": "# Gone"})
	writeMetaWithTreeHash(t, skillDir, "file://"+remoteRepo, commitHash, treeHash, "skills/gone-skill")

	// Delete skill from remote
	os.RemoveAll(filepath.Join(workClone, "skills", "gone-skill"))
	gitAddCommit(t, workClone, "remove skill")
	gitPush(t, workClone)

	// Check without --json — should show stale warning
	result := sb.RunCLI("check")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "stale")
	result.AssertAnyOutputContains(t, "--prune")
}

// TestCheck_Stale_SingleName verifies single-target check shows stale status.
func TestCheck_Stale_SingleName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	remoteRepo := filepath.Join(sb.Root, "stale-single.git")
	workClone := filepath.Join(sb.Root, "stale-single-work")
	gitInit(t, remoteRepo, true)
	gitClone(t, remoteRepo, workClone)

	os.MkdirAll(filepath.Join(workClone, "skills", "vanished"), 0755)
	os.WriteFile(filepath.Join(workClone, "skills", "vanished", "SKILL.md"), []byte("# V"), 0644)
	gitAddCommit(t, workClone, "add skill")
	gitPush(t, workClone)

	commitHash := gitRevParse(t, workClone, "HEAD")
	treeHash := gitRevParse(t, workClone, "HEAD:skills/vanished")

	skillDir := sb.CreateSkill("vanished", map[string]string{"SKILL.md": "# V"})
	writeMetaWithTreeHash(t, skillDir, "file://"+remoteRepo, commitHash, treeHash, "skills/vanished")

	// Remove from remote
	os.RemoveAll(filepath.Join(workClone, "skills", "vanished"))
	gitAddCommit(t, workClone, "remove vanished")
	gitPush(t, workClone)

	result := sb.RunCLI("check", "vanished")
	result.AssertSuccess(t)

	combined := result.Stdout + result.Stderr
	if !strings.Contains(strings.ToLower(combined), "stale") {
		t.Errorf("expected stale in output, got:\nstdout: %s\nstderr: %s", result.Stdout, result.Stderr)
	}
}
