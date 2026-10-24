package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalChanges_DiffsAgainstInstalledCommit(t *testing.T) {
	repo := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("---\nname: alpha\n---\nv1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "v1")

	sourceDir := t.TempDir()
	dest := filepath.Join(sourceDir, "alpha")
	source, err := ParseSource("file://" + repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Install(source, dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true}); err != nil {
		t.Fatal(err)
	}
	entry := LoadMetadataOrNew(sourceDir).Get("alpha")

	// Upstream moves on; the diff must still be against the installed v1.
	if err := os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("---\nname: alpha\n---\nv2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "v2")

	// Edit in place like an editor would: git may have checked out CRLF on Windows.
	installed, err := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "SKILL.md"), []byte(strings.Replace(string(installed), "v1", "mine", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "notes.md"), []byte("note\n"), 0644); err != nil {
		t.Fatal(err)
	}

	changes, err := LocalChanges(entry, ParseOptions{}, dest)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range changes {
		got = append(got, c.Status+" "+c.Path+"\n"+strings.ReplaceAll(c.Diff, "\r", ""))
	}
	want := []string{
		"modified SKILL.md\n  ---\n  name: alpha\n  ---\n- v1\n+ mine\n",
		"added notes.md\n+ note\n",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("changes =\n%q\nwant\n%q", got, want)
	}
}

func TestLocalChanges_UnchangedSkillDoesNotFetch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	hashes, err := ComputeFileHashes(dir)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := LocalChanges(&MetadataEntry{Source: "file:///nonexistent/repo", RepoURL: "file:///nonexistent/repo", FileHashes: hashes}, ParseOptions{}, dir)
	if err != nil || changes != nil {
		t.Errorf("LocalChanges = %v, %v; want nil, nil", changes, err)
	}
}

// Without the installed commit the diff would be against upstream HEAD and
// show upstream changes as the user's.
func TestLocalChanges_NoRecordedCommit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LocalChanges(&MetadataEntry{Source: "file:///nonexistent/repo", RepoURL: "file:///nonexistent/repo", FileHashes: map[string]string{"SKILL.md": "sha256:old"}}, ParseOptions{}, dir)
	if err == nil || !strings.Contains(err.Error(), "no recorded commit") {
		t.Errorf("err = %v, want no recorded commit", err)
	}
}

// Added files diff against nothing, so they need neither a commit nor a fetch.
func TestLocalChanges_OnlyAddedFilesDoNotFetch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	hashes, err := ComputeFileHashes(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("mine\n"), 0644); err != nil {
		t.Fatal(err)
	}
	changes, err := LocalChanges(&MetadataEntry{Source: "file:///nonexistent/repo", RepoURL: "file:///nonexistent/repo", FileHashes: hashes}, ParseOptions{}, dir)
	if err != nil || len(changes) != 1 || changes[0] != (FileChange{Path: "notes.md", Status: "added", Diff: "+ mine\n"}) {
		t.Errorf("LocalChanges = %+v, %v", changes, err)
	}
}

// A short SHA makes git fetch the whole history, so the full recorded commit
// is used when it is the same commit as Version.
func TestInstalledCommit(t *testing.T) {
	full := "0123456789abcdef0123456789abcdef01234567"
	for _, c := range []struct{ version, commit, want string }{
		{"0123456", full, full},
		{"0123456", "", "0123456"},
		{"89abcde", full, "89abcde"}, // stale Commit from an older install
		{"", full, ""},
	} {
		if got := installedCommit(&MetadataEntry{Version: c.version, Commit: c.commit}); got != c.want {
			t.Errorf("installedCommit(%q, %q) = %q, want %q", c.version, c.commit, got, c.want)
		}
	}
}
