package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateExtraRemote(t *testing.T) {
	cases := []struct {
		extra ExtraConfig
		valid bool
	}{
		{ExtraConfig{Name: "rules", Remote: "https://example.com/org/repo"}, true},
		{ExtraConfig{Name: "rules", Remote: "file:///tmp/fixture", Branch: "main"}, true},
		{ExtraConfig{Name: "rules", Remote: "git@example.com:org/repo"}, true},
		{ExtraConfig{Name: "rules", Remote: "https://user:secret@example.com/repo"}, false},
		{ExtraConfig{Name: "rules", Remote: "https://example.com/%zz?token=secret"}, false},
		{ExtraConfig{Name: "rules", Remote: "https://example.com/%zz#secret"}, false},
		{ExtraConfig{Name: "rules", Remote: "https:/example.com/repo?token=secret"}, false},
		{ExtraConfig{Name: "rules", Remote: "owner/repo#secret"}, false},
		{ExtraConfig{Name: "rules", Remote: "user:secret@example.com:org/repo"}, false},
		{ExtraConfig{Name: "rules", Remote: "/tmp/repo"}, false},
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
}

func TestValidateRemoteExtraEdits(t *testing.T) {
	before := []ExtraConfig{{Name: "rules", Remote: "https://example.com/repo", Branch: "main"}}
	for _, after := range [][]ExtraConfig{nil, {{Name: "renamed", Remote: "https://example.com/repo", Branch: "main"}}, {{Name: "rules", Remote: "https://example.com/other", Branch: "main"}}, {{Name: "rules", Remote: "https://example.com/repo", Source: "elsewhere", Branch: "main"}}, {{Name: "rules", Remote: "https://example.com/repo", Branch: "next"}}} {
		if err := ValidateRemoteExtraEdits(before, after); err == nil {
			t.Errorf("allowed edit: %+v", after)
		}
	}
	if err := ValidateRemoteExtraEdits(before, before); err != nil {
		t.Fatal(err)
	}
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
