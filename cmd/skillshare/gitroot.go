package main

import (
	"os"
	"path/filepath"

	"skillshare/internal/config"
	"skillshare/internal/ui"
)

// hasGitDir reports whether dir contains a .git entry directly (not via a parent).
func hasGitDir(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// resolveGitRoot returns the directory commit/push/pull should operate on.
//
// Option (a) mismatch handling: if the configured scope has no repo directly at
// its directory but another scope's directory does, the user changed git_root
// without relocating the repo. We print guidance and return ok=false so the
// caller aborts. When no stray repo is found we return the resolved directory and
// let the caller's existing repo check emit the standard "not a git repository"
// guidance (this also preserves the legacy case of a skills source nested inside
// an unrelated parent git repo).
func resolveGitRoot(cfg *config.Config, spinner *ui.Spinner) (string, bool) {
	root := cfg.EffectiveGitRoot()
	if scope, dir, mismatch := cfg.GitRootMismatch(); mismatch {
		spinner.Fail("Git root mismatch")
		ui.Info("  git_root operates on: %s", root)
		ui.Info("  but the git repo lives at: %s (%s)", dir, scope)
		ui.Info("  To version the configured scope, either:")
		ui.Info("    - run 'git init' in %s (starts a fresh history), or", root)
		ui.Info("    - move the repo: mv %s/.git %s/.git (keeps history)", dir, root)
		ui.Info("  Or set 'git_root: %s' in %s to match the existing repo.", scope, config.ConfigPath())
		return "", false
	}
	return root, true
}
