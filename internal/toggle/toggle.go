// Package toggle enables and disables skills and agents for the CLI, the list
// TUI and the dashboard: which discovered items a name refers to, which ignore
// lines to write or remove, and whether each item ended in the requested state.
// Another rule (a glob, a folder rule, a "!" rule in the .local file, a tracked
// repo's own .skillignore) can decide that state instead of the line a toggle
// wrote or removed, so the outcome always comes from discovery.
package toggle

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"skillshare/internal/resource"
	"skillshare/internal/skillignore"
	"skillshare/internal/sourcewalk"
	ssync "skillshare/internal/sync"
)

// Source is a skills or agents source and how to walk it.
type Source struct {
	Agent bool
	Root  string
	Walk  sourcewalk.Options // skills only
}

// IgnorePath is the ignore file a toggle writes.
func (src Source) IgnorePath() string {
	return filepath.Join(src.Root, src.ignoreFile())
}

func (src Source) ignoreFile() string {
	if src.Agent {
		return ".agentignore"
	}
	return ".skillignore"
}

// Item is one discovered skill or agent.
type Item struct {
	RelPath  string
	Disabled bool
	// Repo is the tracked repo holding a skill; its own .skillignore also applies.
	Repo string
}

// Set is one discovery of every skill or agent in a source, disabled ones included.
type Set struct {
	src       Source
	agents    []resource.DiscoveredResource
	byRelPath map[string]Item
	byFlat    map[string]Item   // skills
	byBase    map[string][]Item // by folder (skills) or file (agents) name, in discovery order
}

// Discover lists every skill (or agent) in src with the discovery sync and the
// dashboard use, including agents inside folders .agentignore ignores.
func (src Source) Discover() (*Set, error) {
	s := &Set{src: src, byRelPath: map[string]Item{}, byBase: map[string][]Item{}}
	if src.Agent {
		found, err := resource.AgentKind{}.DiscoverAll(src.Root)
		if err != nil {
			return nil, fmt.Errorf("failed to discover agents: %w", err)
		}
		s.agents = found
		for _, d := range found {
			it := Item{RelPath: d.RelPath, Disabled: d.Disabled}
			s.byRelPath[d.RelPath] = it
			base := path.Base(d.RelPath)
			s.byBase[base] = append(s.byBase[base], it)
		}
		return s, nil
	}
	found, err := ssync.DiscoverSourceSkillsAll(src.Root, src.Walk)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills: %w", err)
	}
	s.byFlat = map[string]Item{}
	for _, d := range found {
		it := Item{RelPath: d.RelPath, Disabled: d.Disabled, Repo: d.RepoRelPath}
		s.byRelPath[d.RelPath] = it
		s.byFlat[d.FlatName] = it
		base := path.Base(d.RelPath)
		s.byBase[base] = append(s.byBase[base], it)
	}
	return s, nil
}

// Named returns the items name refers to; the first is the one the dashboard
// toggles. An exact relPath or flat name names that item alone. Otherwise an
// agent resolves through resource.FindAgent, then by file name with or without
// .md, and a skill folder name names every skill in a folder of that name.
func (s *Set) Named(name string) []Item {
	if s.src.Agent {
		if d, ok := resource.FindAgent(s.agents, name); ok {
			return []Item{s.byRelPath[d.RelPath]}
		}
		if items := s.byBase[name]; len(items) > 0 || strings.HasSuffix(name, ".md") {
			return items
		}
		return s.byBase[name+".md"]
	}
	if it, ok := s.byRelPath[name]; ok {
		return []Item{it}
	}
	if it, ok := s.byFlat[name]; ok {
		return []Item{it}
	}
	return s.byBase[name]
}

// Lines returns the ignore lines the CLI writes (disable) or removes (enable)
// for arg, and the items arg names. A glob names no item and is written as
// typed. A name is written as typed when that line matches the items it names;
// an agent typed without .md gets .md appended, since a line matches the file
// name. Items the typed line cannot match, such as those named by a flat or
// frontmatter name, get their relPath line instead, as the dashboard writes.
// Enable also removes each item's relPath line, which the TUI and dashboard write.
func (s *Set) Lines(arg string, enable bool) ([]string, []Item) {
	items := s.Named(arg)
	if len(items) == 0 {
		return []string{arg}, nil
	}
	line := arg
	if s.src.Agent && !strings.HasSuffix(arg, ".md") && lineNames(items[0], arg+".md") {
		line = arg + ".md"
	}
	var lines []string
	if enable || slices.ContainsFunc(items, func(it Item) bool { return lineNames(it, line) }) {
		lines = append(lines, line)
	}
	for _, it := range items {
		if (enable || !lineNames(it, line)) && !slices.Contains(lines, it.RelPath) {
			lines = append(lines, it.RelPath)
		}
	}
	return lines, items
}

// lineNames reports whether line, as a plain name or path, names it.
func lineNames(it Item, line string) bool {
	return it.RelPath == line || path.Base(it.RelPath) == line
}

// Outcome is what a toggle did for one item.
type Outcome struct {
	RelPath string
	Lines   []string // lines added or removed that name it
	Blocked string   // why it is not in the requested state; "" when it is
}

// Result is what Apply did.
type Result struct {
	Changed map[string]bool // lines actually added or removed
	Items   []Outcome       // one per relPath passed, in order
}

// Apply adds (disable) or removes (enable) each line in the ignore file, then
// discovers again and reports whether each item ended in the requested state.
func (src Source) Apply(enable bool, lines, relPaths []string) (Result, error) {
	res := Result{Changed: map[string]bool{}}
	for _, line := range lines {
		var changed bool
		var err error
		if enable {
			changed, err = skillignore.RemovePattern(src.IgnorePath(), line)
		} else {
			changed, err = skillignore.AddPattern(src.IgnorePath(), line)
		}
		if err != nil {
			return res, fmt.Errorf("failed to update %s: %w", src.ignoreFile(), err)
		}
		if changed {
			res.Changed[line] = true
		}
	}
	after, err := src.Discover()
	if err != nil {
		return res, err
	}
	for _, rel := range relPaths {
		out := Outcome{RelPath: rel, Blocked: after.blocked(rel, enable)}
		for _, line := range lines {
			if res.Changed[line] && lineNames(Item{RelPath: rel}, line) {
				out.Lines = append(out.Lines, line)
			}
		}
		res.Items = append(res.Items, out)
	}
	return res, nil
}

// blocked explains why the item at relPath is not in the requested state, or
// returns "" when it is (or is no longer there).
func (s *Set) blocked(relPath string, enable bool) string {
	it, ok := s.byRelPath[relPath]
	if !ok || it.Disabled != enable {
		return ""
	}
	file := s.src.ignoreFile()
	if !enable {
		return fmt.Sprintf("still enabled: a \"!\" rule in %[1]s.local or %[1]s re-includes it — remove that rule to disable it", file)
	}
	files := fmt.Sprintf("%[1]s or %[1]s.local", file)
	if it.Repo != "" {
		files = fmt.Sprintf("%[1]s or %[2]s/%[1]s, or their .local files,", file, it.Repo)
	}
	return fmt.Sprintf("still disabled: another rule in %s matches it — remove that rule to enable it", files)
}
