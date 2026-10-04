package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"skillshare/internal/sourcewalk"
)

// CheckSourceMutation returns the pinned incoming commit after checking that a
// source-repository merge/reset/checkout cannot replace a link in the worktree.
// Call it after fetch and immediately before applying that revision.
func CheckSourceMutation(dir, incoming string, follow *sourcewalk.FollowSet) (string, error) {
	revision := exec.Command("git", "rev-parse", "--verify", "--end-of-options", incoming+"^{commit}")
	revision.Dir = dir
	out, err := revision.Output()
	if err != nil {
		return "", fmt.Errorf("resolve incoming revision %q: %w", incoming, err)
	}
	commit := strings.TrimSpace(string(out))
	links, err := declaredLinkLocations(dir, follow, true)
	if err != nil {
		return "", err
	}
	for _, link := range links {
		indexed, err := IsPathIndexed(dir, link.Path)
		if err != nil {
			return "", err
		}
		if indexed {
			return "", fmt.Errorf("refusing incoming commit %s: declared entry %q is indexed; run %s and add %q to %s, or fix the remote", commit, link.Path, UntrackCommand(link.Path), link.IgnoreLine, link.IgnoreFile)
		}
	}
	top := exec.Command("git", "rev-parse", "--show-toplevel")
	top.Dir = dir
	out, err = top.Output()
	if err != nil {
		return "", fmt.Errorf("resolve source git root: %w", err)
	}
	root, err := sourcewalk.Canonicalize(strings.TrimSpace(string(out)))
	if err != nil {
		return "", err
	}
	args := []string{"diff", "--name-only", "--no-renames", "-z", "HEAD", commit, "--"}
	if _, headErr := GetCurrentFullHash(root); headErr != nil {
		// An unborn initial checkout has no HEAD; every incoming tree path is new.
		args = []string{"ls-tree", "-r", "--name-only", "-z", commit}
	}
	diff := exec.Command("git", args...)
	diff.Dir = root
	out, err = diff.Output()
	if err != nil {
		return "", fmt.Errorf("inspect incoming commit %s: %w", commit, err)
	}
	for _, path := range strings.Split(string(out), "\x00") {
		if path == "" {
			continue
		}
		link, err := linkComponent(root, path)
		if err != nil {
			return "", fmt.Errorf("inspect incoming path %q: %w", path, err)
		}
		if link != "" {
			rel, _ := filepath.Rel(root, link)
			return "", fmt.Errorf("refusing incoming commit %s: path %q touches link %q; run %s if indexed, or fix the remote", commit, path, filepath.ToSlash(rel), UntrackCommand(filepath.ToSlash(rel)))
		}
	}
	return commit, nil
}

// sourcePullRevision fetches once and pins the checked upstream; pulling again
// after this check would fetch a potentially different, unchecked revision.
func sourcePullRevision(dir string, extraEnv []string, onProgress func(string), follow *sourcewalk.FollowSet) (string, error) {
	args := []string{"fetch", "--quiet"}
	if onProgress != nil {
		args[1] = "--progress"
	}
	if err := runGitWithProgress(dir, args, extraEnv, onProgress); err != nil {
		return "", err
	}
	return CheckSourceMutation(dir, "@{upstream}", follow)
}
