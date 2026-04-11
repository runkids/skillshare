package rules

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRuleMetadata_RoundTrip(t *testing.T) {
	rulePath := filepath.Join(t.TempDir(), "claude", "manual.md")
	if err := os.MkdirAll(filepath.Dir(rulePath), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	want := ruleMetadata{
		Targets:    []string{"claude-work", "claude-personal"},
		SourceType: "local",
		Disabled:   true,
	}
	if err := saveRuleMetadata(rulePath, want); err != nil {
		t.Fatalf("saveRuleMetadata() error = %v", err)
	}

	got, err := loadRuleMetadata(rulePath)
	if err != nil {
		t.Fatalf("loadRuleMetadata() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadRuleMetadata() = %#v, want %#v", got, want)
	}
}

func TestRuleMetadata_MissingSidecarReturnsDefaults(t *testing.T) {
	rulePath := filepath.Join(t.TempDir(), "claude", "manual.md")
	got, err := loadRuleMetadata(rulePath)
	if err != nil {
		t.Fatalf("loadRuleMetadata() error = %v", err)
	}
	if !reflect.DeepEqual(got, ruleMetadata{}) {
		t.Fatalf("loadRuleMetadata() = %#v, want zero metadata", got)
	}
}

func TestRuleMetadata_ZeroValueRemovesSidecar(t *testing.T) {
	rulePath := filepath.Join(t.TempDir(), "claude", "manual.md")
	if err := os.MkdirAll(filepath.Dir(rulePath), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if err := saveRuleMetadata(rulePath, ruleMetadata{Targets: []string{"claude-work"}}); err != nil {
		t.Fatalf("saveRuleMetadata(non-zero) error = %v", err)
	}
	if err := saveRuleMetadata(rulePath, ruleMetadata{}); err != nil {
		t.Fatalf("saveRuleMetadata(zero) error = %v", err)
	}

	if _, err := os.Stat(ruleMetadataPath(rulePath)); !os.IsNotExist(err) {
		t.Fatalf("metadata sidecar stat error = %v, want not-exist", err)
	}
}

func TestIsRuleMetadataFile(t *testing.T) {
	if !isRuleMetadataFile(".manual.md.metadata.yaml") {
		t.Fatal("expected metadata file name to be recognized")
	}
	if isRuleMetadataFile("manual.md") {
		t.Fatal("expected plain markdown file to not be recognized as metadata")
	}
	if isRuleMetadataFile(".rule-tmp-123") {
		t.Fatal("expected transient temp file to not be recognized as metadata")
	}
}
