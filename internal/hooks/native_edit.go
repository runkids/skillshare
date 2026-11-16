package hooks

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NativeEdit rewrites or removes hooks no Skillshare config manages, in the target's
// own settings file. They stay unmanaged: neither the source nor the ownership ledger
// changes.
type NativeEdit struct {
	Target string `json:"target"`
	// Root edits a hooks.projects root's native file instead of the current scope's.
	Root string `json:"root,omitempty"`
	// Before holds the unmanaged registrations as Import listed them, keyed by event.
	Before map[string]any `json:"before"`
	// After replaces them; empty removes them.
	After map[string]any `json:"after,omitempty"`
}

// nativeEditable reports the Agents whose unmanaged hooks are elements of one event
// map, which is what NativeEdit can address.
func nativeEditable(agent string) bool {
	def, ok := targetDef(agent)
	return ok && def.Kind == KindCommand && agent != "copilot" && agent != "antigravity"
}

type nativeEditFile struct {
	plan *filePlan
	doc  *nativeDoc
	// claimed are the elements a record owns or an earlier edit already addressed.
	claimed map[string]map[int]bool
	// managed are the content hashes Skillshare records for this file, per event.
	managed map[string]bool
}

// PreviewNativeEdits plans the edits without writing. Each Before element must still
// be in the file and must not be content Skillshare manages.
func (s *Service) PreviewNativeEdits(edits []NativeEdit) (*Plan, error) {
	s = s.memoProbe()
	if len(edits) == 0 {
		return nil, fmt.Errorf("no hook changes given")
	}
	source, err := s.loadSource()
	if err != nil {
		return nil, err
	}
	state, stateBytes, err := s.loadLedger()
	if err != nil {
		return nil, err
	}
	p := &Plan{SourcePath: source.ConfigPath, Changes: []Change{}, Warnings: []string{}, source: source, stateBytes: stateBytes, state: state}
	files := map[string]*nativeEditFile{}
	for _, e := range edits {
		target := canonicalTarget(e.Target)
		scope, agent := s.forTarget(target)
		if !nativeEditable(agent) {
			return nil, fmt.Errorf("%s hooks cannot be edited in place; import them to change them", e.Target)
		}
		root := ""
		if e.Root != "" {
			if s.ProjectRoot != "" {
				return nil, fmt.Errorf("hooks.projects belongs in the global config")
			}
			if agent != target {
				return nil, fmt.Errorf("hooks.projects roots are read by every account; use %s", agent)
			}
			if root, err = projectRoot(e.Root); err != nil {
				return nil, err
			}
			if _, ok := source.Projects[root]; !ok {
				return nil, fmt.Errorf("%w: %s", ErrUnknownProject, e.Root)
			}
			scope = s.scoped(root)
		}
		path, err := scope.nativePath(agent)
		if err != nil {
			return nil, err
		}
		file := files[path]
		if file == nil {
			if file, err = s.nativeEditFile(target, agent, root, path, state); err != nil {
				return nil, err
			}
			files[path] = file
			p.files = append(p.files, file.plan)
		}
		if len(e.Before) == 0 {
			return nil, fmt.Errorf("%s: no hooks named to change", e.Target)
		}
		if len(e.After) > 0 {
			if err := validateEvents(agent, e.After); err != nil {
				return nil, err
			}
		}
		for _, event := range sortedKeys(e.Before) {
			before, ok := e.Before[event].([]any)
			if !ok || len(before) == 0 {
				return nil, fmt.Errorf("event %s must be a non-empty array", event)
			}
			after, _ := e.After[event].([]any)
			for i, item := range before {
				index, err := file.find(event, item)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", path, err)
				}
				switch {
				case i >= len(after):
					file.plan.ops = append(file.plan.ops, elementOp{Event: event, Index: index, Before: item})
				case elementHash(after[i]) != elementHash(item):
					if err := file.unmanaged(event, after[i]); err != nil {
						return nil, err
					}
					file.plan.ops = append(file.plan.ops, elementOp{Event: event, Index: index, Value: after[i], Before: item})
				}
			}
			for i := len(before); i < len(after); i++ {
				if err := file.unmanaged(event, after[i]); err != nil {
					return nil, err
				}
				file.plan.ops = append(file.plan.ops, elementOp{Event: event, Index: -1, Value: after[i]})
			}
		}
		for _, event := range sortedKeys(e.After) {
			if _, ok := e.Before[event]; ok {
				continue
			}
			for _, item := range e.After[event].([]any) {
				if err := file.unmanaged(event, item); err != nil {
					return nil, err
				}
				file.plan.ops = append(file.plan.ops, elementOp{Event: event, Index: -1, Value: item})
			}
		}
		action := "update"
		if len(e.After) == 0 {
			action = "remove"
		}
		p.Changes = append(p.Changes, Change{Target: target, Path: path, Root: root, Name: strings.Join(sortedKeys(e.Before), ", "), Action: action})
	}
	proposal, _ := json.Marshal(edits)
	revision := digest(source.bytes) + digest(stateBytes) + digest(proposal)
	for _, f := range p.files {
		after, _, err := files[f.path].doc.edit(f.ops, nil, false)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.path, err)
		}
		f.after = after
		left, err := parseNative(f.agent, after)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.path, err)
		}
		for i, op := range f.ops {
			if op.Before == nil {
				continue
			}
			// A managed element never has this content: find refused it.
			for _, item := range left.events[op.Event] {
				if elementHash(item) == elementHash(op.Before) {
					f.ops[i].Twins++
				}
			}
		}
		revision += digest([]byte(f.path + "\x00" + f.section))
	}
	p.Revision = digest([]byte(revision))
	p.Fingerprint = p.Revision
	return p, nil
}

func (s *Service) nativeEditFile(target, agent, root, path string, state ledger) (*nativeEditFile, error) {
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
	if !exists {
		return nil, fmt.Errorf("%s does not exist", path)
	}
	doc, err := parseNative(agent, data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	file := &nativeEditFile{
		plan:    &filePlan{path: path, target: target, agent: agent, root: root, base: base, kind: kindJSON, before: data, exists: true, mode: mode, section: doc.sectionDigest()},
		doc:     doc,
		claimed: map[string]map[int]bool{},
		managed: map[string]bool{},
	}
	records := map[string]record{}
	for key, r := range state.Records {
		if r.Path == path && r.Event != "" {
			records[key] = r
			file.managed[r.Event+"\x00"+r.Hash] = true
		}
	}
	for key, i := range locate(doc, records) {
		file.claim(records[key].Event, i)
	}
	return file, nil
}

func (f *nativeEditFile) claim(event string, i int) {
	if f.claimed[event] == nil {
		f.claimed[event] = map[int]bool{}
	}
	f.claimed[event][i] = true
}

// unmanaged refuses content Skillshare also records for the event, as what to change
// or as what to write: nobody could tell the copies apart afterwards.
func (f *nativeEditFile) unmanaged(event string, item any) error {
	if f.managed[event+"\x00"+elementHash(item)] {
		return fmt.Errorf("a %s hook with the same content is managed by Skillshare; edit that hook instead", event)
	}
	return nil
}

// find returns the index of an unmanaged element with this content and claims it.
func (f *nativeEditFile) find(event string, item any) (int, error) {
	if err := f.unmanaged(event, item); err != nil {
		return 0, err
	}
	hash := elementHash(item)
	for i, current := range f.doc.events[event] {
		if !f.claimed[event][i] && elementHash(current) == hash {
			f.claim(event, i)
			return i, nil
		}
	}
	return 0, fmt.Errorf("the %s hooks changed since they were read: %w", event, ErrStaleRevision)
}

// ApplyNativeEdits writes previewed edits, with a backup per file.
func (s *Service) ApplyNativeEdits(edits []NativeEdit, revision string) (*Result, error) {
	lock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()
	if err := s.recoverPending(); err != nil {
		return nil, err
	}
	p, err := s.PreviewNativeEdits(edits)
	if err != nil {
		return nil, err
	}
	if p.Revision != revision {
		return nil, ErrStaleRevision
	}
	return s.applyScoped(p, nil)
}
