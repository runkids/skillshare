package git

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// WriteScopeGitignore ensures a .gitignore at dir. If none exists it writes a
// default; at "root" scope the default also excludes config.yaml. If a
// .gitignore already exists it is preserved, but at "root" scope config.yaml is
// appended when missing (idempotent) — this is the safety guarantee that a
// root-scope repo never tracks machine-specific config.yaml.
func WriteScopeGitignore(dir, scope string) error {
	gitignore := filepath.Join(dir, ".gitignore")

	if existing, err := os.ReadFile(gitignore); err == nil {
		if scope == "root" && !gitignoreHasEntry(string(existing), "config.yaml") {
			content := string(existing)
			if content != "" && !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			content += "config.yaml\n"
			return os.WriteFile(gitignore, []byte(content), 0o644)
		}
		return nil
	}

	lines := []string{".DS_Store"}
	if scope == "root" {
		lines = append([]string{"config.yaml"}, lines...)
	}
	return os.WriteFile(gitignore, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// gitignoreHasEntry reports whether content has a line exactly matching entry
// (ignoring surrounding whitespace), so we don't add a duplicate.
func gitignoreHasEntry(content, entry string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == entry {
			return true
		}
	}
	return false
}

// EnsureLocalIdentity sets a repo-local fallback user.name/email when no git
// identity is configured (global or local), so commits succeed in fresh
// environments. Returns true if the fallback was applied.
func EnsureLocalIdentity(dir string) (bool, error) {
	cmd := exec.Command("git", "config", "user.name")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) != "" {
		return false, nil // already configured
	}

	nameCmd := exec.Command("git", "config", "user.name", "skillshare")
	nameCmd.Dir = dir
	if err := nameCmd.Run(); err != nil {
		return false, err
	}
	emailCmd := exec.Command("git", "config", "user.email", "skillshare@local")
	emailCmd.Dir = dir
	if err := emailCmd.Run(); err != nil {
		return false, err
	}
	return true, nil
}

// FallbackIdentityEnv returns GIT_AUTHOR_* and GIT_COMMITTER_* variables
// with the same fallback EnsureLocalIdentity writes, for a single commit in
// a repo the user owns, when git has no name or email for dir. Unlike
// EnsureLocalIdentity it leaves the repo config alone, so an identity the
// user sets later still applies. Variables already in the environment win.
func FallbackIdentityEnv(dir string) []string {
	if gitConfigValue(dir, "user.name") != "" && gitConfigValue(dir, "user.email") != "" {
		return nil
	}
	fallback := []struct{ key, value string }{
		{"GIT_AUTHOR_NAME", "skillshare"}, {"GIT_AUTHOR_EMAIL", "skillshare@local"},
		{"GIT_COMMITTER_NAME", "skillshare"}, {"GIT_COMMITTER_EMAIL", "skillshare@local"},
	}
	var env []string
	for _, f := range fallback {
		if os.Getenv(f.key) == "" {
			env = append(env, f.key+"="+f.value)
		}
	}
	return env
}

func gitConfigValue(dir, key string) string {
	cmd := exec.Command("git", "config", key)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// InitScopeRepo initializes a git repository at dir if one is not already
// present: it creates dir, runs `git init`, writes a scope-aware .gitignore,
// and ensures a local identity. If dir is already a repo it only ensures the
// scope-aware .gitignore (so an existing root repo gains config.yaml exclusion).
// identitySet reports whether a fallback git identity was applied.
func InitScopeRepo(dir, scope string) (identitySet bool, err error) {
	if IsRepo(dir) {
		return false, WriteScopeGitignore(dir, scope)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return false, err
	}
	if err := WriteScopeGitignore(dir, scope); err != nil {
		return false, err
	}
	return EnsureLocalIdentity(dir)
}

// ensureGitignoreEntry appends entry as a line to dir/.gitignore when absent,
// creating the file if needed. It is the single-entry counterpart to
// WriteScopeGitignore and is idempotent.
func ensureGitignoreEntry(dir, entry string) error {
	gitignore := filepath.Join(dir, ".gitignore")
	existing, err := os.ReadFile(gitignore)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(gitignore, []byte(entry+"\n"), 0o644)
		}
		return err
	}
	if gitignoreHasEntry(string(existing), entry) {
		return nil
	}
	content := string(existing)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += entry + "\n"
	return os.WriteFile(gitignore, []byte(content), 0o644)
}

// ConfigGitignoreNeedsRepair reports whether EnsureConfigUntracked would change
// dir/.gitignore: the explicit config.yaml entry is missing, or the rules do
// not actually ignore it. A dry run uses it to preview that change.
func ConfigGitignoreNeedsRepair(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	return err != nil || !gitignoreHasEntry(string(data), "config.yaml") || !isIgnored(dir, "config.yaml")
}

// isIgnored reports whether .gitignore rules ignore path, even if it is tracked.
func isIgnored(dir, path string) bool {
	cmd := exec.Command("git", "check-ignore", "-q", "--no-index", "--", path)
	cmd.Dir = dir
	return cmd.Run() == nil
}

// appendGitignoreLine appends entry to dir/.gitignore even when an earlier line
// matches it, so it overrides any negation before it.
func appendGitignoreLine(dir, entry string) error {
	gitignore := filepath.Join(dir, ".gitignore")
	existing, err := os.ReadFile(gitignore)
	if err != nil {
		return err
	}
	content := string(existing)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(gitignore, []byte(content+entry+"\n"), 0o644)
}

// IsConfigTracked reports whether config.yaml is tracked in the repo at dir.
// Used by the web UI to warn that a root-scope repo is versioning the
// machine-specific config.yaml.
func IsConfigTracked(dir string) bool {
	return isTracked(dir, "config.yaml")
}

// isTracked reports whether path (relative to dir) is tracked in the repo at dir.
func isTracked(dir, path string) bool {
	cmd := exec.Command("git", "ls-files", "--", path)
	cmd.Dir = dir
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// RemoteTracksConfig reports whether the remote ref (e.g. "origin/main")
// tracks config.yaml at its root.
func RemoteTracksConfig(dir, remoteRef string) bool {
	cmd := exec.Command("git", "ls-tree", remoteRef, "--", "config.yaml")
	cmd.Dir = dir
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// HasLocalRootConfig reports whether dir contains a local config.yaml or
// ignores it via .gitignore (the signature of root scope).
func HasLocalRootConfig(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err == nil {
		return true
	}
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	return err == nil && gitignoreHasEntry(string(data), "config.yaml")
}

// EnsureConfigUntracked keeps skillshare's own config.yaml out of a root-scope
// repo: it ensures config.yaml is in .gitignore and, if the file is already
// tracked, removes it from the index with `git rm --cached` (the file stays on
// disk). Returns removed=true only when an actually-tracked config.yaml was
// untracked. This is the push-time safety net for repos created outside
// InitScopeRepo (manual `git_root: root` edits, externally-initialized repos, or
// a config.yaml committed before switching to root scope), where the init-time
// .gitignore guarantee does not apply.
func EnsureConfigUntracked(dir string) (removed bool, err error) {
	if err := ensureGitignoreEntry(dir, "config.yaml"); err != nil {
		return false, err
	}
	// A later "!config.yaml" (e.g. pulled from another machine) overrides the
	// entry, and `git add -A` would track the file again; the last match wins.
	if !isIgnored(dir, "config.yaml") {
		if err := appendGitignoreLine(dir, "config.yaml"); err != nil {
			return false, err
		}
	}
	if !isTracked(dir, "config.yaml") {
		return false, nil
	}
	cmd := exec.Command("git", "rm", "-r", "--cached", "--", "config.yaml")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("untrack config.yaml: %w", err)
	}
	return true, nil
}

// KeepLocalConfig snapshots a root-scope repo's config.yaml (a file or a
// symlink) before a pull. Git treats the ignored file as expendable, so a merge
// or reset that brings in a remote-tracked config.yaml replaces this machine's
// copy. The returned restore reports whether the upstream tracks config.yaml
// after the pull, even when the tracked copy matches this machine's, so callers
// can warn and suggest push to remove it. It puts the snapshot back only when
// the pull changed config.yaml's index entry or removed the file (an aborted
// merge that had brought it in deletes it again), so edits made while the
// pull ran to a file Git left alone are kept.
func KeepLocalConfig(dir string) (restore func() (remoteTracks bool, err error), err error) {
	path := filepath.Join(dir, "config.yaml")
	entryBefore := configIndexEntry(dir)
	put, err := snapshotConfig(path)
	if err != nil {
		return nil, err
	}
	return func() (bool, error) {
		remoteTracks := RemoteTracksConfig(dir, "@{u}")
		entryAfter := configIndexEntry(dir)
		_, statErr := os.Lstat(path)
		if entryAfter == entryBefore && !errors.Is(statErr, fs.ErrNotExist) {
			return remoteTracks, nil
		}
		if editedAfterCheckout(dir, entryAfter) {
			return remoteTracks, nil
		}
		_, err := put()
		return remoteTracks, err
	}, nil
}

// editedAfterCheckout reports whether config.yaml on disk differs from what
// the pull checked out (entry is its merged index entry), i.e. someone wrote a
// newer copy or link after Git did. Conflicts and removals report false so the
// snapshot is restored.
func editedAfterCheckout(dir, entry string) bool {
	fields := strings.Fields(entry)
	if len(fields) != 4 || fields[2] != "0" {
		return false
	}
	path := filepath.Join(dir, "config.yaml")
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	cmd := exec.Command("git", "hash-object", "--stdin")
	cmd.Dir = dir
	switch {
	case info.Mode()&fs.ModeSymlink != 0 && fields[0] == "120000":
		target, err := os.Readlink(path)
		if err != nil {
			return false
		}
		cmd.Stdin = strings.NewReader(target)
	case info.Mode()&fs.ModeSymlink != 0:
		return true // Git checks out a regular entry as a file, not a link
	case info.Mode().IsRegular() && (fields[0] == "100644" || fields[0] == "100755"):
		cmd = exec.Command("git", "hash-object", "--", "config.yaml")
		cmd.Dir = dir
	default:
		return false
	}
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) != fields[1]
}

// configIndexEntry returns config.yaml's index entries (mode, blob, stage), or
// "" when it is untracked. A merge, reset or conflict that touches the file
// changes this value.
func configIndexEntry(dir string) string {
	cmd := exec.Command("git", "ls-files", "-s", "--", "config.yaml")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// removeCheckedOut clears whatever the pull left at path so the snapshot can be
// written back. A pull can check out a tracked config.yaml/ directory there;
// its contents are in Git, so removing it loses nothing.
func removeCheckedOut(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err == nil && info.IsDir() {
		err = os.RemoveAll(path)
	} else if err == nil {
		err = os.Remove(path)
	}
	if err != nil {
		return fmt.Errorf("restore config.yaml: %w", err)
	}
	return nil
}

// snapshotConfig records the config.yaml at path and returns a put that writes
// it back, reporting whether it had changed. A missing file is a no-op.
func snapshotConfig(path string) (put func() (replaced bool, err error), err error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return func() (bool, error) { return false, nil }, nil
	}
	if err != nil {
		return nil, fmt.Errorf("snapshot config.yaml: %w", err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return nil, fmt.Errorf("snapshot config.yaml: %w", err)
		}
		return func() (bool, error) {
			if cur, err := os.Readlink(path); err == nil && cur == target {
				return false, nil
			}
			if err := removeCheckedOut(path); err != nil {
				return true, err
			}
			return true, os.Symlink(target, path)
		}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("snapshot config.yaml: %w", err)
	}
	return func() (bool, error) {
		if cur, err := os.Lstat(path); err == nil && cur.Mode().IsRegular() && cur.Mode().Perm() == info.Mode().Perm() {
			if got, err := os.ReadFile(path); err == nil && bytes.Equal(got, data) {
				return false, nil
			}
		}
		if err := removeCheckedOut(path); err != nil {
			return true, err
		}
		if err := os.WriteFile(path, data, info.Mode().Perm()); err != nil {
			return true, err
		}
		return true, os.Chmod(path, info.Mode().Perm()) // WriteFile's mode is masked by umask
	}, nil
}

// NestedRepos returns subdirectories of dir (relative paths, excluding dir
// itself) that contain their own .git. Git records such directories as gitlinks
// (submodules) on commit, silently dropping all of their files — so callers warn
// about them before staging a root-scope repo. The walk prunes at each match and
// skips .git / .git.disabled internals; results are sorted.
func NestedRepos(dir string) ([]string, error) {
	root := filepath.Clean(dir)
	found := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || !d.IsDir() {
			return nil //nolint:nilerr // skip unreadable entries, keep walking
		}
		if path == root {
			return nil // the scope repo's own .git is not "nested"
		}
		if name := d.Name(); name == ".git" || name == ".git.disabled" {
			return filepath.SkipDir
		}
		if _, statErr := os.Stat(filepath.Join(path, ".git")); statErr == nil {
			rel, _ := filepath.Rel(root, path)
			found = append(found, rel)
			return filepath.SkipDir // don't descend into a nested repo
		}
		return nil
	})
	sort.Strings(found)
	return found, err
}

// DisableNestedRepo neutralizes a nested git repo at dir/sub by renaming its
// .git to .git.disabled (reversible: rename back to re-enable) and ensuring
// .git.disabled is gitignored so the disabled internals are never committed.
// It refuses to clobber an existing .git.disabled.
func DisableNestedRepo(dir, sub string) error {
	gitDir := filepath.Join(dir, sub, ".git")
	disabled := filepath.Join(dir, sub, ".git.disabled")
	if _, err := os.Stat(gitDir); err != nil {
		return fmt.Errorf("%s has no nested .git", sub)
	}
	if _, err := os.Stat(disabled); err == nil {
		return fmt.Errorf("%s/.git.disabled already exists; resolve it manually", sub)
	}
	if err := os.Rename(gitDir, disabled); err != nil {
		return err
	}
	if err := ensureGitignoreEntry(dir, ".git.disabled"); err != nil {
		return err
	}
	// If the parent repo already committed <sub> as a gitlink (160000), renaming
	// .git is not enough: the stale gitlink stays in the index and `git add -A`
	// won't deconvert it. Drop the index entry so the directory's files are
	// re-tracked as blobs on the next add. Best-effort: a repo that never
	// committed the gitlink has no such entry.
	if IsRepo(dir) && isGitlink(dir, sub) {
		cmd := exec.Command("git", "rm", "--cached", "-q", "--", sub)
		cmd.Dir = dir
		_ = cmd.Run()
	}
	return nil
}

// isGitlink reports whether the repo at dir records sub as a gitlink (an
// embedded submodule pointer, index mode 160000) rather than tracked files.
func isGitlink(dir, sub string) bool {
	cmd := exec.Command("git", "ls-files", "-s", "--", sub)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(string(out)), "160000")
}
