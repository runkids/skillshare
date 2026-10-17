package mcp

import (
	"fmt"
	"slices"
)

var portableFields = []string{"command", "args", "env", "url", "headers", "type", "transport"}

func (s Server) validateOptions(name string) error {
	if len(s.Options) == 0 {
		return nil
	}
	if s.Disabled {
		return fmt.Errorf("MCP %s: options cannot be set on a disabled entry; it only switches the server off", name)
	}
	for _, agent := range sortedKeys(s.Options) {
		switch {
		case agent == "pi":
			return fmt.Errorf("MCP %s: options.pi is not used; set Pi's fields in piOptions", name)
		case !validTarget(agent):
			return fmt.Errorf("MCP %s: options.%s is not an Agent; use an Agent name such as codex or claude", name, agent)
		}
		for _, key := range sortedKeys(s.Options[agent]) {
			// Some Agents rename the portable fields, such as OpenCode's environment, so both names are refused.
			if slices.Contains(additionalManagedFields(agent), key) || slices.Contains(portableFields, key) {
				return fmt.Errorf("MCP %s: options.%s cannot set %s; Skillshare writes that field from the server's own settings", name, agent, key)
			}
			if !optionValue(s.Options[agent][key], false) {
				return fmt.Errorf("MCP %s: options.%s.%s must be a string, number, boolean, list of those, or a mapping of them", name, agent, key)
			}
			if field := literalSecret(key, s.Options[agent][key]); field != "" {
				return fmt.Errorf("MCP %s: options.%s.%s holds a credential as plain text; reference an environment variable in the Agent's own syntax, such as ${VAR} or {env:VAR}", name, agent, field)
			}
		}
	}
	return nil
}

// optionValue accepts what every native format writes back unchanged: TOML has no null,
// and Skillshare's TOML writer refuses an array of tables.
func optionValue(value any, inList bool) bool {
	switch v := value.(type) {
	case string, bool, int, int64, uint64, float64:
		return true
	case []any:
		for _, item := range v {
			if inList || !optionValue(item, true) {
				return false
			}
		}
		return !inList
	case map[string]any:
		for _, item := range v {
			if !optionValue(item, false) {
				return false
			}
		}
		return !inList
	}
	return false
}

// literalSecret names the first field holding a credential as plain text, as import judges
// one. The Agent's file may be committed, so only an environment reference may stand there.
func literalSecret(key string, value any) string {
	switch v := value.(type) {
	case string:
		if v != "" && (sensitiveKey.MatchString(key) || hasURLPassword(v)) && !hasInterpolation(v) {
			return key
		}
	case []any:
		for _, item := range v {
			if field := literalSecret(key, item); field != "" {
				return field
			}
		}
	case map[string]any:
		for _, name := range sortedKeys(v) {
			if field := literalSecret(name, v[name]); field != "" {
				return key + "." + field
			}
		}
	}
	return ""
}

// withOptions adds the Agent's options to a rendered entry. A field Render already wrote,
// such as Copilot's tools, belongs to the server's own settings.
func withOptions(target string, s Server, out map[string]any) (map[string]any, error) {
	for _, key := range sortedKeys(s.Options[target]) {
		if _, set := out[key]; set {
			return nil, fmt.Errorf("options.%s cannot set %s; Skillshare writes that field from the server's own settings", target, key)
		}
		out[key] = s.Options[target][key]
	}
	return out, nil
}
