package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateExtensionMode(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{"", "copy", false},
		{"copy", "copy", false},
		{"merge", "", true},
		{"symlink", "", true},
	} {
		got, err := validateExtensionMode(tc.raw)
		if tc.wantErr && err == nil {
			t.Errorf("mode %q: expected error", tc.raw)
		}
		if !tc.wantErr && (err != nil || got != tc.want) {
			t.Errorf("mode %q: got (%q, %v), want (%q, nil)", tc.raw, got, err, tc.want)
		}
	}
}

func TestResolveExtension_BareNameInDir(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, "gemini-commands")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"),
		[]byte("run: [\"cat\"]\noutput_ext: toml\n"), 0644); err != nil {
		t.Fatal(err)
	}
	spec, err := resolveExtension("gemini-commands", dir)
	if err != nil {
		t.Fatalf("resolveExtension: %v", err)
	}
	if spec == nil || spec.OutputExt != "toml" {
		t.Errorf("unexpected spec: %+v", spec)
	}
}

func TestResolveExtension_Empty(t *testing.T) {
	spec, err := resolveExtension("", t.TempDir())
	if err != nil || spec != nil {
		t.Errorf("empty extension should resolve to (nil, nil), got (%v, %v)", spec, err)
	}
}
