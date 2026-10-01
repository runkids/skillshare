package git

import (
	"fmt"
	"os/exec"
)

// DiscardChanges restores tracked files and the index to HEAD, and removes
// untracked files. Ignored files and nested repositories are kept. At root
// scope, the machine-specific config.yaml is also kept, even if tracked.
func DiscardChanges(dir string, preserveConfig bool) error {
	paths := []string{"."}
	cleanArgs := []string{"clean", "-fd"}
	if preserveConfig {
		paths = append(paths, ":(top,exclude)config.yaml")
		cleanArgs = append(cleanArgs, "-e", "/config.yaml")
	}
	changed := false
	// Check both diffs: staged and unstaged edits can cancel each other out.
	// Skip restore when neither has changes, including an empty initial commit.
	for _, args := range [][]string{{"diff", "--name-only", "HEAD"}, {"diff", "--cached", "--name-only", "HEAD"}} {
		cmd := exec.Command("git", append(append(args, "--"), paths...)...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to check changes: %w", err)
		}
		changed = changed || len(out) > 0
	}
	// Clean first, while the local .gitignore still defines ignored files.
	// A single -f deliberately preserves nested repositories.
	commands := [][]string{append(cleanArgs, "--", ".")}
	if changed {
		commands = append(commands, append([]string{"restore", "--source=HEAD", "--staged", "--worktree", "--no-recurse-submodules", "--"}, paths...))
	}
	for _, args := range commands {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to discard changes: %w: %s", err, out)
		}
	}
	return nil
}
