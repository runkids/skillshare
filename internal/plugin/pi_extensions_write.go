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

	"github.com/gofrs/flock"
	"github.com/tailscale/hujson"
)

var (
	// ErrPiExtensionsStale means settings.json changed after the preview; nothing was written.
	ErrPiExtensionsStale = errors.New("Pi settings changed since the preview; review again")
	// ErrPiExtensionsBusy means Pi or another Skillshare operation holds a lock; nothing was written.
	ErrPiExtensionsBusy = errors.New("Pi settings are being changed by another process; try again")
	// ErrPiExtensionsReadOnly means this target's extension selection cannot be changed here.
	ErrPiExtensionsReadOnly = errors.New("this target's extension selection is read-only")
)

// maxPiExtensionChanges bounds one request; a target lists far fewer extensions.
const maxPiExtensionChanges = 500

// PiExtensionChange sets one extension of one entry: "select" or "exclude" writes
// an exact +path or -path rule, "default" removes the exact rules for the path.
// Source is the entry's source as the view showed it, so a reordered file is refused.
// Scope is the settings file the entry is in, as the view showed it: global, or in
// a project view also project. A global entry changed from a project view gets a
// project override; the global file is never written there.
type PiExtensionChange struct {
	Scope  string `json:"scope,omitempty"`
	Index  int    `json:"index"`
	Source string `json:"source"`
	Path   string `json:"path"`
	Action string `json:"action"`
}

// PiExtensionsPlan is a preview: per touched entry, its extensions list before and
// after (null when the entry has none), and per changed extension, its selection.
// It never carries the rest of the file.
type PiExtensionsPlan struct {
	Revision     string                 `json:"revision"`
	SettingsPath string                 `json:"settingsPath"`
	Entries      []PiExtensionEntryDiff `json:"entries"`
	Rows         []PiExtensionRowChange `json:"rows"`
	BackupID     string                 `json:"backupId,omitempty"`
}

type PiExtensionEntryDiff struct {
	Scope     string   `json:"scope,omitempty"`
	Index     int      `json:"index"`
	Source    string   `json:"source"`
	Identity  string   `json:"identity"` // the package, as Pi tells packages apart
	Before    []string `json:"before"`
	After     []string `json:"after"`
	Converted bool     `json:"converted"` // a string entry becomes {"source": ...}
	KeptKeys  []string `json:"keptKeys"`
	// In a project: Created is a new override of a global package, written with
	// Reference as its source; Removed is an override left with no rule, deleted.
	Created   bool   `json:"created,omitempty"`
	Removed   bool   `json:"removed,omitempty"`
	Reference string `json:"reference,omitempty"`
}

type PiExtensionRowChange struct {
	Scope   string   `json:"scope,omitempty"`
	Index   int      `json:"index"`
	Path    string   `json:"path"`
	Before  string   `json:"before"`
	After   string   `json:"after"`
	Removed []string `json:"removed"`
	Added   []string `json:"added"`
}

// PreviewPiExtensions computes the change without touching any file.
func (s *Service) PreviewPiExtensions(ctx context.Context, target string, changes []PiExtensionChange) (*PiExtensionsPlan, error) {
	if s.ProjectRoot != "" {
		plan, _, _, err := s.piProjectPlan(ctx, target, changes)
		return plan, err
	}
	plan, _, _, err := s.piPlan(ctx, target, changes)
	return plan, err
}

// ApplyPiExtensions writes a previewed change. Under Skillshare's plugin lock and
// Pi's own settings lock it rereads the file, refuses it if its bytes are not the
// previewed ones, and replaces it atomically.
func (s *Service) ApplyPiExtensions(ctx context.Context, target string, changes []PiExtensionChange, revision string) (_ *PiExtensionsPlan, err error) {
	if revision == "" {
		return nil, errors.New("preview the extension changes before applying")
	}
	if s.ProjectRoot != "" {
		return s.applyPiProject(ctx, target, changes, revision)
	}
	agentDir, err := s.piAgentDir(target)
	if err != nil {
		return nil, err
	}
	file := filepath.Join(agentDir, "settings.json")
	if err := noSymlink(agentDir, file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPiExtensionsReadOnly, err)
	}
	release, native, err := s.lockPiSettings(file, true)
	if err != nil {
		return nil, err
	}
	defer release()
	plan, st, after, err := s.piPlan(ctx, target, changes)
	if err != nil {
		return nil, err
	}
	if plan.Revision != revision {
		return nil, ErrPiExtensionsStale
	}
	info, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	if err := native.verify(); err != nil {
		return nil, err
	}
	id, discard, err := s.piBackup(file, st.settings.raw, after, plan)
	if err != nil {
		return nil, fmt.Errorf("backup failed; nothing was written: %w", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, discard())
		}
	}()
	piBeforeWrite(native.path)
	// The settings file is locked, the package is not: check it again right before the write.
	if final, _, _, err := s.piPlan(ctx, target, changes); err != nil || final.Revision != revision {
		return nil, ErrPiExtensionsStale
	}
	current, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(current, st.settings.raw) {
		return nil, ErrPiExtensionsStale
	}
	// Pi takes over a lock it finds stale; write only while Pi still sees ours as held.
	if err := native.verify(); err != nil {
		return nil, err
	}
	if err := piWriteGlobal(agentDir, file, after, info.Mode().Perm()); err != nil {
		return nil, err
	}
	plan.BackupID = id
	return plan, nil
}

// piBeforeWrite runs just before the final checks; tests use it to take the lock
// away, or change a file, mid-apply.
var piBeforeWrite = func(lockDir string) {}

// Write seams let tests inject a failed commit after all pre-write checks.
var piWriteGlobal = atomicNativeWrite
var piWriteProject = rootAtomicWrite

// lockPiSettings takes, in order, Skillshare's plugin lock, a cross-process flock
// on the settings file, and Pi's own lock: the settings.json.lock directory that
// proper-lockfile creates. Fresh foreign locks refuse the write; only unchanged
// empty stale directories can be reclaimed. Without fileLock there is no flock:
// a project's .pi is in its repository, where the lock file would stay behind,
// and Pi's lock directory already shuts out every other writer.
func (s *Service) lockPiSettings(file string, fileLock bool) (func(), *piNativeLock, error) {
	if err := os.MkdirAll(filepath.Dir(s.ConfigPath), 0o755); err != nil {
		return nil, nil, err
	}
	pluginLock := s.ConfigPath + ".plugins.lock"
	f, err := os.OpenFile(pluginLock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("%w (another plugin operation holds %s)", ErrPiExtensionsBusy, pluginLock)
	}
	_ = f.Close()
	releases := []func(){func() { _ = os.Remove(pluginLock) }}
	release := func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}
	if fileLock {
		fl := flock.New(file + ".skillshare-plugin.lock")
		if ok, err := fl.TryLock(); err != nil || !ok {
			release()
			return nil, nil, ErrPiExtensionsBusy
		}
		releases = append(releases, func() { _ = fl.Unlock() })
	}
	native, err := acquirePiNativeLock(file + ".lock")
	if err != nil {
		release()
		return nil, nil, err
	}
	releases = append(releases, native.release)
	return release, native, nil
}

// piPlan validates changes against the current file and returns the plan and the
// new file bytes.
func (s *Service) piPlan(ctx context.Context, target string, changes []PiExtensionChange) (*PiExtensionsPlan, *piTargetState, []byte, error) {
	if err := piCheckChanges(target, changes, s.agentOf); err != nil {
		return nil, nil, nil, err
	}
	st, err := s.piGlobalState(ctx, target)
	if err != nil {
		return nil, nil, nil, err
	}
	v := st.view
	if !v.Editable {
		return nil, nil, nil, fmt.Errorf("%w (%s)", ErrPiExtensionsReadOnly, v.ReadOnly)
	}
	byEntry := map[int][]PiExtensionChange{}
	seen := map[string]bool{}
	for _, c := range changes {
		key := strconv.Itoa(c.Index) + "\x00" + c.Path
		if seen[key] {
			return nil, nil, nil, fmt.Errorf("%s is changed twice", c.Path)
		}
		seen[key] = true
		if c.Scope != "" && c.Scope != "global" {
			return nil, nil, nil, fmt.Errorf("%w: this change is for another settings file", ErrPiExtensionsStale)
		}
		if c.Index < 0 || c.Index >= len(v.Packages) || v.Packages[c.Index].Source != c.Source {
			return nil, nil, nil, fmt.Errorf("%w: the package list changed", ErrPiExtensionsStale)
		}
		pkg := v.Packages[c.Index]
		row, ok := findRow(pkg.Rows, c.Path)
		if !ok || !row.Editable {
			return nil, nil, nil, fmt.Errorf("%s in %s can't be changed here", c.Path, pkg.Source)
		}
		if row.File == "missing" && c.Action != "default" || c.Action == "default" && row.Rule == "" {
			return nil, nil, nil, fmt.Errorf("%s in %s has no rule to change that way", c.Path, pkg.Source)
		}
		byEntry[c.Index] = append(byEntry[c.Index], c)
	}
	plan := &PiExtensionsPlan{Revision: v.Revision, SettingsPath: st.settings.path, Entries: []PiExtensionEntryDiff{}, Rows: []PiExtensionRowChange{}}
	value, err := hujson.Parse(st.settings.raw)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w (%v)", ErrPiExtensionsReadOnly, err)
	}
	var expected map[string]any
	if err := decodeNumbers(st.settings.raw, &expected); err != nil {
		return nil, nil, nil, err
	}
	expectedPackages, _ := expected["packages"].([]any)
	indexes := make([]int, 0, len(byEntry))
	for i := range byEntry {
		indexes = append(indexes, i)
	}
	sort.Ints(indexes)
	for _, i := range indexes {
		e := st.settings.entries[i]
		p := st.packages[i]
		rules := piEditRules(e.rules, p.abs, byEntry[i])
		// An empty list would turn every extension off; with no rule left the key goes.
		hasRules := len(rules) > 0
		if err := piGuard(p, e, hasRules, rules, byEntry[i]); err != nil {
			return nil, nil, nil, fmt.Errorf("%s: %w", redactSource(e.source), err)
		}
		before := piEvaluateMap(p.evaluate(e.object, e.hasRules, e.rules))
		after := piEvaluateMap(p.evaluate(true, hasRules, rules))
		plan.Rows = append(plan.Rows, piRowChanges("", i, p.abs, e.rules, rules, before, after, byEntry[i])...)
		diff := PiExtensionEntryDiff{Index: i, Source: redactSource(e.source), Identity: redactSource(p.src.identity), Converted: !e.object, KeptKeys: e.otherKeys()}
		if e.hasRules {
			diff.Before = e.rules
		}
		if hasRules {
			diff.After = rules
		}
		plan.Entries = append(plan.Entries, diff)

		// Edit the file in place: only this entry's extensions value, or the string
		// entry itself, changes; every other byte stays as written.
		var op map[string]any
		var want map[string]any
		pointer := "/packages/" + strconv.Itoa(i)
		switch {
		case !e.object:
			// Built by hand so "source" stays first, as Pi itself writes the entry.
			src, _ := json.Marshal(e.source)
			list, _ := json.Marshal(rules)
			op = map[string]any{"op": "replace", "path": pointer, "value": json.RawMessage(`{"source":` + string(src) + `,"extensions":` + string(list) + `}`)}
			want = map[string]any{"source": e.source, "extensions": stringsToAny(rules)}
		case hasRules:
			op = map[string]any{"op": "add", "path": pointer + "/extensions", "value": rules}
			want, _ = expectedPackages[i].(map[string]any)
			want["extensions"] = stringsToAny(rules)
		default:
			want, _ = expectedPackages[i].(map[string]any)
			delete(want, "extensions")
			if !e.hasRules {
				continue
			}
			op = map[string]any{"op": "remove", "path": pointer + "/extensions"}
		}
		patch, _ := json.Marshal([]map[string]any{op})
		if err := value.Patch(patch); err != nil {
			return nil, nil, nil, err
		}
		expectedPackages[i] = want
	}
	plan.Revision = piPlanRevision(target, st.settings, changes, plan)
	out := value.Pack()
	// The result must be strict JSON that differs from the original only where planned.
	var got map[string]any
	if !json.Valid(out) || decodeNumbers(out, &got) != nil || !reflect.DeepEqual(got, expected) {
		return nil, nil, nil, errors.New("the edited settings would differ beyond the planned extension rules; nothing was written")
	}
	return plan, st, out, nil
}

// piGuard refuses a change unless every changed extension ends as requested and
// every other extension of the entry keeps its selection.
func piGuard(p *piPackage, e piEntry, hasRules bool, rules []string, changes []PiExtensionChange) error {
	switch p.readOnly(e.object) {
	case "singleFile":
		return errors.New("Pi loads this file as it is and ignores filters")
	case "otherResources":
		return errors.New("a first rule makes this entry an object, and Skillshare can't show that the package's skills, prompts and themes would stay as they are; change it with pi config")
	}
	before := piEvaluateMap(p.evaluate(e.object, e.hasRules, e.rules))
	after := piEvaluateMap(p.evaluate(true, hasRules, rules))
	// Exact rules decide only their own path, so an unknown row stays as unknown as
	// it was, unless the list itself appears or disappears.
	return piGuardStates(before, after, changes, e.hasRules && len(e.rules) > 0 && hasRules)
}

// piGuardStates refuses unless every changed path ends as requested and every
// other path keeps its state; unknown stays acceptable only when local.
func piGuardStates(before, after map[string]piRowState, changes []PiExtensionChange, local bool) error {
	changed := map[string]string{}
	for _, c := range changes {
		changed[c.Path] = c.Action
	}
	for rel, b := range before {
		if action, ok := changed[rel]; ok {
			a, present := after[rel]
			switch {
			case action == "select" && (!present || a.eval.state != piOn),
				action == "exclude" && (!present || a.eval.state != piOff),
				action == "default" && b.present && (!present || a.eval.state == piUnknown):
				return errors.New("Pi would not apply this selection as requested")
			}
			continue
		}
		a, present := after[rel]
		if !present || a.present != b.present || a.eval.state != b.eval.state || (a.eval.state == piUnknown && !local) {
			return errors.New("this change would also change other extensions of the package; change it with pi config")
		}
	}
	for rel := range after {
		if _, ok := before[rel]; !ok {
			if _, ok := changed[rel]; !ok {
				return errors.New("this change would also change other extensions of the package; change it with pi config")
			}
		}
	}
	return nil
}

// piCheckChanges checks what a request can be checked for without any file.
func piCheckChanges(target string, changes []PiExtensionChange, agentOf func(string) string) error {
	if agentOf(target) != "pi" {
		return errors.New("not a Pi target")
	}
	if len(changes) == 0 || len(changes) > maxPiExtensionChanges {
		return errors.New("choose at least one extension to change")
	}
	for _, c := range changes {
		if c.Action != "select" && c.Action != "exclude" && c.Action != "default" {
			return fmt.Errorf("unknown extension action %q", c.Action)
		}
	}
	return nil
}

// piEditRules replaces each changed path's exact rules with the requested one;
// "default" only removes them.
func piEditRules(rules []string, pkgAbs string, changes []PiExtensionChange) []string {
	rules = append([]string{}, rules...)
	for _, c := range changes {
		kept := rules[:0:0]
		abs := path.Join(pkgAbs, c.Path)
		for _, r := range rules {
			if !piExactRule(r, c.Path, abs) {
				kept = append(kept, r)
			}
		}
		rules = kept
		switch c.Action {
		case "select":
			rules = append(rules, "+"+c.Path)
		case "exclude":
			rules = append(rules, "-"+c.Path)
		}
	}
	return rules
}

func piRowChanges(scope string, index int, pkgAbs string, rulesBefore, rulesAfter []string, before, after map[string]piRowState, changes []PiExtensionChange) []PiExtensionRowChange {
	rows := []PiExtensionRowChange{}
	for _, c := range changes {
		abs := path.Join(pkgAbs, c.Path)
		removed, added := []string{}, []string{}
		for _, r := range rulesBefore {
			if piExactRule(r, c.Path, abs) {
				removed = append(removed, r)
			}
		}
		if c.Action != "default" {
			added = append(added, rulesAfter[slicesIndexRule(rulesAfter, c.Path, abs)])
		}
		rows = append(rows, PiExtensionRowChange{Scope: scope, Index: index, Path: c.Path, Before: rowState(before, c.Path), After: rowState(after, c.Path), Removed: removed, Added: added})
	}
	return rows
}

func piEvaluateMap(rows []piRowState) map[string]piRowState {
	m := map[string]piRowState{}
	for _, r := range rows {
		m[r.rel] = r
	}
	return m
}

func rowState(m map[string]piRowState, rel string) string {
	r, ok := m[rel]
	if !ok || !r.present {
		return "none"
	}
	return r.eval.state.String()
}

func findRow(rows []PiExtensionRow, rel string) (PiExtensionRow, bool) {
	for _, r := range rows {
		if r.Path == rel {
			return r, true
		}
	}
	return PiExtensionRow{}, false
}

func slicesIndexRule(rules []string, rel, abs string) int {
	for i := len(rules) - 1; i >= 0; i-- {
		if piExactRule(rules[i], rel, abs) {
			return i
		}
	}
	return len(rules) - 1
}

func stringsToAny(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

func decodeNumbers(data []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	return d.Decode(v)
}

// piBackup records the touched extension lists, not the rest of the file, so a
// selection can be restored by hand without copying unrelated settings.
func (s *Service) piBackup(file string, before, after []byte, plan *PiExtensionsPlan) (string, func() error, error) {
	dir := filepath.Join(s.StateDir, "pi-extensions", "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", nil, err
	}
	suffix := hash([]byte(file))[:8]
	id := fmt.Sprintf("%d-%s", time.Now().UnixNano(), suffix)
	data, err := json.MarshalIndent(map[string]any{"path": file, "before": hash(before), "after": hash(after), "entries": plan.Entries}, "", "  ")
	if err != nil {
		return "", nil, err
	}
	// Successful records are never pruned. The caller discards only this new
	// record if the settings write fails, including a late stale/lock refusal.
	record := filepath.Join(dir, id+".json")
	if err := atomicNativeWrite(s.StateDir, record, append(data, '\n'), 0o600); err != nil {
		return "", nil, err
	}
	discard := func() error {
		if err := os.Remove(record); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("could not remove the unapplied extension record: %w", err)
		}
		return nil
	}
	return id, discard, nil
}

// piPlanRevision binds a preview to its target, settings file and bytes, the
// requested changes and the planned result, so an apply never writes a diff that
// was not the one previewed, even when the discovered files changed meanwhile.
func piPlanRevision(target string, settings *piSettings, changes []PiExtensionChange, plan *PiExtensionsPlan) string {
	sorted := slices.Clone(changes)
	sort.Slice(sorted, func(a, b int) bool {
		if sorted[a].Index != sorted[b].Index {
			return sorted[a].Index < sorted[b].Index
		}
		return sorted[a].Path < sorted[b].Path
	})
	data, _ := json.Marshal([]any{target, settings.path, hash(settings.raw), sorted, plan.Entries, plan.Rows})
	return hash(data)
}
