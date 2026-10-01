package hooks

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"skillshare/internal/utils"
)

// Source is the hooks section of one Skillshare config.
type Source struct {
	ConfigPath string
	Entries    map[string]Entry
	// Projects are hooks.projects roots, keyed by absolute clean path.
	Projects map[string]Project
	doc      yaml.Node
	bytes    []byte
	// touched are entries a draft changed; save re-encodes only these.
	touched map[string]bool
	// touchedProjects are roots whose block a draft changed; projectKeys keep each
	// root as the config spells it, so saving keeps a leading ~.
	touchedProjects map[string]bool
	projectKeys     map[string]string
	// unmanaged are the hooks a draft stops managing, keyed by project root and name.
	unmanaged map[string]bool
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
	var n yaml.Node
	if err := n.Encode(value); err != nil {
		return err
	}
	blockStyle(&n)
	node = mapping(node)
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = &n
			return nil
		}
	}
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &n)
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

func blockStyle(node *yaml.Node) {
	if node.Kind == yaml.MappingNode || node.Kind == yaml.SequenceNode {
		node.Style &^= yaml.FlowStyle
	}
	// Multi-line code and scripts stay readable as literal blocks.
	if node.Kind == yaml.ScalarNode && node.Tag == "!!str" && bytes.ContainsRune([]byte(node.Value), '\n') {
		node.Style = yaml.LiteralStyle
	}
	for _, child := range node.Content {
		blockStyle(child)
	}
}

func digest(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

// LoadSource reads hooks.entries. A config without a hooks section has no entries.
func LoadSource(configPath string) (*Source, error) { return loadSource(configPath, nil) }

func (s *Service) loadSource() (*Source, error) {
	source, err := loadSource(s.ConfigPath, s.accountAgents())
	if err == nil && s.ProjectRoot != "" && len(source.Projects) > 0 {
		return nil, fmt.Errorf("hooks.projects belongs in the global config")
	}
	return source, err
}

func loadSource(configPath string, accounts map[string]string) (*Source, error) {
	path, err := filepath.Abs(configPath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read hooks config: %w", err)
	}
	return parseSource(path, data, accounts, false)
}

func parseSource(path string, data []byte, accounts map[string]string, project bool) (*Source, error) {
	s := &Source{ConfigPath: path, Entries: map[string]Entry{}, Projects: map[string]Project{}, touched: map[string]bool{}, touchedProjects: map[string]bool{}, projectKeys: map[string]string{}, unmanaged: map[string]bool{}, bytes: data}
	d := yaml.NewDecoder(bytes.NewReader(s.bytes))
	if err := d.Decode(&s.doc); err != nil {
		if err == io.EOF {
			s.doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
			return s, nil
		}
		return nil, fmt.Errorf("invalid YAML in %s", path)
	}
	if d.Decode(new(yaml.Node)) != io.EOF {
		return nil, fmt.Errorf("%s must contain one YAML document", path)
	}
	if mapping(&s.doc).Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must be a YAML mapping", path)
	}
	var check map[string]any
	if err := s.doc.Decode(&check); err != nil {
		return nil, fmt.Errorf("invalid YAML mapping in %s; check duplicate keys", path)
	}
	section := field(&s.doc, "hooks")
	if section == nil {
		return s, nil
	}
	if section.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("hooks must be a mapping")
	}
	for i := 0; i < len(section.Content); i += 2 {
		if k := section.Content[i].Value; k != "entries" && k != "projects" {
			return nil, fmt.Errorf("unknown hooks field %q", k)
		}
	}
	var err error
	if s.Entries, err = decodeEntries(field(section, "entries"), "hooks.entries", accounts, project); err != nil {
		return nil, err
	}
	projects := field(section, "projects")
	if projects == nil {
		return s, nil
	}
	if projects.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("hooks.projects must be a mapping")
	}
	for i := 0; i+1 < len(projects.Content); i += 2 {
		key, block := projects.Content[i].Value, projects.Content[i+1]
		root, err := projectRoot(key)
		if err != nil {
			return nil, err
		}
		if _, dup := s.Projects[root]; dup {
			return nil, fmt.Errorf("hooks.projects: %s is listed twice", root)
		}
		if block.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("hooks.projects: %s must be a mapping with entries", key)
		}
		for j := 0; j < len(block.Content); j += 2 {
			if block.Content[j].Value != "entries" {
				return nil, fmt.Errorf("hooks.projects: %s takes only entries", key)
			}
		}
		entries, err := decodeEntries(field(block, "entries"), "hooks.projects."+key, accounts, true)
		if err != nil {
			return nil, err
		}
		s.Projects[root] = Project{Entries: entries}
		s.projectKeys[root] = key
	}
	return s, nil
}

func decodeEntries(node *yaml.Node, where string, accounts map[string]string, project bool) (map[string]Entry, error) {
	entries := map[string]Entry{}
	if node == nil || node.Kind == yaml.ScalarNode && node.Tag == "!!null" {
		return entries, nil
	}
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must be a mapping", where)
	}
	encoded, err := yaml.Marshal(node)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(encoded))
	decoder.KnownFields(true)
	if err := decoder.Decode(&entries); err != nil {
		return nil, fmt.Errorf("invalid %s: each entry takes description, enabled and bindings (events, code, files): %v", where, err)
	}
	if entries == nil {
		entries = map[string]Entry{}
	}
	for name, entry := range entries {
		if err := entry.normalize(); err != nil {
			return nil, fmt.Errorf("hook %s: %w", name, err)
		}
		scope := validateGlobal
		if project {
			scope = validateProject
		}
		if err := entry.validate(name, accounts, scope); err != nil {
			return nil, err
		}
		entries[name] = entry
	}
	return entries, nil
}

// projectRoot resolves a hooks.projects key to an absolute clean path.
func projectRoot(key string) (string, error) {
	root := key
	if key == "~" || strings.HasPrefix(key, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, strings.TrimPrefix(key[1:], "/"))
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("hooks.projects: %s must be an absolute path or start with ~", key)
	}
	return filepath.Clean(root), nil
}

// ValidateSection checks a config's hooks node the way LoadSource and a project's
// sync do, so the config editor never accepts a section sync would refuse.
func ValidateSection(node *yaml.Node, project bool, accounts map[string]string) error {
	if node == nil || node.Kind == 0 {
		return nil
	}
	doc := yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!str", Value: "hooks"}, node}}
	data, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	source, err := parseSource("config.yaml", data, accounts, project)
	if err != nil {
		return err
	}
	if project && len(source.Projects) > 0 {
		return fmt.Errorf("hooks.projects belongs in the global config")
	}
	return nil
}

// checkUnchanged rejects edits made after the source was read.
func (s *Source) checkUnchanged() error {
	data, err := os.ReadFile(s.ConfigPath)
	if err != nil || !bytes.Equal(data, s.bytes) {
		return ErrStaleRevision
	}
	return nil
}

// save writes only the entries a draft touched; everything else keeps its bytes' meaning.
func (s *Source) save() error {
	if len(s.touched) == 0 && len(s.touchedProjects) == 0 {
		return nil
	}
	if err := s.checkUnchanged(); err != nil {
		return err
	}
	root := mapping(&s.doc)
	section := field(root, "hooks")
	if section == nil || section.Kind != yaml.MappingNode {
		if err := put(root, "hooks", map[string]any{}); err != nil {
			return err
		}
		section = field(root, "hooks")
	}
	if len(s.touched) > 0 {
		entries := field(section, "entries")
		if entries == nil || entries.Kind != yaml.MappingNode {
			if err := put(section, "entries", map[string]any{}); err != nil {
				return err
			}
			entries = field(section, "entries")
		}
		for _, name := range sortedKeys(s.touched) {
			if entry, ok := s.Entries[name]; ok {
				if err := put(entries, name, entry); err != nil {
					return err
				}
			} else {
				drop(entries, name)
			}
		}
	}
	if len(s.touchedProjects) > 0 {
		projects := field(section, "projects")
		if projects == nil || projects.Kind != yaml.MappingNode {
			if err := put(section, "projects", map[string]any{}); err != nil {
				return err
			}
			projects = field(section, "projects")
		}
		// Only the roots that changed are re-encoded; the rest stay as written.
		for _, root := range sortedKeys(s.touchedProjects) {
			if project, ok := s.Projects[root]; ok {
				if project.Entries == nil {
					project.Entries = map[string]Entry{}
				}
				if err := put(projects, s.projectKeys[root], project); err != nil {
					return err
				}
			} else {
				drop(projects, s.projectKeys[root])
			}
		}
		if len(projects.Content) == 0 {
			drop(section, "projects")
		}
	}
	// Empty mappings are encoded in flow style, and entries added to one would inherit it,
	// so the section is expanded like the MCP section to stay readable in the editor.
	blockStyle(section)
	data, err := utils.MarshalYAML(&s.doc)
	if err != nil {
		return err
	}
	path := s.ConfigPath
	// Dotfile managers often symlink config.yaml; write its target so the link survives.
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err := atomicWrite(path, data, info.Mode().Perm()); err != nil {
		return err
	}
	s.bytes = data
	return nil
}

// ReferenceWarning reads binding keys without validating them, so target removal
// can explain dangling references before the source stops resolving.
func ReferenceWarning(configPath, target string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	var doc struct {
		Hooks struct {
			Entries map[string]struct {
				Bindings map[string]any `yaml:"bindings"`
			} `yaml:"entries"`
		} `yaml:"hooks"`
	}
	if yaml.Unmarshal(data, &doc) != nil {
		return ""
	}
	var places []string
	for _, name := range sortedKeys(doc.Hooks.Entries) {
		if _, ok := doc.Hooks.Entries[name].Bindings[target]; ok {
			places = append(places, "hooks.entries."+name)
		}
	}
	if len(places) == 0 {
		return ""
	}
	verb := "still names"
	if len(places) > 1 {
		verb = "still name"
	}
	return fmt.Sprintf("%s %s %s; remove it there too or the next hooks sync fails", strings.Join(places, ", "), verb, target)
}
