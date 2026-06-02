package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSSHURL(t *testing.T) {
	cases := map[string]bool{
		"git@github.com:owner/repo.git":                 true,
		"git@ghe.corp.com:team/skills.git//hubs/h.json": true,
		"git@ghe.corp.com:team/skills":                  true,
		"https://github.com/owner/repo":                 false,
		"http://example.com/index.json":                 false,
		"/abs/path/index.json":                          false,
		"./rel/index.json":                              false,
		"index.json":                                    false,
		"owner/repo":                                    false,
	}
	for in, want := range cases {
		if got := IsSSHURL(in); got != want {
			t.Errorf("IsSSHURL(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsSSHURL_Scheme(t *testing.T) {
	for _, in := range []string{
		"ssh://git@host/org/repo.git",
		"ssh://git@host:2222/org/repo.git//hubs/h.json",
		"ssh://git@ghe.corp.com/team/skills",
	} {
		if !IsSSHURL(in) {
			t.Errorf("IsSSHURL(%q) = false, want true", in)
		}
	}
}

func TestParseSource_SSHScheme(t *testing.T) {
	cases := []struct {
		in, wantClone, wantSubdir string
	}{
		{"ssh://git@ghe.corp.com/team/skills.git", "ssh://git@ghe.corp.com/team/skills.git", ""},
		{"ssh://git@ghe.corp.com/team/skills.git//hubs/team.json", "ssh://git@ghe.corp.com/team/skills.git", "hubs/team.json"},
		{"ssh://git@ghe.corp.com:2222/team/skills.git", "ssh://git@ghe.corp.com:2222/team/skills.git", ""},
	}
	for _, c := range cases {
		src, err := ParseSource(c.in)
		if err != nil {
			t.Fatalf("ParseSource(%q): %v", c.in, err)
		}
		if src.Type != SourceTypeGitSSH {
			t.Errorf("%q type = %v, want SourceTypeGitSSH", c.in, src.Type)
		}
		if src.CloneURL != c.wantClone {
			t.Errorf("%q CloneURL = %q, want %q", c.in, src.CloneURL, c.wantClone)
		}
		if src.Subdir != c.wantSubdir {
			t.Errorf("%q Subdir = %q, want %q", c.in, src.Subdir, c.wantSubdir)
		}
	}
}

func TestShallowCloneToTemp(t *testing.T) {
	src := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(src, "skillshare-hub.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitAdd(t, src, ".")
	gitCommit(t, src, "init")

	dir, err := ShallowCloneToTemp(src, "")
	if err != nil {
		t.Fatalf("ShallowCloneToTemp: %v", err)
	}
	defer os.RemoveAll(dir)

	if _, err := os.Stat(filepath.Join(dir, "skillshare-hub.json")); err != nil {
		t.Fatalf("expected cloned file in temp dir: %v", err)
	}
}

func TestShallowCloneToTemp_BadURL(t *testing.T) {
	dir, err := ShallowCloneToTemp(t.TempDir()+"/does-not-exist", "")
	if err == nil {
		t.Fatalf("expected error cloning nonexistent repo, got dir %q", dir)
	}
	if dir != "" {
		t.Errorf("expected empty dir on error, got %q", dir)
	}
}
