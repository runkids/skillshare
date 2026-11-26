package check

import (
	"os"
	"path"
	"strings"

	"skillshare/internal/gitenv"
	"skillshare/internal/install"
)

// RemoteTrees holds a remote ref's directory tree hashes and the text of its
// links, so a subdir reached through a link resolves to the folder it leads to.
type RemoteTrees struct {
	Hashes map[string]string
	Links  map[string]string
}

// Hash returns the tree hash of subdir, following the links along it as
// install does when it records a skill installed through a link.
func (t *RemoteTrees) Hash(subdir string) (string, bool) {
	if hash, ok := t.Hashes[subdir]; ok {
		return hash, true
	}
	resolved, ok := install.ResolveTreeLinks(subdir, t.Links)
	if !ok || resolved == subdir {
		return "", false
	}
	hash, ok := t.Hashes[resolved]
	return hash, ok
}

// FetchRemoteTreeHashes performs a blobless fetch of the remote HEAD and
// returns the tree hash of every directory, plus the link texts. This allows
// per-skill version comparison without downloading full blobs.
//
// Returns nil on any error (unsupported git version, network failure, etc.)
// so callers can fall back to commit-level comparison.
func FetchRemoteTreeHashes(repoURL string, opts install.ParseOptions) *RemoteTrees {
	return FetchRemoteTreeHashesForRef(repoURL, "", opts)
}

// FetchRemoteTreeHashesForRef is FetchRemoteTreeHashes for a specific branch.
// An empty ref means the remote HEAD.
func FetchRemoteTreeHashesForRef(repoURL, ref string, opts install.ParseOptions) *RemoteTrees {
	if ref == "" {
		ref = "HEAD"
	}
	tmpDir, err := os.MkdirTemp("", "skillshare-treecheck-*")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(tmpDir)

	// Init bare repo
	initCmd := gitenv.Command("init", "--bare")
	initCmd.Dir = tmpDir
	if out, err := initCmd.CombinedOutput(); err != nil {
		_ = out
		return nil
	}

	// Blobless fetch: downloads only tree + commit objects (~150-200KB)
	fetchArgs := []string{"fetch", "--filter=blob:none", "--depth=1", repoURL, ref}
	fetchCmd := gitenv.Command(fetchArgs...)
	fetchCmd.Dir = tmpDir
	// Inject auth env for private repos
	extraEnv := install.AuthEnvForURL(repoURL, opts)
	fetchCmd.Env = append(fetchCmd.Env, extraEnv...)
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		_ = out
		return nil
	}

	// List all entries recursively, trees included; blob entries need no
	// blob content, so the blobless fetch is enough. -z keeps paths unquoted.
	// Output format: "040000 tree <hash>\t<path>\0"
	lsCmd := gitenv.Command("ls-tree", "-r", "-t", "-z", "FETCH_HEAD")
	lsCmd.Dir = tmpDir
	out, err := lsCmd.Output()
	if err != nil {
		return nil
	}

	links, err := fetchLinkTexts(tmpDir, repoURL, extraEnv, string(out))
	if err != nil {
		// Without the link texts a skill installed through a link would look
		// removed upstream; compare by commit instead.
		return nil
	}
	return &RemoteTrees{Hashes: parseLsTreeOutput(string(out)), Links: links}
}

// fetchLinkTexts reads the text of every link in the ls-tree output. The
// blobless fetch left the link blobs out, so they are fetched in one request
// first; reading them one by one would fetch each on its own.
func fetchLinkTexts(dir, repoURL string, extraEnv []string, lsTree string) (map[string]string, error) {
	var oids []string
	for _, line := range strings.Split(lsTree, "\x00") {
		if fields := strings.Fields(line); len(fields) >= 3 && fields[0] == "120000" {
			oids = append(oids, fields[2])
		}
	}
	if len(oids) == 0 {
		return nil, nil
	}
	// The fetch below moves FETCH_HEAD to the blobs, so pin the commit first.
	revCmd := gitenv.Command("rev-parse", "FETCH_HEAD")
	revCmd.Dir = dir
	commit, err := revCmd.Output()
	if err != nil {
		return nil, err
	}
	fetchCmd := gitenv.Command(append([]string{"fetch", "--quiet", "--filter=blob:none", "--depth=1", repoURL}, oids...)...)
	fetchCmd.Dir = dir
	fetchCmd.Env = append(fetchCmd.Env, extraEnv...)
	_ = fetchCmd.Run() // a server that refuses blob ids leaves TreeLinks to fetch each
	return install.TreeLinks(dir, strings.TrimSpace(string(commit)), extraEnv)
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
