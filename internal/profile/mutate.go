package profile

import (
	"fmt"
	"slices"
	"strings"

	"skillshare/internal/pattern"

	"gopkg.in/yaml.v3"
)

type Mutation struct {
	Op, Name, NewName string
	Description       *string
	Replacement       *SkillSelection

	AddInclude, RemoveInclude, AddExclude, RemoveExclude []string
	Skills                                               []string
}

type Change struct {
	Changed, NeedsSync bool
	Active             string
	Skills             []SkillResult
}

type SkillResult struct {
	Name   string `json:"name"`
	Result string `json:"result"`
}

// BlockedError names an exclude pattern that assigning a skill cannot remove.
type BlockedError struct {
	Profile, Skill, Pattern string
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("profile %q: skill %q is blocked by exclude pattern %q", e.Profile, e.Skill, e.Pattern)
}

// Mutate returns a copy of the raw section, leaving unrelated nodes intact.
func Mutate(node *yaml.Node, active string, m Mutation) (*yaml.Node, Change, error) {
	change := Change{Active: active}
	if err := ValidateName(m.Name); err != nil {
		return nil, change, err
	}
	if node == nil || node.Kind == 0 {
		node = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}
	copy := cloneNode(node, make(map[*yaml.Node]*yaml.Node))
	if copy.Kind != yaml.MappingNode {
		return nil, change, fmt.Errorf("profiles must be a mapping")
	}
	index := nodeIndex(copy, m.Name)
	if m.Op == "create" {
		if index >= 0 {
			return nil, change, fmt.Errorf("profile %q already exists", m.Name)
		}
		copy.Content = append(copy.Content, scalar(m.Name), &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"})
		index = len(copy.Content) - 2
		setValue(copy.Content[index+1], "skills", &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"})
	} else if index < 0 {
		return nil, change, fmt.Errorf("profile %q does not exist", m.Name)
	}
	d := Parse(copy).Definitions[m.Name]
	if !d.Usable {
		return nil, change, fmt.Errorf("profile %q is unusable: %s", m.Name, d.Reason)
	}
	switch m.Op {
	case "create", "edit":
		entry := copy.Content[index+1]
		if m.Description != nil {
			setValue(entry, "description", scalar(*m.Description))
		}
		if len(m.AddInclude)+len(m.RemoveInclude)+len(m.AddExclude)+len(m.RemoveExclude) > 0 {
			si := nodeIndex(entry, "skills")
			if si < 0 {
				setValue(entry, "skills", &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"})
				si = nodeIndex(entry, "skills")
			}
			skills := entry.Content[si+1]
			if skills.Tag == "!!null" {
				*skills = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			}
			editList(skills, "include", m.AddInclude, m.RemoveInclude)
			editList(skills, "exclude", m.AddExclude, m.RemoveExclude)
		}
		updated := Parse(copy).Definitions[m.Name]
		if !updated.Usable {
			return nil, change, fmt.Errorf("profile %q: %s", m.Name, updated.Reason)
		}
		change.Changed = m.Op == "create" || !equalNodes(node, copy)
		change.NeedsSync = change.Changed && active == m.Name
	case "add-skills", "remove-skills":
		// Refuse the whole batch before editing any name, even if an exact
		// exclude also matches a name blocked by a glob.
		if m.Op == "add-skills" && d.Skills != nil {
			for _, name := range m.Skills {
				for _, raw := range d.Skills.Exclude {
					p := strings.TrimSpace(raw)
					if _, matches := pattern.FirstMatch(name, []string{p}); p != "" && p != name && matches {
						return nil, change, &BlockedError{Profile: m.Name, Skill: name, Pattern: p}
					}
				}
			}
		}
		entry := copy.Content[index+1]
		for _, name := range m.Skills {
			d = Parse(copy).Definitions[m.Name]
			adding := m.Op == "add-skills"
			result := "not-in"
			if adding {
				result = "already-in"
			}
			if d.Selects(name) != adding {
				si := nodeIndex(entry, "skills")
				if si < 0 || entry.Content[si+1].Tag == "!!null" {
					setValue(entry, "skills", &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"})
					si = nodeIndex(entry, "skills")
				}
				skills := entry.Content[si+1]
				if adding {
					removeExactSkill(skills, "exclude", name)
					if !Parse(copy).Definitions[m.Name].Selects(name) {
						editList(skills, "include", []string{name}, nil)
					}
					result = "added"
				} else {
					if d.Skills == nil {
						editList(skills, "include", []string{"*"}, nil)
					}
					removeExactSkill(skills, "include", name)
					if Parse(copy).Definitions[m.Name].Selects(name) {
						editList(skills, "exclude", []string{name}, nil)
					}
					result = "removed"
				}
			}
			change.Skills = append(change.Skills, SkillResult{Name: name, Result: result})
		}
		updated := Parse(copy).Definitions[m.Name]
		if !updated.Usable {
			return nil, Change{Active: active}, fmt.Errorf("profile %q: %s", m.Name, updated.Reason)
		}
		change.Changed = !equalNodes(node, copy)
		change.NeedsSync = change.Changed && active == m.Name
	case "rename":
		if err := ValidateName(m.NewName); err != nil {
			return nil, change, err
		}
		if m.NewName == m.Name {
			return copy, change, nil
		}
		if nodeIndex(copy, m.NewName) >= 0 {
			return nil, change, fmt.Errorf("profile %q already exists", m.NewName)
		}
		copy.Content[index].Value = m.NewName
		change.Changed = true
		if active == m.Name {
			change.Active = m.NewName
		}
	case "delete":
		if active == m.Name {
			return nil, change, fmt.Errorf("cannot delete active profile %q", m.Name)
		}
		copy.Content = append(copy.Content[:index], copy.Content[index+2:]...)
		change.Changed = true
	default:
		return nil, change, fmt.Errorf("unknown profile operation %q", m.Op)
	}
	return copy, change, nil
}

func removeExactSkill(parent *yaml.Node, key, name string) {
	if i := nodeIndex(parent, key); i >= 0 {
		list := parent.Content[i+1]
		list.Content = slices.DeleteFunc(list.Content, func(n *yaml.Node) bool { return strings.TrimSpace(n.Value) == name })
	}
}

func editList(parent *yaml.Node, key string, add, remove []string) {
	if len(add)+len(remove) == 0 {
		return
	}
	i := nodeIndex(parent, key)
	if i < 0 {
		setValue(parent, key, &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"})
		i = nodeIndex(parent, key)
	}
	list := parent.Content[i+1]
	list.Content = slices.DeleteFunc(list.Content, func(n *yaml.Node) bool { return slices.Contains(remove, n.Value) })
	for _, value := range add {
		found := false
		for _, n := range list.Content {
			found = found || n.Value == value
		}
		if !found {
			list.Content = append(list.Content, scalar(value))
		}
	}
}

func nodeIndex(n *yaml.Node, key string) int {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return i
		}
	}
	return -1
}

func scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func setValue(n *yaml.Node, key string, value *yaml.Node) {
	if i := nodeIndex(n, key); i >= 0 {
		value.HeadComment, value.LineComment, value.FootComment = n.Content[i+1].HeadComment, n.Content[i+1].LineComment, n.Content[i+1].FootComment
		n.Content[i+1] = value
	} else {
		n.Content = append(n.Content, scalar(key), value)
	}
}

func cloneNode(n *yaml.Node, seen map[*yaml.Node]*yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	if cloned := seen[n]; cloned != nil {
		return cloned
	}
	out := *n
	seen[n] = &out
	out.Content = make([]*yaml.Node, len(n.Content))
	for i, child := range n.Content {
		out.Content[i] = cloneNode(child, seen)
	}
	out.Alias = cloneNode(n.Alias, seen)
	return &out
}

func equalNodes(a, b *yaml.Node) bool {
	x, err1 := yaml.Marshal(a)
	y, err2 := yaml.Marshal(b)
	return err1 == nil && err2 == nil && string(x) == string(y)
}
