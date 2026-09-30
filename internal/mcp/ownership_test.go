package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ownershipSource = "mcp:\n  targets: [pi]\n  servers:\n    mcp-test1:\n      url: https://example.com/mcp\n"

func writeSource(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

// ownedProject syncs one Pi server from the hidden config, then returns a service for
// the visible one, as a project gets when it swaps .skillshare/ for skillshare/. Both
// configs sit in one project, so Pi's project file does not move with them and the
// ledger still names the hidden config.
func ownedProject(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "project")
	stateDir := filepath.Join(dir, "state")

	hidden := filepath.Join(root, ".skillshare", "config.yaml")
	writeSource(t, hidden, ownershipSource)
	if _, err := (&Service{ConfigPath: hidden, ProjectRoot: root, Home: dir, StateDir: stateDir, Platform: "linux"}).Apply(""); err != nil {
		t.Fatal(err)
	}

	visible := filepath.Join(root, "skillshare", "config.yaml")
	writeSource(t, visible, ownershipSource)
	return &Service{ConfigPath: visible, ProjectRoot: root, Home: dir, StateDir: stateDir, Platform: "linux"}
}

// A removed owner can never release the entry, so the conflict has to say so: the
// dashboard offers import and replace on this message and on no other owner conflict,
// and "managed by" sent the user hunting for a file that is not there. Refs: #288.
func TestConflictNamesARemovedOwnerDifferently(t *testing.T) {
	s := ownedProject(t)

	p, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.Changes[0].Message, "managed by another Skillshare config: ") {
		t.Fatalf("a live owner must still be named: %s", p.Changes[0].Message)
	}

	if err = os.RemoveAll(filepath.Join(s.ProjectRoot, ".skillshare")); err != nil {
		t.Fatal(err)
	}
	if p, err = s.Preview(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.Changes[0].Message, orphanedMessage+": ") {
		t.Fatalf("a removed owner reads as if it were still there: %s", p.Changes[0].Message)
	}
}

// The message is only worth changing if the buttons it advertises work, so settle it
// the way the dashboard's Replace does and check the entry ends up owned.
func TestRemovedOwnerIsSettledByReplace(t *testing.T) {
	s := ownedProject(t)
	if err := os.RemoveAll(filepath.Join(s.ProjectRoot, ".skillshare")); err != nil {
		t.Fatal(err)
	}

	m := Mutation{Resolutions: []Resolution{{Target: "pi", Name: "mcp-test1", Action: "replace"}}}
	p, err := s.PreviewMutation(m)
	if err != nil {
		t.Fatal(err)
	}
	if p.Blocked {
		t.Fatalf("replace cannot settle it: %s", p.Changes[0].Message)
	}
	if _, err = s.Mutate(m, p.Revision, true); err != nil {
		t.Fatal(err)
	}

	// Taking it over has to reach the ledger: dropping the server must now clean Pi's
	// file. An entry that syncs but stays unowned is the amnesia deleting state.json gives.
	writeSource(t, s.ConfigPath, "mcp:\n  targets: [pi]\n  servers: {}\n")
	if p, err = s.Preview(); err != nil {
		t.Fatal(err)
	}
	if len(p.Changes) != 1 || p.Changes[0].Action != "remove" {
		t.Fatalf("entry is not owned after replace: %+v", p.Changes)
	}
}
