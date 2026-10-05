package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ompWritableFixture(t *testing.T) (*Service, string, string) {
	t.Helper()
	s, home, agent := ompFixture(t)
	pkg := filepath.Join(home, "npm/omp")
	cli := ompFile(t, pkg, "src/cli.ts", "throw new Error('must never run');")
	if err := os.Chmod(cli, 0755); err != nil {
		t.Fatal(err)
	}
	ompFile(t, pkg, "package.json", `{"name":"@oh-my-pi/pi-coding-agent","version":"18.6.1","bin":{"omp":"src/cli.ts"}}`)
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cli, filepath.Join(bin, "omp")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ompFile(t, agent, "extensions/safe.ts", "throw new Error('must never run');")
	return s, home, agent
}

func TestOMPExtensionsPreviewApplyPreservesYAML(t *testing.T) {
	s, _, agent := ompWritableFixture(t)
	original := "# user comment\nunknown:\n  nested: [one, 2, true] # keep\ndisabledExtensions:\n  - foreign:id # foreign\n  - extension-module:gone\n"
	file := ompFile(t, agent, "config.yml", original)
	v, err := s.OMPExtensions(context.Background(), "omp")
	if err != nil {
		t.Fatal(err)
	}
	if v.ReadOnly || !v.Rows[0].Selectable || v.Revision == "" {
		t.Fatalf("not selectable: %+v", v)
	}
	changes := []OMPExtensionChange{{Key: v.Rows[0].Key, Enabled: false}}
	plan, err := s.PreviewOMPExtensions(context.Background(), "omp", changes, v.Revision)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(file)
	if string(before) != original {
		t.Fatal("preview wrote")
	}
	applied, err := s.ApplyOMPExtensions(context.Background(), "omp", changes, plan.Revision)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(file)
	for _, keep := range []string{"# user comment", "nested: [one, 2, true] # keep", "foreign:id # foreign", "extension-module:gone", "extension-module:safe"} {
		if !strings.Contains(string(after), keep) {
			t.Fatalf("lost %s: %s", keep, after)
		}
	}
	if applied.BackupID == "" {
		t.Fatal("missing backup")
	}
	backup, err := os.ReadFile(filepath.Join(s.StateDir, "omp-extensions", "backups", applied.BackupID+".json"))
	if err != nil || !strings.Contains(string(backup), "foreign:id") || strings.Contains(string(backup), "nested") || strings.Contains(string(backup), "user comment") {
		t.Fatalf("selection-only backup: %v %s", err, backup)
	}
	if _, err = s.ApplyOMPExtensions(context.Background(), "omp", changes, plan.Revision); !errors.Is(err, ErrOMPExtensionsStale) {
		t.Fatalf("stale: %v", err)
	}
}

func TestOMPExtensionsSelectionSafety(t *testing.T) {
	for _, tc := range []struct{ name, config string }{
		{"malformed", "disabledExtensions: [broken"},
		{"wrong-type", "disabledExtensions: [123]"},
		{"duplicate", "disabledExtensions: []\ndisabledExtensions: []\n"},
		{"alias", "x: &a []\ndisabledExtensions: *a\n"},
		{"multi-document", "disabledExtensions: []\n---\n{}\n"},
		{"indented-root", "  disabledExtensions: []\n  unknown: true\n"},
		{"flow-root", "{disabledExtensions: []}\n"},
		{"tagged-list", "disabledExtensions: !custom []\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, agent := ompWritableFixture(t)
			ompFile(t, agent, "config.yml", tc.config)
			v, _ := s.OMPExtensions(context.Background(), "omp")
			if !v.ReadOnly || v.Rows[0].Selectable {
				t.Fatalf("unsafe view: %+v", v)
			}
		})
	}
}

func TestOMPExtensionsCollisionsAndExplicitBypassAreReadOnly(t *testing.T) {
	s, home, agent := ompWritableFixture(t)
	same := ompFile(t, agent, "extensions/safe.js", "x")
	explicit := ompFile(t, home, "explicit.ts", "x")
	ompFile(t, agent, "config.yml", "extensions: ["+explicit+"]\n")
	v, _ := s.OMPExtensions(context.Background(), "omp")
	for _, r := range v.Rows {
		if r.Path == same || r.Name == "safe" || r.Path == explicit {
			if r.Selectable || r.ReadOnlyReason == "" {
				t.Fatalf("fake switch: %+v", r)
			}
		}
	}
}

func TestOMPExtensionsApplyRefusesNativeLock(t *testing.T) {
	s, _, _ := ompWritableFixture(t)
	v, _ := s.OMPExtensions(context.Background(), "omp")
	changes := []OMPExtensionChange{{Key: v.Rows[0].Key, Enabled: false}}
	plan, err := s.PreviewOMPExtensions(context.Background(), "omp", changes, v.Revision)
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := ompAcquireLock(v.SettingsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err = s.ApplyOMPExtensions(context.Background(), "omp", changes, plan.Revision); !errors.Is(err, ErrOMPExtensionsBusy) {
		t.Fatalf("lock: %v", err)
	}
}
