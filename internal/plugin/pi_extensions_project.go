package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/tailscale/hujson"
)

// A project change is saved to <project>/.pi/settings.json, the way pi config
// saves it, and nowhere else: the global settings and trust.json are only read.
// Pi reads the file only when it trusts the project; the dashboard says so and
// never decides it. An entry the project has is changed in place. A global
// package is changed by adding a project override, {"source", "autoload": false,
// "extensions"}, which Pi applies on top of the global entry (scripts/pi
// contract scenarios 22-26).

// piProjectTarget is what a change of one view package needs.
type piProjectTarget struct {
	entry piEntry // the project entry, or for scope global the global entry
	pkg   *piPackage
	delta bool // the selection is an override on top of inherited
	// inherited is the global entry's selection, for an override that has one.
	inherited []piRowState
	hasBase   bool
	// reference is the source a new override is written with (scope global only).
	reference string
}

type piProjectState struct {
	view       *PiExtensionsView
	project    *piSettings
	global     *piSettings
	agentDir   string
	projectDir string
	targets    map[string]*piProjectTarget // by piTargetKey
}

func piTargetKey(scope string, index int) string { return scope + "/" + strconv.Itoa(index) }

// piProjectState reads the project's and the global settings together, as Pi
// merges them if it trusts the project.
func (s *Service) piProjectState(ctx context.Context, target string) (*piProjectState, error) {
	agentDir, err := s.piAgentDir("pi")
	if err != nil {
		return nil, err
	}
	projectDir := filepath.Join(s.ProjectRoot, ".pi")
	project := readPiSettings(s.ProjectRoot, filepath.Join(projectDir, "settings.json"))
	global := readPiSettings(agentDir, filepath.Join(agentDir, "settings.json"))
	version, readOnly := s.piGate(ctx, target)
	problem := project.problem
	if problem == "" {
		problem = global.problem
	}
	// Pi may read an entry Skillshare can't, and then keeps it over an earlier entry
	// or a global one of the same package; which package that is stays unknown. A
	// global entry whose source is unreadable may likewise be the first of any package.
	if problem == "" && (slices.ContainsFunc(project.entries, func(e piEntry) bool { return e.problem != "" }) ||
		slices.ContainsFunc(global.entries, func(e piEntry) bool { return e.badSource })) {
		problem = "unsupportedEntry"
	}
	if readOnly == "" && problem != "" {
		readOnly = piReadOnlySettings
	}
	v := &PiExtensionsView{
		Target: target, Scope: "project", SettingsPath: project.path, GlobalSettingsPath: global.path, Version: version,
		MinVersion: PiMinVersion, Editable: readOnly == "", ReadOnly: readOnly, Problem: problem,
		Revision: piProjectRevision(project, global),
		Packages: []PiExtensionPackage{}, Folders: []PiExtensionFolder{}, Trust: piTrust(agentDir, s.ProjectRoot, global),
	}
	st := &piProjectState{view: v, project: project, global: global, agentDir: agentDir, projectDir: projectDir, targets: map[string]*piProjectTarget{}}
	// Pi keeps the first global entry of a package, and the last project one. A first
	// entry Skillshare can't read still counts: its package is then read-only here.
	type globalPkg struct {
		entry   piEntry
		pkg     *piPackage
		rows    []piRowState
		problem string
	}
	globals := map[string]*globalPkg{}
	globalOrder := []*globalPkg{}
	unresolvedGlobal := false
	for _, e := range global.entries {
		if e.badSource {
			unresolvedGlobal = true
			continue // the view is read-only
		}
		p := openPiPackage(e.source, agentDir, projectDir, "user")
		if p.src.identity != "" && globals[p.src.identity] != nil {
			continue
		}
		g := &globalPkg{entry: e, pkg: p, problem: p.problem}
		switch {
		case e.problem != "":
			g.problem = e.problem
		case unresolvedGlobal:
			g.problem = "sourceUnknown"
		case e.autoloadFalse:
			g.problem = "autoloadGlobal"
		case p.problem == "":
			g.rows = p.evaluate(e.object, e.hasRules, e.rules)
		}
		// Anonymous sources stay visible, but cannot own or shadow a known identity.
		if p.src.identity != "" {
			globals[p.src.identity] = g
		}
		globalOrder = append(globalOrder, g)
		unresolvedGlobal = unresolvedGlobal || p.src.identity == ""
	}
	lastProject := map[string]int{}
	lastUnresolved := -1
	for _, e := range project.entries {
		if e.badSource {
			lastUnresolved = e.index
		} else if e.problem == "" {
			if id := resolvePiSource(e.source, agentDir, projectDir, "project").identity; id != "" {
				lastProject[id] = e.index
			} else {
				lastUnresolved = e.index
			}
		}
	}
	shadowed := map[string]bool{}
	for _, e := range project.entries {
		pkg := PiExtensionPackage{Index: e.index, Source: redactSource(e.source), Form: "string", Scope: "project", Install: "unknown", OtherKeys: e.otherKeys(), Rows: []PiExtensionRow{}}
		if e.object {
			pkg.Form = "object"
		}
		if e.hasRules {
			pkg.Rules = e.rules
		}
		if e.problem != "" {
			pkg.Problem = e.problem
			v.Packages = append(v.Packages, pkg)
			continue
		}
		id := resolvePiSource(e.source, agentDir, projectDir, "project").identity
		pkg.Identity = redactSource(id)
		if id != "" && lastProject[id] != e.index {
			pkg.Problem = "duplicate"
			v.Packages = append(v.Packages, pkg)
			continue
		}
		if id != "" && e.index < lastUnresolved {
			// A later unknown identity may supersede this project entry.
			pkg.Problem = "sourceUnknown"
			v.Packages = append(v.Packages, pkg)
			continue
		}
		g := globals[id]
		if g != nil {
			shadowed[id] = true
			pkg.GlobalRules = g.entry.rules
		}
		switch {
		case e.autoloadFalse && g != nil:
			pkg.Shape = "delta"
			pkg.Kind, pkg.Install, pkg.Problem = g.pkg.src.kind, g.pkg.install, g.problem
			pkg.ReadOnly = g.pkg.readOnly(true)
			if g.problem == "" {
				states := piDeltaStates(e.rules, g.pkg, g.rows, true)
				pkg.Rows = piDeltaRows(states, v.Editable && pkg.ReadOnly == "")
				st.targets[piTargetKey("project", e.index)] = &piProjectTarget{entry: e, pkg: g.pkg, delta: true, inherited: g.rows, hasBase: true}
			}
		case e.autoloadFalse:
			// With no global entry to inherit from, Pi loads only the paths it names.
			pkg.Shape = "deltaOnly"
			p := openPiPackage(e.source, agentDir, projectDir, "project")
			pkg.Kind, pkg.Install, pkg.Problem = p.src.kind, p.install, p.problem
			pkg.ReadOnly = p.readOnly(true)
			if p.problem == "" {
				pkg.Rows = piDeltaRows(piDeltaStates(e.rules, p, nil, false), v.Editable && pkg.ReadOnly == "")
				st.targets[piTargetKey("project", e.index)] = &piProjectTarget{entry: e, pkg: p, delta: true}
			}
		default:
			pkg.Shape = "projectOnly"
			if g != nil {
				pkg.Shape = "replaces"
			}
			p := openPiPackage(e.source, agentDir, projectDir, "project")
			pkg.Kind, pkg.Install, pkg.Problem = p.src.kind, p.install, p.problem
			pkg.ReadOnly = p.readOnly(e.object)
			if p.problem == "" {
				editable := v.Editable && pkg.ReadOnly == ""
				for _, r := range p.evaluate(e.object, e.hasRules, e.rules) {
					pkg.Rows = append(pkg.Rows, r.row(editable))
				}
				st.targets[piTargetKey("project", e.index)] = &piProjectTarget{entry: e, pkg: p}
			}
		}
		v.Packages = append(v.Packages, pkg)
	}
	for _, g := range globalOrder {
		id := g.pkg.src.identity
		if id != "" && shadowed[id] {
			continue
		}
		pkg := PiExtensionPackage{Index: g.entry.index, Source: redactSource(g.entry.source), Identity: redactSource(id), Kind: g.pkg.src.kind, Form: "string", Scope: "global", Shape: "global", Install: g.pkg.install, Problem: g.problem, OtherKeys: g.entry.otherKeys(), Rows: []PiExtensionRow{}}
		if g.entry.object {
			pkg.Form = "object"
		}
		if g.entry.hasRules {
			pkg.Rules = g.entry.rules
		}
		reference, readOnly := piOverrideReference(g.entry.source, g.pkg, agentDir, projectDir)
		pkg.ReadOnly = readOnly
		for _, r := range g.rows {
			pkg.Rows = append(pkg.Rows, r.row(v.Editable && pkg.Problem == "" && readOnly == ""))
		}
		if pkg.Problem == "" && readOnly == "" {
			st.targets[piTargetKey("global", g.entry.index)] = &piProjectTarget{entry: g.entry, pkg: g.pkg, delta: true, inherited: g.rows, hasBase: true, reference: reference}
		}
		v.Packages = append(v.Packages, pkg)
	}
	// Pi reads the project's folders only when it trusts the project, and the global
	// folders either way, each with the rules of its own settings file.
	for _, scope := range []struct {
		name     string
		dir      string
		settings *piSettings
	}{{"project", projectDir, project}, {"global", agentDir, global}} {
		for _, folder := range s.piFolders(scope.dir, scope.settings) {
			folder.Scope = scope.name
			v.Folders = append(v.Folders, folder)
		}
	}
	return st, nil
}

// piOverrideReference is the source a project override of a global entry is
// written with, as pi config writes it: an npm or git source as the global
// settings have it, a local one relative to the project's .pi. It must resolve to
// the global entry's own package. A source that carries credentials or a query is
// never copied into the project.
func piOverrideReference(source string, p *piPackage, agentDir, projectDir string) (string, string) {
	if p.single {
		return "", "singleFile"
	}
	if redactSource(source) != source {
		return "", "credentials"
	}
	reference := source
	if p.src.kind == "local" {
		rel, err := filepath.Rel(projectDir, p.src.install)
		if err != nil {
			return "", "reference"
		}
		reference = filepath.ToSlash(rel)
	}
	if resolvePiSource(reference, agentDir, projectDir, "project").identity != p.src.identity {
		return "", "reference"
	}
	return reference, ""
}

func piProjectRevision(project, global *piSettings) string {
	data, _ := json.Marshal([]any{project.path, project.exists, hash(project.raw), global.path, hash(global.raw)})
	return hash(data)
}

// piProjectPlan validates changes against both settings files and returns the
// plan and the project file's new bytes.
func (s *Service) piProjectPlan(ctx context.Context, target string, changes []PiExtensionChange) (*PiExtensionsPlan, *piProjectState, []byte, error) {
	if err := piCheckChanges(target, changes, s.agentOf); err != nil {
		return nil, nil, nil, err
	}
	st, err := s.piProjectState(ctx, target)
	if err != nil {
		return nil, nil, nil, err
	}
	v := st.view
	if !v.Editable {
		return nil, nil, nil, fmt.Errorf("%w (%s)", ErrPiExtensionsReadOnly, v.ReadOnly)
	}
	byTarget := map[string][]PiExtensionChange{}
	seen := map[string]bool{}
	for _, c := range changes {
		if c.Scope != "project" && c.Scope != "global" {
			return nil, nil, nil, fmt.Errorf("%w: this change is for another settings file", ErrPiExtensionsStale)
		}
		key := piTargetKey(c.Scope, c.Index)
		if seen[key+"\x00"+c.Path] {
			return nil, nil, nil, fmt.Errorf("%s is changed twice", c.Path)
		}
		seen[key+"\x00"+c.Path] = true
		i := slices.IndexFunc(v.Packages, func(p PiExtensionPackage) bool {
			return p.Scope == c.Scope && p.Index == c.Index && p.Source == c.Source
		})
		t := st.targets[key]
		if i < 0 {
			return nil, nil, nil, fmt.Errorf("%w: the package list changed", ErrPiExtensionsStale)
		}
		pkg := v.Packages[i]
		row, ok := findRow(pkg.Rows, c.Path)
		if t == nil || !ok || !row.Editable {
			return nil, nil, nil, fmt.Errorf("%s in %s can't be changed here", c.Path, pkg.Source)
		}
		// Default removes the project's own exact rules; a global rule is never touched.
		if c.Action == "default" && (c.Scope == "global" || !slices.ContainsFunc(t.entry.rules, func(r string) bool { return piExactRule(r, c.Path, path.Join(t.pkg.abs, c.Path)) })) ||
			row.File == "missing" && c.Action != "default" {
			return nil, nil, nil, fmt.Errorf("%s in %s has no rule to change that way", c.Path, pkg.Source)
		}
		byTarget[key] = append(byTarget[key], c)
	}

	raw := st.project.raw
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	value, err := hujson.Parse(raw)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w (%v)", ErrPiExtensionsReadOnly, err)
	}
	var expected map[string]any
	if err := decodeNumbers(raw, &expected); err != nil {
		return nil, nil, nil, err
	}
	_, hasPackages := expected["packages"]
	expectedPackages, _ := expected["packages"].([]any)

	plan := &PiExtensionsPlan{SettingsPath: st.project.path, Entries: []PiExtensionEntryDiff{}, Rows: []PiExtensionRowChange{}}
	var edits, removals []map[string]any
	removed := map[int]bool{}
	var created []json.RawMessage
	var createdExpected []any
	keys := make([]string, 0, len(byTarget))
	for k := range byTarget {
		keys = append(keys, k)
	}
	// Project entries first, then new overrides in global order.
	sort.Slice(keys, func(a, b int) bool {
		ta, tb := byTarget[keys[a]][0], byTarget[keys[b]][0]
		if ta.Scope != tb.Scope {
			return ta.Scope == "project"
		}
		return ta.Index < tb.Index
	})
	for _, key := range keys {
		group := byTarget[key]
		t := st.targets[key]
		e, p := t.entry, t.pkg
		scope := group[0].Scope
		current := e.rules
		if scope == "global" {
			current = nil // a new override starts with no rule of its own
		}
		rules := piEditRules(current, p.abs, group)
		hasRules := len(rules) > 0
		var before, after map[string]piRowState
		if t.delta {
			before = piEvaluateMap(piDeltaStates(current, p, t.inherited, t.hasBase))
			after = piEvaluateMap(piDeltaStates(rules, p, t.inherited, t.hasBase))
			// An override decides only the paths its exact rules name.
			if err := piGuardStates(before, after, group, true); err != nil {
				return nil, nil, nil, fmt.Errorf("%s: %w", redactSource(e.source), err)
			}
		} else {
			before = piEvaluateMap(p.evaluate(e.object, e.hasRules, e.rules))
			after = piEvaluateMap(p.evaluate(true, hasRules, rules))
			if err := piGuard(p, e, hasRules, rules, group); err != nil {
				return nil, nil, nil, fmt.Errorf("%s: %w", redactSource(e.source), err)
			}
		}
		plan.Rows = append(plan.Rows, piRowChanges(scope, e.index, p.abs, current, rules, before, after, group)...)
		diff := PiExtensionEntryDiff{Scope: scope, Index: e.index, Source: redactSource(e.source), Identity: redactSource(p.src.identity), KeptKeys: e.otherKeys()}
		if scope == "project" && e.hasRules {
			diff.Before = e.rules
		}
		if hasRules {
			diff.After = rules
		}

		if scope == "global" {
			// Built by hand in the order pi config writes it.
			src, _ := json.Marshal(t.reference)
			list, _ := json.Marshal(rules)
			created = append(created, json.RawMessage(`{"source":`+string(src)+`,"autoload":false,"extensions":`+string(list)+`}`))
			createdExpected = append(createdExpected, map[string]any{"source": t.reference, "autoload": false, "extensions": stringsToAny(rules)})
			diff.Created, diff.Reference, diff.KeptKeys = true, t.reference, []string{"autoload"}
			plan.Entries = append(plan.Entries, diff)
			continue
		}
		diff.Converted = !e.object
		pointer := "/packages/" + strconv.Itoa(e.index)
		switch {
		case t.delta && !hasRules && piOnlyOverride(e) && !piOverrideShadowsEarlier(st, e, p.src.identity):
			// Drop an exhausted override only when it cannot expose an earlier entry.
			diff.Removed, removed[e.index] = true, true
			removals = append(removals, map[string]any{"op": "remove", "path": pointer})
		case !e.object:
			src, _ := json.Marshal(e.source)
			list, _ := json.Marshal(rules)
			edits = append(edits, map[string]any{"op": "replace", "path": pointer, "value": json.RawMessage(`{"source":` + string(src) + `,"extensions":` + string(list) + `}`)})
			expectedPackages[e.index] = map[string]any{"source": e.source, "extensions": stringsToAny(rules)}
		case hasRules:
			edits = append(edits, map[string]any{"op": "add", "path": pointer + "/extensions", "value": rules})
			want, _ := expectedPackages[e.index].(map[string]any)
			want["extensions"] = stringsToAny(rules)
		case e.hasRules:
			edits = append(edits, map[string]any{"op": "remove", "path": pointer + "/extensions"})
			want, _ := expectedPackages[e.index].(map[string]any)
			delete(want, "extensions")
		}
		plan.Entries = append(plan.Entries, diff)
	}

	// Edits keep every index in place; removals go last to first; new overrides are appended.
	ops := edits
	slices.SortFunc(removals, func(a, b map[string]any) int {
		return piPointerIndex(b["path"].(string)) - piPointerIndex(a["path"].(string))
	})
	ops = append(ops, removals...)
	if len(created) > 0 && !hasPackages {
		ops = append(ops, map[string]any{"op": "add", "path": "/packages", "value": []any{}})
	}
	for _, entry := range created {
		ops = append(ops, map[string]any{"op": "add", "path": "/packages/-", "value": entry})
	}
	if len(ops) > 0 {
		patch, _ := json.Marshal(ops)
		if err := value.Patch(patch); err != nil {
			return nil, nil, nil, err
		}
	}
	if hasPackages || len(created) > 0 {
		kept := []any{}
		for i, p := range expectedPackages {
			if !removed[i] {
				kept = append(kept, p)
			}
		}
		expected["packages"] = append(kept, createdExpected...)
	}
	out := value.Pack()
	if len(st.project.raw) == 0 {
		// A new file is written the way Pi writes its settings.
		var indented bytes.Buffer
		if err := json.Indent(&indented, out, "", "  "); err != nil {
			return nil, nil, nil, err
		}
		out = append(indented.Bytes(), '\n')
	}
	var got map[string]any
	if !json.Valid(out) || decodeNumbers(out, &got) != nil || !reflect.DeepEqual(got, expected) {
		return nil, nil, nil, errors.New("the edited settings would differ beyond the planned extension rules; nothing was written")
	}
	sorted := slices.Clone(changes)
	sort.Slice(sorted, func(a, b int) bool {
		return piTargetKey(sorted[a].Scope, sorted[a].Index)+"\x00"+sorted[a].Path < piTargetKey(sorted[b].Scope, sorted[b].Index)+"\x00"+sorted[b].Path
	})
	data, _ := json.Marshal([]any{target, "project", v.Revision, sorted, plan.Entries, plan.Rows})
	plan.Revision = hash(data)
	return plan, st, out, nil
}

// Keep an empty override when deletion could expose a shadowed registration.
// Retaining the winning entry preserves other resources without guessing the
// identity of an earlier unresolved source.
func piOverrideShadowsEarlier(st *piProjectState, e piEntry, identity string) bool {
	for _, earlier := range st.project.entries[:e.index] {
		id := resolvePiSource(earlier.source, st.agentDir, st.projectDir, "project").identity
		if earlier.badSource || id == "" || id == identity {
			return true
		}
	}
	return false
}

// piOnlyOverride reports whether an override entry has nothing but its source,
// autoload: false and its extensions.
func piOnlyOverride(e piEntry) bool {
	for k := range e.fields {
		if k != "source" && k != "autoload" && k != "extensions" {
			return false
		}
	}
	return e.autoloadFalse
}

func piPointerIndex(pointer string) int {
	n, _ := strconv.Atoi(pointer[len("/packages/"):])
	return n
}

// applyPiProject writes a previewed project change. A preview that is already
// stale is refused before anything is created. The .pi folder is created only
// then, and removed again if nothing was written; a new settings file is linked
// into place, so one that appears meanwhile is never overwritten. Every write goes
// through an os.Root at the project, so no link or path leads outside it.
func (s *Service) applyPiProject(ctx context.Context, target string, changes []PiExtensionChange, revision string) (_ *PiExtensionsPlan, err error) {
	if plan, _, _, err := s.piProjectPlan(ctx, target, changes); err != nil {
		return nil, err
	} else if plan.Revision != revision {
		return nil, ErrPiExtensionsStale
	}
	root, err := os.OpenRoot(s.ProjectRoot)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file := filepath.Join(s.ProjectRoot, ".pi", "settings.json")
	if err := noSymlink(s.ProjectRoot, file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPiExtensionsReadOnly, err)
	}
	info, err := root.Lstat(".pi")
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := root.Mkdir(".pi", 0o755); err != nil {
			return nil, err
		}
		defer func() {
			if err != nil {
				_ = root.Remove(".pi") // only while still empty
			}
		}()
	case err != nil:
		return nil, err
	case !info.IsDir():
		return nil, fmt.Errorf("%w: %s is not a folder", ErrPiExtensionsReadOnly, filepath.Join(s.ProjectRoot, ".pi"))
	}
	release, native, err := s.lockPiSettings(file, false)
	if err != nil {
		return nil, err
	}
	defer release()
	plan, st, after, err := s.piProjectPlan(ctx, target, changes)
	if err != nil {
		return nil, err
	}
	if plan.Revision != revision {
		return nil, ErrPiExtensionsStale
	}
	mode := os.FileMode(0o644)
	if st.project.exists {
		info, err := root.Stat(".pi/settings.json")
		if err != nil {
			return nil, ErrPiExtensionsStale
		}
		mode = info.Mode().Perm()
	}
	if err := native.verify(); err != nil {
		return nil, err
	}
	id, discard, err := s.piBackup(file, st.project.raw, after, plan)
	if err != nil {
		return nil, fmt.Errorf("backup failed; nothing was written: %w", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, discard())
		}
	}()
	piBeforeWrite(native.path)
	// Only the project file is locked; the global settings and the packages are
	// checked again right before the write.
	if final, _, _, err := s.piProjectPlan(ctx, target, changes); err != nil || final.Revision != revision {
		return nil, ErrPiExtensionsStale
	}
	current, err := root.ReadFile(".pi/settings.json")
	switch {
	case st.project.exists && (err != nil || !bytes.Equal(current, st.project.raw)),
		!st.project.exists && !errors.Is(err, os.ErrNotExist):
		return nil, ErrPiExtensionsStale
	}
	if err := native.verify(); err != nil {
		return nil, err
	}
	if err := piWriteProject(root, ".pi/settings.json", after, mode, !st.project.exists); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, ErrPiExtensionsStale
		}
		return nil, err
	}
	plan.BackupID = id
	return plan, nil
}

// rootAtomicWrite puts data at name under root through a temporary file in the
// same folder: a new file is linked into place and is never written over one
// that appeared, an existing one is replaced.
func rootAtomicWrite(root *os.Root, name string, data []byte, mode os.FileMode, create bool) error {
	temp := filepath.Join(filepath.Dir(name), ".skillshare-write-"+strconv.FormatInt(time.Now().UnixNano(), 36))
	f, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(temp) }()
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if create {
		return root.Link(temp, name)
	}
	return root.Rename(temp, name)
}
