package install

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"skillshare/internal/gitenv"
	"skillshare/internal/sourcefs"
)

// MergeConflictError lists the files an update --merge could not combine:
// changed on both sides in the same place, deleted on one side and changed on
// the other, or binary and changed on both. Nothing was written.
type MergeConflictError struct {
	Files []string
}

func (e *MergeConflictError) Error() string {
	return "conflicting changes in " + strings.Join(e.Files, ", ")
}

// errUnverifiableBase means the installed version could not be rebuilt
// exactly, so a merge against it would mistake upstream changes for local ones.
var errUnverifiableBase = errors.New("cannot rebuild the installed version to merge against")

// mergeLocalEdits 3-way merges the local edits in dest onto the new version
// staged at staged, with the version entry records as the base, rewriting
// staged in place. It reports false when upstream changed none of the files,
// leaving staged alone: there is nothing to merge. On a conflict it returns
// *MergeConflictError and the caller must not publish staged.
func mergeLocalEdits(entry *MetadataEntry, dest, staged string, opts InstallOptions) (bool, error) {
	tmp, err := installedVersion(entry, opts.ParseOptions)
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tmp)
	base := filepath.Join(tmp, "skill")

	baseHashes, err := ComputeFileHashes(base)
	if err != nil {
		return false, err
	}
	if !maps.Equal(baseHashes, entry.FileHashes) {
		return false, errUnverifiableBase
	}
	theirs, err := ComputeFileHashes(staged)
	if err != nil {
		return false, err
	}
	if maps.Equal(theirs, baseHashes) {
		return false, nil
	}
	if resolved, ok := opts.SourceFollow.Resolve(dest); ok {
		dest = resolved
	}
	ours, err := ComputeFileHashes(dest)
	if err != nil {
		return false, err
	}

	out, err := sourcefs.Open(staged)
	if err != nil {
		return false, err
	}
	defer out.Close()

	all := maps.Clone(baseHashes)
	maps.Copy(all, ours)
	maps.Copy(all, theirs)

	var conflicts []string
	var writes []func() error
	for _, p := range slices.Sorted(maps.Keys(all)) {
		b, o, t := baseHashes[p], ours[p], theirs[p]
		rel := filepath.FromSlash(p)
		switch {
		case o == t, o == b:
			// Same on both sides, or only upstream changed: staged has it.
		case t == b:
			// Only the local copy changed: carry it over.
			if o == "" {
				writes = append(writes, func() error { return out.Remove(rel) })
				continue
			}
			writes = append(writes, func() error { return copyIntoStage(out, filepath.Join(dest, rel), rel) })
		case o == "" || t == "" || b == "":
			conflicts = append(conflicts, p)
		default:
			merged, conflict, err := mergeFiles(filepath.Join(base, rel), filepath.Join(dest, rel), filepath.Join(staged, rel))
			if err != nil {
				return false, fmt.Errorf("merge %s: %w", p, err)
			}
			if conflict {
				conflicts = append(conflicts, p)
				continue
			}
			mode := fileMode(filepath.Join(dest, rel))
			writes = append(writes, func() error { return out.WriteFile(rel, merged, mode) })
		}
	}
	if len(conflicts) > 0 {
		return false, &MergeConflictError{Files: conflicts}
	}
	for _, write := range writes {
		if err := write(); err != nil {
			return false, err
		}
	}
	return true, nil
}

// mergeAgentEdits 3-way merges the local edits to the agent at destFile onto
// theirs, the agent's new upstream content, with the version entry records as
// the base, and returns the content to write. It reports false when upstream
// left the agent alone; the content is then the local one.
func mergeAgentEdits(entry *MetadataEntry, discovery *DiscoveryResult, agent AgentInfo, destFile string, theirs []byte) ([]byte, bool, error) {
	commit := installedCommit(entry)
	if commit == "" {
		return nil, false, errors.New("no recorded commit to compare with")
	}
	source := *discovery.Source
	source.Commit = commit
	discover := DiscoverFromGit
	if source.HasSubdir() {
		discover = DiscoverFromGitSubdir
	}
	baseDiscovery, err := discover(&source)
	if err != nil {
		return nil, false, fmt.Errorf("fetch the installed version: %w", err)
	}
	defer CleanupDiscovery(baseDiscovery)
	i := slices.IndexFunc(baseDiscovery.Agents, func(a AgentInfo) bool { return a.Path == agent.Path })
	if i < 0 {
		return nil, false, errUnverifiableBase
	}
	baseFile := resolveDiscoveredAgentSourcePath(baseDiscovery, baseDiscovery.Agents[i])
	if hash, err := computeSingleFileHash(baseFile); err != nil || hash != entry.FileHashes[filepath.Base(destFile)] {
		return nil, false, errUnverifiableBase
	}
	base, err := os.ReadFile(baseFile)
	if err != nil {
		return nil, false, err
	}
	ours, err := os.ReadFile(destFile)
	if err != nil {
		return nil, false, err
	}
	if bytes.Equal(base, theirs) {
		return ours, false, nil
	}
	merged, conflict, err := mergeFile(base, ours, theirs)
	if err != nil {
		return nil, false, fmt.Errorf("merge %s: %w", filepath.Base(destFile), err)
	}
	if conflict {
		return nil, false, &MergeConflictError{Files: []string{filepath.Base(destFile)}}
	}
	return merged, true, nil
}

func copyIntoStage(out *sourcefs.Root, src, rel string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := out.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
		return err
	}
	return out.WriteFile(rel, data, fileMode(src))
}

func fileMode(path string) os.FileMode {
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()
	}
	return 0o644
}

func mergeFiles(base, ours, theirs string) ([]byte, bool, error) {
	var data [3][]byte
	for i, p := range []string{base, ours, theirs} {
		var err error
		if data[i], err = os.ReadFile(p); err != nil {
			return nil, false, err
		}
	}
	return mergeFile(data[0], data[1], data[2])
}

// mergeFile 3-way merges ours and theirs with git merge-file. It reports a
// conflict instead of writing conflict markers, which would be synced to
// every target, and for binary content, which git cannot merge.
func mergeFile(base, ours, theirs []byte) ([]byte, bool, error) {
	if bytes.Equal(ours, theirs) {
		return ours, false, nil
	}
	if slices.ContainsFunc([][]byte{base, ours, theirs}, func(b []byte) bool { return bytes.IndexByte(b, 0) >= 0 }) {
		return nil, true, nil
	}
	tmp, err := os.MkdirTemp("", "skillshare-merge-")
	if err != nil {
		return nil, false, err
	}
	defer os.RemoveAll(tmp)
	names := []string{"ours", "base", "theirs"}
	for i, content := range [][]byte{ours, base, theirs} {
		if err := os.WriteFile(filepath.Join(tmp, names[i]), content, 0o600); err != nil {
			return nil, false, err
		}
	}
	cmd := gitenv.Command("merge-file", "-p", "ours", "base", "theirs")
	cmd.Dir = tmp
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	merged, err := cmd.Output()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return merged, false, nil
	// A positive exit status is the number of conflicts; errors exit 255.
	case errors.As(err, &exitErr) && exitErr.ExitCode() > 0 && exitErr.ExitCode() < 128:
		return nil, true, nil
	default:
		return nil, false, fmt.Errorf("git merge-file: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
}
