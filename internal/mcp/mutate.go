package mcp

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"

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
	// Using, with Project and Name, turns that shared server on or off for the root. It
	// changes one name, so two edits made from pages loaded at the same time both stay.
	Using *bool `json:"using,omitempty"`
	// Override, with Project and Name, replaces what that root changes in a shared server it
	// uses; an empty one removes it. It may come with Using, so a root turned on gets it too.
	Override *Override `json:"override,omitempty"`
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
		if m.Server.Global != nil {
			return fmt.Errorf("MCP %s: global applies only to servers in mcp.servers", m.Name)
		}
		if project.Servers == nil {
			project.Servers = map[string]Server{}
		}
		project.Servers[m.Name] = *m.Server
	}
	if m.Using != nil {
		project.Use = slices.DeleteFunc(slices.Clone(project.Use), func(name string) bool { return name == m.Name })
		if *m.Using {
			project.Use = append(project.Use, m.Name)
		} else {
			project.Overrides = setOverride(project.Overrides, m.Name, Override{})
		}
		if len(project.Use) == 0 {
			project.Use = nil
		}
	}
	if m.Override != nil {
		if !slices.Contains(project.Use, m.Name) {
			return fmt.Errorf("project %s does not use %s, so there is nothing to override", m.Project, m.Name)
		}
		project.Overrides = setOverride(project.Overrides, m.Name, *m.Override)
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
		if m.Using != nil && (m.Project == "" || m.Name == "" || m.Server != nil || m.Remove) {
			return nil, fmt.Errorf("using turns one shared server on or off for a project; give only the project and the name")
		}
		if m.Override != nil && (m.Project == "" || m.Name == "" || m.Server != nil || m.Remove) {
			return nil, fmt.Errorf("override changes one shared server for a project; give the project and the name")
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
				// A shared server's entries belong to the projects that use it.
				for _, root := range source.Users(m.Name) {
					source.unmanaged[root+"\x00"+m.Name] = true
				}
			}
			source.dropUse(m.Name)
		}
		if m.Server != nil {
			old, exists := source.Servers[m.Name]
			if exists && !m.Replace {
				return nil, fmt.Errorf("MCP server %q already exists; explicitly choose edit or replace", m.Name)
			}
			if err := m.Server.Validate(m.Name); err != nil {
				return nil, err
			}
			server := *m.Server
			// Writers rebuild a server from its fields, so one that does not say keeps where
			// the server loads; only an explicit global moves it.
			if server.Global == nil {
				server.Global = old.Global
			}
			if !server.Shared() {
				server.Global = nil
				source.dropUse(m.Name)
			} else if s.ProjectRoot != "" {
				return nil, RefuseShared(map[string]Server{m.Name: server})
			}
			source.Servers[m.Name] = server
		}
	}
	return source, nil
}

// dropUse takes a server that is no longer shared out of every root that uses it, so the
// config never names a server that loads everywhere, or none, under use.
func (s *Source) dropUse(name string) {
	for _, root := range s.Users(name) {
		project := s.Projects[root]
		project.Use = slices.DeleteFunc(slices.Clone(project.Use), func(n string) bool { return n == name })
		if len(project.Use) == 0 {
			project.Use = nil
		}
		project.Overrides = setOverride(project.Overrides, name, Override{})
		s.Projects[root] = project
		s.touched[root] = true
	}
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

// ResolutionConflicts reports whether an entry one of resolutions names is still a conflict.
// Other entries may conflict without stopping a save that only adopts or replaces these.
func (p *Plan) ResolutionConflicts(resolutions []Resolution) bool {
	return slices.ContainsFunc(p.Changes, func(c Change) bool {
		return c.Action == "conflict" && slices.ContainsFunc(resolutions, func(r Resolution) bool { return r.Target == c.Target && r.Name == c.Name })
	})
}

func batchResolutions(mutations []Mutation) []Resolution {
	var resolutions []Resolution
	for _, m := range mutations {
		resolutions = append(resolutions, m.Resolutions...)
	}
	return resolutions
}

// save writes the changed sections. projectRoot is set for a project config, whose
// files must stay inside the project.
func (s *Source) save(projectRoot string) error {
	if err := s.CheckUnchanged(); err != nil {
		return err
	}
	external := s.Path != s.ConfigPath
	if external && s.serversChanged {
		if err := put(&s.doc, "servers", s.Servers); err != nil {
			return err
		}
		if err := writeYAML(s.Path, projectRoot, &s.doc, mapping(&s.doc)); err != nil {
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
	return writeYAML(s.ConfigPath, projectRoot, doc, mcp)
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
// each file's current content first. projectRoot is set for a project config.
func (s *Source) saveMigration(backup func(path string) (string, error), projectRoot string) ([]MigratedFile, error) {
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
		if err := writeYAML(path, projectRoot, doc, section); err != nil {
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
// section is expanded first to stay readable in the editor. projectRoot is set for a
// project config, whose files must stay inside the project.
func writeYAML(path, projectRoot string, doc, section *yaml.Node) error {
	blockCollections(section)
	data, err := utils.MarshalYAML(doc)
	if err != nil {
		return err
	}
	// Dotfile managers often symlink config.yaml; write its target so the link survives.
	if path, err = utils.ProjectWritePath(path, projectRoot); err != nil {
		return err
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
		changed = changed || m.Server != nil || m.Remove || m.Settings != nil || m.Using != nil || m.Override != nil
	}
	if changed {
		if err := source.save(s.configProjectRoot()); err != nil {
			return nil, err
		}
	}
	if !sync {
		// Saving an explicit import also records the native baseline. The next
		// sync can update it, but still detects any edits made in the meantime.
		// Only the resolved entries have to settle: another Agent that holds a
		// different definition of the server stays a conflict of its own.
		if len(resolutions) > 0 && preview.ResolutionConflicts(resolutions) {
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
			source.forget(state, s.ProjectRoot)
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
