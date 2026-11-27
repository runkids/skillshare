package check

import (
	"context"
	"os"
	"path"
	"strings"
	"time"

	"skillshare/internal/gitenv"
	"skillshare/internal/install"
)

// RemoteTrees holds a remote ref's directory tree hashes and the text of its
// links, so a subdir reached through a link resolves to the folder it leads to.
type RemoteTrees struct {
	Hashes map[string]string
	Links  map[string]string
	// LinksUnread holds the link paths whose text could not be read.
	LinksUnread map[string]bool
}

// Hash returns the tree hash of subdir, following the links along it as
// install does when it records a skill installed through a link. A subdir
// through a link whose text could not be read hashes to "", so it is
// compared by commit rather than reported as removed.
func (t *RemoteTrees) Hash(subdir string) (string, bool) {
	for p := subdir; p != "." && p != "/"; p = path.Dir(p) {
		if t.LinksUnread[p] {
			return "", true
		}
	}
	// A link is listed too, with "", since it cannot be compared itself.
	if _, isLink := t.Links[subdir]; !isLink {
		if hash, ok := t.Hashes[subdir]; ok {
			return hash, true
		}
	}
	resolved, ok := install.ResolveTreeLinks(subdir, func(p string) (string, bool, error) {
		text, isLink := t.Links[p]
		return text, isLink, nil
	})
	if !ok || resolved == subdir {
		return "", false
	}
	hash, ok := t.Hashes[resolved]
	return hash, ok
}

// FetchRemoteTreeHashes performs a blobless fetch of the remote HEAD and
// returns a map of subdir paths to their git tree hashes and file paths to
// their blob hashes, plus the link texts. This allows per-skill and per-agent
// version comparison without downloading full blobs.
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

	trees := &RemoteTrees{Hashes: parseLsTreeOutput(string(out))}
	oids := linkOids(string(out))
	links, err := fetchLinkTexts(tmpDir, repoURL, extraEnv, oids)
	if err != nil {
		trees.LinksUnread = make(map[string]bool, len(oids))
		for p := range oids {
			trees.LinksUnread[p] = true
		}
		return trees
	}
	trees.Links = links
	return trees
}

// linkOids maps each link path in ls-tree -z output to its blob id.
func linkOids(lsTree string) map[string]string {
	oids := map[string]string{}
	for _, line := range strings.Split(lsTree, "\x00") {
		meta, p, ok := strings.Cut(line, "\t")
		if fields := strings.Fields(meta); ok && len(fields) == 3 && fields[0] == "120000" {
			oids[p] = fields[2]
		}
	}
	return oids
}

// linkFetchTimeout bounds the fetch of link blobs.
const linkFetchTimeout = 3 * time.Minute

// fetchLinkTexts reads the text of each link, given as its path mapped to its
// blob id. The blobless fetch left the link blobs out, so they are fetched in
// one request first, ids on stdin; reading them one by one would fetch each
// on its own.
func fetchLinkTexts(dir, repoURL string, extraEnv []string, oids map[string]string) (map[string]string, error) {
	if len(oids) == 0 {
		return nil, nil
	}
	want := map[string]bool{}
	var stdin strings.Builder
	for _, oid := range oids {
		if !want[oid] {
			want[oid] = true
			stdin.WriteString(oid + "\n")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), linkFetchTimeout)
	defer cancel()
	fetchCmd := gitenv.CommandContext(ctx, "fetch", "--quiet", "--filter=blob:none", "--depth=1", "--stdin", repoURL)
	fetchCmd.Dir = dir
	fetchCmd.Env = append(fetchCmd.Env, extraEnv...)
	fetchCmd.Stdin = strings.NewReader(stdin.String())
	_ = fetchCmd.Run() // a server that refuses blob ids leaves LinkTexts to fetch each
	return install.LinkTexts(dir, oids, extraEnv)
}

// parseLsTreeOutput parses `git ls-tree -r -t` output into a map of each
// directory to its tree hash and each file to its blob hash, which agents
// compare. Each entry: "<mode> <type> <hash>\t<path>", NUL-terminated. A
// symlink, and a directory that holds one, maps to "": install copies the
// link's target, whose changes the hash cannot see, so it must not count as
// unchanged.
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
		if fields[1] != "tree" && fields[1] != "blob" {
			continue
		}
		result[path] = fields[2]
	}
	// shortcut: a link whose target stays inside the directory blanks it too, resolve link targets if skills report update_available too often.
	for _, link := range symlinks {
		result[link] = ""
		for dir := path.Dir(link); dir != "."; dir = path.Dir(dir) {
			result[dir] = ""
		}
	}
	return result
}
