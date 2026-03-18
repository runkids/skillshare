package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateConfig_SourceNotExist(t *testing.T) {
	cfg := &Config{
		Source:  "/nonexistent/source/path",
		Targets: map[string]TargetConfig{},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
	if !strings.Contains(err.Error(), "source path does not exist") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_SourceEmpty(t *testing.T) {
	cfg := &Config{
		Source:  "",
		Targets: map[string]TargetConfig{},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for empty source")
	}
	if !strings.Contains(err.Error(), "source path is empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_SourceIsFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(tmpFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		Source:  tmpFile,
		Targets: map[string]TargetConfig{},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for file source")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_InvalidGlobalMode(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Source:  tmpDir,
		Mode:    "invalid",
		Targets: map[string]TargetConfig{},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid global mode")
	}
	if !strings.Contains(err.Error(), "invalid global sync mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_InvalidTargetMode(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)
	cfg := &Config{
		Source: tmpDir,
		Targets: map[string]TargetConfig{
			"test": {Path: targetDir, Mode: "badmode"},
		},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid target mode")
	}
	if !strings.Contains(err.Error(), "invalid sync mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_TargetNotExist_MergeMode(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Source: tmpDir,
		Targets: map[string]TargetConfig{
			"test": {Path: filepath.Join(tmpDir, "nonexistent"), Mode: "merge"},
		},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for nonexistent target with merge mode")
	}
	if !strings.Contains(err.Error(), "path does not exist") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_TargetNotExist_CopyMode_Error(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Source: tmpDir,
		Targets: map[string]TargetConfig{
			"test": {Path: filepath.Join(tmpDir, "nonexistent"), Mode: "copy"},
		},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for nonexistent target with copy mode")
	}
	if !strings.Contains(err.Error(), "path does not exist") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_TargetParentNotExist_Error(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Source: tmpDir,
		Targets: map[string]TargetConfig{
			"test": {Path: filepath.Join(tmpDir, "no", "parent", "target"), Mode: "copy"},
		},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error when parent directory is missing")
	}
	if !strings.Contains(err.Error(), "path does not exist") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)
	cfg := &Config{
		Source: tmpDir,
		Mode:   "merge",
		Targets: map[string]TargetConfig{
			"test": {Path: targetDir, Mode: "merge"},
		},
	}
	warnings, err := ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestValidateConfig_EmptyMode_OK(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)
	cfg := &Config{
		Source: tmpDir,
		Mode:   "", // empty = default merge
		Targets: map[string]TargetConfig{
			"test": {Path: targetDir, Mode: ""}, // empty = inherit global
		},
	}
	warnings, err := ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestIsValidSyncMode(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{"", true},
		{"merge", true},
		{"symlink", true},
		{"copy", true},
		{"invalid", false},
		{"MERGE", false}, // case sensitive
	}
	for _, tt := range tests {
		if got := IsValidSyncMode(tt.mode); got != tt.want {
			t.Errorf("IsValidSyncMode(%q) = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestValidateProjectConfig_InvalidMode(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".skillshare", "skills"), 0755)
	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{
			{Name: "claude", Mode: "badmode"},
		},
	}
	_, err := ValidateProjectConfig(cfg, tmpDir)
	if err == nil {
		t.Fatal("expected error for invalid project target mode")
	}
	if !strings.Contains(err.Error(), "invalid sync mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateProjectConfig_MissingSource_Warning(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't create .skillshare/skills/
	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{
			{Name: "claude"},
		},
	}
	warnings, err := ValidateProjectConfig(cfg, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "source directory does not exist yet") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected source warning, got: %v", warnings)
	}
}
