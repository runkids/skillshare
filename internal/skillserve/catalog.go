// Package skillserve publishes the skills source as an MCP Skills extension
// (SEP-2640) catalog: one entry per valid skill with its full frontmatter and a
// complete file manifest, and contained reads of the listed files.
package skillserve

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"slices"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/skillpkg"
	"skillshare/internal/sourcewalk"
	ssync "skillshare/internal/sync"
)

// extensionID is the SEP-2640 capability key.
const extensionID = "io.modelcontextprotocol/skills"

var errNotFound = errors.New("not found")

// afterDiscover runs between discovery and the loads; tests change the tree there.
var afterDiscover = func() {}

// File is one manifest entry of a skill.
type File struct {
	URI    string `json:"uri"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`

	rel string // slash path inside the skill that lists it
}

// Skill is one skills/list entry.
type Skill struct {
	URI         string         `json:"uri"`
	Frontmatter map[string]any `json:"frontmatter"`
	Resources   []File         `json:"resources"`

	src ssync.DiscoveredSkill // where it was discovered, for edits
}

// servedFile locates a manifest file: the resolved skill directory, the
// slash path inside it, and the digest it was listed with.
type servedFile struct {
	dir, rel, digest string
	owner            *Skill // the skill whose manifest lists the file
}

// Skip is a skill left out of the catalog, by source path.
type Skip struct {
	Path, Reason string
}

func (s Skip) String() string { return fmt.Sprintf("skipped %s: %s", s.Path, s.Reason) }

// Catalog is one snapshot of the served skills, ordered by URI; Skipped is in source path order.
type Catalog struct {
	Skills  []*Skill
	Skipped []Skip
	bySkill map[string]*Skill
	byFile  map[string]servedFile
}

// Builder builds catalogs from the skills source.
type Builder struct {
	Source string
	Walk   sourcewalk.Options
	// Target names the target whose Skills selection applies. With Skills nil,
	// every enabled skill is served.
	Target string
	Skills *config.ResourceTargetConfig
}

// Build discovers the source and returns the skills to serve. Skills that are
// invalid, over the limits, or contain an unserved nested skill are skipped
// with a warning.
func (b *Builder) Build() (*Catalog, error) {
	all, err := ssync.DiscoverSourceSkillsAll(b.Source, b.Walk)
	if err != nil {
		return nil, err
	}
	afterDiscover()
	// Discovery and the loads read the tree separately. A skill added, removed or
	// toggled in between would leave a parent publishing the files of a skill the
	// catalog does not list, so build again until discovery agrees with the loads.
	for range 3 {
		c, err := b.build(all)
		if err != nil {
			return nil, err
		}
		again, err := ssync.DiscoverSourceSkillsAll(b.Source, b.Walk)
		if err != nil {
			return nil, err
		}
		if slices.EqualFunc(all, again, func(x, y ssync.DiscoveredSkill) bool {
			return x.RelPath == y.RelPath && x.Disabled == y.Disabled
		}) {
			return c, nil
		}
		all = again
	}
	return nil, errors.New("the skills source kept changing while it was read; list the skills again")
}

func (b *Builder) build(all []ssync.DiscoveredSkill) (*Catalog, error) {
	var err error
	var selected []ssync.DiscoveredSkill
	var skipped map[string]string
	var routed map[string]ssync.SkillTargetClassification
	if b.Skills != nil {
		if selected, skipped, err = ssync.SelectTargetSkillsWithDiagnostics(all, b.Target, *b.Skills); err != nil {
			return nil, err
		}
		routed, err = ssync.ClassifySkillsForTarget(all, b.Target, *b.Skills)
		if err != nil {
			return nil, err
		}
	} else {
		selected = slices.DeleteFunc(slices.Clone(all), func(s ssync.DiscoveredSkill) bool { return s.Disabled })
	}
	served := make(map[string]bool, len(selected))
	for _, s := range selected {
		served[s.RelPath] = true
	}
	var unserved []string
	c := &Catalog{bySkill: map[string]*Skill{}, byFile: map[string]servedFile{}}
	for _, s := range all {
		if !served[s.RelPath] && !(s.UsesVariantRouting() && !s.Disabled && routed[s.RelPath].Status == "skill_target_mismatch") {
			unserved = append(unserved, s.RelPath)
		}
		if reason, ok := skipped[s.RelPath]; ok {
			c.Skipped = append(c.Skipped, Skip{Path: s.RelPath, Reason: reason})
		}
	}
	slices.SortFunc(selected, func(x, y ssync.DiscoveredSkill) int { return strings.Compare(x.RelPath, y.RelPath) })

	// Deepest first, so a nested skill skipped for any reason also skips its parents,
	// which would otherwise publish its files as their own.
	for i := len(selected) - 1; i >= 0; i-- {
		s := selected[i]
		skill, files, reason := load(s)
		if j := slices.IndexFunc(unserved, func(o string) bool { return strings.HasPrefix(o, s.RelPath+"/") }); reason == "" && j >= 0 {
			reason = fmt.Sprintf("contains %s, which is not served", unserved[j])
		}
		// A parent lists its nested skills' files under the same URIs. If one changed
		// between the two reads, the entries disagree; serve the parent next refresh
		// rather than overwrite what the nested skill advertises.
		for uri, f := range files {
			if old, ok := c.byFile[uri]; reason == "" && ok && old.digest != f.digest {
				reason = "a nested skill changed while it was read; it is served after the next refresh"
			}
		}
		if reason != "" {
			c.Skipped = append(c.Skipped, Skip{Path: s.RelPath, Reason: reason})
			unserved = append(unserved, s.RelPath)
			continue
		}
		c.Skills = append(c.Skills, skill)
		c.bySkill[skill.URI] = skill
		for uri, f := range files {
			// A nested skill's files are also in the outer manifest; the deepest skill owns them.
			if old, ok := c.byFile[uri]; !ok || len(f.owner.URI) > len(old.owner.URI) {
				c.byFile[uri] = f
			}
		}
	}
	slices.Reverse(c.Skipped)
	// By URI, which skills/list cursors continue from.
	slices.SortFunc(c.Skills, func(x, y *Skill) int { return strings.Compare(x.URI, y.URI) })
	return c, nil
}

// load builds one entry and its files, or returns why the skill is skipped.
// Every build reads every file again: size and mtime cannot prove the content is
// unchanged (cp -p, rsync -t), and a stale digest makes hosts reject the file.
func load(s ssync.DiscoveredSkill) (*Skill, map[string]servedFile, string) {
	p, err := skillpkg.Load(s.SourcePath)
	if err != nil {
		return nil, nil, err.Error()
	}
	base := "skill://" + escapePath(s.RelPath)
	skill := &Skill{URI: base + "/SKILL.md", Frontmatter: p.Frontmatter, src: s}
	files := make(map[string]servedFile, len(p.Files))
	for _, f := range p.Files {
		uri := base + "/" + escapePath(f.Rel)
		files[uri] = servedFile{dir: p.Dir, rel: f.Rel, digest: f.Digest, owner: skill}
		skill.Resources = append(skill.Resources, File{URI: uri, Digest: f.Digest, Size: f.Size, rel: f.Rel})
	}
	return skill, files, ""
}

// Skill returns the served skill whose SKILL.md URI is uri.
func (c *Catalog) Skill(uri string) (*Skill, bool) {
	key, ok := canonical(uri)
	skill := c.bySkill[key]
	return skill, ok && skill != nil
}

// Revision returns the digest a listed file was served with, or "".
func (c *Catalog) Revision(uri string) string {
	key, _ := canonical(uri)
	return c.byFile[key].digest
}

// Read returns the content of a listed file. Only manifest files are served,
// and the read is confined to the skill directory.
func (c *Catalog) Read(uri string) ([]byte, error) {
	key, ok := canonical(uri)
	sf, listed := c.byFile[key]
	if !ok || !listed {
		return nil, errNotFound
	}
	root, err := os.OpenRoot(sf.dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(sf.rel)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, skillpkg.MaxBytes))
	if err != nil {
		return nil, err
	}
	// A file edited since it was listed would fail the client's digest check;
	// the next refresh lists it again.
	if skillpkg.Digest(data) != sf.digest {
		return nil, fmt.Errorf("%s changed since it was listed; list the skills again", uri)
	}
	return data, nil
}

func escapePath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

// canonical decodes a skill:// URI and re-encodes it the way the catalog
// builds URIs. Empty, dot, and separator-bearing segments are rejected.
func canonical(uri string) (string, bool) {
	rest, ok := strings.CutPrefix(uri, "skill://")
	if !ok {
		return "", false
	}
	segs := strings.Split(rest, "/")
	for i, s := range segs {
		d, err := url.PathUnescape(s)
		if err != nil || d == "" || d == "." || d == ".." || strings.ContainsAny(d, "/\x00") {
			return "", false
		}
		segs[i] = url.PathEscape(d)
	}
	return "skill://" + strings.Join(segs, "/"), true
}
