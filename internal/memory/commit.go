package memory

import "os"

// commitNote publishes a prepared draft after backup and temporary-file writes.
func commitNote(root, rel, draft, version string) error {
	path, err := notePath(root, rel)
	if err != nil {
		return err
	}
	if version == "" {
		// Another process may have created this note since the initial read.
		return createNote(path, draft)
	}
	latest, err := Read(root, rel)
	if os.IsNotExist(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if latest.Version != version {
		return ErrConflict
	}
	return os.Rename(draft, path)
}

func createNote(path, draft string) error {
	data, err := os.ReadFile(draft)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if os.IsExist(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			os.Remove(path)
		}
	}()
	if err := file.Chmod(0644); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	committed = true
	return nil
}

// removeNote commits a deletion after the backup has completed.
func removeNote(root, rel, version string) error {
	path, err := notePath(root, rel)
	if err != nil {
		return err
	}
	latest, err := Read(root, rel)
	if os.IsNotExist(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if version == "" || latest.Version != version {
		return ErrConflict
	}
	return os.Remove(path)
}
