package install

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/textdiff"
)

// UpstreamDiff is what updating a skill from From to To would change.
type UpstreamDiff struct {
	From  string       `json:"from"` // installed commit
	To    string       `json:"to"`   // latest commit
	Files []FileChange `json:"files"`
}

// UpstreamChanges compares the version entry was installed at with the
// version at latest, the commit the caller resolved on the remote, and diffs
// each changed file from installed to latest. Neither side includes local
// edits. It fetches nothing when the two commits are the same.
func UpstreamChanges(entry *MetadataEntry, parseOpts ParseOptions, latest string) (*UpstreamDiff, error) {
	if entry == nil || entry.Tracked || entry.RepoURL == "" || installedCommit(entry) == "" {
		return nil, ErrNoInstallRecord
	}
	from := installedCommit(entry)
	diff := &UpstreamDiff{From: from, To: latest, Files: []FileChange{}}
	if from == latest || strings.HasPrefix(from, latest) || strings.HasPrefix(latest, from) {
		return diff, nil
	}

	oldTmp, err := installedVersion(entry, parseOpts)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(oldTmp)
	// latest is the branch tip, abbreviated by ls-remote: fetching the tip keeps
	// the clone shallow, where a short SHA would download the whole history.
	newTmp, err := fetchVersion(entry, parseOpts, "")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(newTmp)

	oldDir, newDir := filepath.Join(oldTmp, "skill"), filepath.Join(newTmp, "skill")
	oldHashes, err := ComputeFileHashes(oldDir)
	if err != nil {
		return nil, err
	}
	newHashes, err := ComputeFileHashes(newDir)
	if err != nil {
		return nil, err
	}
	for path, hash := range newHashes {
		if prev, ok := oldHashes[path]; !ok {
			diff.Files = append(diff.Files, FileChange{Path: path, Status: "added"})
		} else if prev != hash {
			diff.Files = append(diff.Files, FileChange{Path: path, Status: "modified"})
		}
	}
	for path := range oldHashes {
		if _, ok := newHashes[path]; !ok {
			diff.Files = append(diff.Files, FileChange{Path: path, Status: "deleted"})
		}
	}
	sort.Slice(diff.Files, func(i, j int) bool { return diff.Files[i].Path < diff.Files[j].Path })
	for i, c := range diff.Files {
		rel := filepath.FromSlash(c.Path)
		oldPath, newPath := filepath.Join(oldDir, rel), filepath.Join(newDir, rel)
		if c.Status == "added" {
			oldPath = "" // reads as empty
		}
		if c.Status == "deleted" {
			newPath = ""
		}
		diff.Files[i].Diff = textdiff.Files(oldPath, newPath)
	}
	return diff, nil
}
