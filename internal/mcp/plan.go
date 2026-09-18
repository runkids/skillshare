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
	section       string
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
	state := ledger{Version: 1, Entries: map[string]ownership{}}
	// Only writes recover an interrupted write, under the lock. Until then, read
	// the ownership that recovery will record, byte for byte, so a preview's
	// revision stays valid across that recovery.
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
		if json.Unmarshal(data, &state) != nil || state.Version != 1 || state.Entries == nil {
			return state, nil, fmt.Errorf("MCP ownership state is invalid; restore a backup or explicitly import entries again")
		}
	}
	return state, data, nil
}

// Preview reads source, ownership and native files without writing. Writes
// recover an interrupted operation first, while holding the lock.
func (s *Service) Preview() (*Plan, error) {
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	return s.previewSource(source)
}

func (s *Service) previewSource(source *Source) (*Plan, error) {
	return s.previewResolved(source, nil)
}

// render builds every native entry the source asks for, keyed by native path.
func (s *Service) render(source *Source) (map[string]map[string]map[string]any, map[string]string, error) {
	desired := map[string]map[string]map[string]any{}
	targets := map[string]string{}
	piExtension := ""
	for _, name := range sortedKeys(source.Servers) {
		server := source.Servers[name]
		selected := server.Targets
		if selected == nil {
			selected = source.Targets
		}
		if len(selected) == 0 {
			return nil, nil, fmt.Errorf("MCP %s has no targets; select at least one Agent", name)
		}
		for _, target := range selected {
			if target == "pi" {
				if s.ProjectRoot == "" && s.ConfigDirs["pi"] != "" && server.PiExtension == "pi-mcp-extension" {
					return nil, nil, fmt.Errorf("pi-mcp-extension uses ~/.pi/agent/mcp.json and does not honor PI_CODING_AGENT_DIR; unset the override before syncing")
				}
				if piExtension != "" && piExtension != server.PiExtension {
					return nil, nil, fmt.Errorf("Pi servers share one config file; select the same piExtension for every Pi server")
				}
				piExtension = server.PiExtension
			}
			if target == "grok" && (!grokServerName.MatchString(name) || strings.Contains(name, "__") || strings.HasSuffix(name, "_")) {
				return nil, nil, fmt.Errorf("Grok MCP %s: use a name starting with a letter or underscore, containing only letters, digits, hyphens and single underscores, and not ending in underscore", name)
			}
			path, err := s.nativePath(target)
			if err != nil {
				return nil, nil, err
			}
			targets[path] = target
			entry, err := Render(target, server)
			if err != nil {
				return nil, nil, fmt.Errorf("%s / %s: %w", target, name, err)
			}
			if target == "goose" {
				entry["name"] = name
			}
			if desired[path] == nil {
				desired[path] = map[string]map[string]any{}
			}
			desired[path][name] = entry
		}
	}
	if s.ProjectRoot != "" {
		if desired[filepath.Join(s.ProjectRoot, ".mcp.json")] != nil && desired[filepath.Join(s.ProjectRoot, ".github", "mcp.json")] != nil {
			return nil, nil, fmt.Errorf("Claude and Copilot project MCP destinations overlap in precedence; use global mode for one client")
		}
	}
	return desired, targets, nil
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
	desired, targets, err := s.render(source)
	if err != nil {
		return nil, err
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
		f := &filePlan{path: path, target: target, before: data, exists: exists, mode: mode, section: sectionDigest(native), changes: map[string]map[string]any{}}
		revision += path + f.section
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
			currentHash := entryHash(managedEntry(target, current))
			want := desired[path][name]
			wantHash := entryHash(managedEntry(target, want))
			for _, resolution := range resolutions {
				if resolution.Target != target || resolution.Name != name {
					continue
				}
				matched[target+"\x00"+name] = true
				if managed && owned.Owner != source.ConfigPath {
					break
				}
				// adopt claims only an entry that already matches; anything else
				// stays a conflict for an explicit replace.
				if resolution.Action == "adopt" && currentHash != wantHash {
					continue
				}
				owned = ownership{Owner: source.ConfigPath, Target: target, Path: path, Name: name, Hash: currentHash}
				managed = true
				p.state.Entries[key] = owned
			}
			change := Change{Target: target, Path: path, Name: name}
			switch {
			case managed && owned.Owner != source.ConfigPath:
				change.Action, change.Message = "conflict", "managed by another Skillshare config"
			case currentHash == wantHash && managed && want != nil && native.cramped(name):
				// The content is right but it is all on one line. Sync owns this entry, so it
				// writes it again, laid out; the person pressing Sync expects a file they can read.
				change.Action, change.Message = "update", "same settings, laid out one field per line"
				f.changes[name] = withAgentFields(target, current, want)
				p.state.Entries[key] = ownership{Owner: source.ConfigPath, Target: target, Path: path, Name: name, Hash: wantHash}
			case currentHash == wantHash:
				// Already as desired, e.g. after pulling a teammate's change or
				// moving a project. Refresh an owned baseline, but never claim an
				// unmanaged entry: removing the server must not delete the user's own.
				if want == nil {
					delete(p.state.Entries, key)
					continue
				}
				change.Action = "unchanged"
				if managed {
					p.state.Entries[key] = ownership{Owner: source.ConfigPath, Target: target, Path: path, Name: name, Hash: currentHash}
				}
			case managed && currentHash != owned.Hash:
				change.Action, change.Message = "conflict", "Agent configuration changed; import it or explicitly replace this entry"
			case !managed && current != nil:
				change.Action, change.Message = "conflict", "existing entry is not managed; import it to explicitly adopt it"
			case want == nil:
				change.Action = "remove"
				f.changes[name] = nil
				delete(p.state.Entries, key)
			default:
				change.Action = "add"
				if managed {
					change.Action = "update"
				}
				f.changes[name] = withAgentFields(target, current, want)
				p.state.Entries[key] = ownership{Owner: source.ConfigPath, Target: target, Path: path, Name: name, Hash: wantHash}
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
