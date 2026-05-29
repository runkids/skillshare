package git

import (
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
	if !isTracked(dir, "config.yaml") {
		return false, nil
	}
	cmd := exec.Command("git", "rm", "--cached", "--", "config.yaml")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("untrack config.yaml: %w", err)
	}
	return true, nil
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
