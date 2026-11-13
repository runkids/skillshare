package install

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestInterruptedExtraNames_OnlyDirectSafeArtifacts(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		root := t.TempDir()
		stage := filepath.Join(root, extraStagingDir)
		if err := os.MkdirAll(stage, 0755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"incoming-rules", "previous-notes"} {
			if err := os.Mkdir(filepath.Join(stage, name), 0700); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range []string{"publishing-rules.json", "publishing-notes.json"} {
			if err := os.WriteFile(filepath.Join(stage, name), []byte("untrusted SYNTHETIC marker"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range []string{"publishing-.json", "publishing-skills.json", "publishing-bad.name.json", "incoming-..", "previous-extras", "unrelated"} {
			if err := os.MkdirAll(filepath.Join(stage, name), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(stage, name, "publishing-hidden.json"), []byte("{}"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		names, err := InterruptedExtraNames(root)
		if err != nil || !reflect.DeepEqual(names, []string{"notes", "rules"}) {
			t.Fatalf("unsafe or duplicate names discovered: %v %v", names, err)
		}
	})
	t.Run("TestInterruptedExtraNames_DoesNotFollowStagingLink", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink privileges vary")
		}
		root, outside := t.TempDir(), t.TempDir()
		if err := os.WriteFile(filepath.Join(outside, "publishing-outside.json"), []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, extraStagingDir)); err != nil {
			t.Fatal(err)
		}
		if names, err := InterruptedExtraNames(root); err == nil || len(names) != 0 {
			t.Fatalf("followed unsafe staging: %v %v", names, err)
		}
	})
}
