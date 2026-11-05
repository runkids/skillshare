package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSkillBundledReferenceEscape(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# Fixture\n[required](../outside.md)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	scan, err := ScanSkillWithRules(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(scan.Findings, "reference-escapes-root", SeverityHigh) {
		t.Fatalf("missing HIGH escape finding: %+v", scan.Findings)
	}
}
