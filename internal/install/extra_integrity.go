package install

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"

	"skillshare/internal/utils"
)

const extraStagingDir = ".skillshare-staging"

// ExtraIntegrity describes local state without contacting the remote.
type ExtraIntegrity struct {
	Name, Commit                   string
	Modified, Interrupted, Missing bool
	Err                            error
}

// State reports local integrity only; it makes no claim about remote HEAD.
func (s ExtraIntegrity) State() string {
	switch {
	case s.Interrupted:
		return "interrupted"
	case s.Err != nil:
		return "error"
	case s.Missing:
		return "missing"
	case s.Modified:
		return "modified"
	default:
		return "intact"
	}
}

// extraTreeHashes reads regular files without following links; modes are not edits.
func extraTreeHashes(dir string) (map[string]string, error) {
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if utils.IsLinkMode(dir, info.Mode()) || !info.IsDir() {
		return nil, fmt.Errorf("unsafe extra directory %s", dir)
	}
	hashes := make(map[string]string)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if utils.IsLinkMode(path, info.Mode()) {
			return fmt.Errorf("unsafe link %s", path)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsafe file %s", path)
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		hash, err := utils.FileHashFormatted(path)
		if err != nil {
			return err
		}
		hashes[filepath.ToSlash(rel)] = hash
		return nil
	})
	return hashes, err
}

// ExtraLocallyModified treats missing records, unsafe files, and hash failures as edits.
func ExtraLocallyModified(extrasRoot, name string) (bool, *MetadataEntry, error) {
	if err := validateSourceName(name); err != nil {
		return true, nil, err
	}
	store, _, err := ReadMetadata(extrasRoot)
	if err != nil {
		return true, nil, err
	}
	entry := store.GetExact(name)
	if entry == nil || len(entry.FileHashes) == 0 {
		return true, entry, nil
	}
	hashes, err := extraTreeHashes(filepath.Join(extrasRoot, name))
	if err != nil {
		return true, entry, nil
	}
	return !maps.Equal(entry.FileHashes, hashes), entry, nil
}

// InspectRemoteExtra checks only local state, and recognizes unfinished publication first.
func InspectRemoteExtra(extrasRoot, name string) (out ExtraIntegrity) {
	out = ExtraIntegrity{Name: name}
	if err := validateSourceName(name); err != nil {
		out.Err = err
		return out
	}
	// Interruption and missing content remain the primary state, but an
	// existing metadata pin can still identify the installed revision.
	defer func() {
		if out.Commit == "" {
			if store, _, err := ReadMetadata(extrasRoot); err == nil {
				if entry := store.GetExact(name); entry != nil {
					out.Commit = entry.Commit
				}
			}
		}
	}()
	stage := filepath.Join(extrasRoot, extraStagingDir)
	if info, err := os.Lstat(stage); err == nil {
		if utils.IsLinkMode(stage, info.Mode()) || !info.IsDir() {
			out.Interrupted = true
			out.Err = fmt.Errorf("unsafe staging directory %s", stage)
			return out
		}
		for _, file := range []string{"publishing-" + name + ".json", "previous-" + name, "incoming-" + name} {
			if _, err := os.Lstat(filepath.Join(stage, file)); err == nil {
				out.Interrupted = true
				return out
			} else if !os.IsNotExist(err) {
				out.Err = err
				return out
			}
		}
	} else if !os.IsNotExist(err) {
		out.Err = err
		return out
	}
	dir := filepath.Join(extrasRoot, name)
	if _, err := os.Lstat(dir); os.IsNotExist(err) {
		out.Missing = true
		return out
	} else if err != nil {
		out.Err = err
		return out
	}
	modified, entry, err := ExtraLocallyModified(extrasRoot, name)
	out.Modified, out.Err = modified, err
	if entry != nil {
		out.Commit = entry.Commit
	}
	return out
}
