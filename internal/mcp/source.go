package mcp

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Source is a single resolved declaration plus the files used to obtain it.
type Source struct {
	ConfigPath string            `json:"configPath"`
	Path       string            `json:"path"`
	Targets    []string          `json:"targets"`
	Servers    map[string]Server `json:"servers"`
	// Projects are project roots a global config syncs into, keyed by absolute path.
	Projects map[string]Project `json:"projects,omitempty"`
	// Accounts are the config's targets that are another config directory of an Agent.
	Accounts map[string]Account `json:"accounts,omitempty"`
	// Notices name settings the config still has that no longer apply. Loading the config
	// is not an error for them; saving it drops what can be dropped.
	Notices []string `json:"notices,omitempty"`
	// migrateConfig and migrateExternal mark the files loading converted from settings
	// 0.23.0 retired; sync writes them back so the notices go away.
	migrateConfig, migrateExternal bool
	// piExtensionSettings marks settings only pi-mcp-adapter or pi-mcp-extension read, so
	// the plan warns what Pi's built-in MCP needs (PiBuiltinNotice).
	piExtensionSettings bool
	// projectKeys holds each root as config.yaml spells it, so saving keeps a leading ~.
	projectKeys map[string]string
	// What a draft changed, so save re-encodes nothing else.
	touched                         map[string]bool
	serversChanged, settingsChanged bool
	// unmanaged are the servers a draft stops managing, keyed by project root and name.
	unmanaged   map[string]bool
	configDoc   yaml.Node
	doc         yaml.Node
	configBytes []byte
	bytes       []byte
}

// Project is what a project's own config.yaml would hold under mcp, declared in the
// global config instead so one sync reaches every root.
type Project struct {
	Targets []string          `yaml:"targets,omitempty" json:"targets,omitempty"`
	Servers map[string]Server `yaml:"servers,omitempty" json:"servers,omitempty"`
}

func mapping(node *yaml.Node) *yaml.Node {
	if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		return node.Content[0]
	}
	return node
}

func field(node *yaml.Node, key string) *yaml.Node {
	node = mapping(node)
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func put(node *yaml.Node, key string, value any) error {
	node = mapping(node)
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected YAML mapping")
	}
	var n yaml.Node
	if err := n.Encode(value); err != nil {
		return err
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = &n
			return nil
		}
	}
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &n)
	return nil
}

func expandHome(path string) (string, error) {
	// ~\ is how a Windows user writes it; elsewhere a backslash is part of a file name.
	if path == "~" || len(path) > 1 && path[0] == '~' && (path[1] == '/' || path[1] == filepath.Separator) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func parseYAML(data []byte, node *yaml.Node) error {
	d := yaml.NewDecoder(bytes.NewReader(data))
	if err := d.Decode(node); err != nil {
		return fmt.Errorf("invalid YAML; check syntax and duplicate keys")
	}
	if d.Decode(new(yaml.Node)) != io.EOF {
		return fmt.Errorf("MCP configuration must contain one YAML document")
	}
	if mapping(node).Kind != yaml.MappingNode {
		return fmt.Errorf("expected a YAML mapping")
	}
	var check map[string]any
	if err := node.Decode(&check); err != nil {
		return fmt.Errorf("invalid YAML mapping; check duplicate keys")
	}
	return nil
}

// LoadSource resolves exactly one inline or external MCP source. Missing external
// files are errors, never empty manifests eligible for pruning.
func LoadSource(configPath string) (*Source, error) {
	path, err := filepath.Abs(configPath)
	if err != nil {
		return nil, err
	}
	s := &Source{ConfigPath: path, Path: path, Servers: map[string]Server{}, projectKeys: map[string]string{}, touched: map[string]bool{}, unmanaged: map[string]bool{}}
	s.configBytes, err = os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read MCP config: %w", err)
	}
	if err = parseYAML(s.configBytes, &s.configDoc); err != nil {
		return nil, err
	}
	mcp := field(&s.configDoc, "mcp")
	if mcp != nil {
		if mcp.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("mcp must be a mapping")
		}
		for i := 0; i < len(mcp.Content); i += 2 {
			if k := mcp.Content[i].Value; k != "targets" && k != "servers" && k != "projects" && k != "directTools" {
				return nil, fmt.Errorf("unknown mcp field %q", k)
			}
		}
		if n := field(mcp, "targets"); n != nil {
			if err := n.Decode(&s.Targets); err != nil {
				return nil, fmt.Errorf("mcp.targets must be a list of clients")
			}
		}
	}
	if err := validateTargets(s.Targets); err != nil {
		return nil, err
	}
	if s.Accounts, err = parseAccounts(field(&s.configDoc, "targets")); err != nil {
		return nil, err
	}
	global := migration{defaults: s.Targets}
	for name, account := range s.Accounts {
		if account.Agent == "pi" {
			global.piAccounts = append(global.piAccounts, name)
		}
	}
	legacy := map[string][]legacyName{}
	note := func(found map[string][]string, root string, external bool) {
		for key, names := range found {
			for _, name := range names {
				legacy[key] = append(legacy[key], legacyName{name, root})
			}
		}
		if len(found) > 0 {
			s.migrateExternal = s.migrateExternal || external
			s.migrateConfig = s.migrateConfig || !external
		}
	}
	// A directTools default becomes each server's own before the servers are decoded.
	directDefault := func(scope *yaml.Node, label string) (any, error) {
		n := field(scope, "directTools")
		if n == nil {
			return nil, nil
		}
		var value any
		if err := n.Decode(&value); err != nil || !ValidDirectTools(value) {
			return nil, fmt.Errorf("%s must be true, false, \"search\" or a list of tool names", label)
		}
		// The servers it reaches are named; the default itself only needs the config saved.
		drop(scope, "directTools")
		s.migrateConfig = true
		return value, nil
	}
	var servers *yaml.Node
	if mcp != nil {
		servers = field(mcp, "servers")
		if global.directTools, err = directDefault(mcp, "mcp.directTools"); err != nil {
			return nil, err
		}
		if projects := deref(field(mcp, "projects")); projects != nil && projects.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(projects.Content); i += 2 {
				root, project := projects.Content[i].Value, deref(projects.Content[i+1])
				scope := global
				if project.Kind == yaml.MappingNode {
					if value, err := directDefault(project, "directTools ("+root+")"); err != nil {
						return nil, fmt.Errorf("mcp.projects: %s: %w", root, err)
					} else if value != nil {
						scope.directTools = value
					}
					if n := field(project, "targets"); n != nil {
						scope.defaults = nil
						_ = n.Decode(&scope.defaults)
					}
				}
				note(migrateServers(field(project, "servers"), scope), root, false)
			}
		}
		if s.Projects, err = ParseProjects(field(mcp, "projects")); err != nil {
			return nil, err
		}
		if projects := field(mcp, "projects"); projects != nil {
			for i := 0; i+1 < len(projects.Content); i += 2 {
				// ParseProjects has already accepted every key.
				root, _ := expandHome(projects.Content[i].Value)
				s.projectKeys[filepath.Clean(root)] = projects.Content[i].Value
			}
		}
	}
	if sources := field(&s.configDoc, "sources"); sources != nil {
		if sources.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("sources must be a mapping")
		}
		if external := field(sources, "mcp"); external != nil {
			if external.Tag != "!!str" || external.Value == "" {
				return nil, fmt.Errorf("sources.mcp must name a YAML file")
			}
			if servers != nil {
				return nil, fmt.Errorf("choose sources.mcp or mcp.servers, not both")
			}
			s.Path, err = expandHome(external.Value)
			if err != nil {
				return nil, err
			}
			if !filepath.IsAbs(s.Path) {
				s.Path = filepath.Join(filepath.Dir(path), s.Path)
			}
			if s.Path == path {
				return nil, fmt.Errorf("sources.mcp cannot refer to config.yaml itself")
			}
			s.bytes, err = os.ReadFile(s.Path)
			if err != nil {
				return nil, fmt.Errorf("read external MCP source: %w", err)
			}
			if err = parseYAML(s.bytes, &s.doc); err != nil {
				return nil, err
			}
			root := mapping(&s.doc)
			for i := 0; i < len(root.Content); i += 2 {
				if root.Content[i].Value != "servers" {
					return nil, fmt.Errorf("external MCP source only supports servers")
				}
			}
			servers = field(&s.doc, "servers")
			if servers == nil {
				return nil, fmt.Errorf("external MCP source requires servers (use servers: {} for an empty list)")
			}
		}
	}
	if servers != nil {
		if servers.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("MCP servers must be a mapping")
		}
		note(migrateServers(servers, global), "", s.Path != path)
		if s.Servers, err = ParseServers(servers); err != nil {
			return nil, fmt.Errorf("invalid MCP server fields: use command/args/env or url/headers/bearerToken and optional targets/transport/piOptions, or disabled with targets")
		}
	}
	for name, server := range s.Servers {
		if err := server.Validate(name); err != nil {
			return nil, err
		}
	}
	s.Notices = legacyNotices(legacy)
	for key := range legacy {
		if key == "piTools" || strings.HasPrefix(key, "piOptions.") || strings.HasPrefix(key, "piExtension.") {
			s.piExtensionSettings = true
		}
	}
	return s, s.checkTargets()
}

// legacyName is a server that had a setting 0.23.0 retired, and the mcp.projects root it
// is under, empty for the scope's own servers.
type legacyName struct{ name, root string }

// legacyNotices explains, one line per kind, the settings 0.23.0 retired that the config
// still has. Each server is named once; one found only under mcp.projects adds the folder
// names of its roots.
func legacyNotices(legacy map[string][]legacyName) []string {
	list := func(keys ...string) string {
		roots := map[string][]string{}
		for _, key := range keys {
			for _, n := range legacy[key] {
				folder := ""
				if n.root != "" {
					folder = filepath.Base(filepath.Clean(n.root))
				}
				if !slices.Contains(roots[n.name], folder) {
					roots[n.name] = append(roots[n.name], folder)
				}
			}
		}
		var names []string
		for _, name := range sortedKeys(roots) {
			folders := roots[name]
			if slices.Contains(folders, "") {
				names = append(names, name)
				continue
			}
			slices.Sort(folders)
			names = append(names, name+" ("+strings.Join(folders, ", ")+")")
		}
		return strings.Join(names, ", ")
	}
	var notices []string
	add := func(text string, keys ...string) {
		if names := list(keys...); names != "" {
			notices = append(notices, text+": "+names)
		}
	}
	add("Pi now uses its built-in MCP; the next sync updates the config", "piExtension")
	add("piOptionsPrune is no longer used; the next sync removes it", "piOptionsPrune")
	var adapter, keys []string
	for _, key := range adapterPiOptions {
		if len(legacy["piOptions."+key]) > 0 {
			adapter, keys = append(adapter, key), append(keys, "piOptions."+key)
		}
	}
	if len(adapter) > 0 {
		add("Pi's built-in MCP does not read "+strings.Join(adapter, ", ")+"; the next sync removes them", keys...)
	}
	add("directTools, includeTools and excludeTools become tool settings; the next sync converts them", "piTools")
	add("directTools, includeTools or excludeTools the server already covers, or that are not tool lists, are dropped; the next sync removes them", "piTools.directTools", "piTools.excludeTools", "piTools.includeTools")
	return notices
}

// parseAccounts reads the targets that are another config directory of an Agent whose MCP
// file follows that directory. The config package validates the section itself.
func parseAccounts(node *yaml.Node) (map[string]Account, error) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, nil
	}
	var targets map[string]struct {
		Agent string `yaml:"agent"`
		Dir   string `yaml:"config_dir"`
	}
	if err := node.Decode(&targets); err != nil {
		return nil, nil
	}
	accounts := map[string]Account{}
	for name, target := range targets {
		if validTarget(name) || !slices.Contains(accountAgents, target.Agent) || target.Dir == "" {
			continue
		}
		dir, err := expandHome(target.Dir)
		if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(dir) {
			return nil, fmt.Errorf("targets: %s: config_dir must be an absolute path or start with ~", name)
		}
		accounts[name] = Account{Agent: target.Agent, Dir: filepath.Clean(dir)}
	}
	return accounts, nil
}

// checkTargets settles the names validateTargets let through: each is an account. A
// project's files are read by every account of an Agent, so there an account can only
// be named by a switch, which Claude Code keeps in the account's own file.
func (s *Source) checkTargets() error {
	check := func(targets []string, accounts bool) error {
		for _, target := range targets {
			if validTarget(target) {
				continue
			}
			account, ok := s.Accounts[target]
			if !ok {
				return fmt.Errorf("unsupported MCP target %q: it is neither an Agent nor a target with agent and config_dir", target)
			}
			if !accounts {
				return fmt.Errorf("mcp.projects: %s is an account of %s, and every account reads the same project files; use %s", target, account.Agent, account.Agent)
			}
		}
		return nil
	}
	if err := check(s.Targets, true); err != nil {
		return err
	}
	for _, server := range s.Servers {
		if err := check(server.Targets, true); err != nil {
			return err
		}
	}
	for _, project := range s.Projects {
		if err := check(project.Targets, false); err != nil {
			return err
		}
		for _, server := range project.Servers {
			if err := check(server.Targets, server.Disabled); err != nil {
				return err
			}
		}
	}
	return nil
}

// References names every place in the MCP config that still selects target. Removing a
// target it names leaves a name no sync can resolve, so a removal can say where to look.
func (s *Source) References(target string) []string {
	var places []string
	if slices.Contains(s.Targets, target) {
		places = append(places, "mcp.targets")
	}
	for _, name := range sortedKeys(s.Servers) {
		if slices.Contains(s.Servers[name].Targets, target) {
			places = append(places, "mcp.servers."+name)
		}
	}
	return places
}

// ReferenceWarning is what to tell someone removing a target the MCP config still names,
// or "" when nothing does. A config that cannot be read says nothing: the removal stands.
func ReferenceWarning(configPath, target string) string {
	source, err := LoadSource(configPath)
	if err != nil {
		return ""
	}
	places := source.References(target)
	if len(places) == 0 {
		return ""
	}
	names := "still names"
	if len(places) > 1 {
		names = "still name"
	}
	return fmt.Sprintf("%s %s %s; remove it there too or the next mcp sync fails", strings.Join(places, ", "), names, target)
}

// ParseServers decodes a servers mapping strictly, after migrateServers has taken out what
// 0.23.0 retired. It does not validate the servers.
func ParseServers(node *yaml.Node) (map[string]Server, error) {
	data, err := yaml.Marshal(expandAliases(node))
	if err != nil {
		return nil, err
	}
	var copied yaml.Node
	if err := yaml.Unmarshal(data, &copied); err != nil {
		return nil, err
	}
	migrateServers(mapping(&copied), migration{})
	if data, err = yaml.Marshal(&copied); err != nil {
		return nil, err
	}
	servers := map[string]Server{}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&servers); err != nil {
		return nil, err
	}
	return servers, nil
}

// ParseProjects reads and validates mcp.projects. The config editor shares it, so it
// never accepts a section that sync would then refuse.
func ParseProjects(node *yaml.Node) (map[string]Project, error) {
	if node == nil {
		return nil, nil
	}
	// The section is decoded on its own to reject unknown fields, so an alias to an
	// anchor outside it, such as one on mcp.servers, has to be expanded first.
	data, err := yaml.Marshal(expandAliases(node))
	if err != nil {
		return nil, err
	}
	var copied yaml.Node
	if err := yaml.Unmarshal(data, &copied); err != nil {
		return nil, err
	}
	if projects := mapping(&copied); projects.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(projects.Content); i += 2 {
			drop(projects.Content[i+1], "directTools")
			migrateServers(field(projects.Content[i+1], "servers"), migration{})
		}
	}
	if data, err = yaml.Marshal(&copied); err != nil {
		return nil, err
	}
	var declared map[string]Project
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&declared); err != nil {
		reason := "invalid value"
		var typeErr *yaml.TypeError
		if errors.As(err, &typeErr) {
			// Line numbers refer to the re-marshalled section, not the user's file.
			for i, e := range typeErr.Errors {
				if _, after, ok := strings.Cut(e, ": "); ok && strings.HasPrefix(e, "line ") {
					typeErr.Errors[i] = after
				}
			}
			reason = strings.Join(typeErr.Errors, "; ")
		}
		return nil, fmt.Errorf("mcp.projects: %s; each project root takes only targets and servers", reason)
	}
	projects := map[string]Project{}
	for root, project := range declared {
		path, err := expandHome(root)
		if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(path) {
			return nil, fmt.Errorf("mcp.projects: %s must be an absolute path or start with ~", root)
		}
		path = filepath.Clean(path)
		if _, duplicate := projects[path]; duplicate {
			return nil, fmt.Errorf("mcp.projects: %s is listed twice", path)
		}
		if err := validateTargets(project.Targets); err != nil {
			return nil, err
		}
		for name, server := range project.Servers {
			if err := server.Validate(name); err != nil {
				return nil, err
			}
		}
		projects[path] = project
	}
	return projects, nil
}

// expandAliases copies a tree with every alias replaced by what it points to. parseYAML
// has already decoded the whole document, which refuses an anchor that contains itself.
func expandAliases(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.AliasNode {
		return expandAliases(n.Alias)
	}
	c := *n
	c.Anchor, c.Content = "", nil
	for _, child := range n.Content {
		c.Content = append(c.Content, expandAliases(child))
	}
	return &c
}

func digest(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

// CheckUnchanged rejects edits made after a source was read or previewed.
func (s *Source) CheckUnchanged() error {
	data, err := os.ReadFile(s.ConfigPath)
	if err != nil || !bytes.Equal(data, s.configBytes) {
		return fmt.Errorf("configuration changed; preview again")
	}
	if s.Path != s.ConfigPath {
		data, err = os.ReadFile(s.Path)
		if err != nil || !bytes.Equal(data, s.bytes) {
			return fmt.Errorf("external MCP source changed; preview again")
		}
	}
	return nil
}
