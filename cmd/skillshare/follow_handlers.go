package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	gitops "skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/skillfollow"
	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
	ssync "skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

type followResult struct {
	Name           string           `json:"name"`
	Source         string           `json:"source"`
	File           string           `json:"file"`
	Added          bool             `json:"added"`
	Link           string           `json:"link"`
	LinkCreated    bool             `json:"link_created"`
	LinkTarget     string           `json:"link_target,omitempty"`
	IgnoreFile     string           `json:"ignore_file,omitempty"`
	IgnoreLines    []string         `json:"ignore_lines_added"`
	UntrackCommand string           `json:"untrack_command,omitempty"`
	State          sourcewalk.State `json:"state"`
	Reason         string           `json:"reason"`
	ResolvedTarget string           `json:"resolved_target,omitempty"`
}

func (r *followResult) changed() bool {
	return r.Added || r.LinkCreated || len(r.IgnoreLines) > 0
}

type unfollowResult struct {
	Name              string   `json:"name"`
	Source            string   `json:"source"`
	FilesEdited       []string `json:"files_edited"`
	StillDeclaredIn   []string `json:"still_declared_in"`
	NotDeclared       bool     `json:"not_declared"`
	Link              string   `json:"link"`
	LinkRemoved       bool     `json:"link_removed"`
	LinkKept          string   `json:"link_kept,omitempty"`
	IgnoreLine        string   `json:"ignore_line"`
	IgnoreLineRemoved bool     `json:"ignore_line_removed"`
	IgnoreLineKept    bool     `json:"ignore_line_kept"`
}

func (r *unfollowResult) changed() bool {
	return len(r.FilesEdited) > 0 || r.LinkRemoved || r.IgnoreLineRemoved
}

// leftDiscovery reports whether the entry's skills are no longer discovered,
// so the next sync prunes their managed links.
func (r *unfollowResult) leftDiscovery() bool {
	return len(r.FilesEdited) > 0 && len(r.StillDeclaredIn) == 0 && r.LinkKept != linkKeptRealDir
}

const linkKeptRealDir = "not a link; a real directory is never removed"

// sameFollowName matches a declared name the way discovery does. Tests swap
// it to cover Windows semantics.
var sameFollowName = sourcewalk.SameEntryName

// runFollow declares opts.name, creating its link first when opts.to is set,
// and adds the ignore lines a Git source needs.
func runFollow(fc *followContext, opts followOptions) (*followResult, error) {
	root, err := sourcefs.Open(fc.source)
	if err != nil {
		return nil, fmt.Errorf("cannot open skills source: %w", err)
	}
	defer root.Close()

	file := skillfollow.File
	if opts.local {
		file = skillfollow.LocalFile
	}
	r := &followResult{Name: opts.name, Source: fc.source, File: file, Link: filepath.Join(fc.source, opts.name)}
	target, created, err := prepareFollowLink(fc.source, r.Link, opts.to)
	if err != nil {
		return nil, err
	}
	r.LinkCreated, r.LinkTarget = created, target

	added, err := skillfollow.AddEntry(root, file, opts.name)
	if err != nil {
		err = fmt.Errorf("failed to update %s: %w", file, err)
		if created {
			if unlinkErr := root.Unlink(opts.name); unlinkErr != nil {
				err = errors.Join(err, fmt.Errorf("the new link %s was left in place: %w", r.Link, unlinkErr))
			}
		}
		return nil, err
	}
	r.Added = added

	snapshot := fc.snapshot()
	if err := followIgnores(root, fc.source, snapshot, opts, r); err != nil {
		return nil, fmt.Errorf("%s declares %s, but the Git ignore lines were not written: %w", file, opts.name, err)
	}
	if snapshot != nil {
		for _, entry := range snapshot.Entries() {
			if sameFollowName(entry.Name, opts.name) {
				r.State, r.Reason, r.ResolvedTarget = entry.State, entry.Reason, entry.ResolvedTarget
			}
		}
	}
	return r, nil
}

// prepareFollowLink checks the entry and, with to set, creates the link (a
// junction on Windows). It returns the absolute target and whether it
// created the link.
func prepareFollowLink(source, link, to string) (string, bool, error) {
	info, lerr := os.Lstat(link)
	if lerr != nil && !errors.Is(lerr, fs.ErrNotExist) {
		return "", false, lerr
	}
	if to == "" {
		if lerr != nil {
			return "", false, fmt.Errorf("%s does not exist; pass --to <dir> to create the link", link)
		}
		if !utils.IsLinkMode(link, info.Mode()) && !info.IsDir() {
			return "", false, fmt.Errorf("%s is neither a link nor a directory", link)
		}
		return "", false, nil
	}

	target, err := filepath.Abs(to)
	if err != nil {
		return "", false, err
	}
	if st, err := os.Stat(target); err != nil {
		return "", false, fmt.Errorf("--to: %w", err)
	} else if !st.IsDir() {
		return "", false, fmt.Errorf("--to: %s is not a directory", target)
	}
	canonTarget, err := sourcewalk.Canonicalize(target)
	if err != nil {
		return "", false, err
	}
	canonSource, err := sourcewalk.Canonicalize(source)
	if err != nil {
		return "", false, err
	}
	if utils.PathsEqual(canonTarget, canonSource) || utils.PathHasPrefix(canonTarget, strings.TrimRight(canonSource, string(filepath.Separator))+string(filepath.Separator)) {
		return "", false, fmt.Errorf("--to: %s is inside the skills source", target)
	}
	if lerr == nil {
		if utils.IsLinkMode(link, info.Mode()) {
			if current, err := utils.ResolveLinkTarget(link); err == nil {
				if canonCurrent, err := sourcewalk.Canonicalize(current); err == nil && utils.PathsEqual(canonCurrent, canonTarget) {
					return target, false, nil
				}
			}
		}
		return "", false, fmt.Errorf("%s already exists and does not link to %s; remove it, or run follow without --to", link, target)
	}
	if err := ssync.CreateSymlink(link, target, ""); err != nil {
		return "", false, err
	}
	return target, true, nil
}

// followIgnores adds the anchored link line, and with --local the
// .skillfollow.local line, to the file doctor and commit name. An indexed
// link is never untracked; the result carries the command to do it.
func followIgnores(root *sourcefs.Root, source string, snapshot *sourcewalk.FollowSet, opts followOptions, r *followResult) error {
	if snapshot == nil || !gitops.IsRepo(source) {
		return nil
	}
	links, err := gitops.FollowedLinksStaged(source, snapshot)
	if err != nil {
		return err
	}
	for _, link := range links {
		if !sameFollowName(filepath.Base(link.Path), opts.name) {
			continue
		}
		line := link.IgnoreLine
		r.IgnoreFile = link.IgnoreFile
		indexed, err := gitops.IsPathIndexed(source, link.Path)
		if err != nil {
			return err
		}
		ignored, err := gitops.IsPathIgnored(source, link.Path)
		if err != nil {
			return err
		}
		if !ignored {
			if added, err := skillfollow.AddIgnoreLine(root, line); err != nil {
				return err
			} else if added {
				r.IgnoreLines = append(r.IgnoreLines, line)
			}
		}
		if indexed {
			r.UntrackCommand = gitops.UntrackCommandIn(source, link.Path)
		}
	}
	if opts.local {
		ignored, err := gitops.IsPathIgnored(source, skillfollow.LocalFile)
		if err != nil {
			return err
		}
		if !ignored {
			if added, err := skillfollow.AddIgnoreLine(root, skillfollow.LocalIgnoreLine); err != nil {
				return err
			} else if added {
				r.IgnoreLines = append(r.IgnoreLines, skillfollow.LocalIgnoreLine)
			}
		}
		if r.IgnoreFile == "" {
			r.IgnoreFile = filepath.Join(snapshot.SourceRoot(), skillfollow.IgnoreFile)
		}
	}
	return nil
}

// runUnfollow removes opts.name from the declaration files, then the link
// itself and its ignore line. A write failure is returned as a failure even
// when an earlier file was already edited.
func runUnfollow(fc *followContext, opts followOptions) (*unfollowResult, error) {
	root, err := sourcefs.Open(fc.source)
	if err != nil {
		return nil, fmt.Errorf("cannot open skills source: %w", err)
	}
	defer root.Close()

	r := &unfollowResult{Name: opts.name, Source: fc.source, Link: filepath.Join(fc.source, opts.name), IgnoreLine: install.FollowedIgnoreLine(opts.name)}
	files := skillfollow.Files
	if opts.local {
		files = []string{skillfollow.LocalFile}
	}
	declared := ""
	for _, file := range files {
		removed, err := skillfollow.RemoveEntry(root, file, opts.name)
		if err != nil {
			err = fmt.Errorf("failed to update %s: %w", file, err)
			if len(r.FilesEdited) > 0 {
				err = fmt.Errorf("%w; %s was already edited, and %s is still declared", err, strings.Join(r.FilesEdited, ", "), opts.name)
			}
			return nil, err
		}
		if removed != "" {
			r.FilesEdited = append(r.FilesEdited, file)
			if declared == "" {
				declared = removed
			}
		}
	}
	// Follow wrote the ignore line in the declared spelling.
	if declared != "" {
		r.IgnoreLine = install.FollowedIgnoreLine(declared)
	}
	if opts.local {
		declared, err := skillfollow.Declares(root, skillfollow.File, opts.name)
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", skillfollow.File, err)
		}
		if declared {
			r.StillDeclaredIn = []string{skillfollow.File}
		}
	}
	if len(r.StillDeclaredIn) > 0 {
		r.LinkKept = "still declared in " + strings.Join(r.StillDeclaredIn, ", ")
	} else if len(r.FilesEdited) == 0 {
		r.NotDeclared = true
	}
	if r.LinkKept != "" || r.NotDeclared {
		return r, nil
	}

	info, err := root.Lstat(opts.name)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, fmt.Errorf("removed %s from %s, but cannot inspect the link: %w", opts.name, strings.Join(r.FilesEdited, ", "), err)
	case opts.keepLink:
		r.LinkKept = "--keep-link"
	case utils.IsLinkMode(r.Link, info.Mode()):
		if err := root.Unlink(opts.name); err != nil {
			return nil, fmt.Errorf("removed %s from %s, but the link was not removed: %w", opts.name, strings.Join(r.FilesEdited, ", "), err)
		}
		r.LinkRemoved = true
	default:
		r.LinkKept = linkKeptRealDir
	}

	if r.LinkRemoved {
		removed, err := skillfollow.RemoveIgnoreLine(root, r.IgnoreLine)
		if typed := install.FollowedIgnoreLine(opts.name); err == nil && !removed && typed != r.IgnoreLine {
			if removed, err = skillfollow.RemoveIgnoreLine(root, typed); removed {
				r.IgnoreLine = typed
			}
		}
		if err != nil {
			return nil, fmt.Errorf("removed %s and its link, but not its ignore line: %w", opts.name, err)
		}
		r.IgnoreLineRemoved = removed
	} else if r.LinkKept != "" {
		kept, err := install.GitignoreContains(filepath.Join(fc.source, skillfollow.IgnoreFile), r.IgnoreLine)
		if err != nil {
			return nil, err
		}
		r.IgnoreLineKept = kept
	}
	return r, nil
}

func renderFollow(r *followResult) {
	width := ui.RowWidth(r.Name, skillfollow.IgnoreFile)
	if r.LinkCreated {
		ui.Row(ui.MarkOK, r.Name, "linked to "+r.LinkTarget, width)
	}
	if r.Added {
		ui.Row(ui.MarkOK, r.Name, "added to "+r.File, width)
	} else {
		ui.Row(ui.MarkWarn, r.Name, "already in "+r.File, width)
	}
	for _, line := range r.IgnoreLines {
		ui.Row(ui.MarkOK, skillfollow.IgnoreFile, "added "+line, width)
	}
	if r.UntrackCommand != "" {
		ui.Row(ui.MarkWarn, r.Name, "indexed in Git; run "+r.UntrackCommand, width)
	}
	if r.State != "" {
		mark := ui.MarkOK
		if r.State != sourcewalk.Followed && r.State != sourcewalk.NotLink {
			mark = ui.MarkWarn
		}
		ui.Row(mark, r.Name, string(r.State)+" — "+r.Reason, width)
	}
}

func renderUnfollow(r *unfollowResult) {
	width := ui.RowWidth(r.Name, skillfollow.IgnoreFile)
	for _, file := range r.FilesEdited {
		ui.Row(ui.MarkOK, r.Name, "removed from "+file, width)
	}
	if r.NotDeclared {
		ui.Row(ui.MarkWarn, r.Name, "not declared; nothing changed", width)
	}
	for _, file := range r.StillDeclaredIn {
		ui.Row(ui.MarkWarn, r.Name, "still declared in "+file+"; it remains followed", width)
	}
	if r.LinkRemoved {
		ui.Row(ui.MarkOK, r.Name, "link removed; its target was not touched", width)
	} else if r.LinkKept != "" && len(r.StillDeclaredIn) == 0 {
		ui.Row(ui.MarkNone, r.Name, "link kept: "+r.LinkKept, width)
	}
	if r.IgnoreLineRemoved {
		ui.Row(ui.MarkOK, skillfollow.IgnoreFile, "removed "+r.IgnoreLine, width)
	}
	if r.IgnoreLineKept {
		ui.Row(ui.MarkNone, skillfollow.IgnoreFile, "kept "+r.IgnoreLine, width)
	}
}
