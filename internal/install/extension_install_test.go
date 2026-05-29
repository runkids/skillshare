package install

import "testing"

func TestIsBuiltinExtension(t *testing.T) {
	for _, b := range BuiltinExtensions {
		if !IsBuiltinExtension(b.Name) {
			t.Errorf("%q should be a built-in extension", b.Name)
		}
		if b.Description == "" {
			t.Errorf("built-in %q must have a catalog description", b.Name)
		}
	}
	for _, bad := range []string{"", "../evil", "codex-agents/..", "unknown"} {
		if IsBuiltinExtension(bad) {
			t.Errorf("%q must not be reported as built-in", bad)
		}
	}
}

// InstallBuiltinExtension must reject unknown/traversal names before any
// network access, so the whitelist guards the download path.
func TestInstallBuiltinExtension_RejectsUnknown(t *testing.T) {
	if err := InstallBuiltinExtension("../../etc", t.TempDir()); err == nil {
		t.Fatal("expected error for unknown/traversal extension name")
	}
}
