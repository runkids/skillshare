package trash

import (
	"path/filepath"
	"testing"
)

func TestTrashDir_RespectsXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/data")

	got := TrashDir()
	want := filepath.Join("/custom/data", "skillshare", "trash")
	if got != want {
		t.Errorf("TrashDir() = %q, want %q", got, want)
	}
}
