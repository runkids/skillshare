package sync

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverExtraFiles_SkipsAdministrativeRootLock(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{".skillshare-extras.lock", "rules.md"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := DiscoverExtraFiles(root)
	if err != nil || !reflect.DeepEqual(files, []string{"rules.md"}) {
		t.Fatalf("administrative lock leaked into sync/collect discovery: %v %v", files, err)
	}
}
