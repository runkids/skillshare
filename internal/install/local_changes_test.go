package install

import (
	"errors"
	"os"
	"os/exec"
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

func TestUpstreamChanges_DiffsInstalledAgainstLatest(t *testing.T) {
	repo := initTestRepo(t)
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("SKILL.md", "---\nname: alpha\n---\nv1\n")
	write("old.md", "old\n")
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

	// Up to date: nothing is fetched, so even an unusable remote is fine.
	same, err := UpstreamChanges(entry, ParseOptions{}, installedCommit(entry)[:7])
	if err != nil || len(same.Files) != 0 {
		t.Fatalf("up to date: files = %+v, err = %v", same, err)
	}

	write("SKILL.md", "---\nname: alpha\n---\nv2\n")
	write("new.md", "new\n")
	if err := os.Remove(filepath.Join(repo, "old.md")); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "v2")
	headOut, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(string(headOut))

	got, err := UpstreamChanges(entry, ParseOptions{}, head)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, c := range got.Files {
		files = append(files, c.Status+" "+c.Path)
	}
	want := "modified SKILL.md|added new.md|deleted old.md"
	if strings.Join(files, "|") != want {
		t.Errorf("files = %q, want %q", strings.Join(files, "|"), want)
	}
	if got.From == got.To || got.To != head {
		t.Errorf("from %q, to %q: want different commits ending at %q", got.From, got.To, head)
	}
	if !strings.Contains(got.Files[0].Diff, "- v1") || !strings.Contains(got.Files[0].Diff, "+ v2") {
		t.Errorf("SKILL.md diff = %q", got.Files[0].Diff)
	}
}

func TestUpstreamChanges_NoInstallRecord(t *testing.T) {
	for _, e := range []*MetadataEntry{nil, {Tracked: true, RepoURL: "u", Version: "abc1234"}, {RepoURL: "u"}} {
		if _, err := UpstreamChanges(e, ParseOptions{}, "abc1234"); !errors.Is(err, ErrNoInstallRecord) {
			t.Errorf("entry %+v: err = %v, want ErrNoInstallRecord", e, err)
		}
	}
}
