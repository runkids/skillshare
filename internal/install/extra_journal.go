package install

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"

	"skillshare/internal/utils"
)

type extraJournal struct {
	Version     int               `json:"version"`
	Name        string            `json:"name"`
	Remote      string            `json:"remote"`
	Branch      string            `json:"branch,omitempty"`
	Commit      string            `json:"commit"`
	Subdir      *string           `json:"subdir,omitempty"`
	HadPrevious bool              `json:"had_previous"`
	Old         map[string]string `json:"old"`
}

func extraJournalPath(root, name string) string {
	return filepath.Join(root, extraStagingDir, "publishing-"+name+".json")
}

func validateExtraJournal(j extraJournal, name string) error {
	if err := validateSourceName(name); err != nil {
		return err
	}
	if j.Version != 1 || j.Name != name || j.Remote == "" || len(j.Commit) != 40 || !IsCommitSHA(j.Commit) || (j.HadPrevious && j.Old == nil) || (!j.HadPrevious && len(j.Old) != 0) {
		return fmt.Errorf("invalid publication marker for %q", name)
	}
	return validateExtraJournalSource(j, name)
}

func validateExtraJournalSource(j extraJournal, name string) error {
	if err := RejectSourceCredentials(j.Remote); err != nil {
		return fmt.Errorf("invalid publication marker for %q: %w", name, err)
	}
	if j.Subdir != nil {
		return validateRepoSubdir(*j.Subdir)
	}
	return nil
}

func readExtraJournal(root, name string) (extraJournal, error) {
	var j extraJournal
	if err := validateSourceName(name); err != nil {
		return j, err
	}
	marker := extraJournalPath(root, name)
	info, err := os.Lstat(marker)
	if err != nil {
		return j, err
	}
	if utils.IsLinkMode(marker, info.Mode()) || !info.Mode().IsRegular() {
		return j, fmt.Errorf("unsafe publication marker for %q", name)
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		return j, err
	}
	if err := json.Unmarshal(data, &j); err != nil {
		return j, fmt.Errorf("invalid publication marker for %q", name)
	}
	if err := validateExtraJournal(j, name); err != nil {
		return j, err
	}
	return j, nil
}

// writeExtraJournal writes and syncs the marker before publication changes.
func writeExtraJournal(root string, j extraJournal) error {
	if err := validateExtraJournal(j, j.Name); err != nil {
		return err
	}
	stage := filepath.Join(root, extraStagingDir)
	if err := os.MkdirAll(stage, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(stage)
	if err != nil {
		return err
	}
	if utils.IsLinkMode(stage, info.Mode()) || !info.IsDir() {
		return fmt.Errorf("unsafe staging directory %s", stage)
	}
	marker := extraJournalPath(root, j.Name)
	if _, err := os.Lstat(marker); err == nil {
		return fmt.Errorf("publication marker already exists for %q", j.Name)
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := json.Marshal(j)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(stage, ".marker-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(tmp, extraJournalPath(root, j.Name)); err != nil {
		return err
	}
	if d, openErr := os.Open(stage); openErr == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// fingerprintExtraTree records files, links and types without following links.
// Permissions are omitted so mode-only changes never count as edits.
func fingerprintExtraTree(root string) (map[string]string, error) {
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			if utils.IsLinkMode(path, info.Mode()) || !info.IsDir() {
				return fmt.Errorf("unsafe extra directory %s", path)
			}
			return nil
		}
		key := filepath.ToSlash(rel)
		switch {
		case utils.IsLinkMode(path, info.Mode()):
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256([]byte(target))
			result[key] = fmt.Sprintf("link:%x", sum)
		case info.IsDir():
			result[key] = "directory"
		case info.Mode().IsRegular():
			hash, err := utils.FileHashFormatted(path)
			if err != nil {
				return err
			}
			result[key] = hash
		default:
			result[key] = fmt.Sprintf("special:%x", uint32(info.Mode().Type()))
		}
		return nil
	})
	return result, err
}

func sameExtraFingerprint(a, b map[string]string) bool { return maps.Equal(a, b) }
