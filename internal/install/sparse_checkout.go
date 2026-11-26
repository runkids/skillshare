package install

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
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
		return runGitCommandWithProgress([]string{"checkout", "--quiet", "--detach", "FETCH_HEAD"}, destPath, extraEnv, nil)
	}
	return runGitCommandWithProgress([]string{"checkout"}, destPath, extraEnv, nil)
}

// sparseLinksFetched fails when a link under subdir points to something the
// sparse checkout left out, such as ../../shared/ref.md, so a caller that
// copies subdir falls back to a full clone where the link resolves. A
// tracked checkout keeps such links instead (#1030).
func sparseLinksFetched(repoPath, subdir string, extraEnv []string) error {
	entries, err := gitLsTree(repoPath, extraEnv, strings.Trim(subdir, "/"))
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.mode != "120000" {
			continue
		}
		if _, err := os.Stat(filepath.Join(repoPath, filepath.FromSlash(e.path))); err != nil {
			return fmt.Errorf("%s links to something the sparse checkout did not fetch", e.path)
		}
	}
	return nil
}

type gitTreeEntry struct {
	mode string
	kind string
	oid  string
	path string
}

func gitLsTree(repoPath string, extraEnv []string, paths ...string) ([]gitTreeEntry, error) {
	args := append([]string{"ls-tree", "-r", "-z", "HEAD", "--"}, paths...)
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
