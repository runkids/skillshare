package plugin

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

type document struct {
	raw      []byte
	node     yaml.Node
	packages map[string]Package
}

func (s *Service) load() (*document, error) {
	raw, err := os.ReadFile(s.ConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return decodeDocument(raw)
}

// Validate checks plugin declarations without reading or writing native state.
func Validate(raw []byte) error { _, err := decodeDocument(raw); return err }

func decodeDocument(raw []byte) (*document, error) {
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
	for name, p := range d.packages {
		if !namePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid plugin package name %q", name)
		}
		for target, b := range p.Bindings {
			if !slices.Contains(Targets, target) || !validTargetID(target, b.ID) {
				return nil, fmt.Errorf("invalid plugin binding for %s", name)
			}
		}
	}
	return d, nil
}

func (s *Service) save(d *document) error {
	current, err := os.ReadFile(s.ConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !bytes.Equal(current, d.raw) {
		return fmt.Errorf("configuration changed; preview again")
	}
	var n yaml.Node
	if err := n.Encode(struct {
		Packages map[string]Package `yaml:"packages"`
	}{d.packages}); err != nil {
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
	data, err := yaml.Marshal(&d.node)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.ConfigPath), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.ConfigPath), ".plugins-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	mode := os.FileMode(0600)
	if info, err := os.Stat(s.ConfigPath); err == nil {
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
	if err = os.Rename(f.Name(), s.ConfigPath); err == nil {
		d.raw = data
	}
	return err
}
