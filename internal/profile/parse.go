package profile

import (
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"
)

// Parse reports unusable definitions without making configuration load fail.
func Parse(node *yaml.Node) Catalog {
	c := Catalog{Definitions: make(map[string]Definition)}
	if node == nil || node.Kind == 0 {
		return c
	}
	c.Present = true
	if node.Kind != yaml.MappingNode {
		c.Reason = "profiles must be a mapping"
		return c
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		d := Definition{Name: key.Value, Usable: true}
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			d.Usable, d.Reason = false, "profile name must be a string"
		} else if _, exists := c.Definitions[d.Name]; exists {
			d.Usable, d.Reason = false, "duplicate profile name"
		} else if err := parseDefinition(&d, value); err != nil {
			d.Usable, d.Reason = false, err.Error()
		}
		c.Definitions[d.Name] = d
	}
	return c
}

// ValidateSection rejects invalid definitions when a user submits new config.
func ValidateSection(node *yaml.Node) error {
	c := Parse(node)
	if c.Reason != "" {
		return fmt.Errorf("%s", c.Reason)
	}
	names := make([]string, 0, len(c.Definitions))
	for name := range c.Definitions {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if d := c.Definitions[name]; !d.Usable {
			return fmt.Errorf("profile %q: %s", name, d.Reason)
		}
	}
	return nil
}

func parseDefinition(d *Definition, node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("profile %q must be a mapping", d.Name)
	}
	seen := make(map[string]bool)
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i].Value, node.Content[i+1]
		if seen[key] {
			return fmt.Errorf("duplicate profile key %q", key)
		}
		seen[key] = true
		switch key {
		case "description":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return fmt.Errorf("description must be a string")
			}
			d.Description = value.Value
		case "skills":
			d.Skills = &SkillSelection{}
			if err := parseSkills(d.Skills, value); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown profile key %q", key)
		}
	}
	return ValidateDefinition(*d)
}

func parseSkills(skills *SkillSelection, node *yaml.Node) error {
	if node.Tag == "!!null" {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("skills must be a mapping")
	}
	seen := make(map[string]bool)
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i].Value, node.Content[i+1]
		if seen[key] {
			return fmt.Errorf("duplicate skills key %q", key)
		}
		seen[key] = true
		if key != "include" && key != "exclude" {
			return fmt.Errorf("unknown skills key %q", key)
		}
		if value.Kind != yaml.SequenceNode {
			return fmt.Errorf("skills.%s must be a list of strings", key)
		}
		values := make([]string, 0, len(value.Content))
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
				return fmt.Errorf("skills.%s must be a list of strings", key)
			}
			values = append(values, item.Value)
		}
		if key == "include" {
			skills.Include, skills.IncludePresent = values, true
		} else {
			skills.Exclude, skills.ExcludePresent = values, true
		}
	}
	return nil
}
