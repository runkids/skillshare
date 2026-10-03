package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
// without relocating the repo. We print guidance and return an error so the
// caller aborts. When no stray repo is found we return the resolved directory and
// let the caller's existing repo check emit the standard "not a git repository"
// guidance (this also preserves the legacy case of a skills source nested inside
// an unrelated parent git repo).
func resolveGitRoot(cfg *config.Config, spinner *ui.Spinner) (string, error) {
	// Reject an unknown git_root before resolving: ScopeDir silently falls back
	// to the skills scope for any unrecognized value, which would make
	// commit/push/pull operate on the wrong repository without warning.
	if !config.ValidGitRoot(cfg.GitRoot) {
		spinner.Fail("Invalid git_root")
		ui.Note(fmt.Sprintf("git_root %q is not a valid scope", cfg.GitRoot))
		ui.Note(fmt.Sprintf("valid values: %s (or leave empty for skills)", strings.Join(config.ValidGitRoots, ", ")))
		return "", fmt.Errorf("invalid git_root %q", cfg.GitRoot)
	}

	root := cfg.EffectiveGitRoot()
	if scope, dir, mismatch := cfg.GitRootMismatch(); mismatch {
		configured := cfg.GitRoot
		if configured == "" {
			configured = "skills"
		}
		spinner.Fail("Git root mismatch")
		ui.Note(fmt.Sprintf("git_root operates on: %s", root))
		ui.Note(fmt.Sprintf("but the git repo lives at: %s (%s)", dir, scope))
		ui.Note("Fix it with one of:")
		ui.Note(fmt.Sprintf("  - skillshare init --git-root %s   (start a fresh repo at the configured scope)", configured))
		ui.Note(fmt.Sprintf("  - mv \"%s/.git\" \"%s/.git\"   (move the existing repo over, keeps history)", dir, root))
		ui.Note(fmt.Sprintf("  - set 'git_root: %s' in %s   (keep using the existing repo)", scope, config.ConfigPath()))
		return "", fmt.Errorf("git root mismatch (see guidance above)")
	}
	return root, nil
}

// rootSweepResult reports what the root-scope safety sweep changed or found.
type rootSweepResult struct {
	configUntracked bool     // config.yaml was (or, on dry-run, would be) untracked
	ignoreRepair    bool     // dry-run: .gitignore would gain a config.yaml rule
	nested          []string // subdirs (relative) that have their own .git
	dryRun          bool     // when true, the sweep made no changes (notice wording differs)
}

func (r rootSweepResult) hasNotice() bool {
	return r.configUntracked || r.ignoreRepair || len(r.nested) > 0
}

// previewChanges adjusts a dry run's `git status --porcelain` output for the
// .gitignore repair a real run makes first: config.yaml drops out and
// .gitignore is listed as modified.
func (r rootSweepResult) previewChanges(changes string) string {
	if !r.ignoreRepair {
		return changes
	}
	kept := []string{}
	hasGitignore := false
	for _, line := range strings.Split(changes, "\n") {
		path := strings.TrimSpace(strings.TrimLeft(line, "?MADRCU "))
		switch {
		case line == "", path == "config.yaml", strings.HasPrefix(path, "config.yaml/"):
			continue
		case path == ".gitignore":
			hasGitignore = true
		}
		kept = append(kept, line)
	}
	if !hasGitignore {
		kept = append(kept, " M .gitignore")
	}
	return strings.Join(kept, "\n")
}

// rootScopeSafetySweep guards a root-scope repo before staging. It detects
// nested git repos that would otherwise be committed as empty submodules and
// keeps skillshare's own config.yaml out of version control (the push-time net
// for repos not created via InitScopeRepo). It is a no-op (zero result) for
// non-root scopes.
//
// When dryRun is true the sweep is strictly read-only: it reports whether
// config.yaml WOULD be untracked (via IsConfigTracked) without writing
// .gitignore or running `git rm --cached`. On a real run the config mutation is
// best-effort — its failure is swallowed so the worst case is the prior
// behavior, never a blocked push. Nested-repo detection is always read-only.
// Push checks unpushed config history separately before this sweep: removing
// config.yaml from the index cannot remove it from earlier commits.
func rootScopeSafetySweep(cfg *config.Config, dir string, dryRun bool) rootSweepResult {
	res := rootSweepResult{dryRun: dryRun}
	if cfg.GitRoot != "root" {
		return res
	}
	if nested, err := gitops.NestedRepos(dir); err == nil {
		res.nested = nested
	}
	if dryRun {
		res.configUntracked = gitops.IsConfigTracked(dir)
		res.ignoreRepair = gitops.ConfigGitignoreNeedsRepair(dir)
	} else if removed, err := gitops.EnsureConfigUntracked(dir); err == nil {
		res.configUntracked = removed
	}
	return res
}

// printNotices reports the sweep outcome. Callers stop any active spinner first
// so the warning box renders cleanly.
func (r rootSweepResult) printNotices(dir string) {
	if r.configUntracked {
		if r.dryRun {
			fmt.Println("  Would remove config.yaml from version control")
			ui.Note("It holds machine-specific paths")
		} else {
			ui.Success("Removed config.yaml from version control")
			ui.Note("Kept on disk; it holds machine-specific paths")
		}
	}
	if r.ignoreRepair && !r.configUntracked {
		fmt.Println("  Would add config.yaml to .gitignore")
		ui.Note("It holds machine-specific paths")
	}
	if len(r.nested) > 0 {
		lines := []string{"These directories have their own .git and upload as EMPTY submodules:"}
		for _, sub := range r.nested {
			lines = append(lines, "  - "+sub)
		}
		lines = append(lines,
			"",
			"Their files will NOT be tracked. Disable each nested repo, then retry:",
			fmt.Sprintf("  mv \"%s/<dir>/.git\" \"%s/<dir>/.git.disabled\"", dir, dir),
			"(or use one-click disable on the web UI Git Sync page)",
		)
		ui.WarningBox("Nested git repositories found", lines...)
	}
}

// errNestedRepos blocks commit/push when a root-scope repo contains nested git
// repositories. They would upload as empty submodules (silent data loss), so
// the operation aborts until the user disables them. printNotices renders the
// per-directory remediation box; this carries only the short headline for the
// CLI's error line and a non-zero exit code.
var errNestedRepos = fmt.Errorf("nested git repositories must be disabled first (see guidance above)")
