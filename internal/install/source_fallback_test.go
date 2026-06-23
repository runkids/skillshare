package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneFallbackSourcesForNestedGitLabURL(t *testing.T) {
	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      "https://domain.com/dir1/dir2/dir3/dir4",
		CloneURL: "https://domain.com/dir1/dir2.git",
		Subdir:   "dir3/dir4",
		Name:     "dir4",
	}

	fallbacks := cloneFallbackSourcesForNestedGitLabURL(source)
	if len(fallbacks) != 2 {
		t.Fatalf("expected 2 fallback sources, got %d: %+v", len(fallbacks), fallbacks)
	}

	if fallbacks[0].CloneURL != "https://domain.com/dir1/dir2/dir3.git" {
		t.Errorf("fallback[0].CloneURL = %q", fallbacks[0].CloneURL)
	}
	if fallbacks[0].Subdir != "dir4" {
		t.Errorf("fallback[0].Subdir = %q", fallbacks[0].Subdir)
	}
	if fallbacks[0].Name != "dir4" {
		t.Errorf("fallback[0].Name = %q", fallbacks[0].Name)
	}

	if fallbacks[1].CloneURL != "https://domain.com/dir1/dir2/dir3/dir4.git" {
		t.Errorf("fallback[1].CloneURL = %q", fallbacks[1].CloneURL)
	}
	if fallbacks[1].Subdir != "" {
		t.Errorf("fallback[1].Subdir = %q", fallbacks[1].Subdir)
	}
	if fallbacks[1].Name != "dir4" {
		t.Errorf("fallback[1].Name = %q", fallbacks[1].Name)
	}
}

func TestCloneFallbackSourcesForNestedGitLabURL_SkipsExplicitGitLabHosts(t *testing.T) {
	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      "https://gitlab.com/group/subgroup/project",
		CloneURL: "https://gitlab.com/group/subgroup/project.git",
		Name:     "project",
	}

	if fallbacks := cloneFallbackSourcesForNestedGitLabURL(source); len(fallbacks) != 0 {
		t.Fatalf("expected no fallback sources, got %+v", fallbacks)
	}
}

func TestCloneRepoForSource_RetriesNestedGitLabURL(t *testing.T) {
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "git.log")
	gitPath := filepath.Join(binDir, "git")
	script := fmt.Sprintf(`#!/bin/bash
set -eu
args=("$@")
url="${args[$(( $# - 2 ))]}"
dest="${args[$(( $# - 1 ))]}"
printf '%%s\n' "$url" >> %q
if [ "$url" = "https://domain.com/dir1/dir2/dir3/dir4.git" ]; then
  mkdir -p "$dest/.git"
  printf '# skill\n' > "$dest/SKILL.md"
  exit 0
fi
echo "fatal: repository '$url' not found" >&2
exit 128
`, logPath)
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      "https://domain.com/dir1/dir2/dir3/dir4",
		CloneURL: "https://domain.com/dir1/dir2.git",
		Subdir:   "dir3/dir4",
		Name:     "dir4",
	}
	destPath := filepath.Join(t.TempDir(), "repo")

	if err := cloneRepoForSource(source, destPath, "", true, nil); err != nil {
		t.Fatalf("cloneRepoForSource() error = %v", err)
	}

	if source.CloneURL != "https://domain.com/dir1/dir2/dir3/dir4.git" {
		t.Fatalf("CloneURL = %q", source.CloneURL)
	}
	if source.Subdir != "" {
		t.Fatalf("Subdir = %q", source.Subdir)
	}

	rawLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(rawLog)), "\n")
	want := []string{
		"https://domain.com/dir1/dir2.git",
		"https://domain.com/dir1/dir2/dir3.git",
		"https://domain.com/dir1/dir2/dir3/dir4.git",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("clone attempts = %#v, want %#v", got, want)
	}
}

func TestShouldRetryNestedGitLabURL(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "repository not found",
			err:  errString("repository 'https://domain.com/dir1/dir2.git' not found"),
			want: true,
		},
		{
			name: "not a valid git repo",
			err:  errString("https://domain.com/dir1/dir2 is not a valid Git repo"),
			want: true,
		},
		{
			name: "https 404",
			err:  errString("unable to access 'https://domain.com/dir1/dir2.git/': The requested URL returned error: 404"),
			want: true,
		},
		{
			name: "auth failure",
			err:  errString("authentication required — could not read Username"),
			want: false,
		},
		{
			name: "ssl failure",
			err:  errString("SSL certificate verification failed"),
			want: false,
		},
		{
			name: "missing branch",
			err:  errString("Remote branch feature/foo not found in upstream origin"),
			want: false,
		},
		{
			name: "network failure",
			err:  errString("Could not resolve host: domain.com"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRetryNestedGitLabURL(tt.err); got != tt.want {
				t.Fatalf("shouldRetryNestedGitLabURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneRepoForSource_DoesNotRetryAuthFailure(t *testing.T) {
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "git.log")
	gitPath := filepath.Join(binDir, "git")
	script := fmt.Sprintf(`#!/bin/bash
set -eu
args=("$@")
url="${args[$(( $# - 2 ))]}"
printf '%%s\n' "$url" >> %q
echo "fatal: could not read Username for '$url': terminal prompts disabled" >&2
exit 128
`, logPath)
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      "https://domain.com/dir1/dir2/dir3/dir4",
		CloneURL: "https://domain.com/dir1/dir2.git",
		Subdir:   "dir3/dir4",
		Name:     "dir4",
	}

	err := cloneRepoForSource(source, filepath.Join(t.TempDir(), "repo"), "", true, nil)
	if err == nil {
		t.Fatal("expected clone error")
	}
	if source.CloneURL != "https://domain.com/dir1/dir2.git" {
		t.Fatalf("CloneURL mutated to %q", source.CloneURL)
	}

	rawLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(rawLog)), "\n")
	if len(got) != 1 || got[0] != "https://domain.com/dir1/dir2.git" {
		t.Fatalf("clone attempts = %#v", got)
	}
}

func TestCloneRepoForSource_ReportsLastNestedFallbackError(t *testing.T) {
	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")
	script := `#!/bin/bash
set -eu
args=("$@")
url="${args[$(( $# - 2 ))]}"
echo "fatal: repository '$url' not found" >&2
exit 128
`
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      "https://domain.com/dir1/dir2/dir3/dir4",
		CloneURL: "https://domain.com/dir1/dir2.git",
		Subdir:   "dir3/dir4",
		Name:     "dir4",
	}

	err := cloneRepoForSource(source, filepath.Join(t.TempDir(), "repo"), "", true, nil)
	if err == nil {
		t.Fatal("expected clone error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "nested GitLab fallback also failed") {
		t.Fatalf("error %q does not mention nested fallback failure", msg)
	}
	if !strings.Contains(msg, "https://domain.com/dir1/dir2/dir3/dir4.git") {
		t.Fatalf("error %q does not include final fallback URL", msg)
	}
}
