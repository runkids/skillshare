package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupportsSparseCheckoutVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{version: "git version 2.24.9", want: false},
		{version: "git version 2.25.0", want: true},
		{version: "git version 2.39.3 (Apple Git-146)", want: true},
		{version: "git version 3.0.0", want: true},
		{version: "invalid", want: false},
	}

	for _, tt := range tests {
		got := supportsSparseCheckoutVersion(tt.version)
		if got != tt.want {
			t.Fatalf("supportsSparseCheckoutVersion(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestSparseCloneSubdir_FileURL(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if !gitSupportsSparseCheckout() {
		t.Skip("git does not support sparse-checkout")
	}

	remote := sparseFixtureRemote(t)
	dest := filepath.Join(t.TempDir(), "clone")

	if err := sparseCloneSubdir("file://"+remote, "skills/one", dest, "", nil, nil); err != nil {
		t.Fatalf("sparseCloneSubdir() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dest, "skills", "one", "SKILL.md")); err != nil {
		t.Fatalf("expected sparse path file to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "other", "README.md")); err == nil {
		t.Fatalf("expected non-sparse path to be absent")
	}
}

// Git exports GIT_DIR to hooks in a linked worktree; a hook running install
// must not turn on sparse checkout in that repository (#852).
func TestSparseCloneSubdir_IgnoresCallerGitDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if !gitSupportsSparseCheckout() {
		t.Skip("git does not support sparse-checkout")
	}

	remote := sparseFixtureRemote(t)
	host := filepath.Join(t.TempDir(), "host")
	mustRunGit(t, "", "init", host)
	mustRunGit(t, host, "-c", "user.email=test@test.com", "-c", "user.name=Test", "commit", "--allow-empty", "-m", "init")
	dest := filepath.Join(t.TempDir(), "clone")

	t.Setenv("GIT_DIR", filepath.Join(host, ".git"))
	if err := sparseCloneSubdir("file://"+remote, "skills/one", dest, "", nil, nil); err != nil {
		t.Fatalf("sparseCloneSubdir() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(host, ".git", "info", "sparse-checkout")); err == nil {
		t.Fatal("sparse checkout was set up in the caller's repository")
	}
}

// sparseFixtureRemote returns a bare repository holding skills/one and other.
func sparseFixtureRemote(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	remote := filepath.Join(tmp, "remote.git")

	mustRunGit(t, "", "init", work)
	mustRunGit(t, work, "config", "user.email", "test@test.com")
	mustRunGit(t, work, "config", "user.name", "Test")

	if err := os.MkdirAll(filepath.Join(work, "skills", "one"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(work, "other"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "skills", "one", "SKILL.md"), []byte("# one"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "other", "README.md"), []byte("other"), 0644); err != nil {
		t.Fatal(err)
	}

	mustRunGit(t, work, "add", ".")
	mustRunGit(t, work, "commit", "-m", "init")
	mustRunGit(t, "", "clone", "--bare", work, remote)
	return remote
}

func mustRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestParseLsTreeEntries(t *testing.T) {
	raw := []byte("100644 blob aaaa\tskills/demo/SKILL.md\x00120000 blob bbbb\tskills/demo/ref with space.md\x00040000 tree cccc\tshared/refs\x00")
	entries := parseLsTreeEntries(raw)
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
	if entries[1].mode != "120000" || entries[1].kind != "blob" || entries[1].oid != "bbbb" || entries[1].path != "skills/demo/ref with space.md" {
		t.Fatalf("unexpected entry[1]: %+v", entries[1])
	}
	if entries[2].mode != "040000" || entries[2].kind != "tree" || entries[2].path != "shared/refs" {
		t.Fatalf("unexpected entry[2]: %+v", entries[2])
	}
}

// sparseSymlinkFixtureRemote creates a bare repo where skills/demo links to:
//   - ../../shared/ref.md (file outside subdir)
//   - ../../shared/refs (directory outside subdir, containing a chained link to ../extra/deep.md)
//   - ../../ROOT.md (repo-root file)
func sparseSymlinkFixtureRemote(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	remote := filepath.Join(tmp, "remote.git")

	for _, dir := range []string{
		filepath.Join(work, "skills", "demo"),
		filepath.Join(work, "shared", "refs"),
		filepath.Join(work, "shared", "extra"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"ROOT.md":              "root doc",
		"skills/demo/SKILL.md": "---\nname: demo\n---\n# Demo\n",
		"shared/ref.md":        "shared file body",
		"shared/refs/guide.md": "shared dir guide",
		"shared/extra/deep.md": "chained extra doc",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(work, filepath.FromSlash(rel)), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join("..", "..", "shared", "ref.md"), filepath.Join(work, "skills", "demo", "ref.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "shared", "refs"), filepath.Join(work, "skills", "demo", "refs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "ROOT.md"), filepath.Join(work, "skills", "demo", "root.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "extra", "deep.md"), filepath.Join(work, "shared", "refs", "deep.md")); err != nil {
		t.Fatal(err)
	}

	mustRunGit(t, "", "init", work)
	mustRunGit(t, work, "config", "user.email", "test@test.com")
	mustRunGit(t, work, "config", "user.name", "Test")
	mustRunGit(t, work, "add", ".")
	mustRunGit(t, work, "commit", "-m", "add skill with cross-repo symlinks")
	mustRunGit(t, "", "clone", "--bare", work, remote)
	return remote
}

func TestSparseLinksFetched(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if !gitSupportsSparseCheckout() {
		t.Skip("git does not support sparse-checkout")
	}

	remote := sparseSymlinkFixtureRemote(t)
	tests := []struct {
		subdir  string
		wantErr bool
	}{
		{"skills/demo", true}, // links to shared/ and ROOT.md, which the checkout left out
		{"shared", false},     // refs/deep.md links to extra/, inside shared
	}
	for _, tt := range tests {
		t.Run(tt.subdir, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "clone")
			if err := sparseCloneSubdir(fileURL(remote), tt.subdir, dest, "", nil, nil); err != nil {
				t.Fatalf("sparseCloneSubdir() error = %v", err)
			}
			if err := sparseLinksFetched(dest, tt.subdir, nil); (err != nil) != tt.wantErr {
				t.Fatalf("sparseLinksFetched() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInstallFromGitSubdir_ExternalFileAndDirSymlinks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	remote := sparseSymlinkFixtureRemote(t)
	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      fileURL(remote) + "//skills/demo",
		CloneURL: fileURL(remote),
		Subdir:   "skills/demo",
		Name:     "demo",
	}
	sourceDir := t.TempDir()
	destPath := filepath.Join(sourceDir, "demo")
	if _, err := Install(source, destPath, InstallOptions{SourceDir: sourceDir}); err != nil {
		t.Fatalf("Install subdir with external symlinks failed: %v", err)
	}

	checks := map[string]string{
		"ref.md":        "shared file body",
		"refs/guide.md": "shared dir guide",
		"refs/deep.md":  "chained extra doc",
		"root.md":       "root doc",
	}
	for rel, want := range checks {
		got, err := os.ReadFile(filepath.Join(destPath, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("missing installed %s: %v", rel, err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", rel, string(got), want)
		}
	}
}

func TestDiscoverFromGitSubdir_ExternalFileAndDirSymlinks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	remote := sparseSymlinkFixtureRemote(t)
	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      fileURL(remote) + "//skills/demo",
		CloneURL: fileURL(remote),
		Subdir:   "skills/demo",
		Name:     "demo",
	}
	discovery, err := DiscoverFromGitSubdir(source)
	if err != nil {
		t.Fatalf("DiscoverFromGitSubdir failed: %v", err)
	}
	defer CleanupDiscovery(discovery)

	if len(discovery.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(discovery.Skills))
	}
	sourceDir := t.TempDir()
	destPath := filepath.Join(sourceDir, "demo")
	if _, err := InstallFromDiscovery(discovery, discovery.Skills[0], destPath, InstallOptions{SourceDir: sourceDir}); err != nil {
		t.Fatalf("InstallFromDiscovery failed: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(destPath, "refs", "deep.md"))
	if err != nil {
		t.Fatalf("expected refs/deep.md to be installed: %v", err)
	}
	if strings.TrimSpace(string(got)) != "chained extra doc" {
		t.Errorf("refs/deep.md = %q, want %q", string(got), "chained extra doc")
	}
}
