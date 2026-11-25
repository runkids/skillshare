package install

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInferTrackedKind_MixedRepoReturnsTypedError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	remote := filepath.Join(tmp, "remote.git")

	mustRunGit(t, "", "init", work)
	mustRunGit(t, work, "config", "user.email", "test@test.com")
	mustRunGit(t, work, "config", "user.name", "Test")

	// Two skills under skills/
	for _, name := range []string{"one", "two"} {
		dir := filepath.Join(work, "skills", name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Three agents under agents/
	agentsDir := filepath.Join(work, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha.md", "beta.md", "gamma.md"} {
		if err := os.WriteFile(filepath.Join(agentsDir, name), []byte("# "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	mustRunGit(t, work, "add", ".")
	mustRunGit(t, work, "commit", "-m", "init")
	mustRunGit(t, "", "clone", "--bare", work, remote)

	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      "file://" + remote,
		CloneURL: "file://" + remote,
	}

	_, err := InferTrackedKind(source, "")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}

	var ambig *TrackKindAmbiguousError
	if !errors.As(err, &ambig) {
		t.Fatalf("expected *TrackKindAmbiguousError, got %T: %v", err, err)
	}
	if ambig.Skills != 2 {
		t.Errorf("Skills = %d, want 2", ambig.Skills)
	}
	if ambig.Agents != 3 {
		t.Errorf("Agents = %d, want 3", ambig.Agents)
	}

	// Error() must preserve the legacy message verbatim so CLI output is
	// unchanged and any existing string-based tests keep passing.
	const wantMsg = "tracked install is ambiguous for mixed repositories; pass --kind skill or --kind agent"
	if err.Error() != wantMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), wantMsg)
	}
}

func TestInferTrackedKind_ExplicitKindSkipsDiscovery(t *testing.T) {
	// No git invocation required — explicit kind short-circuits before clone.
	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      "https://example.invalid/repo",
		CloneURL: "https://example.invalid/repo",
	}

	for _, kind := range []string{"skill", "agent"} {
		got, err := InferTrackedKind(source, kind)
		if err != nil {
			t.Fatalf("kind=%q: unexpected error %v", kind, err)
		}
		if got != kind {
			t.Errorf("kind=%q: got %q", kind, got)
		}
	}
}

// Regression for #1013: a tracked install of one agent file is refused, with
// or without --kind, instead of tracking the whole repository.
func TestInferTrackedKind_AgentFileRefused(t *testing.T) {
	for _, kind := range []string{"", "agent", "skill"} {
		source, err := ParseSource("github.com/you/repo/agents/demo.md")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := InferTrackedKind(source, kind); err == nil || !strings.Contains(err.Error(), "not the agent file 'agents/demo.md'") {
			t.Errorf("kind %q: expected the agent file to be refused, got %v", kind, err)
		}
	}
}
