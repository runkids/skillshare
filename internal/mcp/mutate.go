package mcp

import (
	"bytes"
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"skillshare/internal/utils"
)

// Resolution is an explicit per-entry decision, never a global force switch.
type Resolution struct {
	Target string `json:"target"`
	Name   string `json:"name"`
	Action string `json:"action"`
}

// Mutation is shared by CLI and UI preview/save/apply flows.
type Mutation struct {
	// Project is a root under mcp.projects. Set, the rest of the mutation applies to that
	// project; with Remove and no Name, the project itself is dropped.
	Project string `json:"project,omitempty"`
	// Settings replaces the targets of the scope, so a value left out is cleared.
	Settings *Settings `json:"settings,omitempty"`
	Name     string    `json:"name,omitempty"`
	Server   *Server   `json:"server,omitempty"`
	Remove   bool      `json:"remove,omitempty"`
	// Unmanage, with Remove and a Name, also forgets which Agent entries the server wrote.
	// No Agent file changes, and sync then leaves those entries alone as the user's own.
	Unmanage    bool         `json:"unmanage,omitempty"`
	Resolutions []Resolution `json:"resolutions,omitempty"`
	Replace     bool         `json:"replace,omitempty"`
}

// Settings are a scope's defaults: mcp.targets, or a project's own.
type Settings struct {
	Targets []string `json:"targets,omitempty"`
	// DirectTools is ignored since 0.23.0, which converts it to each server's own tool
	// exposure. It is accepted so a dashboard that still sends it can save.
	DirectTools any `json:"directTools,omitempty"`
}

func (v Settings) validate() error {
	return validateTargets(v.Targets)
}

// draftProject applies one mutation to a root under mcp.projects.
func (s *Source) draftProject(m Mutation) error {
	root, err := expandHome(m.Project)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(root) {
		return fmt.Errorf("mcp.projects: %s must be an absolute path or start with ~", m.Project)
	}
	root = filepath.Clean(root)
	project, exists := s.Projects[root]
	if _, known := s.projectKeys[root]; !known {
		s.projectKeys[root] = m.Project
	}
	s.touched[root] = true
	switch {
	case m.Remove && m.Name == "":
		if !exists {
			return fmt.Errorf("project %s is not in mcp.projects", m.Project)
		}
		delete(s.Projects, root)
		return nil
	case m.Remove:
		if _, ok := project.Servers[m.Name]; !ok {
			return fmt.Errorf("MCP server %q not found in %s", m.Name, m.Project)
		}
		delete(project.Servers, m.Name)
		if m.Unmanage {
			s.unmanaged[root+"\x00"+m.Name] = true
		}
	case m.Server != nil:
		if _, ok := project.Servers[m.Name]; ok && !m.Replace {
			return fmt.Errorf("MCP server %q already exists in %s; explicitly choose edit or replace", m.Name, m.Project)
		}
		if err := m.Server.Validate(m.Name); err != nil {
			return err
		}
		if project.Servers == nil {
			project.Servers = map[string]Server{}
		}
		project.Servers[m.Name] = *m.Server
	}
	if m.Settings != nil {
		if exists && m.Name == "" && !m.Replace {
			return fmt.Errorf("project %s is already in mcp.projects", m.Project)
		}
		if err := m.Settings.validate(); err != nil {
			return err
		}
		project.Targets = m.Settings.Targets
	}
	if s.Projects == nil {
		s.Projects = map[string]Project{}
	}
	s.Projects[root] = project
	return nil
}

func (s *Service) draftMutations(mutations []Mutation) (*Source, error) {
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, m := range mutations {
		key := m.Project + "\x00" + m.Name
		if m.Name != "" && seen[key] {
			return nil, fmt.Errorf("duplicate MCP mutation for %q", m.Name)
		}
		seen[key] = true
		if m.Remove && m.Server != nil {
			return nil, fmt.Errorf("cannot add and remove the same server")
		}
		if m.Unmanage && (!m.Remove || m.Name == "") {
			return nil, fmt.Errorf("stop managing applies only to removing a named server")
		}
		if m.Project != "" {
			if s.ProjectRoot != "" {
				return nil, fmt.Errorf("mcp.projects belongs in the global config")
			}
			if err := source.draftProject(m); err != nil {
				return nil, err
			}
			continue
		}
		if m.Settings != nil {
			if err := m.Settings.validate(); err != nil {
				return nil, err
			}
			source.Targets, source.settingsChanged = m.Settings.Targets, true
		}
		source.serversChanged = source.serversChanged || m.Remove || m.Server != nil
		if m.Remove {
			if _, ok := source.Servers[m.Name]; !ok {
				return nil, fmt.Errorf("MCP server %q not found", m.Name)
			}
			delete(source.Servers, m.Name)
			if m.Unmanage {
				source.unmanaged["\x00"+m.Name] = true
			}
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
	external := s.Path != s.ConfigPath
	if external && s.serversChanged {
		if err := put(&s.doc, "servers", s.Servers); err != nil {
			return err
		}
		if err := writeYAML(s.Path, &s.doc, mapping(&s.doc)); err != nil {
			return err
		}
	}
	if external && !s.settingsChanged && len(s.touched) == 0 {
		return nil
	}
	doc := &s.configDoc
	mcp := field(doc, "mcp")
	if mcp == nil {
		if err := put(doc, "mcp", map[string]any{}); err != nil {
			return err
		}
		mcp = field(doc, "mcp")
	}
	if !external && s.serversChanged {
		if err := put(mcp, "servers", s.Servers); err != nil {
			return err
		}
	}
	if s.settingsChanged {
		if err := putOrDrop(mcp, "targets", s.Targets, len(s.Targets) > 0); err != nil {
			return err
		}
	}
	if len(s.touched) > 0 {
		if field(mcp, "projects") == nil {
			if err := put(mcp, "projects", map[string]any{}); err != nil {
				return err
			}
		}
		// Only the roots that changed are re-encoded; the rest stay as the user wrote them.
		projects := field(mcp, "projects")
		for root := range s.touched {
			project, kept := s.Projects[root]
			if err := putOrDrop(projects, s.projectKeys[root], project, kept); err != nil {
				return err
			}
		}
		if len(projects.Content) == 0 {
			drop(mcp, "projects")
		}
	}
	inlineOrphanAliases(doc)
	return writeYAML(s.ConfigPath, doc, mcp)
}

// NeedsMigration reports settings 0.23.0 retired that loading converted in memory only.
func (s *Source) NeedsMigration() bool { return s.migrateConfig || s.migrateExternal }

// MigratedFile is a config file a sync saved without the settings 0.23.0 retired, and the
// backup of its previous content.
type MigratedFile struct {
	Path   string `json:"path"`
	Backup string `json:"backup,omitempty"`
}

// saveMigration writes back what loading converted from settings 0.23.0 retired, and
// nothing else: the documents already hold the converted fields. backup, when set, keeps
// each file's current content first.
func (s *Source) saveMigration(backup func(path string) (string, error)) ([]MigratedFile, error) {
	var saved []MigratedFile
	if !s.NeedsMigration() {
		return nil, nil
	}
	if err := s.CheckUnchanged(); err != nil {
		return nil, err
	}
	write := func(path string, doc, section *yaml.Node) error {
		file := MigratedFile{Path: path}
		if backup != nil {
			var err error
			if file.Backup, err = backup(path); err != nil {
				return err
			}
		}
		if err := writeYAML(path, doc, section); err != nil {
			return err
		}
		saved = append(saved, file)
		return nil
	}
	if s.migrateExternal {
		if err := write(s.Path, &s.doc, mapping(&s.doc)); err != nil {
			return saved, err
		}
	}
	if s.migrateConfig {
		inlineOrphanAliases(&s.configDoc)
		if err := write(s.ConfigPath, &s.configDoc, field(&s.configDoc, "mcp")); err != nil {
			return saved, err
		}
	}
	s.migrateConfig, s.migrateExternal, s.piExtensionSettings, s.Notices = false, false, false, nil
	return saved, nil
}

func putOrDrop(node *yaml.Node, key string, value any, keep bool) error {
	if keep {
		return put(node, key, value)
	}
	drop(node, key)
	return nil
}

func drop(node *yaml.Node, key string) {
	node = mapping(node)
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return
		}
	}
}

// writeYAML saves doc to path. Empty mappings are encoded in flow style, so the generated
// section is expanded first to stay readable in the editor.
func writeYAML(path string, doc, section *yaml.Node) error {
	blockCollections(section)
	data, err := utils.MarshalYAML(doc)
	if err != nil {
		return err
	}
	// Dotfile managers often symlink config.yaml; write its target so the link survives.
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	_, _, mode, err := safeRead(path)
	if err != nil {
		return err
	}
	return atomicWrite(path, data, mode)
}

// inlineOrphanAliases writes out in full every alias whose anchor put just dropped by
// re-encoding mcp.servers, such as a project reusing a global server. Left alone, the
// saved file would not parse.
func inlineOrphanAliases(doc *yaml.Node) {
	kept := map[*yaml.Node]bool{}
	var walk func(n *yaml.Node, visit func(*yaml.Node))
	walk = func(n *yaml.Node, visit func(*yaml.Node)) {
		visit(n)
		for _, child := range n.Content {
			walk(child, visit)
		}
	}
	walk(doc, func(n *yaml.Node) { kept[n] = true })
	walk(doc, func(n *yaml.Node) {
		for i, child := range n.Content {
			if child.Kind == yaml.AliasNode && !kept[child.Alias] {
				n.Content[i] = expandAliases(child.Alias)
			}
		}
	})
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
	for _, m := range mutations {
		if m.Unmanage && sync {
			return nil, fmt.Errorf("stop managing keeps Agent files as they are, so it cannot be combined with sync")
		}
	}
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
		if _, err := s.render(source); err != nil {
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
		changed = changed || m.Server != nil || m.Remove || m.Settings != nil
	}
	if changed {
		if err := source.save(); err != nil {
			return nil, err
		}
	}
	if !sync {
		// Saving an explicit import also records the native baseline. The next
		// sync can update it, but still detects any edits made in the meantime.
		if len(resolutions) > 0 && preview.Blocked {
			return nil, fmt.Errorf("source saved; unresolved conflicts prevented adoption")
		}
		// Stopping to manage a server drops those records instead, so sync leaves its entries be.
		if len(resolutions) > 0 || len(source.unmanaged) > 0 {
			state, stateBytes, err := s.loadLedger()
			if err != nil {
				return nil, err
			}
			if preview != nil && !bytes.Equal(stateBytes, preview.stateBytes) {
				return nil, fmt.Errorf("source saved; ownership changed, preview again")
			}
			if len(resolutions) > 0 {
				for _, f := range preview.files {
					_, _, _, native, err := refreshFile(f)
					if err != nil {
						return nil, fmt.Errorf("source saved; %w", err)
					}
					for _, r := range resolutions {
						if r.Target != s.shownAs(f.target, f.path, source.Accounts) || native.Entries[r.Name] == nil {
							continue
						}
						approved, ok := preview.state.Entries[ownershipKey(f.target, f.path, r.Name)]
						if !ok || approved.Owner != source.ConfigPath {
							continue
						}
						approved.Hash = entryHash(managedEntry(f.target, native.Entries[r.Name]))
						state.Entries[ownershipKey(f.target, f.path, r.Name)] = approved
					}
				}
			}
			source.forget(state)
			if err := writeJSONFile(s.statePath(), state); err != nil {
				return nil, fmt.Errorf("source saved; ownership update failed: %w", err)
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
	return result, s.migrate(result)
}
