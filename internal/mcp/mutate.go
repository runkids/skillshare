package mcp

import (
	"bytes"
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Resolution is an explicit per-entry decision, never a global force switch.
type Resolution struct {
	Target string `json:"target"`
	Name   string `json:"name"`
	Action string `json:"action"`
}

// Mutation is shared by CLI and UI preview/save/apply flows.
type Mutation struct {
	Name        string       `json:"name,omitempty"`
	Server      *Server      `json:"server,omitempty"`
	Remove      bool         `json:"remove,omitempty"`
	Resolutions []Resolution `json:"resolutions,omitempty"`
	Replace     bool         `json:"replace,omitempty"`
}

func (s *Service) draftMutations(mutations []Mutation) (*Source, error) {
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, m := range mutations {
		if m.Name != "" && seen[m.Name] {
			return nil, fmt.Errorf("duplicate MCP mutation for %q", m.Name)
		}
		seen[m.Name] = true
		if m.Remove && m.Server != nil {
			return nil, fmt.Errorf("cannot add and remove the same server")
		}
		if m.Remove {
			if _, ok := source.Servers[m.Name]; !ok {
				return nil, fmt.Errorf("MCP server %q not found", m.Name)
			}
			delete(source.Servers, m.Name)
		}
		if m.Server != nil {
			if _, exists := source.Servers[m.Name]; exists && !m.Replace {
				return nil, fmt.Errorf("MCP server %q already exists; explicitly choose edit or replace", m.Name)
			}
			if err := m.Server.Validate(m.Name); err != nil {
				return nil, err
			}
			source.Servers[m.Name] = *m.Server
		}
	}
	return source, nil
}

// PreviewMutation does not persist a draft or claim ownership.
func (s *Service) PreviewMutation(m Mutation) (*Plan, error) {
	return s.PreviewMutations([]Mutation{m})
}

// PreviewMutations validates a complete batch without persisting any entry.
func (s *Service) PreviewMutations(mutations []Mutation) (*Plan, error) {
	source, err := s.draftMutations(mutations)
	if err != nil {
		return nil, err
	}
	return s.previewResolved(source, batchResolutions(mutations))
}

func batchResolutions(mutations []Mutation) []Resolution {
	var resolutions []Resolution
	for _, m := range mutations {
		resolutions = append(resolutions, m.Resolutions...)
	}
	return resolutions
}

func (s *Source) save() error {
	if err := s.CheckUnchanged(); err != nil {
		return err
	}
	doc := &s.configDoc
	if s.Path != s.ConfigPath {
		doc = &s.doc
		if err := put(doc, "servers", s.Servers); err != nil {
			return err
		}
	} else {
		mcp := field(doc, "mcp")
		if mcp == nil {
			if err := put(doc, "mcp", map[string]any{}); err != nil {
				return err
			}
			mcp = field(doc, "mcp")
		}
		if err := put(mcp, "servers", s.Servers); err != nil {
			return err
		}
	}
	// Empty mappings are encoded in flow style; expand the generated MCP
	// subtree before adding connections so it remains readable in the editor.
	section := field(doc, "mcp")
	if s.Path != s.ConfigPath {
		section = mapping(doc)
	}
	blockCollections(section)
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc); err != nil {
		return err
	}
	data := buffer.Bytes()
	// Dotfile managers often symlink config.yaml; write its target so the link survives.
	path := s.Path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	_, _, mode, err := safeRead(path)
	if err != nil {
		return err
	}
	return atomicWrite(path, data, mode)
}

func blockCollections(node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind == yaml.MappingNode || node.Kind == yaml.SequenceNode {
		node.Style &^= yaml.FlowStyle
	}
	for _, child := range node.Content {
		blockCollections(child)
	}
}

// Mutate validates before saving. With sync enabled, it preflights all native
// destinations before saving the source. Later I/O failures report partial work.
func (s *Service) Mutate(m Mutation, revision string, sync bool) (*Result, error) {
	return s.MutateBatch([]Mutation{m}, revision, sync)
}

// MutateBatch preflights every entry and saves the source once under one lock.
// Native writes use the existing recovery journal, just like a single mutation.
func (s *Service) MutateBatch(mutations []Mutation, revision string, sync bool) (*Result, error) {
	lock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()
	if err := s.recoverPending(); err != nil {
		return nil, err
	}
	source, err := s.draftMutations(mutations)
	if err != nil {
		return nil, err
	}
	var preview *Plan
	resolutions := batchResolutions(mutations)
	if !sync && revision == "" && len(resolutions) == 0 {
		// Save only still refuses definitions that could never synchronize.
		if _, _, err := s.render(source); err != nil {
			return nil, err
		}
	} else {
		p, err := s.previewResolved(source, resolutions)
		if err != nil {
			return nil, err
		}
		preview = p
		if revision != "" && revision != p.Revision {
			return nil, fmt.Errorf("MCP configuration changed since preview; preview again")
		}
		if sync && p.Blocked {
			return &Result{Plan: p}, fmt.Errorf("MCP conflicts found; no files changed")
		}
	}
	changed := false
	for _, m := range mutations {
		changed = changed || m.Server != nil || m.Remove
	}
	if changed {
		if err := source.save(); err != nil {
			return nil, err
		}
	}
	if !sync {
		// Saving an explicit import also records the native baseline. The next
		// sync can update it, but still detects any edits made in the meantime.
		if len(resolutions) > 0 {
			if preview.Blocked {
				return nil, fmt.Errorf("source saved; unresolved conflicts prevented adoption")
			}
			state, stateBytes, err := s.loadLedger()
			if err != nil {
				return nil, err
			}
			if !bytes.Equal(stateBytes, preview.stateBytes) {
				return nil, fmt.Errorf("source saved; ownership changed, preview again")
			}
			for _, f := range preview.files {
				_, _, _, native, err := refreshFile(f)
				if err != nil {
					return nil, fmt.Errorf("source saved; %w", err)
				}
				for _, r := range resolutions {
					if r.Target != f.target || native.Entries[r.Name] == nil {
						continue
					}
					approved, ok := preview.state.Entries[ownershipKey(f.path, r.Name)]
					if !ok || approved.Owner != source.ConfigPath {
						continue
					}
					state.Entries[ownershipKey(f.path, r.Name)] = ownership{Owner: source.ConfigPath, Target: f.target, Path: f.path, Name: r.Name, Hash: entryHash(managedEntry(f.target, native.Entries[r.Name]))}
				}
			}
			if err := writeJSONFile(s.statePath(), state); err != nil {
				return nil, fmt.Errorf("source saved; adoption failed: %w", err)
			}
		}
		return &Result{Applied: []string{}, BackupIDs: []string{}}, nil
	}
	source, err = LoadSource(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	p, err := s.previewResolved(source, resolutions)
	if err != nil {
		return nil, err
	}
	if preview != nil {
		if !bytes.Equal(preview.stateBytes, p.stateBytes) {
			return nil, fmt.Errorf("source saved; ownership changed, preview again")
		}
		for _, f := range preview.files {
			if _, _, _, _, err := refreshFile(f); err != nil {
				return nil, fmt.Errorf("source saved; %w", err)
			}
		}
	}
	result, err := s.applyPlan(p)
	if err != nil {
		return result, fmt.Errorf("source saved; synchronization incomplete: %w", err)
	}
	return result, nil
}
