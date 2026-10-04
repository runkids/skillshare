package config

import (
	"os"
	"path/filepath"
	"testing"
)

// A registry.yaml in the source root that is a dangling link must not be
// written through: that would create its target outside the source.
func TestMigrateRegistryToSource_RefusesLinkedRegistry(t *testing.T) {
	configDir, sourceRoot := t.TempDir(), t.TempDir()
	external := filepath.Join(t.TempDir(), "registry.yaml")
	if err := os.WriteFile(filepath.Join(configDir, "registry.yaml"), []byte("skills: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(sourceRoot, "registry.yaml")); err != nil {
		t.Fatal(err)
	}

	MigrateRegistryToSource(configDir, sourceRoot)

	if _, err := os.Lstat(external); !os.IsNotExist(err) {
		t.Fatalf("the link target was created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "registry.yaml")); err != nil {
		t.Fatalf("the old registry was removed after a refused write: %v", err)
	}
}
