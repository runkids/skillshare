// Package projectdir resolves the directory that marks a skillshare project.
//
// A project root is identified by a config.yaml inside a recognized directory.
// ".skillshare" is the default. "skillshare" is an alternative for repositories
// that treat skills as reviewable content rather than tool state.
//
// The hidden name is always resolved first, so a repository that already has
// .skillshare/ keeps working unchanged even if a visible directory also exists.
//
// This package has no dependencies on other internal packages so that every
// layer (config, audit, trash, backup, oplog, CLI, server) can resolve project
// paths the same way.
package projectdir

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const (
	// Default is the directory name used by "skillshare init -p".
	Default = ".skillshare"

	// Visible is the directory name used by "skillshare init -p --visible".
	Visible = "skillshare"

	// ConfigFileName is the file whose presence marks a project directory.
	ConfigFileName = "config.yaml"
)

// names lists recognized project directory names in resolution order.
var names = []string{Default, Visible}

// Names returns the recognized project directory names in resolution order.
func Names() []string {
	return append([]string(nil), names...)
}

// IsName reports whether name is a recognized project directory name.
func IsName(name string) bool {
	for _, candidate := range names {
		if name == candidate {
			return true
		}
	}
	return false
}

// Find returns the absolute path of the project directory under projectRoot
// that holds a config.yaml. The second return value is false when projectRoot
// is not an initialized project.
func Find(projectRoot string) (string, bool) {
	for _, name := range names {
		dir := filepath.Join(projectRoot, name)
		if info, err := os.Stat(filepath.Join(dir, ConfigFileName)); err == nil && !info.IsDir() {
			return dir, true
		}
	}
	return "", false
}

// FindPartial returns a project directory that exists but holds no config.yaml,
// which is how a shared skills repository looks after a clone that gitignored
// the config. Only the hidden default is accepted on name alone; the visible
// name must also carry a marker of its own, so an unrelated directory called
// "skillshare" is never mistaken for a project.
//
// Two markers count: a skills/ or agents/ directory, and a .gitignore that
// ignores config.yaml. The second is needed because a repository shared before
// any skill was committed clones with nothing else — git carries the .gitignore
// but drops the empty skills/ and agents/ directories.
func FindPartial(projectRoot string) (string, bool) {
	for _, name := range names {
		dir := filepath.Join(projectRoot, name)
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		if name == Default || hasResourceDir(dir) || hasGitignoredConfig(dir) {
			return dir, true
		}
	}
	return "", false
}

func hasResourceDir(dir string) bool {
	for _, name := range []string{"skills", "agents"} {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// hasGitignoredConfig reports whether dir has a .gitignore listing config.yaml,
// the marker written by "init -p --config local".
//
// This repeats what install.GitignoreContains does for that one pattern rather
// than calling it, because this package must not depend on other internal
// packages. An unreadable .gitignore is treated as absent: the marker only ever
// widens what counts as a project, so failing to read it leaves the directory
// unclaimed rather than claiming it wrongly.
func hasGitignoredConfig(dir string) bool {
	f, err := os.Open(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == ConfigFileName {
			return true
		}
	}
	return false
}

// Resolve returns the project directory to use for projectRoot. An initialized
// project resolves to its own directory; anything else resolves to the default
// name so new projects keep the current layout.
func Resolve(projectRoot string) string {
	if dir, ok := Find(projectRoot); ok {
		return dir
	}
	if dir, ok := FindPartial(projectRoot); ok {
		return dir
	}
	return filepath.Join(projectRoot, Default)
}

// ConfigPath returns the project config path for projectRoot.
func ConfigPath(projectRoot string) string {
	return filepath.Join(Resolve(projectRoot), ConfigFileName)
}

// Name returns the directory name to create for a new project.
func Name(visible bool) string {
	if visible {
		return Visible
	}
	return Default
}
