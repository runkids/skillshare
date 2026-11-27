package mcp

import (
	"maps"
	"slices"
)

// Override is what one mcp.projects root changes in a shared server it uses. The
// connection (command, url, headers, env) stays the shared server's, so it is defined once.
type Override struct {
	Targets   TargetList                `yaml:"targets,omitempty" json:"targets,omitzero"`
	Tools     ToolPolicy                `yaml:"tools,omitempty" json:"tools,omitzero"`
	PiOptions PiOptions                 `yaml:"piOptions,omitempty" json:"piOptions,omitempty"`
	Options   map[string]map[string]any `yaml:"options,omitempty" json:"options,omitempty"`
}

// IsZero reports an override that changes nothing.
func (o Override) IsZero() bool {
	return o.Targets == nil && o.Tools.IsZero() && len(o.PiOptions) == 0 && len(o.Options) == 0
}

// setOverride sets one server's override, or removes it when the override is empty. It
// returns nil rather than an empty map, so the root saves no empty overrides.
func setOverride(overrides map[string]Override, name string, o Override) map[string]Override {
	overrides = maps.Clone(overrides)
	if o.IsZero() {
		delete(overrides, name)
	} else {
		if overrides == nil {
			overrides = map[string]Override{}
		}
		overrides[name] = o
	}
	if len(overrides) == 0 {
		return nil
	}
	return overrides
}

// apply is the shared server as the root loads it. Targets and tools replace the server's;
// piOptions and options are merged key by key, so a root changes one field and inherits the
// rest. The shared server itself is left as it was.
func (o Override) apply(s Server) Server {
	if o.Targets != nil {
		s.Targets = slices.Clone(o.Targets)
	}
	if !o.Tools.IsZero() {
		s.Tools = o.Tools
	}
	if len(o.PiOptions) > 0 {
		merged := PiOptions{}
		maps.Copy(merged, s.PiOptions)
		maps.Copy(merged, o.PiOptions)
		s.PiOptions = merged
	}
	if len(o.Options) > 0 {
		merged := map[string]map[string]any{}
		for agent, fields := range s.Options {
			merged[agent] = maps.Clone(fields)
		}
		for agent, fields := range o.Options {
			if merged[agent] == nil {
				merged[agent] = map[string]any{}
			}
			maps.Copy(merged[agent], fields)
		}
		s.Options = merged
	}
	return s
}

// inRoot is a shared server as one root that uses it loads it: the root's override
// applied, and the targets it goes to there.
func (s *Source) inRoot(root, name string, server Server) Server {
	project := s.Projects[root]
	server = project.Overrides[name].apply(server)
	server.Targets = slices.Clone(server.Targets.orDefault(project.defaults(s)))
	return server
}
