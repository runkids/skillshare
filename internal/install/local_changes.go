package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"skillshare/internal/sourcewalk"
	"skillshare/internal/textdiff"
)

// FileChange is a file of an installed skill that differs from the version
// the skill was installed at.
type FileChange struct {
	Path   string `json:"path"`
	Status string `json:"status"` // "modified", "added" or "deleted"
	Diff   string `json:"diff"`   // textdiff.Files; empty for archives without historical contents
}

// ErrNoInstallRecord means the skill has no remote source with recorded file
// hashes, so there is nothing to compare it with.
var ErrNoInstallRecord = errors.New("no install record to compare with")

// LocalChanges lists the files of the skill at dir whose hashes differ from
// the ones entry recorded. Git sources include text diffs at the recorded
// commit, fetched only when a recorded file changed; archive diffs stay empty.
func LocalChanges(entry *MetadataEntry, parseOpts ParseOptions, dir string, follow ...*sourcewalk.Follow) ([]FileChange, error) {
	if !entry.hasRemoteFileHashes() {
		return nil, ErrNoInstallRecord
	}
	local, err := ComputeFileHashes(dir, follow...)
	if err != nil {
		return nil, err
	}
	var changes []FileChange
	for path, hash := range local {
		if recorded, ok := entry.FileHashes[path]; !ok {
			changes = append(changes, FileChange{Path: path, Status: "added"})
		} else if recorded != hash {
			changes = append(changes, FileChange{Path: path, Status: "modified"})
		}
	}
	for path := range entry.FileHashes {
		if _, ok := local[path]; !ok {
			changes = append(changes, FileChange{Path: path, Status: "deleted"})
		}
	}
	if len(changes) == 0 {
		return nil, nil
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	// Archives and edited sources without an installed commit have file hashes only.
	commit := installedCommit(entry)
	needsInstalled := slices.ContainsFunc(changes, func(c FileChange) bool { return c.Status != "added" })
	if entry.IsArchive() || needsInstalled && commit == "" {
		return changes, nil
	}

	// An added file diffs against nothing, so the installed version is only
	// fetched for files it had.
	installed := ""
	if needsInstalled {
		// Upstream HEAD would show its own changes as the user's.
		source, err := RecordedSource(entry, parseOpts)
		if err != nil {
			return nil, fmt.Errorf("invalid source: %w", err)
		}
		source.ApplyRecordedBranch(entry.Branch)
		source.Commit = commit
		tmp, err := os.MkdirTemp("", "skillshare-changes-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(tmp)
		installed = filepath.Join(tmp, "skill")
		// Only read, never used, so the audit gate does not apply.
		if _, err := Install(source, installed, InstallOptions{SkipAudit: true}); err != nil {
			return nil, fmt.Errorf("fetch the installed version: %w", err)
		}
	}
	if len(follow) > 0 {
		if resolved, ok := follow[0].Resolve(dir); ok {
			dir = resolved
		}
	}
	for i, c := range changes {
		rel := filepath.FromSlash(c.Path)
		old := "" // reads as empty
		if c.Status != "added" {
			old = filepath.Join(installed, rel)
		}
		changes[i].Diff = textdiff.Files(old, filepath.Join(dir, rel))
	}
	return changes, nil
}

// installedCommit is the commit the skill was installed at: the full SHA when
// it is the same commit as the short Version, since fetching a short SHA falls
// back to downloading the whole history. "" when none was recorded.
func installedCommit(entry *MetadataEntry) string {
	if !IsCommitSHA(entry.Version) {
		return ""
	}
	if IsCommitSHA(entry.Commit) && strings.HasPrefix(entry.Commit, entry.Version) {
		return entry.Commit
	}
	return entry.Version
}
