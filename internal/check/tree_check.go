package check

import (
	"os"
	"os/exec"
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
	fetchArgs := []string{"fetch", "--filter=blob:none", "--depth=1", repoURL, "HEAD"}
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

	// List all tree objects recursively
	// Output format: "040000 tree <hash>\t<path>"
	lsCmd := exec.Command("git", "ls-tree", "-r", "-d", "FETCH_HEAD")
	lsCmd.Dir = tmpDir
	out, err := lsCmd.Output()
	if err != nil {
		return nil
	}

	return parseLsTreeOutput(string(out))
}

// parseLsTreeOutput parses `git ls-tree -r -d` output into a map[path]treeHash.
// Each line: "040000 tree <hash>\t<path>"
func parseLsTreeOutput(output string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
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
		hash := fields[2]
		result[path] = hash
	}
	return result
}
