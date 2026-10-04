package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
)

// PullResolution applies whole-file choices only to the revisions the user reviewed.
type PullResolution struct {
	LocalHash  string            `json:"localHash"`
	RemoteHash string            `json:"remoteHash"`
	Choices    map[string]string `json:"choices"`
}

type ConflictVersion struct {
	Deleted bool   `json:"deleted"`
	Content string `json:"content"`
	// Binary and large files can still be chosen, but have no text preview.
	NoPreview bool `json:"noPreview"`
}

type ConflictFile struct {
	Path   string          `json:"path"`
	Local  ConflictVersion `json:"local"`
	Remote ConflictVersion `json:"remote"`
}

// PullConflictError holds previews captured before the failed merge is aborted.
type PullConflictError struct {
	LocalHash  string         `json:"localHash"`
	RemoteHash string         `json:"remoteHash"`
	Files      []ConflictFile `json:"files"`
	RepoPath   string         `json:"-"`
}

func (e *PullConflictError) Error() string {
	paths := make([]string, 0, len(e.Files))
	for _, file := range e.Files {
		paths = append(paths, file.Path)
	}
	return fmt.Sprintf("pull stopped: this machine and the remote both changed %s; the merge was undone, resolve it with git in %s", strings.Join(paths, ", "), e.RepoPath)
}

// PullWithResolution retries a pull with the user's reviewed choices, preserving both histories.
func PullWithResolution(repoPath string, resolution *PullResolution, follows ...*sourcewalk.FollowSet) (*UpdateInfo, error) {
	return pullWithResolution(repoPath, AuthEnvForRepo(repoPath), nil, resolution, follows...)
}

func conflictVersion(dir, path string, stage int) ConflictVersion {
	ref := fmt.Sprintf(":%d:%s", stage, path)
	size := exec.Command("git", "cat-file", "-s", ref)
	size.Dir = dir
	out, err := size.Output()
	if err != nil {
		return ConflictVersion{Deleted: true}
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || n > 16*1024 {
		return ConflictVersion{NoPreview: true}
	}
	show := exec.Command("git", "show", ref)
	show.Dir = dir
	out, err = show.Output()
	if err != nil || bytes.IndexByte(out, 0) >= 0 || !utf8.Valid(out) {
		return ConflictVersion{NoPreview: true}
	}
	return ConflictVersion{Content: string(out)}
}

func resolvePullConflicts(dir string, paths []string, resolution *PullResolution) error {
	metadata := make([]string, 0)
	conflict := &PullConflictError{RepoPath: dir}
	conflict.LocalHash, _ = GetCurrentFullHash(dir)
	remote := exec.Command("git", "rev-parse", "MERGE_HEAD")
	remote.Dir = dir
	out, _ := remote.Output()
	conflict.RemoteHash = strings.TrimSpace(string(out))
	for _, path := range paths {
		if filepath.Base(path) == install.MetadataFileName {
			metadata = append(metadata, path)
			continue
		}
		conflict.Files = append(conflict.Files, ConflictFile{
			Path: path, Local: conflictVersion(dir, path, 2), Remote: conflictVersion(dir, path, 3),
		})
	}
	if len(conflict.Files) > 0 {
		if resolution == nil || resolution.LocalHash != conflict.LocalHash || resolution.RemoteHash != conflict.RemoteHash || len(resolution.Choices) != len(conflict.Files) {
			return conflict
		}
		// Validate every choice before touching the index or working tree.
		for _, file := range conflict.Files {
			if side := resolution.Choices[file.Path]; side != "local" && side != "remote" {
				return conflict
			}
		}
		for _, file := range conflict.Files {
			version, side := file.Local, "--ours"
			if resolution.Choices[file.Path] == "remote" {
				version, side = file.Remote, "--theirs"
			}
			args := []string{"checkout", side, "--", file.Path}
			if version.Deleted {
				args = []string{"rm", "--", file.Path}
			}
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("resolve %s: %w: %s", file.Path, err, out)
			}
			if !version.Deleted {
				add := exec.Command("git", "add", "--", file.Path)
				add.Dir = dir
				if err := add.Run(); err != nil {
					return err
				}
			}
		}
	}
	// This also commits the merge, including any whole-file choices above.
	return resolveMetadataConflicts(dir, metadata)
}
