package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
	"skillshare/internal/utils"
)

type document struct {
	raw      []byte
	node     yaml.Node
	packages map[string]Package
	intent   map[string]packageDeclaration
	legacy   map[string]Package
	state    localState
	stateRaw []byte
}

func (s *Service) load() (*document, error) {
	raw, err := os.ReadFile(s.ConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	d, err := decodeDocument(raw, s.accountAgents())
	if err != nil {
		return nil, err
	}
	if err = s.loadState(d); err != nil {
		return nil, err
	}
	// Capture passive local hints before another machine can replace legacy YAML.
	// Reads leave shared intent untouched; ownership is still proved before adoption.
	if len(d.legacy) > 0 {
		before, _ := json.Marshal(d.state)
		s.captureLegacyAndVersions(d)
		after, _ := json.Marshal(d.state)
		if !bytes.Equal(before, after) {
			// A concurrent mutation owns persistence. Keep passive hints in memory
			// until that operation, or the next unlocked read, records them.
			unlock, lockErr := s.Lock()
			if lockErr == nil {
				defer unlock()
				// Recompute under the lock: another reader may already have saved.
				if err := s.loadState(d); err != nil {
					return nil, err
				}
				before, _ = json.Marshal(d.state)
				s.captureLegacyAndVersions(d)
				after, _ = json.Marshal(d.state)
				if !bytes.Equal(before, after) {
					if err := s.saveState(d); err != nil {
						return nil, err
					}
				}
			}
		}
	}

	return d, nil
}

// Validate checks plugin declarations without reading or writing native state. accounts
// maps each target that is another config directory of an Agent to that Agent, so a
// binding is validated by the Agent that would install it.
func Validate(raw []byte, accounts map[string]string) error {
	_, err := decodeDocument(raw, accounts)
	return err
}

// decodeDocument reads the plugin section. accounts maps each account to its Agent, or to
// "" where only the names are known.
func decodeDocument(raw []byte, accounts map[string]string) (*document, error) {
	d := &document{raw: raw, packages: map[string]Package{}}
	content := raw
	if len(content) == 0 {
		content = []byte("{}\n")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	if err := decoder.Decode(&d.node); err != nil {
		return nil, err
	}
	if decoder.Decode(new(yaml.Node)) != io.EOF {
		return nil, fmt.Errorf("configuration must contain one YAML document")
	}
	var check map[string]any
	if err := d.node.Decode(&check); err != nil {
		return nil, err
	}
	if len(d.node.Content) != 1 || d.node.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("configuration must be a YAML mapping")
	}
	root := d.node.Content[0]
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "plugins" {
			var version struct {
				Version int `yaml:"version"`
			}
			if err := root.Content[i+1].Decode(&version); err != nil {
				return nil, err
			}
			if version.Version != 0 && version.Version != 2 {
				return nil, fmt.Errorf("unsupported plugin configuration version %d", version.Version)
			}
			if version.Version == 2 {
				var body struct {
					Version  int                           `yaml:"version"`
					Packages map[string]packageDeclaration `yaml:"packages"`
				}
				data, err := yaml.Marshal(root.Content[i+1])
				if err != nil {
					return nil, err
				}
				dec := yaml.NewDecoder(bytes.NewReader(data))
				dec.KnownFields(true)
				if err := dec.Decode(&body); err != nil {
					return nil, fmt.Errorf("invalid plugins configuration: %w", err)
				}
				d.intent = body.Packages
			} else {
				var body struct {
					Packages map[string]Package `yaml:"packages"`
				}
				data, err := yaml.Marshal(root.Content[i+1])
				if err != nil {
					return nil, err
				}
				dec := yaml.NewDecoder(bytes.NewReader(data))
				dec.KnownFields(true)
				if err := dec.Decode(&body); err != nil {
					return nil, fmt.Errorf("invalid plugins configuration: %w", err)
				}
				if body.Packages != nil {
					d.packages = body.Packages
				}
			}
		}
	}
	for name, p := range d.packages {
		if !namePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid plugin package name %q", name)
		}
		p.Source = canonicalSource(p.Source)
		for target, b := range p.Bindings {
			b.Source = canonicalSource(b.Source)
			p.Bindings[target] = b
			agent, account := accounts[target]
			if !account && !slices.Contains(Targets, target) {
				return nil, fmt.Errorf("invalid plugin binding for %s", name)
			}
			if !account {
				agent = target
			}
			if b.PiRegistration != "" && (agent != "pi" || !piRegistrationDigestOK(b.PiRegistration)) {
				return nil, fmt.Errorf("invalid preserved Pi registration for %s", name)
			}
			if !validTargetID(agent, b.ID) {
				return nil, fmt.Errorf("invalid plugin binding for %s", name)
			}
		}
		d.packages[name] = p
	}

	d.legacy = d.packages
	if d.intent == nil {
		d.intent = map[string]packageDeclaration{}
		for name, p := range d.legacy {
			decl := packageDeclaration{Source: p.Source, SourceRef: p.SourceRef, Plugin: p.Plugin, Entry: p.Entry, Bindings: map[string]declaration{}}
			for target, b := range p.Bindings {
				if b.Pending == "remove" {
					continue
				}
				decl.Bindings[target] = declareBinding(b)
			}
			if len(p.Bindings) == 0 || len(decl.Bindings) > 0 {
				d.intent[name] = decl
			}
		}
	}
	for name, p := range d.intent {
		if !namePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid plugin package name %q", name)
		}
		p.Source = canonicalSource(p.Source)
		for target, b := range p.Bindings {
			agent, account := accounts[target]
			if !account {
				agent = target
				if !slices.Contains(Targets, target) {
					return nil, fmt.Errorf("invalid plugin binding for %s", name)
				}
			}
			if b.Imported {
				if b.Source != "" || b.SourceRef != "" || b.Plugin != "" || b.Entry != "" {
					return nil, fmt.Errorf("imported binding cannot have a source for %s", name)
				}
			} else {
				if b.Source == "" {
					b.Source, b.SourceRef, b.Plugin, b.Entry = p.Source, p.SourceRef, p.Plugin, p.Entry
				}
				b.Source = canonicalSource(b.Source)
				if b.Source == "" {
					return nil, fmt.Errorf("source or imported required for %s", name)
				}
				if isNpmSource(b.Source) && (agent != "pi" || !validNpmSource(b.Source) || b.SourceRef != "" || b.Plugin != "" || b.Entry != "") {
					return nil, fmt.Errorf("invalid npm declaration for %s", name)
				}
			}
			p.Bindings[target] = b
		}
		d.intent[name] = p
	}
	return d, nil
}

// canonicalSource spells a local source the way add records it, so ~/plug and
// /home/me/plug name the same snapshot and owner. It expands every form
// utils.FoldHomePathWith writes: ~, ~/..., and ~\... on Windows.
func canonicalSource(source string) string {
	if source == "~" || strings.HasPrefix(source, "~/") || strings.HasPrefix(source, "~"+string(filepath.Separator)) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, source[1:])
		}
	}
	if filepath.IsAbs(source) {
		return filepath.Clean(source)
	}
	return source
}

func (s *Service) save(d *document) error {
	current, err := os.ReadFile(s.ConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !bytes.Equal(current, d.raw) {
		return fmt.Errorf("configuration changed; preview again")
	}
	packages := make(map[string]packageDeclaration, len(d.intent))
	var opts struct {
		PreserveTilde bool `yaml:"preserve_tilde_on_save"`
	}
	_ = d.node.Decode(&opts)
	home, _ := os.UserHomeDir()
	for name, p := range d.intent {
		bindings := map[string]declaration{}
		for target, b := range p.Bindings {
			if opts.PreserveTilde {
				b.Source = utils.FoldHomePathWith(b.Source, home)
			}
			bindings[target] = b
		}
		p.Bindings = bindings
		if opts.PreserveTilde {
			p.Source = utils.FoldHomePathWith(p.Source, home)
		}
		packages[name] = p
	}
	var n yaml.Node
	if err := n.Encode(struct {
		Version  int                           `yaml:"version"`
		Packages map[string]packageDeclaration `yaml:"packages"`
	}{2, packages}); err != nil {
		return err
	}
	root := d.node.Content[0]
	found := false
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "plugins" {
			root.Content[i+1] = &n
			found = true
		}
	}
	if !found {
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "plugins"}, &n)
	}
	data, err := utils.MarshalYAML(&d.node)
	if err != nil {
		return err
	}
	// Dotfile managers often symlink config.yaml; write its target so the link survives.
	path, err := utils.ConfigWritePath(s.ConfigPath, s.ProjectRoot != "")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".plugins-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	mode := os.FileMode(0600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err == nil {
		d.raw = data
	}
	return err
}
