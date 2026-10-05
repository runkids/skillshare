package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// List reads the source, ownership and native files without writing. A plan that
// cannot be computed is reported in PreviewError so the rest stays visible.
func (s *Service) List() (*Inventory, error) {
	source, err := s.loadSource()
	if err != nil {
		return nil, err
	}
	inv := &Inventory{
		ProjectGit:     map[string]*GitInfo{},
		Source:         SourceInfo{Path: source.ConfigPath, ConfigPath: source.ConfigPath, Entries: source.Entries, Projects: source.Projects},
		Targets:        s.targetDefs(),
		Paths:          s.Paths(),
		Unmanaged:      []Unmanaged{},
		ProjectConfigs: []string{},
		ProjectPaths:   map[string]map[string]string{},
	}
	if inv.Plan, err = s.previewSource(source, nil, nil); err != nil {
		inv.Plan, inv.PreviewError = nil, err.Error()
	}
	if inv.Backups, err = s.Backups(); err != nil {
		return nil, err
	}
	state, _, err := s.loadLedger()
	if err != nil {
		return nil, err
	}
	inv.Unmanaged = append(inv.Unmanaged, s.unmanaged(state)...)
	inv.Git = s.gitInventory("", state)
	for _, root := range sortedKeys(source.Projects) {
		if ownConfig(root) {
			inv.ProjectConfigs = append(inv.ProjectConfigs, root)
			continue
		}
		scoped := s.scoped(root)
		inv.ProjectGit[root] = scoped.gitInventory(root, state)
		inv.ProjectPaths[root] = scoped.Paths()
		unmanaged := scoped.unmanaged(state)
		for i := range unmanaged {
			unmanaged[i].Project = root
		}
		inv.Unmanaged = append(inv.Unmanaged, unmanaged...)
	}
	return inv, nil
}

// unmanaged lists native hooks in this scope that no Skillshare config owns, plus
// additional sources Skillshare reads but never edits.
func (s *Service) unmanaged(state ledger) []Unmanaged {
	var out []Unmanaged
	add := func(target, path string, names []string) {
		if len(names) > 0 {
			out = append(out, Unmanaged{Target: target, Path: path, Names: names})
		}
	}
	for _, t := range s.targetDefs() {
		if t.Kind == KindGit {
			out = append(out, s.gitUnmanaged(state)...)
			continue
		}
		sc, agent := s.forTarget(t.Name)
		path, err := sc.nativePath(agent)
		if err != nil {
			continue
		}
		if agent == "codex" {
			dir, _ := sc.configDir(agent)
			inline := filepath.Join(dir, "config.toml")
			add(t.Name, inline, codexInlineEvents(inline))
		}
		if agent == "omp" {
			// OMP also loads hook factories from hooks/pre and hooks/post; Skillshare never writes there.
			dir, _ := sc.configDir(agent)
			for _, sub := range []string{"pre", "post"} {
				factories := filepath.Join(dir, "hooks", sub)
				add(t.Name, factories, sc.unownedFiles(agent, factories, state))
			}
		}
		switch {
		case t.Kind == KindCode || agent == "copilot":
			add(t.Name, path, sc.unownedFiles(agent, path, state))
		default:
			doc, _, err := sc.readDoc(agent, path)
			if err != nil || doc == nil {
				continue
			}
			add(t.Name, path, unownedEvents(doc, path, state))
		}
	}
	dir := func(target string) string { d, _ := s.configDir(target); return d }
	// Droid runs settings.json hooks only while hooks.json is absent; its legacy
	// hooks/hooks.json always loads.
	if !exists(filepath.Join(dir("droid"), "hooks.json")) {
		if inline, _ := droidInlineHooks(filepath.Join(dir("droid"), "settings.json")); len(inline) > 0 {
			add("droid", filepath.Join(dir("droid"), "settings.json"), sortedKeys(inline))
		}
	}
	legacy := filepath.Join(dir("droid"), "hooks", "hooks.json")
	if doc, _, err := s.readDoc("droid", legacy); err == nil && doc != nil {
		add("droid", legacy, sortedKeys(doc.events))
	}
	if s.ProjectRoot != "" {
		local := filepath.Join(dir("claude"), "settings.local.json")
		if doc, _, err := s.readDoc("claude", local); err == nil && doc != nil {
			add("claude", local, sortedKeys(doc.events))
		}
	}
	return out
}

func (s *Service) readDoc(target, path string) (*nativeDoc, bool, error) {
	data, exists, _, err := safeRead(path)
	if err != nil || !exists {
		return nil, exists, err
	}
	doc, err := parseNative(target, data)
	return doc, true, err
}

// unownedEvents names events holding elements no record locates.
func unownedEvents(doc *nativeDoc, path string, state ledger) []string {
	records := map[string]record{}
	for key, r := range state.Records {
		if r.Path == path && r.Event != "" {
			records[key] = r
		}
	}
	owned := map[string]int{}
	for key := range locate(doc, records) {
		owned[records[key].Event]++
	}
	var names []string
	for _, event := range sortedKeys(doc.events) {
		if len(doc.events[event]) > owned[event] {
			names = append(names, event)
		}
	}
	return names
}

// unownedFiles lists hook files in a Copilot hooks folder or a code Agent's plugin
// folder that no Skillshare config owns.
func (s *Service) unownedFiles(target, dir string, state ledger) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		ext := filepath.Ext(name)
		if e.IsDir() || !(target == "copilot" && ext == ".json" || target != "copilot" && (ext == ".ts" || ext == ".js")) {
			continue
		}
		if _, owned := state.Records[fileKey(filepath.Join(dir, name))]; !owned {
			names = append(names, name)
		}
	}
	return names
}

// codexInlineEvents names the [hooks] events inline in Codex's config.toml.
func codexInlineEvents(path string) []string {
	data, exists, _, err := safeRead(path)
	if err != nil || !exists {
		return nil
	}
	var doc struct {
		Hooks map[string]any `toml:"hooks"`
	}
	if toml.Unmarshal(data, &doc) != nil {
		return nil
	}
	return sortedKeys(doc.Hooks)
}

var nonName = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

// candidateName turns an event or file name into a hook name.
func candidateName(parts ...string) string {
	name := strings.Trim(nonName.ReplaceAllString(strings.Join(parts, "-"), "-"), "-.")
	if len(name) > 128 {
		name = name[:128]
	}
	return strings.ToLower(name)
}

// Import reads native hooks without executing them. Without Content it reads the
// Agent's files in scope and offers only hooks no Skillshare config owns. A Name that
// matches a candidate picks that one; any other Name merges everything found into one
// candidate.
func (s *Service) Import(req ImportRequest) ([]Candidate, error) {
	target := canonicalTarget(req.From)
	if target == "git" {
		return nil, fmt.Errorf("Git hooks import is not supported yet; define commands explicitly")
	}
	scope, agent := s.forTarget(target)
	def, ok := targetDef(agent)
	if !ok {
		return nil, fmt.Errorf("unsupported hooks Agent %q", req.From)
	}
	if req.Root != "" && agent != target {
		return nil, fmt.Errorf("hooks.projects roots are read by every account; import --from %s", agent)
	}
	if req.Root != "" {
		if s.ProjectRoot != "" {
			return nil, fmt.Errorf("hooks.projects belongs in the global config")
		}
		source, err := s.loadSource()
		if err != nil {
			return nil, err
		}
		root, err := projectRoot(req.Root)
		if err != nil {
			return nil, err
		}
		if _, ok := source.Projects[root]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownProject, req.Root)
		}
		scope = s.scoped(root)
	}
	state, _, err := s.loadLedger()
	if err != nil {
		return nil, err
	}
	found := []Candidate{}
	switch {
	case def.Kind == KindCode:
		found, err = scope.importCode(agent, req, state)
	case target == "copilot":
		found, err = scope.importCopilot(req, state)
	default:
		found, err = scope.importShared(agent, req, state)
	}
	if err != nil {
		return nil, err
	}
	shared := def.Kind == KindCommand && target != "copilot"
	picked := []Candidate{}
	for _, c := range found {
		if c.Name == req.Name {
			picked = append(picked, c)
		}
	}
	if req.Name != "" && (len(picked) > 0 || req.Content == "" && !shared) {
		// Candidates are per event or per file; a name picks one of them.
		found = picked
	} else if req.Name != "" && len(found) > 1 {
		merged := Entry{Bindings: map[string]Binding{}}
		var warnings []string
		for _, c := range found {
			b := c.Entry.Bindings[target]
			m := merged.Bindings[target]
			if m.Events == nil {
				m.Events = map[string]any{}
			}
			for event, items := range b.Events {
				existing, _ := m.Events[event].([]any)
				m.Events[event] = append(existing, items.([]any)...)
			}
			merged.Bindings[target] = m
			warnings = append(warnings, c.Warnings...)
		}
		found = []Candidate{{Name: req.Name, Entry: merged, Warnings: dedupe(warnings)}}
	} else if req.Name != "" && len(found) == 1 {
		found[0].Name = req.Name
	}
	for i := range found {
		if found[i].Problems == nil {
			found[i].Problems = []string{}
		}
		if found[i].Warnings == nil {
			found[i].Warnings = []string{}
		}
		if err := s.validateEntry(found[i].Name, found[i].Entry, req.Root); err != nil {
			found[i].Problems = append(found[i].Problems, err.Error())
		}
	}
	return found, nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func dedupe(items []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

const adoptWarning = "saving the import takes over the existing registrations in place; sync leaves them as they are instead of adding duplicates"

func (s *Service) importShared(target string, req ImportRequest, state ledger) ([]Candidate, error) {
	path, err := s.nativePath(target)
	if err != nil {
		return nil, err
	}
	var doc *nativeDoc
	var warnings []string
	if req.Content != "" {
		if doc, err = parseNative(target, []byte(req.Content)); err != nil && target == "droid" {
			// Pasted from settings.json, where Droid's hooks sit under "hooks".
			doc, err = parseNative("claude", []byte(req.Content))
		}
		if err != nil {
			return nil, err
		}
		state = ledger{Records: map[string]record{}} // pasted content has no owners
	} else if settings := filepath.Join(filepath.Dir(path), "settings.json"); target == "droid" && !exists(path) {
		// Without hooks.json, the hooks Droid runs are the inline ones in settings.json.
		if err := checkPath(s.baseOrDir(target), settings); err != nil {
			return nil, err
		}
		if doc, _, err = s.readDoc("claude", settings); err != nil {
			return nil, fmt.Errorf("%s: %w", settings, err)
		}
		if doc == nil {
			return []Candidate{}, nil
		}
		state = ledger{Records: map[string]record{}}
		warnings = []string{"Droid runs these from " + settings + " only while hooks.json is absent; after saving, remove the hooks key from " + settings + " and sync to move them into hooks.json"}
	} else {
		if err := checkPath(s.baseOrDir(target), path); err != nil {
			return nil, err
		}
		if doc, _, err = s.readDoc(target, path); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if doc == nil {
			return []Candidate{}, nil
		}
	}
	records := map[string]record{}
	for key, r := range state.Records {
		if r.Path == path && r.Event != "" {
			records[key] = r
		}
	}
	owned := map[string]map[int]bool{}
	for key, i := range locate(doc, records) {
		event := records[key].Event
		if owned[event] == nil {
			owned[event] = map[int]bool{}
		}
		owned[event][i] = true
	}
	out := []Candidate{}
	for _, event := range sortedKeys(doc.events) {
		var items []any
		for i, item := range doc.events[event] {
			if !owned[event][i] {
				items = append(items, item)
			}
		}
		if len(items) == 0 {
			continue
		}
		c := Candidate{Name: candidateName(s.targetKey(target), event), Entry: Entry{Bindings: map[string]Binding{s.targetKey(target): {Events: map[string]any{event: items}}}}}
		switch {
		case warnings != nil:
			c.Warnings = warnings
		case req.Content == "":
			c.Warnings = []string{adoptWarning}
		}
		if target == "antigravity" {
			c = antigravityCandidate(event, items[0].(map[string]any), c.Warnings)
		}
		out = append(out, c)
	}
	return out, nil
}

// antigravityCandidate offers one named block. The hook keeps the block's name, so
// saving the import takes that block over in place.
func antigravityCandidate(name string, block map[string]any, warnings []string) Candidate {
	events := map[string]any{}
	for event, value := range block {
		if event != "enabled" {
			events[event] = value
		}
	}
	c := Candidate{Name: name, Entry: Entry{Bindings: map[string]Binding{"antigravity": {Events: events}}}, Warnings: warnings}
	if !entryName.MatchString(name) {
		c.Name = candidateName(name)
		c.Warnings = []string{fmt.Sprintf("%q is not a valid hook name, so the import is written as %q next to it; remove the original afterwards", name, c.Name)}
	}
	if _, disabled := block["enabled"]; disabled {
		c.Problems = []string{"this hook is disabled in Antigravity (enabled: false); enable it there to import it"}
	}
	return c
}

func (s *Service) baseOrDir(target string) string {
	if s.ProjectRoot != "" {
		return s.ProjectRoot
	}
	dir, _ := s.configDir(target)
	return dir
}

func (s *Service) importCopilot(req ImportRequest, state ledger) ([]Candidate, error) {
	parse := func(name string, data []byte) (Candidate, error) {
		var doc struct {
			Version any            `json:"version"`
			Hooks   map[string]any `json:"hooks"`
		}
		if err := json.Unmarshal(data, &doc); err != nil || doc.Version != float64(1) {
			return Candidate{}, fmt.Errorf("%s: Copilot hooks files need version 1 and a hooks object", name)
		}
		return Candidate{Name: name, Entry: Entry{Bindings: map[string]Binding{"copilot": {Events: doc.Hooks}}}}, nil
	}
	if req.Content != "" {
		c, err := parse("copilot-hooks", []byte(req.Content))
		if err != nil {
			return nil, err
		}
		return []Candidate{c}, nil
	}
	dir, err := s.nativePath("copilot")
	if err != nil {
		return nil, err
	}
	out := []Candidate{}
	for _, file := range s.unownedFiles("copilot", dir, state) {
		path := filepath.Join(dir, file)
		if err := checkPath(s.baseOrDir("copilot"), path); err != nil {
			return nil, err
		}
		data, _, _, err := safeRead(path)
		if err != nil {
			return nil, err
		}
		c, err := parse(candidateName(strings.TrimSuffix(strings.TrimPrefix(file, "skillshare-"), ".json")), data)
		if err != nil {
			out = append(out, Candidate{Name: candidateName(strings.TrimSuffix(file, ".json")), Entry: Entry{Bindings: map[string]Binding{}}, Problems: []string{err.Error()}})
			continue
		}
		c.Warnings = []string{fmt.Sprintf("%s keeps loading; remove it after syncing so Copilot does not run these hooks twice", path)}
		out = append(out, c)
	}
	return out, nil
}

func (s *Service) importCode(target string, req ImportRequest, state ledger) ([]Candidate, error) {
	if req.Content != "" {
		name := req.Name
		if name == "" {
			name = candidateName(s.targetKey(target), "plugin")
		}
		return []Candidate{{Name: name, Entry: Entry{Bindings: map[string]Binding{s.targetKey(target): {Code: req.Content}}}}}, nil
	}
	dir, err := s.nativePath(target)
	if err != nil {
		return nil, err
	}
	out := []Candidate{}
	for _, file := range s.unownedFiles(target, dir, state) {
		path := filepath.Join(dir, file)
		if err := checkPath(s.baseOrDir(target), path); err != nil {
			return nil, err
		}
		data, _, _, err := safeRead(path)
		if err != nil {
			return nil, err
		}
		name := candidateName(strings.TrimSuffix(strings.TrimPrefix(file, "skillshare-"), filepath.Ext(file)))
		c := Candidate{Name: name, Entry: Entry{Bindings: map[string]Binding{s.targetKey(target): {Code: string(data)}}}}
		c.Warnings = []string{fmt.Sprintf("%s keeps loading; remove it after syncing so the plugin does not run twice", path)}
		if filepath.Ext(file) == ".js" {
			c.Warnings = append(c.Warnings, "Skillshare writes code as a .ts file; plain JavaScript is valid TypeScript for the loader, but check any CommonJS syntax")
		}
		out = append(out, c)
	}
	return out, nil
}
