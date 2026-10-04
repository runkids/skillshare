//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/memory"
	"skillshare/internal/testutil"
)

func TestExtrasMemory_GlobalWorkflow(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	sb.RunCLI("extras", "memory", "init", "-g").AssertSuccess(t)
	sb.RunCLI("extras", "memory", "show", "INDEX.md", "-g").AssertOutputContains(t, "[Lessons learned](LEARNED.md)")
	sb.RunCLI("extras", "memory", "show", "LEARNED.md", "-g").AssertOutputContains(t, "Evidence:")
	input := filepath.Join(sb.Home, "note.md")
	os.WriteFile(input, []byte("# Build\nUse the devcontainer.\n"), 0644)
	sb.RunCLI("extras", "memory", "write", "build.md", "--from", input, "-g").AssertSuccess(t)
	sb.RunCLI("extras", "memory", "list", "--search", "devcontainer", "--json", "-g").AssertOutputContains(t, "build.md")
	sb.RunCLI("extras", "memory", "show", "build.md", "-g").AssertOutputContains(t, "# Build")
	sb.RunCLI("extras", "memory", "instructions", "-g").AssertOutputContains(t, "Read only the notes relevant")
	sb.RunCLI("extras", "memory", "instructions", "--update-mode", "active", "-g").AssertOutputContains(t, "propose the note to the user")
	sb.RunCLI("extras", "memory", "instructions", "--update-mode", "eager", "-g").AssertFailure(t)
	sb.RunCLI("extras", "memory", "write", "../escape.md", "--from", input, "-g").AssertFailure(t)
	// A second create without an explicit version must preserve the existing note.
	sb.RunCLI("extras", "memory", "write", "build.md", "--from", input, "-g").AssertFailure(t)
}

func TestExtrasMemory_ProjectSourceAndUpdate(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	project := filepath.Join(sb.Home, "project")
	sb.WriteProjectConfig(project, "targets: []\nextras:\n  - name: memory\n    source: notes\n    targets: []\n")
	sb.RunCLIInDir(project, "extras", "memory", "init", "-p").AssertSuccess(t)
	sb.RunCLIInDir(project, "extras", "memory", "show", "LEARNED.md", "-p").AssertOutputContains(t, "Evidence:")
	sb.RunCLIInDir(project, "extras", "memory", "instructions", "-p").AssertOutputContains(t, "`notes` (relative to the project root)")
	if _, err := os.Stat(filepath.Join(project, "notes/INDEX.md")); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(sb.Home, "note.md")
	if err := os.WriteFile(input, []byte("# First\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sb.RunCLIInDir(project, "extras", "memory", "write", "decisions.md", "--from", input, "-p").AssertSuccess(t)
	shown := sb.RunCLIInDir(project, "extras", "memory", "show", "decisions.md", "--json")
	shown.AssertSuccess(t)
	var note memory.Note
	if err := json.Unmarshal([]byte(shown.Stdout), &note); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, []byte("# Second\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sb.RunCLIInDir(project, "extras", "memory", "write", "decisions.md", "--from", input, "--version", note.Version).AssertSuccess(t)
	sb.RunCLIInDir(project, "extras", "memory", "write", "decisions.md", "--from", input, "--version", note.Version).AssertFailure(t)
	sb.RunCLIInDir(project, "extras", "memory", "show", "decisions.md").AssertOutputContains(t, "# Second")
}

func TestExtrasMemory_Delete(t *testing.T) {
	for _, projectMode := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[projectMode], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			dir, scope := sb.Home, "-g"
			if projectMode {
				dir, scope = filepath.Join(sb.Home, "project"), "-p"
				sb.WriteProjectConfig(dir, "targets: []\n")
			} else {
				sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			}
			sb.RunCLIInDir(dir, "extras", "memory", "init", scope).AssertSuccess(t)
			shown := sb.RunCLIInDir(dir, "extras", "memory", "show", "LEARNED.md", "--json", scope)
			shown.AssertSuccess(t)
			var note memory.Note
			if err := json.Unmarshal([]byte(shown.Stdout), &note); err != nil {
				t.Fatal(err)
			}
			sb.RunCLIInDir(dir, "extras", "memory", "delete", "LEARNED.md", scope).AssertFailure(t)
			sb.RunCLIInDir(dir, "extras", "memory", "delete", "LEARNED.md", "--version", "stale", scope).AssertFailure(t)
			sb.RunCLIInDir(dir, "extras", "memory", "show", "LEARNED.md", scope).AssertSuccess(t)
			deleted := sb.RunCLIInDir(dir, "extras", "memory", "delete", "LEARNED.md", "--version", note.Version, "--json", scope)
			deleted.AssertSuccess(t)
			deleted.AssertOutputContains(t, `"success":true`)
			sb.RunCLIInDir(dir, "extras", "memory", "show", "LEARNED.md", scope).AssertFailure(t)
			sb.RunCLIInDir(dir, "extras", "memory", "show", "INDEX.md", scope).AssertSuccess(t)
		})
	}
}
