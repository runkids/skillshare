package install

import (
	"os"
	"os/exec"
	"path/filepath"
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
