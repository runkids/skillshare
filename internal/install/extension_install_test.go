package install

import "testing"

func TestIsBuiltinExtension(t *testing.T) {
	for _, name := range BuiltinExtensions {
		if !IsBuiltinExtension(name) {
			t.Errorf("%q should be a built-in extension", name)
		}
	}
	for _, bad := range []string{"", "../evil", "md2codex/..", "unknown"} {
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
