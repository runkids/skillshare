package install

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/utils"
)

type extraFileSnapshot struct {
	exists bool
	data   []byte
	mode   os.FileMode
}

func snapshotExtraFile(path string) (extraFileSnapshot, error) {
	var s extraFileSnapshot
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if utils.IsLinkMode(path, info.Mode()) || !info.Mode().IsRegular() {
		return s, fmt.Errorf("unsafe state file %s", path)
	}
	s.data, err = os.ReadFile(path)
	if err != nil {
		return s, err
	}
	s.exists, s.mode = true, info.Mode().Perm()
	return s, nil
}

// restoreExtraSnapshotIfUnchanged preserves a concurrent writer's contents.
func restoreExtraSnapshotIfUnchanged(path string, before, written extraFileSnapshot) (bool, error) {
	now, err := snapshotExtraFile(path)
	if err != nil {
		return false, err
	}
	if now.exists != written.exists || !bytes.Equal(now.data, written.data) {
		return false, nil
	}
	if !before.exists {
		if !now.exists {
			return true, nil
		}
		return true, os.Remove(path)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".extra-restore-*.tmp")
	if err != nil {
		return false, err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(before.data); err == nil {
		err = f.Chmod(before.mode)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return false, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return false, err
	}
	return true, nil
}
