package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var grokServerName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

// Change deliberately contains no native values, which may include credentials.
type Change struct {
	Target  string `json:"target"`
	Path    string `json:"path"`
	Name    string `json:"name"`
	Action  string `json:"action"`
	Message string `json:"message,omitempty"`
}

// Plan is a redacted, optimistic-concurrency-protected preview.
type Plan struct {
	Revision   string   `json:"revision"`
	SourcePath string   `json:"sourcePath"`
	Blocked    bool     `json:"blocked"`
	Changes    []Change `json:"changes"`
	files      []*filePlan
	state      ledger
	stateBytes []byte
	source     *Source
}

type ownership struct {
	Owner  string `json:"owner"`
	Target string `json:"target"`
	Path   string `json:"path"`
	Name   string `json:"name"`
	Hash   string `json:"hash"`
}

type ledger struct {
	Version int                  `json:"version"`
	Entries map[string]ownership `json:"entries"`
}

type filePlan struct {
	path, target  string
	before, after []byte
	exists        bool
	mode          os.FileMode
	changes       map[string]map[string]any
}

func ownershipKey(path, name string) string { return digest([]byte(path + "\x00" + name)) }

func safeRead(path string) ([]byte, bool, os.FileMode, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, 0600, nil
	}
	if err != nil {
		return nil, false, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, 0, fmt.Errorf("MCP file must be a regular file, not a symlink or directory: %s", path)
	}
	data, err := os.ReadFile(path)
	return data, true, info.Mode().Perm(), err
}

func (s *Service) loadLedger() (ledger, []byte, error) {
	data, exists, _, err := safeRead(s.statePath())
	state := ledger{Version: 1, Entries: map[string]ownership{}}
	if err != nil {
		return state, nil, err
	}
	if exists {
		if json.Unmarshal(data, &state) != nil || state.Version != 1 || state.Entries == nil {
			return state, nil, fmt.Errorf("MCP ownership state is invalid; restore a backup or explicitly import entries again")
		}
	}
	return state, data, nil
}

// Preview reads source, ownership and native files without creating files.
func (s *Service) Preview() (*Plan, error) {
	if _, err := os.Stat(filepath.Join(s.StateDir, "mcp", "pending.json")); err == nil {
		return nil, fmt.Errorf("an interrupted MCP operation needs recovery; run sync mcp again")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	return s.previewSource(source)
}

func (s *Service) previewSource(source *Source) (*Plan, error) {
	return s.previewResolved(source, nil)
}

func (s *Service) previewResolved(source *Source, resolutions []Resolution) (*Plan, error) {
	matched := map[string]bool{}
	for _, r := range resolutions {
		key := r.Target + "\x00" + r.Name
		if _, exists := matched[key]; exists || !validTarget(r.Target) || (r.Action != "replace" && r.Action != "adopt") {
			return nil, fmt.Errorf("invalid or duplicate MCP conflict resolution")
		}
		matched[key] = false
	}
	state, stateBytes, err := s.loadLedger()
	if err != nil {
		return nil, err
	}
	p := &Plan{SourcePath: source.Path, Changes: []Change{}, source: source, state: state, stateBytes: stateBytes}
	desired := map[string]map[string]map[string]any{}
	targets := map[string]string{}
	for _, name := range sortedKeys(source.Servers) {
		server := source.Servers[name]
		selected := server.Targets
		if selected == nil {
			selected = source.Targets
		}
		if len(selected) == 0 {
			return nil, fmt.Errorf("MCP %s has no targets; select at least one Agent", name)
		}
		for _, target := range selected {
			if target == "grok" && (!grokServerName.MatchString(name) || strings.Contains(name, "__") || strings.HasSuffix(name, "_")) {
				return nil, fmt.Errorf("Grok MCP %s: use a name starting with a letter or underscore, containing only letters, digits, hyphens and single underscores, and not ending in underscore", name)
			}
			path, err := s.nativePath(target)
			if err != nil {
				return nil, err
			}
			targets[path] = target
			entry, err := Render(target, server)
			if err != nil {
				return nil, fmt.Errorf("%s / %s: %w", target, name, err)
			}
			if desired[path] == nil {
				desired[path] = map[string]map[string]any{}
			}
			desired[path][name] = entry
		}
	}
	for _, owned := range state.Entries {
		if owned.Owner == source.ConfigPath {
			targets[owned.Path] = owned.Target
		}
	}
	proposal, _ := json.Marshal(struct {
		Servers     map[string]Server
		Targets     []string
		Resolutions []Resolution
	}{source.Servers, source.Targets, resolutions})
	revision := digest(source.configBytes) + digest(source.bytes) + digest(stateBytes) + digest(proposal)
	for _, path := range sortedKeys(targets) {
		target := targets[path]
		data, exists, mode, err := safeRead(path)
		if err != nil {
			return nil, err
		}
		native, err := ParseNative(target, data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		f := &filePlan{path: path, target: target, before: data, exists: exists, mode: mode, changes: map[string]map[string]any{}}
		revision += path + fmt.Sprint(exists) + digest(data)
		names := map[string]bool{}
		for name := range desired[path] {
			names[name] = true
		}
		for _, owned := range state.Entries {
			if owned.Owner == source.ConfigPath && owned.Path == path {
				names[owned.Name] = true
			}
		}
		for _, name := range sortedKeys(names) {
			key := ownershipKey(path, name)
			owned, managed := state.Entries[key]
			current := native.Entries[name]
			want := desired[path][name]
			for _, resolution := range resolutions {
				if resolution.Target != target || resolution.Name != name {
					continue
				}
				matched[target+"\x00"+name] = true
				if managed && owned.Owner != source.ConfigPath {
					break
				}
				if resolution.Action != "replace" && resolution.Action != "adopt" {
					return nil, fmt.Errorf("unknown conflict resolution")
				}
				if resolution.Action == "adopt" && !sameEntry(current, want) {
					return nil, fmt.Errorf("%s / %s differs from the imported source; preview an explicit replacement", target, name)
				}
				owned = ownership{Owner: source.ConfigPath, Target: target, Path: path, Name: name, Hash: entryHash(current)}
				managed = true
				p.state.Entries[key] = owned
			}
			change := Change{Target: target, Path: path, Name: name}
			switch {
			case managed && owned.Owner != source.ConfigPath:
				change.Action, change.Message = "conflict", "managed by another Skillshare config"
			case managed && entryHash(current) != owned.Hash:
				change.Action, change.Message = "conflict", "Agent configuration changed; import it or explicitly replace this entry"
			case !managed && current != nil:
				change.Action, change.Message = "conflict", "existing entry is not managed; import it to explicitly adopt it"
			case want == nil:
				change.Action = "remove"
				f.changes[name] = nil
				delete(p.state.Entries, key)
			case sameEntry(current, want):
				change.Action = "unchanged"
			default:
				change.Action = "add"
				if managed {
					change.Action = "update"
				}
				f.changes[name] = want
				p.state.Entries[key] = ownership{Owner: source.ConfigPath, Target: target, Path: path, Name: name, Hash: entryHash(want)}
			}
			if change.Action == "conflict" {
				p.Blocked = true
			}
			p.Changes = append(p.Changes, change)
		}
		f.after, err = native.Edit(f.changes)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		p.files = append(p.files, f)
	}
	p.Revision = digest([]byte(revision))
	for _, found := range matched {
		if !found {
			return nil, fmt.Errorf("conflict resolution does not refer to a selected MCP entry")
		}
	}
	return p, nil
}
