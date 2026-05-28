package git

import (
	"os"
	"os/exec"
	"path/filepath"
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
