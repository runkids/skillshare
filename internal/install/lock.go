package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"skillshare/internal/audit"
)

// LockFileName sits next to the project config and is meant to be committed.
// The config says what to follow ("main"); the lock says which commit that was
// when someone last installed or updated, so everyone else gets the same one.
const LockFileName = "skills.lock.json"

// Lock pins each remote skill of a project to a commit.
type Lock struct {
	Version int                  `json:"version"`
	Skills  map[string]LockEntry `json:"skills"`
}

// LockEntry is one pinned skill. Source guards against a stale pin: once the
// config points the name at another source, the entry no longer applies.
type LockEntry struct {
	Source   string `json:"source"`
	Commit   string `json:"commit"`
	TreeHash string `json:"tree_hash,omitempty"`
}

// LoadLock reads the lockfile in dir. A missing file is an empty lock.
func LoadLock(dir string) (*Lock, error) {
	data, err := os.ReadFile(filepath.Join(dir, LockFileName))
	if os.IsNotExist(err) {
		return &Lock{}, nil
	}
	if err != nil {
		return nil, err
	}
	lock := &Lock{}
	if err := json.Unmarshal(data, lock); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", LockFileName, err)
	}
	return lock, nil
}

// CommitFor returns the pinned commit for name, or "" when there is none, the
// pin was made for a different source, or it is not a full commit SHA.
func (l *Lock) CommitFor(name, source string) string {
	if l == nil {
		return ""
	}
	// The file is committed, so anyone's pull request can edit it, and the value
	// ends up as a git argument: accept a full SHA and nothing else.
	if e, ok := l.Skills[name]; ok && e.Source == source && len(e.Commit) == 40 && IsCommitSHA(e.Commit) {
		return e.Commit
	}
	return ""
}

// MovePin re-keys the pin of a skill that moved to another path and reports
// whether there was one.
func (l *Lock) MovePin(oldName, newName string) bool {
	pin, ok := l.Skills[oldName]
	if ok {
		delete(l.Skills, oldName)
		l.Skills[newName] = pin
	}
	return ok
}

// MoveLockPins re-keys the pins of the skills in moves (old path to new path)
// in dir's lockfile and saves it when one changed.
func MoveLockPins(dir string, moves map[string]string) error {
	lock, err := LoadLock(dir)
	if err != nil {
		return err
	}
	changed := false
	for from, to := range moves {
		changed = lock.MovePin(from, to) || changed
	}
	if !changed {
		return nil
	}
	return lock.Save(dir)
}

// Save writes the lockfile, leaving it untouched when nothing changed and
// removing it when there is nothing to pin.
func (l *Lock) Save(dir string) error {
	path := filepath.Join(dir, LockFileName)
	if len(l.Skills) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	l.Version = 1
	// encoding/json sorts map keys, so the file diffs cleanly.
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	return os.WriteFile(path, data, 0o644)
}

// InstalledCommit is the full SHA a skill is at: HEAD for a tracked repo, the
// recorded commit otherwise. "" when unknown (local source, API download).
func InstalledCommit(skillPath string, entry *MetadataEntry) string {
	if IsGitRepo(skillPath) {
		hash, _ := getGitFullHash(skillPath)
		return hash
	}
	if entry == nil {
		return ""
	}
	return entry.Commit
}

// resetTrackedToCommit moves a freshly cloned tracked repo to the locked
// commit. HEAD stays on its branch so `update` can still pull.
func resetTrackedToCommit(repoPath, sha string, extraEnv []string) error {
	if head, _ := getGitFullHash(repoPath); strings.HasPrefix(head, sha) {
		return nil
	}
	// Local edits are the user's; update refuses to run over them and so does this.
	if out, err := gitOutput(repoPath, "status", "--porcelain"); err != nil || out != "" {
		return fmt.Errorf("uncommitted changes in %s: commit or discard them, then install again", filepath.Base(repoPath))
	}
	// Tracked repos are cloned at depth 1. Resetting such a clone to an older
	// commit leaves the branch with no ancestor in common with origin, and the
	// next pull refuses to merge. Fetch the history first; the clone filters
	// blobs, so this is commits and trees only.
	if isShallowRepo(repoPath) {
		if err := runGitCommandEnv([]string{"fetch", "--quiet", "--unshallow", "origin"}, repoPath, extraEnv); err != nil {
			return fmt.Errorf("fetch history for locked commit %s: %w", shortHash(sha), err)
		}
	}
	if gitResetHard(repoPath, sha) == nil {
		return nil
	}
	// Not on the cloned branch (the single-branch clone has nothing else).
	if err := runGitCommandEnv([]string{"fetch", "--quiet", "origin", sha}, repoPath, extraEnv); err != nil {
		return fmt.Errorf("locked commit %s is not available: %w", shortHash(sha), err)
	}
	return gitResetHard(repoPath, "FETCH_HEAD")
}

func gitOutput(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// relockGitRepo moves an installed git checkout (tracked repo, or a skill that
// is a whole repo) to the locked commit, behind the same audit gate as update.
func relockGitRepo(repoPath, sha string, opts InstallOptions) error {
	before, err := getGitFullHash(repoPath)
	if err != nil || before == "" {
		return fmt.Errorf("cannot read the current commit of %s", repoPath)
	}
	if err := resetTrackedToCommit(repoPath, sha, AuthEnvForURL(getRemoteURL(repoPath))); err != nil {
		return err
	}
	if opts.SkipAudit {
		return nil
	}
	threshold, err := audit.NormalizeThreshold(opts.AuditThreshold)
	if err != nil {
		threshold = audit.DefaultThreshold()
	}
	_, err = auditGateFailClosed(opts.SourceDir, repoPath, before, threshold, opts.AuditProjectRoot, opts.AuditOverride)
	return err
}
