package install

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"skillshare/internal/utils"
)

const extraStagingDir = ".skillshare-staging"

// InterruptedExtraNames lists only direct publication artifacts. Marker contents
// are untrusted and are never read to discover an undeclared extra's identity.
func InterruptedExtraNames(extrasRoot string) ([]string, error) {
	stage := filepath.Join(extrasRoot, extraStagingDir)
	info, err := os.Lstat(stage)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if utils.IsLinkMode(stage, info.Mode()) || !info.IsDir() {
		return nil, fmt.Errorf("unsafe staging directory %s", stage)
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, entry := range entries {
		file, name := entry.Name(), ""
		switch {
		case strings.HasPrefix(file, "publishing-") && strings.HasSuffix(file, ".json"):
			name = strings.TrimSuffix(strings.TrimPrefix(file, "publishing-"), ".json")
		case strings.HasPrefix(file, "incoming-"):
			name = strings.TrimPrefix(file, "incoming-")
		case strings.HasPrefix(file, "previous-"):
			name = strings.TrimPrefix(file, "previous-")
		}
		if ValidateExtraName(name) == nil {
			names[name] = true
		}
	}
	return slices.Sorted(maps.Keys(names)), nil
}

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
	return ExtraTreeModified(filepath.Join(extrasRoot, name), entry), entry, nil
}

// ExtraTreeModified compares against saved provenance even after its metadata
// entry is removed. Missing hashes and unreadable or unsafe trees are edits.
func ExtraTreeModified(dir string, entry *MetadataEntry) bool {
	if entry == nil || entry.FileHashes == nil {
		return true
	}
	hashes, err := extraTreeHashes(dir)
	return err != nil || !maps.Equal(entry.FileHashes, hashes)
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
