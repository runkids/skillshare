package main

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/config"
	gitops "skillshare/internal/git"
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
		configured := cfg.GitRoot
		if configured == "" {
			configured = "skills"
		}
		spinner.Fail("Git root mismatch")
		ui.Info("  git_root operates on: %s", root)
		ui.Info("  but the git repo lives at: %s (%s)", dir, scope)
		ui.Info("  Fix it with one of:")
		ui.Info("    - skillshare init --git-root %s   (start a fresh repo at the configured scope)", configured)
		ui.Info("    - mv %s/.git %s/.git   (move the existing repo over, keeps history)", dir, root)
		ui.Info("    - set 'git_root: %s' in %s   (keep using the existing repo)", scope, config.ConfigPath())
		return "", false
	}
	return root, true
}

// rootSweepResult reports what the root-scope safety sweep changed or found.
type rootSweepResult struct {
	configUntracked bool     // config.yaml was removed from version control
	nested          []string // subdirs (relative) that have their own .git
}

func (r rootSweepResult) hasNotice() bool {
	return r.configUntracked || len(r.nested) > 0
}

// rootScopeSafetySweep guards a root-scope repo before staging. It keeps
// skillshare's own config.yaml out of version control (the push-time net for
// repos not created via InitScopeRepo) and detects nested git repos that would
// otherwise be committed as empty submodules. It is a no-op (zero result) for
// non-root scopes. Mutation failures are swallowed: the worst case is the prior
// behavior, never a blocked push.
func rootScopeSafetySweep(cfg *config.Config, dir string) rootSweepResult {
	var res rootSweepResult
	if cfg.GitRoot != "root" {
		return res
	}
	if removed, err := gitops.EnsureConfigUntracked(dir); err == nil {
		res.configUntracked = removed
	}
	if nested, err := gitops.NestedRepos(dir); err == nil {
		res.nested = nested
	}
	return res
}

// printNotices reports the sweep outcome. Callers stop any active spinner first
// so the warning box renders cleanly.
func (r rootSweepResult) printNotices(dir string) {
	if r.configUntracked {
		ui.Info("Removed config.yaml from version control (kept on disk; it holds machine-specific paths)")
	}
	if len(r.nested) > 0 {
		lines := []string{"These directories have their own .git and upload as EMPTY submodules:"}
		for _, sub := range r.nested {
			lines = append(lines, "  - "+sub)
		}
		lines = append(lines,
			"",
			"Their files will NOT be tracked. Disable each nested repo, then push again:",
			fmt.Sprintf("  mv %s/<dir>/.git %s/<dir>/.git.disabled", dir, dir),
			"(or use one-click disable on the web UI Git Sync page)",
		)
		ui.WarningBox("Nested git repositories found", lines...)
	}
}
