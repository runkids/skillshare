package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newRefsRemote makes a repo with a "feature/x" branch and a "v1.0" tag.
func newRefsRemote(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "r") // matches the repo name in the test URLs
	runGit(t, "", "init", "-q", "-b", "main", dir)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-q", "-m", "init")
	runGit(t, dir, "branch", "feature/x")
	runGit(t, dir, "tag", "v1.0")
	return dir
}

func parseWithRemote(t *testing.T, raw, remote string) *Source {
	t.Helper()
	source, err := ParseSource(raw)
	if err != nil {
		t.Fatalf("ParseSource(%q): %v", raw, err)
	}
	source.CloneURL = "file://" + remote
	return source
}

func TestResolveWebRef(t *testing.T) {
	remote := newRefsRemote(t)

	tests := []struct {
		name       string
		raw        string
		branch     string // set after parsing, as --branch or a saved config would
		repoRoot   bool   // Subdir cleared, as for a grouped whole-repo clone
		wantBranch string
		wantSubdir string
		wantName   string
		wantErr    string
	}{
		{
			name:       "single-segment tag is kept",
			raw:        "github.com/o/r/tree/v1.0/skills/foo",
			wantBranch: "v1.0",
			wantSubdir: "skills/foo",
		},
		{
			name:       "branch containing a slash",
			raw:        "github.com/o/r/tree/feature/x/skills/foo",
			wantBranch: "feature/x",
			wantSubdir: "skills/foo",
		},
		{
			name:       "branch containing a slash with no subdir",
			raw:        "github.com/o/r/tree/feature/x",
			wantBranch: "feature/x",
			wantSubdir: "",
			wantName:   "r",
		},
		{
			name:       "saved slash branch that is the whole tail",
			raw:        "github.com/o/r/tree/feature/x",
			branch:     "feature/x",
			wantBranch: "feature/x",
			wantSubdir: "",
			wantName:   "r",
		},
		{
			name:       "unrelated --branch keeps the URL's real subdir",
			raw:        "github.com/o/r/tree/feature/x/skills/foo",
			branch:     "main",
			wantBranch: "main",
			wantSubdir: "skills/foo",
		},
		{
			name:       "repo-root copy resolves the ref but keeps its root",
			raw:        "github.com/o/r/tree/feature/x/skills/foo",
			repoRoot:   true,
			wantBranch: "feature/x",
			wantSubdir: "",
		},
		{
			name:       "gitlab branch containing a slash",
			raw:        "https://gitlab.com/o/r/-/tree/feature/x/skills/foo",
			wantBranch: "feature/x",
			wantSubdir: "skills/foo",
		},
		{
			name:       "saved branch moves the subdir without asking the remote",
			raw:        "github.com/o/r/tree/other/y/skills/foo",
			branch:     "other/y",
			wantBranch: "other/y",
			wantSubdir: "skills/foo",
		},
		{
			name:       "unrelated --branch wins",
			raw:        "github.com/o/r/tree/v1.0/skills/foo",
			branch:     "main",
			wantBranch: "main",
			wantSubdir: "skills/foo",
		},
		{
			name:    "unknown ref fails with a hint",
			raw:     "github.com/o/r/tree/v9.9/skills/foo",
			wantErr: "--branch",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := parseWithRemote(t, tt.raw, remote)
			if tt.branch != "" {
				source.Branch = tt.branch
			}
			if tt.repoRoot {
				source.Subdir = ""
			}
			err := resolveWebRef(source)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to mention %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveWebRef: %v", err)
			}
			if source.Branch != tt.wantBranch {
				t.Errorf("Branch = %q, want %q", source.Branch, tt.wantBranch)
			}
			if source.Subdir != tt.wantSubdir {
				t.Errorf("Subdir = %q, want %q", source.Subdir, tt.wantSubdir)
			}
			if tt.wantName != "" && source.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", source.Name, tt.wantName)
			}
		})
	}
}
