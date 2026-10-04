package sourcewalk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// State is the first applicable classification rule for an entry.
type State string

const (
	Missing        State = "missing"
	NotLink        State = "not-link"
	InvalidTarget  State = "invalid-target"
	Cycle          State = "cycle"
	TargetOverlap  State = "target-overlap"
	InsideGitRoot  State = "inside-git-root"
	EntryOverlap   State = "entry-overlap"
	SingleSkill    State = "single-skill"
	Followed       State = "followed"
	UndeclaredLink State = "undeclared-link"
)

// Entry describes a logical first-level name and its classification.
type Entry struct {
	Name           string `json:"name"`
	State          State  `json:"state"`
	ResolvedTarget string `json:"resolved_target,omitempty"`
	Reason         string `json:"reason"`
}

// FollowOptions supplies the operation's active skills targets and staging root.
type FollowOptions struct {
	TargetPaths []string
	GitRoot     string
}

// FollowSet is a read-only filesystem snapshot with mutable discovery states.
// Walkers using its pointer record read failures in the same set. It is intended
// for one operation, and must not be mutated concurrently with traversal.
type FollowSet struct {
	walkErrors    []error
	root          string
	canonicalRoot string
	entries       []Entry
	parsed        []string
	warnings      []string
	active        bool
	local         bool
}

// Follow parses the root declarations and classifies all declared entries and
// undeclared first-level links. It never writes to the filesystem.
func Follow(root string, opts FollowOptions) FollowSet { return follow(root, opts, platformLinks()) }

func follow(root string, opts FollowOptions, links linkOps) FollowSet {
	parsed := readDeclarations(root)
	set := FollowSet{root: filepath.Clean(root), parsed: parsed.names, warnings: parsed.warnings, active: parsed.active, local: parsed.local}
	canon := func(path string) (string, error) { return canonicalize(path, links, 0) }
	canonicalRoot, rootErr := canon(root)
	set.canonicalRoot = canonicalRoot
	var targets []string
	var optionsErr error
	for _, path := range opts.TargetPaths {
		target, err := canon(path)
		if err != nil {
			optionsErr = err
			break
		}
		targets = append(targets, target)
	}
	gitRoot := ""
	if opts.GitRoot != "" {
		var err error
		gitRoot, err = canon(opts.GitRoot)
		if err != nil {
			optionsErr = err
		}
	}
	declared := make(map[string]bool)
	for _, name := range parsed.names {
		declared[name] = true
		entry := Entry{Name: name}
		path := filepath.Join(root, name)
		info, err := os.Lstat(path)
		switch {
		case err != nil:
			entry.State, entry.Reason = Missing, err.Error()
		case !links.isLink(path, info.Mode()):
			if info.IsDir() {
				entry.State, entry.Reason = NotLink, "real directory; discovered normally"
			} else {
				entry.State, entry.Reason = InvalidTarget, "entry is neither a link nor a directory"
			}
		default:
			target, err := links.resolve(path)
			if err == nil {
				target, err = canon(target)
			}
			if err != nil {
				entry.State, entry.Reason = Missing, err.Error()
				break
			}
			entry.ResolvedTarget = target
			targetInfo, err := os.Stat(target)
			if err != nil {
				entry.State, entry.Reason = Missing, err.Error()
				break
			}
			if !targetInfo.IsDir() {
				entry.State, entry.Reason = InvalidTarget, "link target is not a directory"
				break
			}
			if _, err := readFollowDir(target); err != nil {
				entry.State, entry.Reason = Missing, err.Error()
				break
			}
			if rootErr != nil {
				entry.State, entry.Reason = Missing, rootErr.Error()
				break
			}
			if overlaps(canonicalRoot, target) {
				entry.State, entry.Reason = Cycle, "target overlaps the source root"
				break
			}
			if optionsErr != nil {
				entry.State, entry.Reason = Missing, "cannot canonicalize safety boundary: "+optionsErr.Error()
				break
			}
			for _, active := range targets {
				if overlaps(active, target) {
					entry.State, entry.Reason = TargetOverlap, "target overlaps active skills target "+active
					break
				}
			}
			if entry.State != "" {
				break
			}
			if gitRoot != "" && containsPath(gitRoot, target) {
				entry.State, entry.Reason = InsideGitRoot, "target is inside git root "+gitRoot
			}
		}
		set.entries = append(set.entries, entry)
	}
	// Compare all safety-qualified candidates before assigning overlap states, so
	// a chain of overlaps rejects every participant regardless of declaration order.
	reasons := make(map[int][]string)
	for i, a := range set.entries {
		if a.State != "" {
			continue
		}
		for j := i + 1; j < len(set.entries); j++ {
			b := set.entries[j]
			if b.State == "" && overlaps(a.ResolvedTarget, b.ResolvedTarget) {
				reason := fmt.Sprintf("entries %q and %q have overlapping targets", a.Name, b.Name)
				reasons[i] = append(reasons[i], reason)
				reasons[j] = append(reasons[j], reason)
			}
		}
	}
	for i := range set.entries {
		entry := &set.entries[i]
		if entry.State == "" {
			if len(reasons[i]) > 0 {
				entry.State, entry.Reason = EntryOverlap, strings.Join(reasons[i], "; ")
			} else if _, err := os.Stat(filepath.Join(entry.ResolvedTarget, "SKILL.md")); err == nil {
				entry.State, entry.Reason = SingleSkill, "followed single skills are not supported yet"
			} else {
				entry.State, entry.Reason = Followed, "following directory"
			}
		}
		if entry.State != Followed && entry.State != NotLink {
			set.warnings = append(set.warnings, entry.Name+": "+string(entry.State)+": "+entry.Reason)
		}
	}
	children, err := readFollowDir(root)
	if err == nil {
		for _, child := range children {
			if !declared[child.Name()] && links.isLink(filepath.Join(root, child.Name()), child.Type()) {
				set.entries = append(set.entries, Entry{Name: child.Name(), State: UndeclaredLink, Reason: "not declared in .skillfollow or .skillfollow.local"})
			}
		}
	}
	return set
}

func readFollowDir(path string) ([]os.DirEntry, error) { return os.ReadDir(path) }

// Entries returns classifications in declaration order, followed by undeclared links.
func (s FollowSet) Entries() []Entry { return append([]Entry(nil), s.entries...) }

// ParsedEntries returns accepted, deduplicated declaration names in file order.
func (s FollowSet) ParsedEntries() []string { return append([]string(nil), s.parsed...) }

// Warnings returns parse and classification diagnostics.
func (s FollowSet) Warnings() []string { return append([]string(nil), s.warnings...) }

// Active reports whether either declaration file was read.
func (s FollowSet) Active() bool { return s.active }

// HasLocal reports whether .skillfollow.local was read.
func (s FollowSet) HasLocal() bool { return s.local }

// Followed returns only available followed directories.
func (s FollowSet) Followed() []Entry {
	return s.selectEntries(func(e Entry) bool { return e.State == Followed })
}

// Unavailable returns declared entries that cannot supply complete discovery.
func (s FollowSet) Unavailable() []Entry {
	return s.selectEntries(func(e Entry) bool { return e.State != Followed && e.State != NotLink && e.State != UndeclaredLink })
}
func (s FollowSet) selectEntries(match func(Entry) bool) []Entry {
	var result []Entry
	for _, entry := range s.entries {
		if match(entry) {
			result = append(result, entry)
		}
	}
	return result
}

// Owns reports containment in the canonical source or a currently followed target.
// The input must already be canonical; this is containment, not link identity.
func (s FollowSet) Owns(resolvedPath string) bool {
	if s.canonicalRoot != "" && containsPath(s.canonicalRoot, resolvedPath) {
		return true
	}
	for _, entry := range s.entries {
		if entry.State == Followed && containsPath(entry.ResolvedTarget, resolvedPath) {
			return true
		}
	}
	return false
}

// InFollowed finds a declared logical prefix, including unavailable entries.
// Real directories (not-link) and undeclared links are not followed boundaries.
func (s FollowSet) InFollowed(logicalRel string) (Entry, bool) {
	clean := filepath.ToSlash(filepath.Clean(logicalRel))
	if filepath.IsAbs(logicalRel) || clean == ".." || strings.HasPrefix(clean, "../") {
		return Entry{}, false
	}
	name := strings.SplitN(clean, "/", 2)[0]
	for _, entry := range s.entries {
		if entry.Name == name && entry.State != NotLink && entry.State != UndeclaredLink {
			return entry, true
		}
	}
	return Entry{}, false
}

// Err reports read failures recorded during traversal, even if callbacks ignored them.
func (s FollowSet) Err() error { return errors.Join(s.walkErrors...) }

func (s *FollowSet) markMissing(name string, err error) {
	s.walkErrors = append(s.walkErrors, fmt.Errorf("incomplete discovery of %s: %w", name, err))
	for i := range s.entries {
		if s.entries[i].Name == name {
			s.entries[i].State, s.entries[i].Reason = Missing, err.Error()
			s.warnings = append(s.warnings, name+": missing: "+err.Error())
			return
		}
	}
}
