package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type manifestSpec struct{ path, target string }

func inspect(root string, explicit ...string) (Candidate, error) {
	c := Candidate{Targets: []string{}, Components: []string{}, TargetInfo: map[string]TargetPackage{}}
	specs := []manifestSpec{{".codex-plugin/plugin.json", "codex"}, {".claude-plugin/plugin.json", "claude"}, {".cursor-plugin/plugin.json", "cursor"}, {"package.json", "pi"}, {"package.json", "opencode"}, {".plugin/plugin.json", "copilot"}, {".github/plugin/plugin.json", "copilot"}, {".grok-plugin/plugin.json", "grok"}, {"kimi.plugin.json", "kimi"}, {".kimi-plugin/plugin.json", "kimi"}, {".devin-plugin/plugin.json", "devin"}, {".hermes-plugin/plugin.yaml", "hermes"}, {"plugin.json", "codex"}}
	for _, spec := range specs {
		data, err := os.ReadFile(filepath.Join(root, spec.path))
		if os.IsNotExist(err) {
			continue
		}
		info := TargetPackage{Manifest: spec.path, Components: []string{}}
		var m map[string]json.RawMessage
		if err == nil && strings.HasSuffix(spec.path, ".yaml") {
			var raw map[string]any
			if err = yaml.Unmarshal(data, &raw); err == nil {
				data, err = json.Marshal(raw)
			}
		}
		if err == nil {
			err = json.Unmarshal(data, &m)
		}
		if err != nil {
			info.Problem = "Invalid " + spec.path
			if _, exists := c.TargetInfo[spec.target]; !exists {
				c.TargetInfo[spec.target] = info
			}
			continue
		}
		var name, desc, schema string
		_ = json.Unmarshal(m["name"], &name)
		_ = json.Unmarshal(m["version"], &info.Version)
		_ = json.Unmarshal(m["description"], &desc)
		_ = json.Unmarshal(m["$schema"], &schema)
		if spec.path == "plugin.json" {
			switch schema {
			case "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json":
			case "", "https://antigravity.google/schemas/v1/plugin.json":
				spec.target = "antigravity"
			default:
				info.Problem = "Unsupported root plugin.json schema"
				c.TargetInfo[spec.target] = info
				continue
			}
		}
		if spec.path == "package.json" {
			name = strings.ReplaceAll(strings.TrimPrefix(name, "@"), "/", "-")
			if spec.target == "pi" {
				var resources map[string]json.RawMessage
				if m["pi"] != nil {
					if json.Unmarshal(m["pi"], &resources) != nil || resources == nil {
						info.Problem = "package.json pi must be an object"
					}
				} else {
					var keywords []string
					_ = json.Unmarshal(m["keywords"], &keywords)
					if !slices.Contains(keywords, "pi-package") {
						continue
					}
				}
				for _, key := range []string{"extensions", "skills", "prompts", "themes"} {
					if resources[key] != nil || (resources == nil && isDirectory(filepath.Join(root, key))) {
						info.Components = append(info.Components, key)
					}
				}
			} else {
				var main string
				_ = json.Unmarshal(m["main"], &main)
				sdk := false
				for _, key := range []string{"dependencies", "peerDependencies", "devDependencies"} {
					var deps map[string]json.RawMessage
					_ = json.Unmarshal(m[key], &deps)
					sdk = sdk || deps["@opencode-ai/plugin"] != nil
				}
				if !sdk && (len(explicit) == 0 || explicit[0] == "") && !strings.HasPrefix(filepath.ToSlash(filepath.Clean(main)), ".opencode/plugins/") {
					continue
				}
				info.Entry, err = openCodeEntry(root, explicit...)
				if err != nil {
					info.Problem = err.Error()
				}
				info.Components = append(info.Components, "extensions")
			}
		}
		if !namePattern.MatchString(name) {
			info.Problem = spec.path + " requires a valid plugin name"
		}
		if c.Name != "" && c.Name != name {
			info.Problem = "Native manifest name differs from " + c.Name
		}
		if info.Problem == "" {
			if c.Name == "" {
				c.Name, c.Description, c.Version = name, desc, info.Version
			}
			info.Components = manifestComponents(root, spec.target, m, info.Components)
		}
		// The first native manifest wins; explicit invalid manifests remain errors.
		if _, exists := c.TargetInfo[spec.target]; !exists {
			c.TargetInfo[spec.target] = info
		}
		if spec.path == "plugin.json" && spec.target == "codex" {
			for _, target := range []string{"cursor", "copilot"} {
				if _, exists := c.TargetInfo[target]; !exists {
					c.TargetInfo[target] = info
				}
			}
		}
	}
	if info, ok := c.TargetInfo["claude"]; ok && info.Problem == "" {
		for _, target := range []string{"copilot", "grok", "antigravity-cli"} {
			if _, exists := c.TargetInfo[target]; !exists {
				c.TargetInfo[target] = info
			}
		}
	}
	if info, ok := c.TargetInfo["antigravity"]; ok {
		c.TargetInfo["antigravity-cli"] = info
	}
	for target, info := range c.TargetInfo {
		if info.Problem != "" {
			continue
		}
		c.Targets = append(c.Targets, target)
		for _, component := range info.Components {
			if !slices.Contains(c.Components, component) {
				c.Components = append(c.Components, component)
			}
		}
		if target == "opencode" {
			c.Entry = info.Entry
		}
	}
	if c.Name == "" {
		return c, fmt.Errorf("no valid supported plugin manifest found; choose a plugin or marketplace directory")
	}
	slices.Sort(c.Targets)
	slices.Sort(c.Components)
	return c, nil
}

func isDirectory(path string) bool { info, err := os.Stat(path); return err == nil && info.IsDir() }

func manifestComponents(root, target string, m map[string]json.RawMessage, components []string) []string {
	if target == "pi" || target == "opencode" {
		return components
	}
	for _, key := range []string{"rules", "themes", "contextFileName", "skills", "commands", "agents", "hooks", "mcpServers", "lspServers", "apps", "sessionStart", "systemPromptPath", "provides_hooks"} {
		if raw, ok := m[key]; ok && string(raw) != "{}" && string(raw) != "[]" && string(raw) != "null" {
			kind := key
			switch key {
			case "sessionStart", "provides_hooks":
				kind = "hooks"
			case "contextFileName", "systemPromptPath":
				kind = "instructions"
			}
			if !slices.Contains(components, kind) {
				components = append(components, kind)
			}
		}
	}
	entries := []struct{ path, kind string }{{"skills", "skills"}, {"agents", "agents"}, {"commands", "commands"}, {"hooks/hooks.json", "hooks"}, {".mcp.json", "mcpServers"}}
	if target == "antigravity" {
		entries = []struct{ path, kind string }{{"skills", "skills"}, {"agents", "agents"}, {"rules", "rules"}, {"hooks.json", "hooks"}, {"mcp_config.json", "mcpServers"}}
	}
	if target == "kimi" {
		entries = []struct{ path, kind string }{{"skills", "skills"}, {"agents", "agents"}, {"commands", "commands"}}
	}
	if target == "hermes" {
		entries = nil
	}
	var schema string
	_ = json.Unmarshal(m["$schema"], &schema)
	if schema == "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json" {
		entries = []struct{ path, kind string }{{"skills", "skills"}, {"mcp.json", "mcpServers"}}
	} else if target == "codex" {
		entries = []struct{ path, kind string }{{"skills", "skills"}, {".mcp.json", "mcpServers"}, {".app.json", "apps"}}
	}
	for _, entry := range entries {
		if _, err := os.Stat(filepath.Join(root, entry.path)); err == nil && !slices.Contains(components, entry.kind) {
			components = append(components, entry.kind)
		}
	}
	slices.Sort(components)
	return components
}
