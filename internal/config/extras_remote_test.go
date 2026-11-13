package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateExtraRemote(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		cases := []struct {
			extra ExtraConfig
			valid bool
		}{
			{ExtraConfig{Name: "rules", Remote: "https://example.com/org/repo"}, true},
			{ExtraConfig{Name: "rules", Remote: "https://example.com/org/repo.git/rules"}, true},
			{ExtraConfig{Name: "rules", Remote: "https://example.com/org/repo.git//outside"}, false},
			{ExtraConfig{Name: "rules", Remote: "https://example.com/org/repo.git/%2Foutside"}, false},
			{ExtraConfig{Name: "rules", Remote: "file:///tmp/fixture", Branch: "main"}, true},
			{ExtraConfig{Name: "rules", Remote: "git@example.com:org/repo"}, true},
			{ExtraConfig{Name: "rules", Remote: "https://user:secret@example.com/repo"}, false},
			{ExtraConfig{Name: "rules", Remote: "https://example.com/%zz?token=secret"}, false},
			{ExtraConfig{Name: "rules", Remote: "https://example.com/%zz#secret"}, false},
			{ExtraConfig{Name: "rules", Remote: "https:/example.com/repo?token=secret"}, false},
			{ExtraConfig{Name: "rules", Remote: "owner/repo#secret"}, false},
			{ExtraConfig{Name: "rules", Remote: "user:secret@example.com:org/repo"}, false},
			{ExtraConfig{Name: "rules", Remote: "/tmp/repo"}, false},
			{ExtraConfig{Name: "rules", Remote: " \t/tmp/repo "}, false},
			{ExtraConfig{Name: "rules", Remote: " ./repo "}, false},
			{ExtraConfig{Name: "rules", Remote: " ~/repo "}, false},
			{ExtraConfig{Name: "rules", Remote: " C:\\repo "}, false},
			{ExtraConfig{Name: "rules", Remote: " \t\n"}, false},
			{ExtraConfig{Name: "rules", Remote: "C:\\repo"}, false},
			{ExtraConfig{Name: "rules", Branch: "main"}, false},
			{ExtraConfig{Name: "rules", Remote: "https://example.com/repo", Source: "rules"}, false},
		}
		for _, tc := range cases {
			err := ValidateExtraRemote(tc.extra)
			if (err == nil) != tc.valid {
				t.Errorf("%+v: %v", tc.extra, err)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatalf("credential disclosed: %v", err)
			}
		}
	})
	t.Run("TestValidateExtraRemote_RequiresInstallableGitSource", func(t *testing.T) {
		for _, remote := range []string{
			"nonsense", "owner", "https://github.com/owner", "https://example.com",
			"https://github.com:443/owner", "https://GitHub.com:443/owner",
			"http://github.com:80/owner", "github.com:443/owner",
		} {
			t.Run(remote, func(t *testing.T) {
				entry := ExtraConfig{Name: "rules", Remote: remote}
				if err := ValidateExtraRemote(entry); err == nil {
					t.Errorf("unparseable origin admitted: %q", remote)
				}
				for name, after := range map[string][]ExtraConfig{
					"delete":  nil,
					"correct": {{Name: "rules", Remote: "example-org/ai-hub"}},
				} {
					if err := ValidateRemoteExtraEdits([]ExtraConfig{entry}, after); err != nil {
						t.Errorf("invalid baseline cannot %s: %v", name, err)
					}
				}
			})
		}
		for _, remote := range []string{
			"https://github.com:443/owner/repo", "https://GitHub.com:443/owner/repo",
			"http://github.com:80/owner/repo", "https://example.com:8443/repo",
		} {
			t.Run("valid/"+remote, func(t *testing.T) {
				if err := ValidateExtraRemote(ExtraConfig{Name: "rules", Remote: remote}); err != nil {
					t.Errorf("supported port-bearing origin refused: %q: %v", remote, err)
				}
			})
		}
	})
}

func TestValidateRemoteExtraEdits(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		before := []ExtraConfig{{Name: "rules", Remote: "https://example.com/repo", Branch: "main"}}
		for _, after := range [][]ExtraConfig{nil, {{Name: "renamed", Remote: "https://example.com/repo", Branch: "main"}}, {{Name: "rules", Remote: "https://example.com/other", Branch: "main"}}, {{Name: "rules", Remote: "https://example.com/repo", Source: "elsewhere", Branch: "main"}}, {{Name: "rules", Remote: "https://example.com/repo", Branch: "next"}}} {
			if err := ValidateRemoteExtraEdits(before, after); err == nil {
				t.Errorf("allowed edit: %+v", after)
			}
		}
		if err := ValidateRemoteExtraEdits(before, before); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("TestValidateRemoteExtraEdits_InvalidBaselineRepairable", func(t *testing.T) {
		for _, entry := range []ExtraConfig{{Name: "rules", Remote: "https://user:SYNTHETIC@example.com/repo"}, {Name: "rules", Remote: "./path"}, {Name: "rules", Branch: "main"}} {
			if err := ValidateRemoteExtraEdits([]ExtraConfig{entry}, nil); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestLoadProjectReadOnlyDoesNotMigrate(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".skillshare")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	original := []byte("extras:\n  - name: rules\n    remote: https://example.com/repo\n    branch: main\n    targets: []\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadProjectReadOnly(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Extras) != 1 || cfg.Extras[0].Remote != "https://example.com/repo" || cfg.Extras[0].Branch != "main" {
		t.Fatalf("lost remote: %+v", cfg.Extras)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("read-only load wrote the config")
	}
}

func TestSaveProjectRejectsRemoteCredentialsBeforeCreatingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config")
	cfg := ProjectConfig{Extras: []ExtraConfig{{Name: "rules", Remote: "https://user:secret@example.com/repo"}}}
	if err := cfg.SaveIn(dir); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("created directory: %v", err)
	}
}

func TestRawExtras_IgnoresOtherKeysAndTheirValidity(t *testing.T) {
	for _, other := range []string{"gitlab_hosts: [not-valid]\n", "projects_storage: {invalid: shape}\n"} {
		entries, err := RawExtras([]byte(other + "extras:\n  - name: rules\n    remote: example-org/ai-hub\n    targets: []\n"))
		if err != nil || len(entries) != 1 || entries[0].Remote != "example-org/ai-hub" {
			t.Fatal(entries, err)
		}
	}
}

func TestRawExtras_ErrorsCarryNoValue(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		for _, raw := range []string{"extras: SYNTHETIC\n", "extras: [SYNTHETIC]\n", "extras:\n  - remote: [SYNTHETIC]\n", "extras: *SYNTHETIC\n", "extras: &SYNTHETIC [*SYNTHETIC]\n", "extras: [SYNTHETIC\n", "SYNTHETIC: x\nSYNTHETIC: y\n"} {
			if _, err := RawExtras([]byte(raw)); err == nil || strings.Contains(err.Error(), "SYNTHETIC") {
				t.Fatalf("unsafe error: %v", err)
			}
		}
	})
	t.Run("TestRawExtras_SyntaxLocationWithoutParserValues", func(t *testing.T) {
		// yaml.v3 itself pins this input's numeric location in decode_test.go.
		_, err := RawExtras([]byte("a: 1\nb: 2\nSYNTHETIC 2\nd: 3\n"))
		if err == nil || !strings.Contains(err.Error(), "line 3") || strings.Contains(err.Error(), "SYNTHETIC") {
			t.Fatalf("safe syntax location missing: %v", err)
		}
		for _, raw := range []string{"extras: *SYNTHETIC\n", "extras: [SYNTHETIC\n", "extras: &SYNTHETIC [*SYNTHETIC]\n", "SYNTHETIC: x\nSYNTHETIC: y\n", "extras: SYNTHETIC\n"} {
			_, err := RawExtras([]byte(raw))
			if err == nil || strings.Contains(err.Error(), "SYNTHETIC") {
				t.Errorf("parser data leaked: %v", err)
			}
		}
		if _, err := RawExtras([]byte("extras: SYNTHETIC\n")); !errors.Is(err, ErrRawExtrasUnreadable) {
			t.Fatalf("type-error recovery classification changed: %v", err)
		}
	})
}

func TestRawExtrasRemoteError(t *testing.T) {
	for _, remote := range []string{"https://user:SYNTHETIC@example.com/repo", "https:/example.com/repo?token=SYNTHETIC", "user:SYNTHETIC@example.com:org/repo", "https://example.com/%zz#SYNTHETIC"} {
		err := RawExtrasRemoteError([]byte("extras:\n  - name: rules\n    remote: " + remote + "\n"))
		if err == nil || !strings.Contains(err.Error(), "rules") || strings.Contains(err.Error(), "SYNTHETIC") {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{"extras: [SYNTHETIC\n", "extras: *SYNTHETIC\n", "extras:\n  - name: rules\n    remote: ./path\n", "extras:\n  - name: rules\n    remote: ssh://git@example.com/repo\n", "extras:\n  - name: rules\n    branch: main\n"} {
		if err := RawExtrasRemoteError([]byte(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if err := RawExtrasRemoteError([]byte("extras: SYNTHETIC\n")); !errors.Is(err, ErrRawExtrasUnreadable) {
		t.Fatal(err)
	}
}
