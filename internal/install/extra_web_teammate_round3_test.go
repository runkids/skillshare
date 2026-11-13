package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallRemoteExtra_TeammateDeletedSlashRefWithoutMetadata(t *testing.T) {
	for _, sparse := range []bool{false, true} {
		t.Run(fmt.Sprintf("sparse=%t", sparse), func(t *testing.T) {
			req := newRemoteExtraRequest(t)
			oldSparse := sparseCheckoutSupported
			sparseCheckoutSupported = func() bool { return sparse }
			t.Cleanup(func() { sparseCheckoutSupported = oldSparse })
			req.ProjectRoot = t.TempDir()
			req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
			req.ExtrasRoot = filepath.Join(req.LockDir, "extras")
			remote := req.RawSource
			runGit(t, strings.TrimPrefix(remote, "file://"), "branch", "-D", "feature")
			seed := filepath.Join(t.TempDir(), "seed")
			runGit(t, "", "clone", "--branch", "main", remote, seed)
			runGit(t, seed, "config", "user.name", "Fixture")
			runGit(t, seed, "config", "user.email", "fixture@example.invalid")
			for path, content := range map[string]string{"rules/rule.md": "pinned correct", "x/rules/rule.md": "WRONG competing directory"} {
				file := filepath.Join(seed, path)
				if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			runGit(t, seed, "add", ".")
			runGit(t, seed, "commit", "-m", "teammate selected directory fixture")
			runGit(t, seed, "push", "origin", "HEAD:main", "HEAD:feature/x")
			gitConfig := filepath.Join(t.TempDir(), "gitconfig")
			runGit(t, "", "config", "--file", gitConfig, "url."+remote+".insteadOf", "https://github.com/example-org/ai-hub.git")
			t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			t.Setenv("GIT_ALLOW_PROTOCOL", "file")
			req.RawSource = "https://github.com/example-org/ai-hub/tree/feature/x/rules"
			var err error
			req.Source, err = ParseSource(req.RawSource)
			if err != nil {
				t.Fatal(err)
			}
			req.Branch = "main"
			req.PinBranch = func() string { return "main" }
			first, err := InstallRemoteExtra(req)
			if err != nil {
				t.Fatal(err)
			}
			lock, err := LoadLock(req.LockDir)
			if err != nil || lock.Extras[req.Name].TreeHash == "" {
				t.Fatalf("existing selected tree hash fixture absent: %+v %v", lock, err)
			}
			runGit(t, seed, "push", "origin", ":feature/x")
			other := req
			other.ProjectRoot = t.TempDir()
			other.LockDir = filepath.Join(other.ProjectRoot, ".skillshare")
			other.ExtrasRoot = filepath.Join(other.LockDir, "extras")
			other.ConfigPath = filepath.Join(other.ProjectRoot, "config.yaml")
			if err := os.WriteFile(other.ConfigPath, []byte("before"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(other.LockDir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := lock.Save(other.LockDir); err != nil {
				t.Fatal(err)
			}
			other.Source, err = ParseSource(other.RawSource)
			if err != nil {
				t.Fatal(err)
			}
			other.Branch = ""
			other.WriteConfig = func() error { return nil }
			other.RevertConfig = func() error { return nil }
			result, err := InstallRemoteExtra(other)
			if err != nil || result.Commit != first.Commit || result.Branch != "main" {
				t.Fatalf("teammate restore: %+v %v", result, err)
			}
			if data, err := os.ReadFile(filepath.Join(other.ExtrasRoot, other.Name, "rule.md")); err != nil || string(data) != "pinned correct" {
				t.Errorf("teammate changed selected directory: %q %v", data, err)
			}
			store, _, err := ReadMetadata(other.ExtrasRoot)
			if err != nil || store.GetExact(other.Name) == nil || store.GetExact(other.Name).Subdir != "rules" {
				t.Errorf("teammate directory provenance: %+v %v", store, err)
			}
		})
	}
}
