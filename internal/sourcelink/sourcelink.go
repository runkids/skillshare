// Package sourcelink creates and removes the first-level links under the
// skills source that follow_source_links follows. The CLI and the dashboard
// both call it, so a link is refused for the same reasons a walk would skip it.
package sourcelink

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/utils"
)

// Result describes a created link.
type Result struct {
	Path    string // the link, <source>/<name>
	Target  string // absolute path the link points at
	Kind    string // "symlink" or "junction"
	Warning string // non-empty when the target is not a git checkout
}

// DefaultName is the link name used when none is given: "_" and the target's
// base name.
func DefaultName(target string) string {
	return "_" + filepath.Base(filepath.Clean(target))
}

// Create links <source>/<name> to target, a directory. targets are the skills
// paths of the active sync targets. An empty name means DefaultName. It
// refuses a target a walk would not follow (see sourcewalk.Follow.Allow) and
// a name that already exists. On Windows it makes a junction, or a directory
// symlink when a junction cannot be made; elsewhere an absolute symlink.
func Create(source string, targets []string, target, name string) (Result, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return Result{}, err
	}
	if name == "" {
		name = DefaultName(abs)
	}
	if err := checkName(name); err != nil {
		return Result{}, err
	}
	r, err := sourcefs.Open(source)
	if err != nil {
		return Result{}, err
	}
	defer r.Close()
	if _, err := r.Lstat(name); err == nil {
		return Result{}, fmt.Errorf("%s already exists", name)
	} else if !os.IsNotExist(err) {
		return Result{}, err
	}
	if err := sourcewalk.NewFollow(source, targets).Allow(abs); err != nil {
		return Result{}, err
	}
	link := filepath.Join(source, name)
	if err := sync.CreateSymlink(link, abs, ""); err != nil {
		return Result{}, err
	}
	res := Result{Path: link, Target: abs, Kind: "junction"}
	if info, err := os.Lstat(link); err == nil && info.Mode()&os.ModeSymlink != 0 {
		res.Kind = "symlink"
	}
	if _, err := os.Stat(filepath.Join(abs, ".git")); err != nil {
		res.Warning = "target is not a git checkout"
	}
	return res, nil
}

// Remove moves the link <source>/<name> to trashDir, the way uninstall
// removes a first-level link: only the link entry moves, its target stays.
// It returns the moved entry's trash path and refuses a non-first-level link.
func Remove(source, trashDir, name string) (string, error) {
	name = strings.TrimRight(name, `/\`)
	if err := checkName(name); err != nil {
		return "", err
	}
	link := filepath.Join(source, name)
	info, err := os.Lstat(link)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("%s not found in source", name)
	} else if err != nil {
		return "", err
	}
	if !utils.IsLinkMode(link, info.Mode()) {
		return "", fmt.Errorf("%s is not a link", name)
	}
	if err := sourcefs.CheckMoveOut(source, link); err != nil {
		return "", err
	}
	trashPath, err := trash.MoveToTrash(link, name, trashDir)
	if err != nil {
		return "", fmt.Errorf("failed to move to trash: %w", err)
	}
	return trashPath, nil
}

// checkName refuses a name that is not a single entry directly under the
// source.
func checkName(name string) error {
	if name == "" || name == "." || name == ".." || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%q is not a first-level name", name)
	}
	return nil
}

// Discard removes a link this operation just created, for rolling back when
// a later step fails. It goes through the source handle so only the link
// entry goes and its target is never touched.
func Discard(source, name string) error {
	root, err := sourcefs.Open(source)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.Unlink(name)
}
