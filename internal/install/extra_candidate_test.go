package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareExtraCandidate_PinnedAndPrivate(t *testing.T) {
	remote, first, _ := setupPinRemote(t)
	source := &Source{Type: SourceTypeGitHTTPS, Raw: remote, CloneURL: remote, Commit: first}
	root := filepath.Join(t.TempDir(), "extras")
	candidate, cleanup, err := prepareExtraCandidate(source, root, filepath.Join(root, "rules"), InstallOptions{SkipAudit: true})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if candidate.fetch.Commit != first || !candidate.audit.Skipped {
		t.Fatalf("candidate: %+v", candidate)
	}
	contents, err := os.ReadFile(filepath.Join(candidate.staged, "SKILL.md"))
	if err != nil || string(contents) != "# v1" {
		t.Fatalf("staged: %q %v", contents, err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("publication before gate: %v", err)
	}
}

func TestInstallRemoteExtra_RejectsInvalidThresholdBeforeFetch(t *testing.T) {
	for _, skip := range []bool{false, true} {
		root := filepath.Join(t.TempDir(), "extras")
		source := &Source{Type: SourceTypeGitHTTPS, Raw: "file:///does-not-exist", CloneURL: "file:///does-not-exist"}
		_, err := InstallRemoteExtra(RemoteExtraRequest{Name: "rules", RawSource: source.Raw, Source: source, ExtrasRoot: root,
			Opts: InstallOptions{SkipAudit: skip, AuditThreshold: "not-a-level"}})
		if err == nil || !strings.Contains(err.Error(), "threshold") {
			t.Fatalf("skip=%t got %v", skip, err)
		}
		if _, err := os.Lstat(root); !os.IsNotExist(err) {
			t.Fatalf("invalid threshold created administrative root: %v", err)
		}
	}
}
