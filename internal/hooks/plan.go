package hooks

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"skillshare/internal/projectdir"
)

// record is Skillshare's ownership of one native output, kept outside native files.
// An array element is identified by this record plus its content hash, with the last
// written index only breaking ties between identical elements; neither index nor
// equality alone ever claims an element.
type record struct {
	Owner   string `json:"owner"`
	Target  string `json:"target"`
	Path    string `json:"path"`
	Root    string `json:"root,omitempty"`
	Entry   string `json:"entry"`
	Event   string `json:"event,omitempty"`
	Ordinal int    `json:"ordinal,omitempty"`
	Index   int    `json:"index,omitempty"`
	// Peers is how many identical elements the event held when Skillshare last wrote or
	// synced it. A different count now means a copy was added or deleted and nobody can
	// tell whose, so the record no longer locates any of them.
	Peers     int      `json:"peers,omitempty"`
	Hash      string   `json:"hash"`
	GitEvents []string `json:"gitEvents,omitempty"`
	// Adopted marks a registration an import claimed without syncing; the next sync
	// reports it as adopted once, then as unchanged.
	Adopted bool `json:"adopted,omitempty"`
}

type ledger struct {
	Version int               `json:"version"`
	Records map[string]record `json:"records"`
	// Created are shared files whose hooks key Skillshare added, so it removes the key
	// again when it removes the last hook there.
	Created map[string]bool `json:"created,omitempty"`
	// NewFiles are shared files that did not exist before Skillshare wrote them, so
	// it deletes them once they hold nothing but what Skillshare put there.
	NewFiles map[string]bool `json:"newFiles,omitempty"`
	// Dirs are folders Skillshare created for its files, removed again once empty.
	Dirs map[string]bool `json:"dirs,omitempty"`
}

func elementKey(owner, root, path, event, entry string, ordinal int) string {
	return digest(fmt.Appendf(nil, "element\x00%s\x00%s\x00%s\x00%s\x00%s\x00%d", owner, root, path, event, entry, ordinal))
}

// fileKey is one whole file; a path has at most one owner.
func fileKey(path string) string { return digest([]byte("file\x00" + path)) }

// ownerGone reports a missing owning config, the one case where it can never
// release its outputs and an explicit replace may take them over.
func ownerGone(path string) bool {
	_, err := os.Lstat(path)
	return os.IsNotExist(err)
}

const (
	kindJSON = "json"
	kindFile = "file"
)

type filePlan struct {
	gitService         *Service
	include            *gitIncludeOp
	gitSection         *gitSectionOp
	gitSectionAfter    string
	gitGuard           *gitGuard
	gitCreatedDirs     []string
	path, target, root string
	agent              string
	// base is where no symlink may redirect the write; apply checks it again.
	base          string
	kind          string
	before, after []byte
	exists        bool
	mode          os.FileMode
	// section identifies what apply rechecks: the hooks of a shared file, or a whole file.
	section string
	ops     []elementOp
	// write is a whole file's new content; remove deletes it.
	write  []byte
	remove bool
	// keys are the ledger records this file's changes move.
	keys []string
	// created drops the hooks key once the edit empties it, because Skillshare added it.
	created bool
}

func (f *filePlan) changed() bool {
	if f.kind == kindGitInclude {
		return f.include != nil
	}
	if f.kind == kindGitSection {
		return f.gitSection != nil
	}
	if f.kind == kindJSON {
		return len(f.ops) > 0
	}
	return f.write != nil || f.remove
}

type wantElement struct {
	target, path, root, entry, event string
	ordinal                          int
	value                            any
}

type wantFile struct {
	target, path, root, entry string
	content                   []byte
	mode                      os.FileMode
}

type desired struct {
	git        map[string]gitWant
	gitSkipped map[string]bool
	elements   map[string][]wantElement // by path
	targets    map[string]string        // path -> target
	roots      map[string]string        // path -> root
	files      map[string]wantFile
	warnings   []string
	notes      []Change
}

func (d *desired) addOwned(r record) {
	if r.Event != "" {
		if _, ok := d.targets[r.Path]; !ok {
			d.targets[r.Path], d.roots[r.Path] = r.Target, r.Root
		}
	} else if _, ok := d.files[r.Path]; !ok {
		d.files[r.Path] = wantFile{target: r.Target, path: r.Path, root: r.Root, entry: r.Entry}
	}
}

func (s *Service) scoped(root string) *Service {
	c := *s
	c.ProjectRoot = root
	return &c
}

func (s *Service) scopedFor(root string) *Service {
	if root == "" {
		return s
	}
	return s.scoped(root)
}

// ownConfig reports a root with its own Skillshare config, which the global config
// cannot manage.
func ownConfig(root string) bool {
	_, ok := projectdir.Find(root)
	return ok
}

// render builds every native output the source asks for. A global source's projects
// land in the same plan, so one revision and one owner cover every root.
func (s *Service) render(source *Source) (*desired, error) {
	d := &desired{elements: map[string][]wantElement{}, targets: map[string]string{}, roots: map[string]string{}, files: map[string]wantFile{}}
	if s.ProjectRoot != "" && len(source.Projects) > 0 {
		return nil, fmt.Errorf("hooks.projects belongs in the global config")
	}
	if err := s.renderScope(d, "", source.Entries); err != nil {
		return nil, err
	}
	for _, root := range sortedKeys(source.Projects) {
		if ownConfig(root) {
			d.notes = append(d.notes, Change{Path: root, Root: root, Action: "conflict", Message: "project has its own Skillshare config; manage its hooks there with -p"})
			continue
		}
		if err := s.scoped(root).renderScope(d, root, source.Projects[root].Entries); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func (s *Service) renderScope(d *desired, root string, entries map[string]Entry) error {
	if err := s.renderGit(d, root, entries); err != nil {
		return err
	}
	dirOwner := map[string]string{}
	missing := map[string]bool{}
	addFile := func(f wantFile) error {
		if _, dup := d.files[f.path]; dup {
			return fmt.Errorf("two hooks write %s", f.path)
		}
		d.files[f.path] = f
		return nil
	}
	for _, name := range sortedKeys(entries) {
		entry := entries[name]
		if !entry.IsEnabled() {
			continue
		}
		for _, target := range sortedKeys(entry.Bindings) {
			if target == "git" {
				continue
			}
			b := entry.Bindings[target]
			if s.accountMissing(target) {
				if !missing[target] {
					d.warnings = append(d.warnings, fmt.Sprintf("target %s: config_dir %s does not exist on this machine; its hook bindings are skipped", target, s.Accounts[target].Dir))
					missing[target] = true
				}
				continue
			}
			sc, agent := s.forTarget(target)
			dir, err := sc.configDir(agent)
			if err != nil {
				return err
			}
			canonical := canonicalPath(dir)
			if previous := dirOwner[canonical]; previous != "" && previous != target {
				hint := "give each account its own config_dir"
				if env := configDirEnv[s.agentOf(previous)]; env != "" && env == configDirEnv[agent] && previous == s.agentOf(previous) && target == agent {
					// Two built-in Agents share one directory only through their common variable.
					hint = "both read " + env + "; unset it or bind only one of them"
				}
				return fmt.Errorf("hooks: %s and %s both write %s; %s", previous, target, canonical, hint)
			}
			dirOwner[canonical] = target
			def, _ := targetDef(agent)
			if def.Kind == KindCode || agent == "copilot" {
				path, err := sc.codePath(agent, name)
				if err != nil {
					return err
				}
				content := []byte(b.Code)
				if agent == "copilot" {
					data, err := json.MarshalIndent(map[string]any{"version": 1, "hooks": b.Events}, "", "  ")
					if err != nil {
						return err
					}
					content = append(data, '\n')
				}
				if err := addFile(wantFile{target: target, path: path, root: root, entry: name, content: content, mode: 0644}); err != nil {
					return err
				}
			} else {
				path, err := sc.nativePath(agent)
				if err != nil {
					return err
				}
				d.targets[path], d.roots[path] = target, root
				if agent == "antigravity" {
					// One named block per hook, holding its whole event map.
					d.elements[path] = append(d.elements[path], wantElement{target: target, path: path, root: root, entry: name, event: name, value: b.Events})
					continue
				}
				for _, event := range sortedKeys(b.Events) {
					for i, value := range b.Events[event].([]any) {
						d.elements[path] = append(d.elements[path], wantElement{target: target, path: path, root: root, entry: name, event: event, ordinal: i, value: value})
					}
				}
			}
			if len(b.Files) == 0 {
				continue
			}
			dir, err = sc.scriptDir(agent, name)
			if err != nil {
				return err
			}
			for _, file := range sortedKeys(b.Files) {
				if err := addFile(wantFile{target: target, path: filepath.Join(dir, file), root: root, entry: name, content: []byte(b.Files[file]), mode: 0755}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// base is the directory below which no symlink may redirect a write.
func (s *Service) base(target, root string) (string, error) {
	if target == "git" {
		d, err := s.scopedFor(root).gitDestination(root)
		return d.base, err
	}
	if root != "" {
		return root, nil
	}
	if s.ProjectRoot != "" {
		return s.ProjectRoot, nil
	}
	sc, agent := s.forTarget(target)
	return sc.configDir(agent)
}

// checkPath rejects symlinks between base and path, so a link can never send a
// write outside the Agent's folder or the project. base itself may be a link.
func checkPath(base, path string) error {
	rel, err := filepath.Rel(base, path)
	if err != nil || !filepath.IsLocal(rel) {
		return fmt.Errorf("hooks path %s is outside %s", path, base)
	}
	current := base
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write through symlink %s", current)
		}
	}
	return nil
}

func (s *Service) loadLedger() (ledger, []byte, error) {
	state := ledger{Version: 1, Records: map[string]record{}}
	pending, written, err := s.readPending()
	if err != nil {
		return state, nil, err
	}
	if written {
		data, err := json.MarshalIndent(pending.State, "", "  ")
		return pending.State, append(data, '\n'), err
	}
	data, exists, _, err := safeRead(s.statePath())
	if err != nil {
		return state, nil, err
	}
	if exists {
		if json.Unmarshal(data, &state) != nil || state.Version != 1 || state.Records == nil {
			return state, nil, fmt.Errorf("hooks ownership state is invalid; restore a backup or import hooks again")
		}
	}
	return state, data, nil
}

// locate finds each record's element: same event, same content hash, never two records
// on one element. Identical elements are resolved only while their number matches both
// the records holding that content and the count at the last write or sync; any copy
// added or removed since makes identity ambiguous, so none of them is located and the
// plan reports a conflict instead of guessing by position. The last written index only
// orders records among identical elements that all belong to Skillshare.
func locate(doc *nativeDoc, records map[string]record) map[string]int {
	found := map[string]int{}
	taken := map[string]map[int]bool{}
	keys := sortedKeys(records)
	slices.SortStableFunc(keys, func(a, b string) int { return records[a].Index - records[b].Index })
	hashes := map[string][]string{}
	counts := map[string]int{}
	for event, items := range doc.events {
		for _, item := range items {
			h := elementHash(item)
			hashes[event] = append(hashes[event], h)
			counts[event+"\x00"+h]++
		}
	}
	holders := map[string]int{}
	for _, r := range records {
		holders[r.Event+"\x00"+r.Hash]++
	}
	for _, key := range keys {
		r := records[key]
		group := r.Event + "\x00" + r.Hash
		if counts[group] != holders[group] || r.Peers != 0 && counts[group] != r.Peers {
			continue
		}
		best := -1
		for i, h := range hashes[r.Event] {
			if h != r.Hash || taken[r.Event][i] {
				continue
			}
			if best < 0 || abs(i-r.Index) < abs(best-r.Index) {
				best = i
			}
		}
		if best >= 0 {
			if taken[r.Event] == nil {
				taken[r.Event] = map[int]bool{}
			}
			taken[r.Event][best] = true
			found[key] = best
		}
	}
	return found
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

type planner struct {
	s       *Service
	source  *Source
	p       *Plan
	replace map[string]bool // root + "\x00" + entry
	adopt   map[string]bool // root + "\x00" + entry
	parked  map[string]bool // owned outputs whose former home must remain untouched
	changes map[string]*Change
	order   []string
	// events is each change's event detail: event -> added, updated or removed.
	events map[string]map[string]string
}

func changeKey(target, path, root, entry string) string {
	return target + "\x00" + path + "\x00" + root + "\x00" + entry
}

// event records what one change does to one event; "updated" wins over the others.
func (pl *planner) event(target, path, root, entry, event, kind string) {
	key := changeKey(target, path, root, entry)
	if pl.events[key] == nil {
		pl.events[key] = map[string]string{}
	}
	if pl.events[key][event] != "updated" {
		pl.events[key][event] = kind
	}
}

func (pl *planner) eventChanges(key string) *EventChanges {
	events := pl.events[key]
	if len(events) == 0 {
		return nil
	}
	out := &EventChanges{}
	for _, event := range sortedKeys(events) {
		switch events[event] {
		case "added":
			out.Added = append(out.Added, event)
		case "removed":
			out.Removed = append(out.Removed, event)
		default:
			out.Updated = append(out.Updated, event)
		}
	}
	return out
}

// keeps reports an action after which the entry still has registrations in the file.
func keeps(action string) bool {
	return action == "add" || action == "update" || action == "unchanged" || action == "adopt"
}

func (pl *planner) note(target, path, root, entry, action, message string) {
	key := changeKey(target, path, root, entry)
	c, ok := pl.changes[key]
	if !ok {
		c = &Change{Target: target, Path: path, Name: entry, Root: root, Action: action, Message: message}
		pl.changes[key] = c
		pl.order = append(pl.order, key)
		return
	}
	rank := map[string]int{"unchanged": 0, "adopt": 1, "release": 2, "add": 3, "remove": 3, "update": 4, "inactive": 5, "conflict": 6}
	// An entry that leaves some events but keeps others in the file updates it; remove
	// means it leaves the file entirely.
	switch {
	case action == c.Action:
	case action == "remove" && keeps(c.Action) || c.Action == "remove" && keeps(action):
		if c.Action != "update" {
			c.Action, c.Message = "update", ""
		}
	case rank[action] > rank[c.Action] && action != "remove":
		c.Action, c.Message = action, message
	}
}

// Preview reads source, ownership and native files without writing.
func (s *Service) Preview() (*Plan, error) {
	source, err := s.loadSource()
	if err != nil {
		return nil, err
	}
	return s.previewSource(source, nil, nil)
}

// previewSource plans a sync; replace and adopt are root + "\x00" + entry sets.
func (s *Service) previewSource(source *Source, replace, adopt map[string]bool) (*Plan, error) {
	state, stateBytes, err := s.loadLedger()
	if err != nil {
		return nil, err
	}
	// A hook the draft stops managing already plans as the user's own.
	source.forget(state)
	d, err := s.render(source)
	if err != nil {
		return nil, err
	}
	p := &Plan{SourcePath: source.ConfigPath, Changes: []Change{}, Warnings: append([]string{}, d.warnings...), source: source, stateBytes: stateBytes, state: ledger{Version: 1, Records: maps.Clone(state.Records), Created: maps.Clone(state.Created), NewFiles: maps.Clone(state.NewFiles), Dirs: maps.Clone(state.Dirs)}}
	pl := &planner{s: s, source: source, p: p, replace: replace, adopt: adopt, parked: map[string]bool{}, changes: map[string]*Change{}, events: map[string]map[string]string{}}
	for _, name := range sortedKeys(source.Entries) {
		p.Warnings = append(p.Warnings, eventWarnings(name, source.Entries[name], s.agentOf)...)
	}
	for _, root := range sortedKeys(source.Projects) {
		entries := source.Projects[root].Entries
		for _, name := range sortedKeys(entries) {
			p.Warnings = append(p.Warnings, EventWarnings(name, entries[name])...)
		}
	}
	for _, agent := range sortedKeys(configDirEnv) {
		if key := s.envShadow(agent); key != "" {
			dir, err := s.defaultConfigDir(agent)
			if err != nil {
				return nil, err
			}
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s is %s, the config_dir of target %s; the %s binding syncs %s instead", configDirEnv[agent], s.ConfigDirs[agent], key, agent, dir))
		}
	}
	if a, ok := s.Accounts["git"]; ok && s.ProjectRoot == "" && a.Dir != "" && slices.Contains(accountAgents, a.Agent) {
		// Warn only when a git binding exists, which this account would otherwise appear to receive.
		for _, name := range sortedKeys(source.Entries) {
			if _, uses := source.Entries[name].Bindings["git"]; uses {
				p.Warnings = append(p.Warnings, fmt.Sprintf("target git is a %s account, but the git binding always manages Git config hooks; rename the target to sync hooks into %s", a.Agent, a.Dir))
				break
			}
		}
	}
	type parkedGroup struct {
		target, dir, reason string
		count               int
		names               map[string]bool
	}
	parked := map[string]*parkedGroup{}
	owner := source.ConfigPath
	pl.collectGitRetired(d, state)
	// A file the source no longer writes still needs a plan, to remove what it left there.
	for key, r := range state.Records {
		if r.Owner != owner {
			continue
		}
		if r.Target == "git" {
			if d.gitSkipped[r.Root] || r.Event != "" || r.Entry == "" {
				continue
			}
			d.addOwned(r)
			continue
		}
		if reason := s.parkReason(r); reason != "" {
			if pl.replaces(r.Root, r.Entry) {
				delete(p.state.Records, key)
				delete(state.Records, key)
				pl.note(r.Target, r.Path, r.Root, r.Entry, "release", "left in place; Skillshare no longer manages it")
			} else {
				pl.parked[key] = true
				dir := recordDir(r)
				groupKey := r.Target + "\x00" + dir
				if parked[groupKey] == nil {
					parked[groupKey] = &parkedGroup{target: r.Target, dir: dir, reason: reason}
				}
				parked[groupKey].count++
			}
			continue
		}
		d.addOwned(r)
	}
	for _, key := range sortedKeys(parked) {
		g := parked[key]
		p.Warnings = append(p.Warnings, fmt.Sprintf("%s: %d hook outputs Skillshare manages in %s are left as they are because %s; sync from a shell where they resolve, or run 'skillshare hooks sync NAME --replace' to stop managing them", g.target, g.count, g.dir, g.reason))
	}
	// Cleanup outputs also claim their target's home. A symlink introduced since
	// the last sync must not let cleanup remove another target's registrations.
	if err := s.checkDesiredHomes(d); err != nil {
		return nil, err
	}
	proposal, _ := json.Marshal(struct {
		Entries  map[string]Entry
		Projects map[string]Project
		Replace  []string
	}{source.Entries, source.Projects, sortedKeys(replace)})
	revision := digest(source.bytes) + digest(stateBytes) + digest(proposal)
	owned, _ := json.Marshal(state)
	fingerprint := digest(proposal) + digest(owned)
	for _, note := range d.notes {
		p.Changes = append(p.Changes, note)
		p.Blocked = p.Blocked || note.Action == "conflict"
	}
	for _, identity := range sortedKeys(d.git) {
		w := d.git[identity]
		if w.destination.root != "" {
			for _, global := range d.git {
				if global.destination.root != "" {
					continue
				}
				for _, localCommand := range w.commands {
					for _, globalCommand := range global.commands {
						if localCommand.name == globalCommand.name {
							pl.note("git", w.destination.hooksFile, w.destination.root, localCommand.entry, "conflict", "hook."+localCommand.name+" is also planned globally; use distinct friendly names to avoid merged definitions")
						}
					}
				}
			}
		}
		files, section, err := pl.planGit(d.git[identity], state)
		if err != nil {
			return nil, err
		}
		revision += identity + section
		for _, f := range files {
			revision += fmt.Sprintf("%s\x00%s\x00%s\n", f.path, f.kind, f.section)
		}
		fingerprint += identity + section
		p.files = append(p.files, files...)
	}
	for _, path := range sortedKeys(d.targets) {
		f, err := pl.planShared(path, d.targets[path], d.roots[path], d.elements[path], state)
		if err != nil {
			return nil, err
		}
		revision += path + f.section
		p.files = append(p.files, f)
	}
	for _, path := range sortedKeys(d.files) {
		f, err := pl.planFile(d.files[path], state)
		if err != nil {
			return nil, err
		}
		revision += path + f.section
		p.files = append(p.files, f)
	}
	slices.SortStableFunc(p.files, func(a, b *filePlan) int { return gitWritePriority(a) - gitWritePriority(b) })
	for _, f := range p.files {
		fingerprint += fmt.Sprintf("%s\x00%s\x00%t\x00%s\n", f.path, f.kind, f.exists, f.section)
	}
	for _, key := range pl.order {
		c := pl.changes[key]
		c.Events = pl.eventChanges(key)
		// The reason is reported once in Warnings, not repeated in every row's message.
		if c.Target == "git" && c.Action != "conflict" && p.gitInactive[c.Root] != "" {
			c.InactiveReason = p.gitInactive[c.Root]
		}
		if c.Action == "conflict" {
			p.Blocked = true
		}
		p.Changes = append(p.Changes, *c)
	}
	p.Revision = digest([]byte(revision))
	changes, _ := json.Marshal(p.Changes)
	p.Fingerprint = digest([]byte(fmt.Sprintf("%s%s\x00%t\x00%s", fingerprint, source.ConfigPath, p.Blocked, digest(changes))))
	return p, nil
}

func (pl *planner) replaces(root, entry string) bool { return pl.replace[root+"\x00"+entry] }

func (pl *planner) adopts(root, entry string) bool { return pl.adopt[root+"\x00"+entry] }

func (pl *planner) planShared(path, target, root string, want []wantElement, state ledger) (*filePlan, error) {
	s, p, owner := pl.s, pl.p, pl.source.ConfigPath
	agent := s.agentOf(target)
	base, err := s.base(target, root)
	if err != nil {
		return nil, err
	}
	if err := checkPath(base, path); err != nil {
		return nil, err
	}
	data, exists, mode, err := safeRead(path)
	if err != nil {
		return nil, err
	}
	doc, err := parseNative(agent, data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if mode == 0 {
		mode = 0644
	}
	f := &filePlan{path: path, target: target, agent: agent, root: root, base: base, kind: kindJSON, before: data, exists: exists, mode: mode, section: doc.sectionDigest()}
	if agent == "droid" && !exists && len(want) > 0 {
		settings := filepath.Join(filepath.Dir(path), "settings.json")
		if inline, _ := droidInlineHooks(settings); len(inline) > 0 {
			pl.note(target, path, root, want[0].entry, "conflict", "Droid runs the hooks in "+settings+" only while hooks.json is absent, so creating hooks.json would silently stop them; import them from droid, remove the hooks key from "+settings+", then sync")
		}
	}
	all := map[string]record{}
	for key, r := range state.Records {
		if r.Path == path && r.Event != "" {
			all[key] = r
		}
	}
	located := locate(doc, all)
	taken := map[string]map[int]string{} // event -> index -> key
	for key, i := range located {
		r := all[key]
		if taken[r.Event] == nil {
			taken[r.Event] = map[int]string{}
		}
		taken[r.Event][i] = key
	}
	track := map[string]map[int]string{}
	keep := func(event string, i int, key string) {
		if track[event] == nil {
			track[event] = map[int]string{}
		}
		track[event][i] = key
	}
	// prior and wantEvents are each entry's events in this file before and after.
	prior, wantEvents := map[string]map[string]bool{}, map[string]map[string]bool{}
	mark := func(sets map[string]map[string]bool, root, entry, event string) {
		k := root + "\x00" + entry
		if sets[k] == nil {
			sets[k] = map[string]bool{}
		}
		sets[k][event] = true
	}
	for _, r := range all {
		if r.Owner == owner {
			mark(prior, r.Root, r.Entry, r.Event)
		}
	}
	for _, w := range want {
		mark(wantEvents, w.root, w.entry, w.event)
	}
	// touch records the events one element change adds, updates or removes; before is
	// nil for an addition and after nil for a removal.
	touch := func(root, entry, event string, before, after any) {
		if agent == "antigravity" {
			// The element is a whole block: compare the events inside it.
			old, _ := before.(map[string]any)
			next, _ := after.(map[string]any)
			for _, e := range sortedKeys(mergeKeys(old, next)) {
				_, had := old[e]
				_, has := next[e]
				switch {
				case !had:
					pl.event(target, path, root, entry, e, "added")
				case !has:
					pl.event(target, path, root, entry, e, "removed")
				case elementHash(old[e]) != elementHash(next[e]):
					pl.event(target, path, root, entry, e, "updated")
				}
			}
			return
		}
		k, kind := root+"\x00"+entry, "updated"
		switch {
		case after == nil && !wantEvents[k][event]:
			kind = "removed"
		case after != nil && !prior[k][event]:
			kind = "added"
		}
		pl.event(target, path, root, entry, event, kind)
	}
	wanted := map[string]bool{}
	for _, w := range want {
		key := elementKey(owner, w.root, path, w.event, w.entry, w.ordinal)
		wanted[key] = true
		wantHash := elementHash(w.value)
		next := record{Owner: owner, Target: target, Path: path, Root: w.root, Entry: w.entry, Event: w.event, Ordinal: w.ordinal, Hash: wantHash}
		items := doc.events[w.event]
		if pl.parked[key] {
			pl.note(target, path, w.root, w.entry, "conflict", "still managed by parked target "+state.Records[key].Target+"; explicitly replace its owning entry to stop managing it")
			continue
		}
		if r, has := state.Records[key]; has {
			if i, ok := located[key]; ok {
				if r.Hash == wantHash {
					next.Index = i
					p.state.Records[key] = next
					keep(w.event, i, key)
					action := "unchanged"
					if r.Adopted {
						action = "adopt"
					}
					pl.note(target, path, w.root, w.entry, action, "")
				} else {
					f.ops = append(f.ops, elementOp{Key: key, Event: w.event, Index: i, Value: w.value, Before: items[i]})
					p.state.Records[key] = next
					touch(w.root, w.entry, w.event, items[i], w.value)
					pl.note(target, path, w.root, w.entry, "update", "")
				}
				continue
			}
			if !pl.replaces(w.root, w.entry) {
				pl.note(target, path, w.root, w.entry, "conflict", "a hook Skillshare wrote was changed or removed outside Skillshare; import it or explicitly replace it")
				continue
			}
			op := elementOp{Key: key, Event: w.event, Index: -1, Value: w.value}
			if i := r.Index; i < len(items) && taken[w.event][i] == "" {
				// The edited registration sits where Skillshare last wrote it and no one
				// owns it: replace it in place rather than adding a second one. For
				// Antigravity that place is the block's name.
				op.Index, op.Before = i, items[i]
				if taken[w.event] == nil {
					taken[w.event] = map[int]string{}
				}
				taken[w.event][i] = key
			} else if agent == "antigravity" && len(items) > 0 {
				pl.note(target, path, w.root, w.entry, "conflict", "a hook with this name is managed by another Skillshare config")
				continue
			}
			f.ops = append(f.ops, op)
			p.state.Records[key] = next
			touch(w.root, w.entry, w.event, op.Before, w.value)
			pl.note(target, path, w.root, w.entry, "update", "replacing a registration changed outside Skillshare")
			continue
		}
		identical, other := -1, ""
		for i, item := range items {
			if elementHash(item) != wantHash {
				continue
			}
			if holder, held := taken[w.event][i]; held {
				if (all[holder].Owner != owner || pl.parked[holder]) && other == "" {
					other = holder
				}
				continue
			}
			identical = i
			break
		}
		switch {
		case identical >= 0 && !pl.replaces(w.root, w.entry) && !pl.adopts(w.root, w.entry):
			pl.note(target, path, w.root, w.entry, "conflict", "an identical hook exists that Skillshare does not manage; import it or explicitly replace it")
		case identical >= 0:
			if pl.adopts(w.root, w.entry) {
				p.adopted = append(p.adopted, key)
			}
			if taken[w.event] == nil {
				taken[w.event] = map[int]string{}
			}
			taken[w.event][identical] = key
			next.Index = identical
			p.state.Records[key] = next
			keep(w.event, identical, key)
			pl.note(target, path, w.root, w.entry, "adopt", "")
		case other != "" && pl.parked[other]:
			pl.note(target, path, w.root, w.entry, "conflict", "still managed by parked target "+all[other].Target+"; explicitly replace its owning entry to stop managing it")
		case other != "" && !(ownerGone(all[other].Owner) && pl.replaces(w.root, w.entry)):
			pl.note(target, path, w.root, w.entry, "conflict", "an identical hook is managed by another Skillshare config: "+all[other].Owner)
		case other != "":
			i := located[other]
			delete(p.state.Records, other)
			taken[w.event][i] = key
			next.Index = i
			p.state.Records[key] = next
			keep(w.event, i, key)
			pl.note(target, path, w.root, w.entry, "adopt", "taking over a hook left by a removed Skillshare config")
		case agent == "antigravity" && len(items) > 0 && other == "" && !pl.replaces(w.root, w.entry) && !pl.adopts(w.root, w.entry):
			pl.note(target, path, w.root, w.entry, "conflict", "a hook with this name exists that Skillshare does not manage; import it or explicitly replace it")
		case agent == "antigravity" && len(items) > 0 && other == "":
			if _, held := taken[w.event][0]; held {
				pl.note(target, path, w.root, w.entry, "conflict", "a hook with this name is managed by another Skillshare config")
				continue
			}
			// Importing or replacing takes over the block of the same name in place.
			f.ops = append(f.ops, elementOp{Key: key, Event: w.event, Index: 0, Value: w.value, Before: items[0]})
			p.state.Records[key] = next
			touch(w.root, w.entry, w.event, items[0], w.value)
			pl.note(target, path, w.root, w.entry, "update", "replacing a hook Skillshare did not write")
		case agent == "antigravity" && pl.adopts(w.root, w.entry):
			// An Antigravity hook is the block of that name. Taking over a block under
			// another name would add a second block and leave the original running.
			pl.note(target, path, w.root, w.entry, "conflict", "Antigravity names each hook after its block; import it under the block's own name")
		default:
			f.ops = append(f.ops, elementOp{Key: key, Event: w.event, Index: -1, Value: w.value})
			p.state.Records[key] = next
			touch(w.root, w.entry, w.event, nil, w.value)
			pl.note(target, path, w.root, w.entry, "add", "")
		}
	}
	for _, key := range sortedKeys(all) {
		r := all[key]
		if r.Owner != owner || wanted[key] || pl.parked[key] {
			continue
		}
		if i, ok := located[key]; ok {
			f.ops = append(f.ops, elementOp{Key: key, Event: r.Event, Index: i, Before: doc.events[r.Event][i]})
			delete(p.state.Records, key)
			touch(r.Root, r.Entry, r.Event, doc.events[r.Event][i], nil)
			pl.note(target, path, r.Root, r.Entry, "remove", "")
			continue
		}
		if pl.replaces(r.Root, r.Entry) {
			delete(p.state.Records, key)
			pl.note(target, path, r.Root, r.Entry, "release", "left in place; Skillshare no longer manages it")
			continue
		}
		pl.note(target, path, r.Root, r.Entry, "conflict", "a hook Skillshare wrote was changed or removed outside Skillshare, so it is not removed; explicitly replace to stop managing it")
	}
	f.created = p.state.Created[path] || !doc.hasSection && slices.ContainsFunc(f.ops, func(op elementOp) bool { return op.Value != nil })
	after, positions, err := doc.edit(f.ops, track, f.created)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	f.after = after
	if len(f.ops) > 0 && p.state.NewFiles[path] && skeleton(agent, after) {
		f.remove, f.after = true, nil
		for _, key := range pl.order {
			if c := pl.changes[key]; c.Path == path && c.Action == "remove" {
				c.Message = "deletes the file: Skillshare created it and nothing else is left"
			}
		}
	}
	for _, op := range f.ops {
		f.keys = append(f.keys, op.Key)
	}
	for key, i := range positions {
		if r, ok := p.state.Records[key]; ok {
			r.Index = i
			p.state.Records[key] = r
		}
	}
	written, err := parseNative(agent, after)
	if err != nil {
		return nil, err
	}
	if len(f.ops) > 0 {
		if f.created && written.hasSection {
			if p.state.Created == nil {
				p.state.Created = map[string]bool{}
			}
			p.state.Created[path] = true
		} else {
			delete(p.state.Created, path)
		}
	}
	counts := map[string]int{}
	for event, items := range written.events {
		for _, item := range items {
			counts[event+"\x00"+elementHash(item)]++
		}
	}
	for key, r := range p.state.Records {
		if r.Owner == owner && r.Path == path && r.Event != "" {
			r.Peers = counts[r.Event+"\x00"+r.Hash]
			p.state.Records[key] = r
		}
	}
	return f, nil
}

func (pl *planner) planFile(w wantFile, state ledger) (*filePlan, error) {
	s, p, owner := pl.s, pl.p, pl.source.ConfigPath
	base, err := s.base(w.target, w.root)
	if err != nil {
		return nil, err
	}
	if err := checkPath(base, w.path); err != nil {
		return nil, err
	}
	data, exists, mode, err := safeRead(w.path)
	if err != nil {
		return nil, err
	}
	current := ""
	if exists {
		current = digest(data)
	}
	f := &filePlan{path: w.path, target: w.target, agent: s.agentOf(w.target), root: w.root, base: base, kind: kindFile, before: data, exists: exists, mode: w.mode, section: current}
	if exists && (w.target != "git" || w.content == nil) {
		f.mode = mode
	}
	key := fileKey(w.path)
	r, has := state.Records[key]
	if pl.parked[key] {
		pl.note(w.target, w.path, w.root, w.entry, "conflict", "still managed by parked target "+r.Target+"; explicitly replace its owning entry to stop managing it")
		return f, nil
	}
	replace := pl.replaces(w.root, w.entry)
	if has && r.Owner != owner {
		if !ownerGone(r.Owner) || !replace || w.content == nil {
			pl.note(w.target, w.path, w.root, w.entry, "conflict", "managed by another Skillshare config: "+r.Owner)
			return f, nil
		}
		has = false
	}
	next := record{Owner: owner, Target: w.target, Path: w.path, Root: w.root, Entry: w.entry, Hash: digest(w.content)}
	set := func(action, message string) {
		f.keys = []string{key}
		p.state.Records[key] = next
		pl.note(w.target, w.path, w.root, w.entry, action, message)
	}
	switch {
	case w.content == nil && !exists:
		delete(p.state.Records, key)
	case w.content == nil && current == r.Hash:
		f.remove, f.keys = true, []string{key}
		delete(p.state.Records, key)
		pl.note(w.target, w.path, w.root, w.entry, "remove", "")
	case w.content == nil && replace:
		delete(p.state.Records, key)
		pl.note(w.target, w.path, w.root, w.entry, "release", "left in place; Skillshare no longer manages it")
	case w.content == nil:
		pl.note(w.target, w.path, w.root, w.entry, "conflict", "file changed outside Skillshare, so it is not removed; explicitly replace to stop managing it")
	case has && exists && current == r.Hash && current == next.Hash:
		if w.target == "git" && s.platform() != "windows" && mode.Perm() != f.mode.Perm() {
			f.write = w.content
			set("update", "restore Git helper permissions")
		} else {
			set("unchanged", "")
		}
	case has && exists && current == r.Hash:
		f.write = w.content
		set("update", "")
	case has && !replace:
		pl.note(w.target, w.path, w.root, w.entry, "conflict", "file changed or removed outside Skillshare; import it or explicitly replace it")
	case !has && !exists:
		f.write = w.content
		set("add", "")
	case !has && current == next.Hash && replace:
		set("adopt", "")
	case !has && !replace:
		pl.note(w.target, w.path, w.root, w.entry, "conflict", "a file Skillshare does not manage already exists here; import it or explicitly replace it")
	default:
		f.write = w.content
		set("update", "replacing a file Skillshare did not write")
	}
	if f.write != nil {
		f.after = f.write
	} else if !f.remove {
		f.after = data
	}
	if w.target == "copilot" && (f.write != nil || f.remove) {
		pl.fileEvents(w, data, f.write)
	}
	return f, nil
}

// fileEvents names the events a Copilot hook file gains, changes or loses, as a shared
// file's changes do. A file that does not parse contributes no events.
func (pl *planner) fileEvents(w wantFile, before, after []byte) {
	hooksOf := func(data []byte) map[string]any {
		var doc struct {
			Hooks map[string]any `json:"hooks"`
		}
		_ = json.Unmarshal(data, &doc)
		return doc.Hooks
	}
	old, next := hooksOf(before), hooksOf(after)
	for event, value := range next {
		prior, had := old[event]
		switch {
		case !had:
			pl.event(w.target, w.path, w.root, w.entry, event, "added")
		case !reflect.DeepEqual(prior, value):
			pl.event(w.target, w.path, w.root, w.entry, event, "updated")
		}
	}
	for event := range old {
		if _, ok := next[event]; !ok {
			pl.event(w.target, w.path, w.root, w.entry, event, "removed")
		}
	}
}

// refresh confirms a file still matches its preview; other settings may change.
func (f *filePlan) refresh() ([]byte, bool, os.FileMode, *nativeDoc, error) {
	if f.kind == kindGitInclude {
		return f.refreshGit()
	}
	if f.kind == kindGitSection {
		return f.refreshGitSection()
	}
	if err := checkPath(f.base, f.path); err != nil {
		return nil, false, 0, nil, err
	}
	data, exists, mode, err := safeRead(f.path)
	if err != nil {
		return nil, false, 0, nil, err
	}
	if f.kind == kindFile {
		current := ""
		if exists {
			current = digest(data)
		}
		if current != f.section {
			return nil, false, 0, nil, fmt.Errorf("%s changed since preview: %w", f.path, ErrStaleRevision)
		}
		if !exists {
			mode = f.mode
		}
		return data, exists, mode, nil, nil
	}
	doc, err := parseNative(f.agent, data)
	if err != nil {
		return nil, false, 0, nil, fmt.Errorf("%s: %w", f.path, err)
	}
	if doc.sectionDigest() != f.section {
		return nil, false, 0, nil, fmt.Errorf("%s hooks changed since preview: %w", f.path, ErrStaleRevision)
	}
	if !exists {
		mode = f.mode
	}
	return data, exists, mode, doc, nil
}

// droidInlineHooks reads the hooks key of Droid's settings.json, an additional source
// Skillshare never edits.
func droidInlineHooks(path string) (map[string]any, error) {
	data, exists, _, err := safeRead(path)
	if err != nil || !exists {
		return nil, err
	}
	doc, err := parseNative("claude", data) // same wrapped "hooks" shape
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	for event, items := range doc.events {
		out[event] = items
	}
	return out, nil
}
