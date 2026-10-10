// Package skillmove moves skills and folders to another folder of the skills
// source and carries what the install recorded about them: the metadata
// records with their audit acceptances, the project lockfile pin, the project
// .gitignore entry and a literal .skillignore line. Callers resolve the mode,
// pass the same discovery they list from and report the per-name results.
package skillmove

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/projectdir"
	"skillshare/internal/skillignore"
	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
	"skillshare/internal/utils"
	"skillshare/internal/validate"
)

// Code names why a move was refused. The values are stable: the CLI prints
// them in --json and the dashboard API sends them as error_code.
type Code string

const (
	CodeNotFound              Code = "skill_not_found"
	CodeAmbiguousName         Code = "ambiguous_name"
	CodeDestExists            Code = "dest_exists"
	CodeInsideTrackedRepo     Code = "inside_tracked_repo"
	CodeDestInsideTrackedRepo Code = "dest_inside_tracked_repo"
	CodeLinkedFolder          Code = "linked_folder"
	CodeDestIsSkill           Code = "dest_is_skill"
	CodeInvalidDest           Code = "invalid_dest"
	CodeDestInsideSource      Code = "dest_inside_source_folder"
	CodeDuplicateDest         Code = "duplicate_dest"
	CodeOverlappingSources    Code = "overlapping_sources"
	CodeSameFolder            Code = "same_folder"
	CodeNameCollision         Code = "name_collision"
	CodeAmbiguousRecord       Code = "ambiguous_record"
	CodeMoveFailed            Code = "move_failed"
)

// Refusal is why one name was not moved. It is also the error of a rename that
// failed after the checks passed (CodeMoveFailed).
type Refusal struct {
	Code Code
	Msg  string
}

func (r *Refusal) Error() string { return r.Msg }

// Target is a sync target as the move needs it: its name, for a skill's
// targets: frontmatter, and the skills config its filters and naming come from.
type Target struct {
	Name   string
	Config config.ResourceTargetConfig
}

// EnabledTargets lists the skills targets a move is checked against, in name
// order. A symlink-mode target links the whole source, so neither its filters
// nor its naming apply.
func EnabledTargets(targets map[string]config.TargetConfig) []Target {
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []Target
	for _, name := range names {
		target := targets[name]
		if sc := target.SkillsConfig(); sc.IsEnabled() && sc.Mode != "symlink" {
			out = append(out, Target{Name: name, Config: sc})
		}
	}
	return out
}

// Options describes the source the names live in.
type Options struct {
	SourceDir string
	Follow    *sourcewalk.Follow
	Store     *install.MetadataStore // re-keyed and saved to SourceDir by Run
	Targets   []Target               // enabled skills targets, for collisions and filter warnings
	Force     bool                   // accept a name collision

	// DryRun makes Run report the plan's results and change nothing.
	DryRun bool
	// ProjectRoot is set in project mode: the lockfile pin follows the skill.
	ProjectRoot string
	// GitignoreDir and GitignorePrefix locate the project .gitignore line of an
	// installed skill, "<prefix>/<relPath>"; an empty dir skips the cleanup.
	GitignoreDir    string
	GitignorePrefix string
	// Install is for CheckDest on an install --into folder: links are left to
	// the install, which writes through a followed first-level link or is
	// refused by sourcefs, so a path below one is not refused here.
	Install bool
	// Reconcile runs after the records are saved: it sets groups, prunes and,
	// in project mode, rewrites config.yaml and the lockfile. May be nil.
	Reconcile func() error
}

// Skill is one discovered skill that moves with its root.
type Skill struct {
	From, To         string // source-relative slash paths
	FlatFrom, FlatTo string
}

// Planned is one requested name: what it resolved to, or why it is refused.
type Planned struct {
	Name     string // as requested
	From, To string // the skill or folder being renamed
	Folder   bool   // a folder rather than a skill
	Skills   []Skill
	Records  int // metadata entries that move with it
	Warnings []string
	Err      *Refusal // nil: will move (Plan) or moved (Run)
}

// renames maps each path that moves to where it goes: the root, its skills and
// whatever carries a record below it, so a literal ignore line or a lock pin
// naming any of them follows.
func (p Planned) renames() map[string]string {
	m := map[string]string{p.From: p.To}
	for _, s := range p.Skills {
		m[s.From] = s.To
	}
	return m
}

// Moved reports whether the name was moved, or would be.
func (p Planned) Moved() bool { return p.Err == nil }

// Skipped reports a name that is already in the destination: a no-op, not a
// failure.
func (p Planned) Skipped() bool { return p.Err != nil && p.Err.Code == CodeSameFolder }

// FlatName is the flat name a moved skill goes by afterwards; "" for a folder.
func (p Planned) FlatName() string {
	if !p.Folder {
		for _, s := range p.Skills {
			if s.From == p.From {
				return s.FlatTo
			}
		}
	}
	return ""
}

// Outcome is the result of Run: one entry per planned name in input order, and
// the first failure of a step after the renames, which leaves records,
// lockfile or ignore files out of step until fixed.
type Outcome struct {
	Items []Planned
	Err   error
}

// Plan resolves names to skills or folders below dest and refuses what cannot
// move, without touching disk. discovered is the source's full discovery,
// disabled skills included. Every refusal is final: a folder is checked as a
// whole, so none of it ends half-moved.
func Plan(discovered []sync.DiscoveredSkill, names []string, dest string, o Options) []Planned {
	planned := make([]Planned, len(names))
	destRel, destErr := CheckDest(dest, o)
	for i, name := range names {
		p := Planned{Name: name}
		if destErr != nil {
			p.Err = destErr
		} else {
			resolve(&p, discovered, normalize(name), destRel, o)
		}
		planned[i] = p
	}
	if destErr != nil {
		return planned
	}
	refuseBatchConflicts(planned)
	checkRecords(planned, o)
	checkTargets(planned, discovered, o)
	files := readIgnoreFiles(o.SourceDir)
	before := files.matcher(nil)
	for i := range planned {
		if planned[i].Err == nil {
			planned[i].Warnings = append(planned[i].Warnings, ignoreWarnings(&planned[i], files, before)...)
		}
	}
	return planned
}

// normalize turns a typed name into a slash path without a trailing slash.
func normalize(name string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), `\`, "/")
	name = strings.TrimRight(name, "/")
	return strings.TrimPrefix(name, "./")
}

// CheckDest validates dest once for the whole batch and returns it as a slash
// path below the source ("" for the source root). Plan applies the same check,
// so a caller that wants a bad destination to fail the request, not each name,
// can ask first.
func CheckDest(dest string, o Options) (string, *Refusal) {
	dest = normalize(dest)
	if dest == "." {
		return "", nil
	}
	// A _ folder cannot be typed, so look for the checkout before validating:
	// the refusal then says what is wrong instead of what is not allowed.
	if repo, ok := install.TrackedAncestor(o.SourceDir, dest+"/x", o.Follow); ok {
		return "", &Refusal{CodeDestInsideTrackedRepo, fmt.Sprintf("destination %q is inside the tracked repo %s", dest, repo)}
	}
	if err := validate.IntoPath(dest); err != nil {
		return "", &Refusal{CodeInvalidDest, fmt.Sprintf("invalid destination %q: %v", dest, err)}
	}
	prefix := ""
	for _, seg := range strings.Split(dest, "/") {
		prefix = path.Join(prefix, seg)
		abs := filepath.Join(o.SourceDir, filepath.FromSlash(prefix))
		info, err := os.Lstat(abs)
		linked := err == nil && utils.IsLinkMode(abs, info.Mode())
		switch {
		case err != nil:
			return dest, nil // the rest does not exist yet
		case linked && !o.Install:
			return "", &Refusal{CodeLinkedFolder, fmt.Sprintf("destination %q is below the source link %s", dest, prefix)}
		case linked:
		case !info.IsDir():
			return "", &Refusal{CodeInvalidDest, fmt.Sprintf("destination %q: %s is a file", dest, prefix)}
		}
		if _, err := os.Stat(filepath.Join(abs, "SKILL.md")); err == nil {
			return "", &Refusal{CodeDestIsSkill, fmt.Sprintf("destination %q is inside the skill %s", dest, prefix)}
		}
	}
	return dest, nil
}

// resolve fills p for one typed name. An exact path wins over a flat name or
// a base name, so a short name never moves something other than what it spells.
func resolve(p *Planned, discovered []sync.DiscoveredSkill, name string, destRel string, o Options) {
	if name == "" || name == "." || strings.HasPrefix(name, "/") || slices.Contains(strings.Split(name, "/"), "..") {
		p.Err = &Refusal{CodeNotFound, fmt.Sprintf("invalid name %q", p.Name)}
		return
	}
	root, folder, err := lookup(discovered, name, o)
	if err != nil {
		p.Err = err
		return
	}
	p.From, p.Folder = root, folder
	p.To = path.Join(destRel, path.Base(root))

	if r := checkSource(root, o); r != nil {
		p.Err = r
		return
	}
	for _, d := range discovered {
		if d.RelPath == root || strings.HasPrefix(d.RelPath, root+"/") {
			p.Skills = append(p.Skills, Skill{From: d.RelPath, FlatFrom: d.FlatName})
		}
	}
	if len(p.Skills) == 0 {
		p.Err = &Refusal{CodeNotFound, fmt.Sprintf("no skill found in %s", root)}
		return
	}
	switch {
	case p.To == p.From:
		p.Err = &Refusal{CodeSameFolder, fmt.Sprintf("%s is already in %s", root, folderLabel(destRel))}
	case destRel == root || strings.HasPrefix(destRel, root+"/"):
		p.Err = &Refusal{CodeDestInsideSource, fmt.Sprintf("cannot move %s into itself", root)}
	default:
		if _, err := os.Lstat(filepath.Join(o.SourceDir, filepath.FromSlash(p.To))); err == nil {
			p.Err = &Refusal{CodeDestExists, fmt.Sprintf("%s already exists", p.To)}
		}
	}
	for i := range p.Skills {
		p.Skills[i].To = p.To + strings.TrimPrefix(p.Skills[i].From, root)
		p.Skills[i].FlatTo = utils.PathToFlatName(p.Skills[i].To)
	}
}

func folderLabel(rel string) string {
	if rel == "" {
		return "the source root"
	}
	return rel
}

// lookup finds the skill or folder a typed name stands for.
func lookup(discovered []sync.DiscoveredSkill, name string, o Options) (root string, folder bool, err *Refusal) {
	for _, d := range discovered {
		if d.RelPath == name {
			return name, false, nil
		}
	}
	abs := filepath.Join(o.SourceDir, filepath.FromSlash(name))
	if info, statErr := os.Lstat(abs); statErr == nil && (info.IsDir() || utils.IsLinkMode(abs, info.Mode())) {
		return name, true, nil
	}
	var flat, base []string
	for _, d := range discovered {
		if d.FlatName == name {
			flat = append(flat, d.RelPath)
		}
		if path.Base(d.RelPath) == name {
			base = append(base, d.RelPath)
		}
	}
	for _, found := range [][]string{flat, base} {
		switch len(found) {
		case 0:
		case 1:
			return found[0], false, nil
		default:
			sort.Strings(found)
			return "", false, &Refusal{CodeAmbiguousName, fmt.Sprintf("%q matches %s: use the full path", name, strings.Join(found, ", "))}
		}
	}
	return "", false, &Refusal{CodeNotFound, fmt.Sprintf("skill %q not found in the source", name)}
}

// checkSource refuses a root that sits in, is, or holds a tracked checkout or
// a followed source link. The whole tree is walked, so a checkout with no
// skill of its own is found too.
func checkSource(root string, o Options) *Refusal {
	if repo, ok := install.TrackedAncestor(o.SourceDir, root+"/x", o.Follow); ok {
		if repo == root {
			return &Refusal{CodeInsideTrackedRepo, fmt.Sprintf("%s is a tracked repo; uninstall or update it instead", root)}
		}
		return &Refusal{CodeInsideTrackedRepo, fmt.Sprintf("%s is inside the tracked repo %s; uninstall or update the repo instead", root, repo)}
	}
	prefix := ""
	for _, seg := range strings.Split(root, "/") {
		prefix = path.Join(prefix, seg)
		abs := filepath.Join(o.SourceDir, filepath.FromSlash(prefix))
		if _, followed := o.Follow.Resolve(abs); followed {
			return &Refusal{CodeLinkedFolder, fmt.Sprintf("%s is, or is inside, the source link %s", root, prefix)}
		}
		if info, err := os.Lstat(abs); err == nil && utils.IsLinkMode(abs, info.Mode()) {
			return &Refusal{CodeLinkedFolder, fmt.Sprintf("%s is, or is inside, the link %s", root, prefix)}
		}
	}
	var refusal *Refusal
	base := filepath.Join(o.SourceDir, filepath.FromSlash(root))
	// The policy follows only direct children of the source, so a link below
	// root is an ordinary entry that moves with its folder (root itself was
	// checked above); what is left to find is a checkout with no skill of its own.
	_ = sourcewalk.WalkDir(base, sourcewalk.Options{Follow: o.Follow}, func(p string, d fs.DirEntry, err error) error {
		if err != nil || refusal != nil || !d.IsDir() {
			return nil
		}
		if d.Name() == ".git" {
			return filepath.SkipDir
		}
		if install.IsTrackedCheckout(p) {
			rel, _ := filepath.Rel(o.SourceDir, p)
			refusal = &Refusal{CodeInsideTrackedRepo, fmt.Sprintf("%s holds the tracked repo %s; a folder is moved whole or not at all", root, filepath.ToSlash(rel))}
		}
		return nil
	})
	return refusal
}

// refuseBatchConflicts refuses names that cannot be satisfied together: two
// resolving to one destination, or one root equal to or below another. Both
// sides go, as neither order is clearly what was meant.
func refuseBatchConflicts(planned []Planned) {
	for i := range planned {
		for j := i + 1; j < len(planned); j++ {
			a, b := &planned[i], &planned[j]
			if a.From == "" || b.From == "" || (a.Err != nil && a.Err.Code != CodeSameFolder) || (b.Err != nil && b.Err.Code != CodeSameFolder) {
				continue
			}
			switch {
			case a.From == b.From || strings.HasPrefix(a.From, b.From+"/") || strings.HasPrefix(b.From, a.From+"/"):
				r := &Refusal{CodeOverlappingSources, fmt.Sprintf("%s and %s overlap: the folder already carries what is below it", a.From, b.From)}
				a.Err, b.Err = r, r
			case a.To == b.To:
				r := &Refusal{CodeDuplicateDest, fmt.Sprintf("%s and %s would both become %s", a.From, b.From, a.To)}
				a.Err, b.Err = r, r
			}
		}
	}
}

// checkRecords counts the records each name carries and refuses a name whose
// records cannot be re-keyed one to one: two entries claiming one path would
// merge on the move and drop one with its audit acceptances.
func checkRecords(planned []Planned, o Options) {
	for i := range planned {
		p := &planned[i]
		if p.Err != nil || o.Store == nil {
			continue
		}
		seen := map[string]bool{}
		for _, key := range o.Store.KeysUnder(p.From) {
			rel := filepath.ToSlash(install.KeyToRelPath(key, o.Store.Get(key)))
			if seen[rel] {
				p.Err = &Refusal{CodeAmbiguousRecord, fmt.Sprintf("two install records claim %s; remove the stale one from .metadata.json first", rel)}
				break
			}
			seen[rel] = true
			p.Records++
		}
	}
}

// checkTargets looks at what the new flat names do to each target: a name
// another skill already has there is a collision (refused unless forced), and
// an include or exclude rule that no longer decides the same is a warning.
func checkTargets(planned []Planned, discovered []sync.DiscoveredSkill, o Options) {
	moved := map[string]string{} // old relPath -> new
	for _, p := range planned {
		if p.Err != nil {
			continue
		}
		for _, s := range p.Skills {
			moved[s.From] = s.To
		}
	}
	if len(moved) == 0 || len(o.Targets) == 0 {
		return
	}
	after := make([]sync.DiscoveredSkill, len(discovered))
	copy(after, discovered)
	for i, d := range after {
		if to, ok := moved[d.RelPath]; ok {
			// SourcePath stays: the SKILL.md is read from where it still is.
			after[i].RelPath, after[i].FlatName = to, utils.PathToFlatName(to)
		}
	}

	for _, t := range o.Targets {
		var before map[string][]string // what already clashed; read only when something does now
		for name, paths := range collisions(t, after) {
			if before == nil {
				before = collisions(t, discovered)
			}
			for i := range planned {
				p := &planned[i]
				if p.Err != nil || !joins(p, paths, before[name]) {
					continue
				}
				msg := fmt.Sprintf("target %s would get two skills named %q (%s)", t.Name, name, strings.Join(paths, ", "))
				if o.Force {
					p.Warnings = append(p.Warnings, msg+"; sync skips both until one is renamed")
				} else {
					p.Err = &Refusal{CodeNameCollision, msg + "; use --force to move anyway"}
				}
			}
		}
		for i := range planned {
			p := &planned[i]
			if p.Err != nil {
				continue
			}
			for _, s := range p.Skills {
				was, _ := sync.ShouldSyncFlatName(s.FlatFrom, t.Config.Include, t.Config.Exclude)
				now, _ := sync.ShouldSyncFlatName(s.FlatTo, t.Config.Include, t.Config.Exclude)
				if was != now {
					p.Warnings = append(p.Warnings, fmt.Sprintf("target %s: its include/exclude rules treat %s differently from %s (%s); config.yaml is not changed",
						t.Name, s.FlatTo, s.FlatFrom, syncedWord(was)))
				}
			}
		}
	}
}

func syncedWord(was bool) string {
	if was {
		return "it was synced, it no longer is"
	}
	return "it was not synced, it now is"
}

// joins reports whether one of p's skills becomes a member of the collision at
// paths that it was not part of before, where members are the old paths of the
// collision already there. A skill that merely stays in a collision it was in
// is no new problem; one that adds itself to it is.
func joins(p *Planned, paths, members []string) bool {
	for _, s := range p.Skills {
		if slices.Contains(paths, s.To) && !slices.Contains(members, s.From) {
			return true
		}
	}
	return false
}

// collisions maps each target name two skills share to their relPaths. Flat
// naming returns before sync's own detection, so equal flat names are compared
// here.
func collisions(t Target, skills []sync.DiscoveredSkill) map[string][]string {
	out := map[string][]string{}
	if config.EffectiveTargetNaming(t.Config.TargetNaming) == "flat" {
		selected, err := sync.SelectTargetSkills(skills, t.Name, t.Config)
		if err != nil {
			return out
		}
		byName := map[string][]string{}
		for _, s := range selected {
			byName[s.FlatName] = append(byName[s.FlatName], s.RelPath)
		}
		for name, paths := range byName {
			if len(paths) > 1 {
				sort.Strings(paths)
				out[name] = paths
			}
		}
		return out
	}
	resolution, err := sync.ResolveTargetSkillsForTarget(t.Name, t.Config, skills)
	if err != nil {
		return out
	}
	for _, c := range resolution.Collisions {
		out[c.Name] = c.Paths
	}
	return out
}

// ignoreFiles is the source's .skillignore and .skillignore.local, read once
// per Plan.
type ignoreFiles struct{ base, local string }

func readIgnoreFiles(dir string) ignoreFiles {
	base, _ := os.ReadFile(filepath.Join(dir, ".skillignore"))
	local, _ := os.ReadFile(filepath.Join(dir, ".skillignore.local"))
	return ignoreFiles{string(base), string(local)}
}

// matcher compiles the files as discovery reads them, with the literal lines
// of .skillignore renamed. The local file is a machine's own and never follows.
func (f ignoreFiles) matcher(renames map[string]string) *skillignore.Matcher {
	base, _ := skillignore.RenamePatterns(f.base, renames)
	return skillignore.Compile(strings.Split(base+"\n"+f.local, "\n"))
}

// ignoreWarnings predicts whether the rules still decide the same for each
// moved skill once a literal line has followed it. Glob rules are left alone,
// so they may now match differently.
func ignoreWarnings(p *Planned, files ignoreFiles, before *skillignore.Matcher) []string {
	if !before.HasRules() {
		return nil
	}
	after := files.matcher(p.renames())
	var warnings []string
	for _, s := range p.Skills {
		if was, now := before.Match(s.From, true), after.Match(s.To, true); was != now {
			state := "enabled"
			if now {
				state = "disabled"
			}
			warnings = append(warnings, fmt.Sprintf("%s becomes %s by the .skillignore rules", s.To, state))
		}
	}
	return warnings
}

// Run renames each planned name that has no refusal, then re-keys the records
// of what moved. A failed rename changes nothing for that name; the steps after
// the renames run once for everything that moved, and the first failure is
// returned in Outcome.Err.
func Run(planned []Planned, o Options) Outcome {
	out := Outcome{Items: append([]Planned(nil), planned...)}
	if o.DryRun {
		return out
	}
	root, err := sourcefs.Create(o.SourceDir, o.Follow)
	if err != nil {
		for i := range out.Items {
			if out.Items[i].Err == nil {
				out.Items[i].Err = &Refusal{CodeMoveFailed, fmt.Sprintf("cannot open the skills source: %v", err)}
			}
		}
		return out
	}
	defer root.Close()
	var moved []*Planned
	for i := range out.Items {
		p := &out.Items[i]
		if p.Err != nil {
			continue
		}
		if dir := path.Dir(p.To); dir != "." {
			if err := root.MkdirAll(filepath.FromSlash(dir), 0o755); err != nil {
				p.Err = &Refusal{CodeMoveFailed, fmt.Sprintf("cannot create %s: %v", dir, err)}
				continue
			}
		}
		if err := root.Rename(filepath.FromSlash(p.From), filepath.FromSlash(p.To)); err != nil {
			p.Err = &Refusal{CodeMoveFailed, fmt.Sprintf("cannot move %s: %v", p.From, err)}
			continue
		}
		moved = append(moved, p)
	}
	if len(moved) > 0 {
		out.Err = followUp(root, moved, o)
	}
	return out
}

// followUp carries everything recorded about the moved names. Each step runs
// even when an earlier one failed, so one bad file does not strand the rest.
func followUp(root *sourcefs.Root, moved []*Planned, o Options) error {
	var errs []error
	note := func(what string, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", what, err))
		}
	}

	pins := map[string]string{} // every old path that follows its root
	for _, p := range moved {
		maps.Copy(pins, p.renames())
		if o.Store != nil {
			// A record below an installed skill may have no skill of its own.
			for _, key := range o.Store.KeysUnder(p.From) {
				rel := filepath.ToSlash(install.KeyToRelPath(key, o.Store.Get(key)))
				pins[rel] = p.To + strings.TrimPrefix(rel, p.From)
			}
			o.Store.MovePath(p.From, p.To)
		}
	}
	if o.Store != nil {
		note("save install records", o.Store.Save(o.SourceDir))
	}
	// The gitignore line of an installed skill sits at its old path; reconcile
	// adds the new one but never removes the old.
	if o.GitignoreDir != "" {
		var stale []string
		for from := range pins {
			stale = append(stale, o.GitignorePrefix+"/"+from)
		}
		_, err := install.RemoveFromGitIgnoreBatch(o.GitignoreDir, stale)
		note("update .gitignore", err)
	}
	if o.ProjectRoot != "" {
		dir := projectdir.Resolve(o.ProjectRoot)
		lock, err := install.LoadLock(dir)
		if err == nil {
			changed := false
			for from, to := range pins {
				changed = lock.MovePin(from, to) || changed
			}
			if changed {
				err = lock.Save(dir)
			}
		}
		note("move the "+install.LockFileName+" pin", err)
	}
	if data, err := os.ReadFile(filepath.Join(o.SourceDir, ".skillignore")); err == nil {
		if out, changed := skillignore.RenamePatterns(string(data), pins); changed {
			note("update .skillignore", root.WriteFileAtomic(".skillignore", []byte(out), 0o644))
		}
	}
	if o.Reconcile != nil {
		note("reconcile install records", o.Reconcile())
	}
	return errors.Join(errs...)
}
