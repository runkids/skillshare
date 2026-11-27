package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
	ssync "skillshare/internal/sync"
	"skillshare/internal/ui"
)

// projectSkillsWalk returns a fresh traversal policy for one operation on a
// project's skills source, for commands that do not load a projectRuntime.
func projectSkillsWalk(root string, cfg *config.ProjectConfig) sourcewalk.Options {
	targets, _ := config.ResolveValidProjectTargets(root, cfg)
	return config.SkillsWalk(cfg.FollowSourceLinks, cfg.EffectiveSkillsSource(root), targets)
}

// followedRepoSkipped reports whether a _-prefixed directory met by an
// update --all walk sits behind a followed source link, which is the user's
// own checkout rather than a clone skillshare manages: --all must not pull,
// reset or audit-rollback it. The warning is non-empty only for the link
// entry itself; the walk skips the directory either way.
func followedRepoSkipped(walk sourcewalk.Options, walkRoot, path string) (warning string, skip bool) {
	if _, followed := walk.Follow.Resolve(path); !followed || !install.IsGitRepo(path) {
		return "", false
	}
	rel, err := filepath.Rel(walkRoot, path)
	if err != nil || strings.ContainsRune(rel, filepath.Separator) {
		return "", true
	}
	return fmt.Sprintf("%s: followed source link, not updated by --all; run `skillshare update %s` to pull that checkout", rel, rel), true
}

func printSkippedSourceLinkWarnings(walk sourcewalk.Options, jsonOutput bool) {
	printWarningLines(ssync.SourceLinkWarnings(walk, false), jsonOutput)
}

func printWarningLines(warnings []string, jsonOutput bool) {
	for _, warning := range warnings {
		if jsonOutput {
			fmt.Fprintf(os.Stderr, "! %s\n", warning)
		} else {
			ui.Warning("%s", warning)
		}
	}
}
