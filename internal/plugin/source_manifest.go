package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func inspect(root string) (Candidate, error) {
	c := Candidate{Targets: []string{}, Components: []string{}}
	for _, entry := range []struct{ path, target string }{{"plugin.json", "codex"}, {".codex-plugin/plugin.json", "codex"}, {".claude-plugin/plugin.json", "claude"}, {".cursor-plugin/plugin.json", "cursor"}, {"package.json", "pi"}, {"package.json", "opencode"}} {
		data, err := os.ReadFile(filepath.Join(root, entry.path))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return c, err
		}
		var m map[string]json.RawMessage
		if err = json.Unmarshal(data, &m); err != nil {
			return c, fmt.Errorf("invalid %s", entry.path)
		}
		if entry.path == "package.json" {
			var deps, peers, dev map[string]json.RawMessage
			_ = json.Unmarshal(m["dependencies"], &deps)
			_ = json.Unmarshal(m["peerDependencies"], &peers)
			_ = json.Unmarshal(m["devDependencies"], &dev)
			if entry.target == "pi" && m["pi"] == nil {
				continue
			}
			if entry.target == "opencode" && deps["@opencode-ai/plugin"] == nil && peers["@opencode-ai/plugin"] == nil && dev["@opencode-ai/plugin"] == nil {
				continue
			}
		}
		var name, version, desc, schema string
		_ = json.Unmarshal(m["name"], &name)
		_ = json.Unmarshal(m["version"], &version)
		_ = json.Unmarshal(m["description"], &desc)
		_ = json.Unmarshal(m["$schema"], &schema)
		if entry.path == "plugin.json" {
			switch schema {
			case "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json":
			case "", "https://antigravity.google/schemas/v1/plugin.json":
				entry.target = "antigravity"
			default:
				return c, fmt.Errorf("unsupported root plugin.json schema")
			}
		}
		if entry.path == "package.json" {
			name = strings.ReplaceAll(strings.TrimPrefix(name, "@"), "/", "-")
		}
		if !namePattern.MatchString(name) {
			return c, fmt.Errorf("%s requires a valid plugin name", entry.path)
		}
		if c.Name != "" && c.Name != name {
			return c, fmt.Errorf("native manifest names disagree")
		}
		c.Name = name
		if c.Version == "" {
			c.Version = version
		}
		if c.Description == "" {
			c.Description = desc
		}
		if !slices.Contains(c.Targets, entry.target) {
			c.Targets = append(c.Targets, entry.target)
		}
		if entry.path == "plugin.json" && entry.target == "codex" && !slices.Contains(c.Targets, "cursor") {
			c.Targets = append(c.Targets, "cursor")
		}
		if entry.target == "pi" {
			var resources map[string]json.RawMessage
			if json.Unmarshal(m["pi"], &resources) != nil || resources == nil {
				return c, fmt.Errorf("package.json pi must be an object")
			}
			for _, key := range []string{"extensions", "skills", "prompts", "themes"} {
				if resources[key] != nil && !slices.Contains(c.Components, key) {
					c.Components = append(c.Components, key)
				}
			}
		}
		if entry.target == "opencode" {
			entry, err := openCodeEntry(root)
			if err != nil {
				return c, err
			}
			c.Entry = entry
			if !slices.Contains(c.Components, "extensions") {
				c.Components = append(c.Components, "extensions")
			}
		}
		for _, key := range []string{"rules", "themes", "contextFileName", "skills", "commands", "agents", "hooks", "mcpServers", "lspServers", "apps"} {
			if _, ok := m[key]; ok && !slices.Contains(c.Components, key) {
				c.Components = append(c.Components, key)
			}
		}
	}
	if c.Name == "" {
		return c, fmt.Errorf("no supported plugin manifest found; choose a plugin or marketplace directory")
	}
	for _, entry := range []struct{ path, kind string }{{"rules", "rules"}, {"hooks.json", "hooks"}, {"mcp_config.json", "mcpServers"}, {"sidecars", "sidecars"}, {"skills", "skills"}, {"agents", "agents"}, {"commands", "commands"}, {"hooks/hooks.json", "hooks"}, {".mcp.json", "mcpServers"}, {"mcp.json", "mcpServers"}, {".app.json", "apps"}} {
		if _, err := os.Stat(filepath.Join(root, entry.path)); err == nil && !slices.Contains(c.Components, entry.kind) {
			c.Components = append(c.Components, entry.kind)
		}
	}
	slices.Sort(c.Components)
	slices.Sort(c.Targets)
	return c, nil
}
