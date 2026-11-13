package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusDoctorExtras_OrphanPublicationDiagnostics(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, ".skillshare-staging")
	if err := os.MkdirAll(stage, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "publishing-rules.json"), []byte(`{"remote":"SYNTHETIC"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{
		captureStdout(t, func() { printUndeclaredExtraPublications(nil, root) }),
		captureStdout(t, func() { checkExtras(nil, &doctorResult{}, false, "", root, "", "", func() error { return nil }) }),
		captureStdout(t, func() {
			checkExtras(nil, &doctorResult{}, true, "", "", t.TempDir(), root, func() error { return nil })
		}),
	} {
		if !strings.Contains(output, "rules") || !strings.Contains(output, "interrupted") || !strings.Contains(output, "original install") || strings.Contains(output, "SYNTHETIC") {
			t.Errorf("orphan diagnostic missing/unsafe: %s", output)
		}
	}
}
