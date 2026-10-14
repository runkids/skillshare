package check

import (
	"os"
	"os/exec"
	"path"
	"strings"

	"skillshare/internal/install"
)

// FetchRemoteTreeHashes performs a blobless fetch of the remote HEAD and
// returns a map of subdir paths to their git tree hashes. This allows
// per-skill version comparison without downloading full blobs.
//
// Returns nil on any error (unsupported git version, network failure, etc.)
// so callers can fall back to commit-level comparison.
func FetchRemoteTreeHashes(repoURL string) map[string]string {
	return FetchRemoteTreeHashesForRef(repoURL, "")
}

// FetchRemoteTreeHashesForRef is FetchRemoteTreeHashes for a specific branch.
// An empty ref means the remote HEAD.
func FetchRemoteTreeHashesForRef(repoURL, ref string) map[string]string {
	if ref == "" {
		ref = "HEAD"
	}
	tmpDir, err := os.MkdirTemp("", "skillshare-treecheck-*")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(tmpDir)

	// Init bare repo
	initCmd := exec.Command("git", "init", "--bare")
	initCmd.Dir = tmpDir
	if out, err := initCmd.CombinedOutput(); err != nil {
		_ = out
		return nil
	}

	// Blobless fetch: downloads only tree + commit objects (~150-200KB)
	fetchArgs := []string{"fetch", "--filter=blob:none", "--depth=1", repoURL, ref}
	fetchCmd := exec.Command("git", fetchArgs...)
	fetchCmd.Dir = tmpDir
	// Inject auth env for private repos
	if extraEnv := install.AuthEnvForURL(repoURL); len(extraEnv) > 0 {
		fetchCmd.Env = append(os.Environ(), extraEnv...)
	}
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		_ = out
		return nil
	}

	// List all entries recursively, trees included; blob entries need no
	// blob content, so the blobless fetch is enough. -z keeps paths unquoted.
	// Output format: "040000 tree <hash>\t<path>\0"
	lsCmd := exec.Command("git", "ls-tree", "-r", "-t", "-z", "FETCH_HEAD")
	lsCmd.Dir = tmpDir
	out, err := lsCmd.Output()
	if err != nil {
		return nil
	}

	return parseLsTreeOutput(string(out))
}

// parseLsTreeOutput parses `git ls-tree -r -t` output into a map[path]treeHash.
// Each entry: "<mode> <type> <hash>\t<path>", NUL-terminated. A directory
// that holds a symlink maps to "": install copies the link's target, whose
// changes its tree hash cannot see, so it must not count as unchanged.
func parseLsTreeOutput(output string) map[string]string {
	result := make(map[string]string)
	var symlinks []string
	for _, line := range strings.Split(output, "\x00") {
		// Split on tab: left side has "mode type hash", right side is path
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		path := parts[1]
		// Extract hash from "mode type hash"
		fields := strings.Fields(parts[0])
		if len(fields) != 3 {
			continue
		}
		if fields[0] == "120000" {
			symlinks = append(symlinks, path)
		}
		if fields[1] != "tree" {
			continue
		}
		result[path] = fields[2]
	}
	// shortcut: a link whose target stays inside the directory blanks it too, resolve link targets if skills report update_available too often.
	for _, link := range symlinks {
		for dir := path.Dir(link); dir != "."; dir = path.Dir(dir) {
			result[dir] = ""
		}
	}
	return result
}
