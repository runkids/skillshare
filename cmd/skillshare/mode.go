package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/projectdir"
	"skillshare/internal/ui"
)

type runMode int

const (
	modeAuto runMode = iota
	modeProject
	modeGlobal
)

func parseModeArgs(args []string) (runMode, []string, error) {
	mode := modeAuto
	rest := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project", "-p":
			if mode == modeGlobal {
				return modeAuto, nil, fmt.Errorf("--project and --global are mutually exclusive")
			}
			mode = modeProject
		case "--global", "-g":
			if mode == modeProject {
				return modeAuto, nil, fmt.Errorf("--project and --global are mutually exclusive")
			}
			mode = modeGlobal
		default:
			rest = append(rest, args[i])
		}
	}

	return mode, rest, nil
}

// applyModeLabel sets ui.ModeLabel when in project mode.
// Call after mode is resolved and before any UI output.
func applyModeLabel(mode runMode) {
	if mode == modeProject {
		ui.ModeLabel = "project"
	}
}

func modeString(mode runMode) string {
	if mode == modeProject {
		return "project"
	}
	return "global"
}

func projectConfigExists(root string) bool {
	_, ok := projectdir.Find(root)
	return ok
}

// ensureProjectConfig initializes a project when that is safe, and reports the
// case where initializing would discard an existing project instead.
//
// Auto-init is intentional in two situations and both are preserved: a
// repository with no project directory at all, and the shared skills repo
// produced by 'init -p --config local', which gitignores config.yaml so every
// developer regenerates their own.
//
// The case in between used to be silent. A project directory that already holds
// skills or agents, whose config.yaml is simply gone, was re-initialized into an
// empty config — dropping every configured target, leaving stale target links,
// and exiting 0.
func ensureProjectConfig(root string) error {
	if projectConfigExists(root) {
		return nil
	}
	if dir, ok := projectdir.FindPartial(root); ok {
		gitignored, err := isConfigGitignored(dir)
		if (err != nil || !gitignored) && projectDirHasContent(dir) {
			return fmt.Errorf(
				"%s/ in %s holds skills or agents but has no %s\n"+
					"Restore it from version control, or run 'skillshare init -p' to write a new one",
				filepath.Base(dir), root, projectdir.ConfigFileName)
		}
	}
	return performProjectInit(root, projectInitOptions{})
}

// projectDirHasContent reports whether the project directory holds authored
// skills or agents that a regenerated config would no longer describe.
func projectDirHasContent(dir string) bool {
	for _, name := range []string{"skills", "agents"} {
		entries, err := os.ReadDir(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), ".") {
				return true
			}
		}
	}
	return false
}

// ensureProjectConfig initializes a project when that is safe, and reports the
// case where initializing would discard an existing project instead.
//
// Auto-init is intentional in two situations and both are preserved: a
// repository with no .skillshare/ at all, and the shared skills repo produced
// by 'init -p --config local', which gitignores config.yaml so every developer
// regenerates their own.
//
// The case in between used to be silent. A .skillshare/ that already holds
// skills or agents, whose config.yaml is simply gone, was re-initialized into
// an empty config — dropping every configured target, leaving stale target
// links, and exiting 0.
func ensureProjectConfig(root string) error {
	if projectConfigExists(root) {
		return nil
	}
	dir := filepath.Join(root, ".skillshare")
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		// isConfigGitignored reports false whenever it cannot tell, so an
		// unreadable .gitignore falls through to the guard rather than the
		// shared-repo path.
		gitignored, _ := isConfigGitignored(root)
		if !gitignored && projectDirHasContent(dir) {
			return fmt.Errorf(
				".skillshare/ in %s holds skills or agents but has no config.yaml\n"+
					"Restore it from version control, or run 'skillshare init -p' to write a new one",
				root)
		}
	}
	return performProjectInit(root, projectInitOptions{})
}

// projectDirHasContent reports whether .skillshare/ holds authored skills or
// agents that a regenerated config would no longer describe.
func projectDirHasContent(dir string) bool {
	for _, name := range []string{"skills", "agents"} {
		entries, err := os.ReadDir(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), ".") {
				return true
			}
		}
	}
	return false
}
