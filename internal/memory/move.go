package memory

import (
	"fmt"
	"os"
	"path/filepath"

	syncpkg "skillshare/internal/sync"
)

// Move preserves content and permissions, and never replaces an existing destination.
// Markdown links and the source path's backup history are left unchanged.
func Move(root, rel, newRel, version string) (Note, error) {
	writeMu.Lock()
	defer writeMu.Unlock()
	source, err := notePath(root, rel)
	if err != nil {
		return Note{}, err
	}
	destination, err := notePath(root, newRel)
	if err != nil {
		return Note{}, err
	}
	current, err := Read(root, rel)
	if err != nil {
		return Note{}, err
	}
	if version == "" || current.Version != version {
		return Note{}, ErrConflict
	}
	if source == destination {
		return current, nil
	}
	if _, err := os.Lstat(destination); err == nil {
		return Note{}, fmt.Errorf("destination already exists: %w", os.ErrExist)
	} else if !os.IsNotExist(err) {
		return Note{}, err
	}
	info, err := os.Stat(source)
	if err != nil {
		return Note{}, err
	}
	if err := syncpkg.BackupFile(source, syncpkg.BackupReasonEdit); err != nil {
		return Note{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return Note{}, err
	}
	// Exclusive creation also rejects a destination created after validation.
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return Note{}, err
	}
	committed := false
	defer func() {
		if !committed {
			os.Remove(destination)
		}
	}()
	if err := file.Chmod(info.Mode().Perm()); err != nil {
		file.Close()
		return Note{}, err
	}
	if _, err := file.WriteString(current.Content); err != nil {
		file.Close()
		return Note{}, err
	}
	if err := file.Close(); err != nil {
		return Note{}, err
	}
	// Keep the source if an external editor changed it during the copy.
	latest, err := Read(root, rel)
	if err != nil {
		return Note{}, err
	}
	if latest.Version != version {
		return Note{}, ErrConflict
	}
	if err := os.Remove(source); err != nil {
		return Note{}, err
	}
	committed = true
	return Read(root, newRel)
}
