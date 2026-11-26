package install

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"skillshare/internal/gitenv"
)

func gitSupportsSparseCheckout() bool {
	out, err := gitenv.Command("--version").Output()
	if err != nil {
		return false
	}
	return supportsSparseCheckoutVersion(strings.TrimSpace(string(out)))
}

func supportsSparseCheckoutVersion(versionOutput string) bool {
	// Examples:
	//   git version 2.44.0
	//   git version 2.39.3 (Apple Git-146)
	fields := strings.Fields(versionOutput)
	if len(fields) < 3 {
		return false
	}
	ver := fields[2]
	parts := strings.Split(ver, ".")
	if len(parts) < 2 {
		return false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}

	if major > 2 {
		return true
	}
	return major == 2 && minor >= 25
}

func sparseCloneSubdir(url, subdir, destPath, branch string, extraEnv []string, onProgress ProgressCallback) error {
	subdir = strings.TrimSpace(subdir)
	if subdir == "" {
		return fmt.Errorf("sparse checkout requires non-empty subdir")
	}

	cloneArgs := []string{"clone", "--filter=blob:none", "--no-checkout", "--depth", "1"}
	// clone --branch rejects a commit SHA: clone the default branch and fetch
	// the commit below instead.
	sha := IsCommitSHA(branch)
	if branch != "" && !sha {
		cloneArgs = append(cloneArgs, "--branch", branch)
	}
	if onProgress != nil {
		cloneArgs = append(cloneArgs, "--progress")
	} else {
		cloneArgs = append(cloneArgs, "--quiet")
	}
	cloneArgs = append(cloneArgs, url, destPath)
	if err := runGitCommandWithProgress(cloneArgs, "", extraEnv, onProgress); err != nil {
		return err
	}

	if err := runGitCommandWithProgress([]string{"sparse-checkout", "set", subdir}, destPath, extraEnv, nil); err != nil {
		return err
	}

	if sha {
		// Servers only resolve a full SHA here; an abbreviated one fails and the
		// caller falls back to a full clone.
		if err := runGitCommandWithProgress([]string{"fetch", "--quiet", "--depth", "1", "--filter=blob:none", "origin", branch}, destPath, extraEnv, nil); err != nil {
			return err
		}
		if err := runGitCommandWithProgress([]string{"checkout", "--quiet", "--detach", "FETCH_HEAD"}, destPath, extraEnv, nil); err != nil {
			return err
		}
	} else {
		if err := runGitCommandWithProgress([]string{"checkout"}, destPath, extraEnv, nil); err != nil {
			return err
		}
	}

	return materializeSparseSymlinkTargets(destPath, subdir, extraEnv)
}

const maxSparseSymlinkHops = 16

type gitTreeEntry struct {
	mode string
	kind string
	oid  string
	path string
}

// materializeSparseSymlinkTargets expands a sparse checkout when symlinks in
// subdir point to other files or folders inside the same repository.
func materializeSparseSymlinkTargets(repoPath, subdir string, extraEnv []string) error {
	cleanSubdir := normalizeGitTreePath(subdir)
	if cleanSubdir == "" {
		return nil
	}

	checkedDirs := map[string]bool{cleanSubdir: true}
	scannedPaths := map[string]bool{cleanSubdir: true}
	visitedLinks := map[string]bool{}
	pendingPaths := []string{cleanSubdir}

	for hop := 0; hop < maxSparseSymlinkHops && len(pendingPaths) > 0; hop++ {
		entries, err := gitLsTree(repoPath, extraEnv, true, pendingPaths...)
		if err != nil {
			return err
		}

		toAddSet := map[string]bool{}
		nextPendingSet := map[string]bool{}

		// If the initial subdir path itself traverses a symlinked ancestor,
		// resolve it so its target directory is checked out too.
		if hop == 0 && len(entries) == 0 {
			dirs, inspect, err := resolveGitTreePath(repoPath, extraEnv, cleanSubdir)
			if err != nil {
				return err
			}
			for _, d := range dirs {
				if !dirCoveredBy(d, checkedDirs) {
					toAddSet[d] = true
				}
			}
			for _, p := range inspect {
				if !scannedPaths[p] {
					nextPendingSet[p] = true
				}
			}
		}

		for _, e := range entries {
			if e.mode != "120000" || e.kind != "blob" {
				continue
			}
			if visitedLinks[e.path] {
				continue
			}
			visitedLinks[e.path] = true

			targetBytes, err := gitBytesOutput(repoPath, extraEnv, "cat-file", "blob", e.oid)
			if err != nil {
				return err
			}
			targetRel, ok := resolveRepoRelativeLink(e.path, string(targetBytes))
			if !ok {
				continue
			}

			dirs, inspect, err := resolveGitTreePath(repoPath, extraEnv, targetRel)
			if err != nil {
				return err
			}
			for _, d := range dirs {
				if !dirCoveredBy(d, checkedDirs) {
					toAddSet[d] = true
				}
			}
			for _, p := range inspect {
				if !scannedPaths[p] {
					nextPendingSet[p] = true
				}
			}
		}

		if len(toAddSet) > 0 {
			toAdd := make([]string, 0, len(toAddSet))
			for d := range toAddSet {
				toAdd = append(toAdd, d)
			}
			sort.Strings(toAdd)
			args := append([]string{"sparse-checkout", "add", "--"}, toAdd...)
			if err := runGitCommandWithProgress(args, repoPath, extraEnv, nil); err != nil {
				return err
			}
			for _, d := range toAdd {
				checkedDirs[d] = true
			}
		}

		pendingPaths = pendingPaths[:0]
		for p := range nextPendingSet {
			scannedPaths[p] = true
			pendingPaths = append(pendingPaths, p)
		}
		sort.Strings(pendingPaths)
	}

	return nil
}

// resolveGitTreePath walks path components in HEAD, following any intermediate
// symlinks inside the repository, and returns directories to add to
// sparse-checkout plus target paths to inspect for further symlinks.
func resolveGitTreePath(repoPath string, extraEnv []string, targetRel string) (dirsToAdd []string, inspectPaths []string, err error) {
	cur := normalizeGitTreePath(targetRel)
	for hop := 0; hop < maxSparseSymlinkHops && cur != ""; hop++ {
		parts := strings.Split(cur, "/")
		followed := false
		for i := range parts {
			prefix := strings.Join(parts[:i+1], "/")
			entries, lsErr := gitLsTree(repoPath, extraEnv, false, prefix)
			if lsErr != nil {
				return nil, nil, lsErr
			}
			if len(entries) == 0 {
				return dirsToAdd, inspectPaths, nil
			}
			e := entries[0]
			if e.mode == "120000" && e.kind == "blob" {
				if parent := path.Dir(e.path); parent != "." {
					dirsToAdd = append(dirsToAdd, parent)
				} else if _, statErr := os.Lstat(filepath.Join(repoPath, e.path)); os.IsNotExist(statErr) {
					dirsToAdd = append(dirsToAdd, e.path)
				}
				targetBytes, catErr := gitBytesOutput(repoPath, extraEnv, "cat-file", "blob", e.oid)
				if catErr != nil {
					return nil, nil, catErr
				}
				linkTarget := string(targetBytes)
				if i+1 < len(parts) {
					linkTarget = strings.TrimRight(linkTarget, "/") + "/" + strings.Join(parts[i+1:], "/")
				}
				next, ok := resolveRepoRelativeLink(e.path, linkTarget)
				if !ok {
					return dirsToAdd, inspectPaths, nil
				}
				cur = next
				followed = true
				break
			}
			if i == len(parts)-1 {
				switch e.kind {
				case "tree":
					dirsToAdd = append(dirsToAdd, e.path)
					inspectPaths = append(inspectPaths, e.path)
				case "blob":
					if parent := path.Dir(e.path); parent != "." {
						dirsToAdd = append(dirsToAdd, parent)
					} else if _, statErr := os.Lstat(filepath.Join(repoPath, e.path)); os.IsNotExist(statErr) {
						dirsToAdd = append(dirsToAdd, e.path)
					}
				}
				return dirsToAdd, inspectPaths, nil
			}
		}
		if !followed {
			break
		}
	}
	return dirsToAdd, inspectPaths, nil
}

func gitLsTree(repoPath string, extraEnv []string, recursive bool, paths ...string) ([]gitTreeEntry, error) {
	args := []string{"ls-tree"}
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, "-z", "HEAD", "--")
	args = append(args, paths...)
	out, err := gitBytesOutput(repoPath, extraEnv, args...)
	if err != nil {
		return nil, err
	}
	return parseLsTreeEntries(out), nil
}

func gitBytesOutput(repoPath string, extraEnv []string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := gitCommand(ctx, args...)
	cmd.Dir = repoPath
	cmd.Env = append(cmd.Env, extraEnv...)
	return cmd.Output()
}

func parseLsTreeEntries(raw []byte) []gitTreeEntry {
	var entries []gitTreeEntry
	for _, rec := range bytes.Split(raw, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		tab := bytes.IndexByte(rec, '\t')
		if tab <= 0 {
			continue
		}
		fields := strings.Fields(string(rec[:tab]))
		if len(fields) < 3 {
			continue
		}
		entries = append(entries, gitTreeEntry{
			mode: fields[0],
			kind: fields[1],
			oid:  fields[2],
			path: string(rec[tab+1:]),
		})
	}
	return entries
}

func normalizeGitTreePath(p string) string {
	p = strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
	p = strings.Trim(path.Clean(p), "/")
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return ""
	}
	return p
}

// resolveRepoRelativeLink resolves linkTarget relative to linkPath within the
// repository root. Returns ("", false) for absolute links, links escaping the
// repository, links targeting `.git`, or links pointing to the repo root.
func resolveRepoRelativeLink(linkPath, linkTarget string) (string, bool) {
	if linkTarget == "" ||
		strings.HasPrefix(linkTarget, "/") ||
		strings.HasPrefix(linkTarget, "\\") ||
		filepath.IsAbs(linkTarget) ||
		filepath.VolumeName(linkTarget) != "" {
		return "", false
	}
	normLink := strings.ReplaceAll(linkPath, "\\", "/")
	normTarget := strings.ReplaceAll(linkTarget, "\\", "/")
	resolved := path.Clean(path.Join(path.Dir(normLink), normTarget))
	if resolved == "" || resolved == "." || resolved == ".." || strings.HasPrefix(resolved, "../") {
		return "", false
	}
	if resolved == ".git" || strings.HasPrefix(resolved, ".git/") {
		return "", false
	}
	return resolved, true
}

func dirCoveredBy(dir string, checked map[string]bool) bool {
	if checked[dir] {
		return true
	}
	for existing := range checked {
		if strings.HasPrefix(dir, existing+"/") {
			return true
		}
	}
	return false
}
