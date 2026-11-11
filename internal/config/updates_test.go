package config

import (
	"os"
	"path/filepath"
	"testing"
)

// An opt-out must suppress even a cached update, and env overrides must not be persisted.
func TestUpdatePolicy(t *testing.T) {
	for _, tt := range []struct {
		name, yaml, env                 string
		configured, enabled, overridden bool
	}{
		{"default", "targets: {}\n", "", true, true, false},
		{"disabled", "updates:\n  auto_check: false\n", "", false, false, false},
		{"env disables", "updates:\n  auto_check: true\n", "false", true, false, true},
		{"env enables", "updates:\n  auto_check: false\n", "1", false, true, true},
		{"invalid env ignored", "updates:\n  auto_check: false\n", "invalid", false, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			t.Setenv("SKILLSHARE_CONFIG", path)
			t.Setenv("SKILLSHARE_AUTO_UPDATE_CHECK", tt.env)
			if err := os.WriteFile(path, []byte(tt.yaml), 0600); err != nil {
				t.Fatal(err)
			}
			p, err := ReadUpdatePolicy()
			if err != nil {
				t.Fatal(err)
			}
			if p.Configured != tt.configured || p.Enabled != tt.enabled || p.EnvOverride != tt.overridden {
				t.Fatalf("policy = %+v", p)
			}
		})
	}
}

func TestUpdatePolicyMissingAndInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", path)
	t.Setenv("SKILLSHARE_AUTO_UPDATE_CHECK", "")
	p, err := ReadUpdatePolicy()
	if err != nil || !p.Enabled {
		t.Fatalf("missing config: %+v, %v", p, err)
	}
	if err := os.WriteFile(path, []byte("updates: ["), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadUpdatePolicy(); err == nil {
		t.Fatal("invalid YAML must not enable background checks")
	}
}

func TestSetAutoUpdateCheckPreservesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", path)
	t.Setenv("SKILLSHARE_AUTO_UPDATE_CHECK", "true")
	if err := os.WriteFile(path, []byte("# my config\nmode: copy\ntargets: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := SetAutoUpdateCheck(false); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "copy" || cfg.Updates.AutoCheck == nil || *cfg.Updates.AutoCheck {
		t.Fatalf("saved config = %+v", cfg)
	}
	p, err := ReadUpdatePolicy()
	if err != nil || !p.Enabled || p.Configured {
		t.Fatalf("policy = %+v, %v", p, err)
	}
}
