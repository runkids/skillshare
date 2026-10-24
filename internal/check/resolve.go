package check

import (
	"context"
	"strings"
	"sync"

	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
)

// Skill is an installed skill to check. Entry is nil when the skill has no
// metadata. Dir is the skill's directory, resolved through Follow, and is
// hashed to detect local changes to a remote skill.
type Skill struct {
	Name   string
	Entry  *install.MetadataEntry
	Dir    string
	Follow *sourcewalk.Follow
}

// SkillResult is the update status of one skill.
type SkillResult struct {
	Name        string
	Source      string
	Version     string
	InstalledAt string // "2006-01-02", empty when unknown
	Status      string // "up_to_date", "update_available", "stale", "modified", "local", "error"
	Message     string
	Local       bool // the skill has no remote; Status comes from LocalSourceStatus
}

// Remote reports the state of a remote repository. An empty branch means the
// remote HEAD.
type Remote interface {
	// Hash returns the commit the ref points at.
	Hash(repoURL, branch string) (string, error)
	// TreeHashes maps every directory at the ref to its git tree hash. It
	// returns nil when the hashes cannot be fetched.
	TreeHashes(repoURL, branch string) map[string]string
}

// GitRemote is the Remote backed by git, with token auth for private repos.
type GitRemote struct{}

func (GitRemote) Hash(repoURL, branch string) (string, error) {
	if branch != "" {
		return git.GetRemoteRefHashWithAuth(repoURL, branch)
	}
	return git.GetRemoteHeadHashWithAuth(repoURL)
}

func (GitRemote) TreeHashes(repoURL, branch string) map[string]string {
	return FetchRemoteTreeHashesForRef(repoURL, branch)
}

// Options configures Resolution.Run.
type Options struct {
	// Remote is the source of remote hashes; nil means GitRemote.
	Remote Remote
	// Parallel checks up to maxWorkers remotes at once instead of one by one.
	Parallel bool
	// OnRemoteDone, if set, is called after each remote is resolved with the
	// number of skills installed from it. With Parallel it is called from
	// several goroutines.
	OnRemoteDone func(skills int)
}

// Resolution is a planned check: skills without a remote are already
// resolved, the rest are grouped by clone URL and installed branch so each
// remote is asked once.
type Resolution struct {
	local  []SkillResult
	groups []remoteGroup
}

type remoteKey struct{ url, branch string }

type remoteGroup struct {
	remoteKey
	skills []Skill
}

// Plan resolves the skills that have no remote and groups the others by
// remote. projectRoot is the base of relative local sources; "" in global mode.
func Plan(skills []Skill, projectRoot string) *Resolution {
	r := &Resolution{}
	index := make(map[remoteKey]int)
	for _, s := range skills {
		if s.Entry == nil || s.Entry.RepoURL == "" {
			result := baseResult(s)
			result.Local = true
			result.Status, result.Message = LocalSourceStatus(s.Entry, projectRoot)
			r.local = append(r.local, result)
			continue
		}
		key := remoteKey{s.Entry.RepoURL, s.Entry.Branch}
		i, ok := index[key]
		if !ok {
			i = len(r.groups)
			index[key] = i
			r.groups = append(r.groups, remoteGroup{remoteKey: key})
		}
		r.groups[i].skills = append(r.groups[i].skills, s)
	}
	return r
}

// Remotes returns how many remotes Run asks.
func (r *Resolution) Remotes() int { return len(r.groups) }

// RemoteSkills returns how many skills are installed from those remotes.
func (r *Resolution) RemoteSkills() int {
	n := 0
	for _, g := range r.groups {
		n += len(g.skills)
	}
	return n
}

// Run asks each remote and returns every skill's status: skills without a
// remote first, then each remote's skills in the order Plan received them.
// When ctx is cancelled it stops before the next remote and returns ctx's
// error.
func (r *Resolution) Run(ctx context.Context, opts Options) ([]SkillResult, error) {
	remote := opts.Remote
	if remote == nil {
		remote = GitRemote{}
	}

	resolved := make([][]SkillResult, len(r.groups))
	resolve := func(i int) {
		g := r.groups[i]
		resolved[i] = g.resolve(remote)
		g.markModified(resolved[i])
		if opts.OnRemoteDone != nil {
			opts.OnRemoteDone(len(g.skills))
		}
	}

	if !opts.Parallel {
		// On the caller's goroutine, so a panic reaches the caller.
		for i := range r.groups {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			resolve(i)
		}
	} else {
		sem := make(chan struct{}, maxWorkers)
		var wg sync.WaitGroup
		var err error
		for i := range r.groups {
			sem <- struct{}{}
			if err = ctx.Err(); err != nil {
				break
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				resolve(i)
			}()
		}
		wg.Wait()
		if err != nil {
			return nil, err
		}
	}

	results := append([]SkillResult(nil), r.local...)
	for _, group := range resolved {
		results = append(results, group...)
	}
	return results, nil
}

func (g remoteGroup) resolve(remote Remote) []SkillResult {
	results := make([]SkillResult, len(g.skills))
	for i, s := range g.skills {
		results[i] = baseResult(s)
	}

	remoteHash, err := remote.Hash(g.url, g.branch)
	if err != nil {
		for i := range results {
			results[i].Status = "error"
		}
		return results
	}

	// Tree hashes cost a fetch, so they are skipped when every skill is at the
	// remote commit and when no skill recorded one.
	allMatch, hasTreeHash := true, false
	for _, s := range g.skills {
		allMatch = allMatch && s.Entry.Version == remoteHash
		hasTreeHash = hasTreeHash || (s.Entry.TreeHash != "" && s.Entry.Subdir != "")
	}
	var trees map[string]string
	if !allMatch && hasTreeHash {
		trees = remote.TreeHashes(g.url, g.branch)
	}

	for i, s := range g.skills {
		results[i].Status = remoteStatus(s.Entry, remoteHash, trees)
	}
	return results
}

// markModified reports skills whose files changed since install as
// "modified": update skips them unless forced, so they are not counted as
// updatable. A failed remote check keeps its error.
func (g remoteGroup) markModified(results []SkillResult) {
	for i, s := range g.skills {
		if results[i].Status == "error" || !s.Entry.LocallyModified(s.Dir, s.Follow) {
			continue
		}
		results[i].Message = "local changes since install"
		if results[i].Status == "update_available" {
			results[i].Message += "; update available upstream"
		}
		results[i].Status = "modified"
	}
}

// remoteStatus compares one skill with its remote. A moved commit alone does
// not mean the skill changed: when the skill's subdir tree hash is known on
// both sides, that decides. trees is nil when it was not fetched.
func remoteStatus(entry *install.MetadataEntry, remoteHash string, trees map[string]string) string {
	if entry.Version == remoteHash {
		return "up_to_date"
	}
	if entry.TreeHash == "" || entry.Subdir == "" || trees == nil {
		return "update_available"
	}
	// ls-tree paths never have a leading "/", but Subdir may.
	tree, ok := trees[strings.TrimPrefix(entry.Subdir, "/")]
	switch {
	case !ok:
		return "stale" // removed upstream
	case tree == entry.TreeHash:
		return "up_to_date"
	default:
		return "update_available"
	}
}

func baseResult(s Skill) SkillResult {
	result := SkillResult{Name: s.Name}
	if s.Entry != nil {
		result.Source = s.Entry.Source
		result.Version = s.Entry.Version
		if !s.Entry.InstalledAt.IsZero() {
			result.InstalledAt = s.Entry.InstalledAt.Format("2006-01-02")
		}
	}
	return result
}
