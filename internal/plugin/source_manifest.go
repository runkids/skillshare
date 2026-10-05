package plugin

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type manifestSpec struct{ path, target string }

func inspect(root string, explicit ...string) (Candidate, error) {
	c := Candidate{Targets: []string{}, Components: []string{}, TargetInfo: map[string]TargetPackage{}}
	specs := []manifestSpec{{".codex-plugin/plugin.json", "codex"}, {".claude-plugin/plugin.json", "claude"}, {".omp-plugin/plugin.json", "omp"}, {".cursor-plugin/plugin.json", "cursor"}, {"package.json", "pi"}, {"package.json", "opencode"}, {".plugin/plugin.json", "copilot"}, {".github/plugin/plugin.json", "copilot"}, {".grok-plugin/plugin.json", "grok"}, {"kimi.plugin.json", "kimi"}, {".kimi-plugin/plugin.json", "kimi"}, {".devin-plugin/plugin.json", "devin"}, {".hermes-plugin/plugin.yaml", "hermes"}, {"plugin.json", "codex"}}
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
			info.block("plugins.problem.invalidManifest", "Invalid "+spec.path, map[string]string{"manifest": spec.path})
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
				info.block("plugins.problem.rootSchema", "Unsupported root plugin.json schema", nil)
				c.TargetInfo[spec.target] = info
				continue
			}
		}
		if spec.target == "codex" {
			info.Logo = codexLogo(root, m["interface"])
		}
		if spec.path == "package.json" {
			name = strings.ReplaceAll(strings.TrimPrefix(name, "@"), "/", "-")
			if spec.target == "pi" {
				var resources map[string]json.RawMessage
				if m["pi"] != nil {
					if json.Unmarshal(m["pi"], &resources) != nil || resources == nil {
						info.block("plugins.problem.piObject", "package.json pi must be an object", nil)
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
					var keyed agentError
					errors.As(err, &keyed)
					info.block(keyed.key, err.Error(), keyed.args)
				}
				info.Components = append(info.Components, "extensions")
			}
		}
		// Pi and OpenCode install by path, so a package.json name may be scoped (@owner/pkg),
		// differ, or be missing; it only names the plugin when nothing else does.
		npm := spec.path == "package.json"
		if !namePattern.MatchString(name) && !npm {
			info.block("plugins.problem.manifestName", spec.path+" requires a valid plugin name", map[string]string{"manifest": spec.path})
		}
		if c.Name != "" && c.Name != name && !npm {
			info.block("plugins.problem.nameDiffers", "Native manifest name differs from "+c.Name, map[string]string{"name": c.Name})
		}
		if info.Problem == "" {
			if c.Name == "" && namePattern.MatchString(name) {
				c.Name, c.Description, c.Version = name, desc, info.Version
			}
			info.Components = manifestComponents(root, spec.target, m, info.Components)
		}
		// The first native manifest wins; explicit invalid manifests remain errors. Codex is the
		// exception: it installs from a portable root plugin.json ahead of .codex-plugin/plugin.json,
		// and reports 1.0.0 when that manifest has no version.
		native, exists := c.TargetInfo[spec.target]
		if !exists || spec.path == "plugin.json" && spec.target == "codex" && info.Problem == "" {
			c.TargetInfo[spec.target] = info
		}
		if spec.path == "plugin.json" && spec.target == "codex" {
			for _, target := range []string{"cursor", "copilot", "omp"} {
				if _, exists := c.TargetInfo[target]; !exists {
					c.TargetInfo[target] = info
				}
			}
			if codex := c.TargetInfo["codex"]; codex.Manifest == "plugin.json" {
				// .codex-plugin/plugin.json still overlays the portable manifest, so keep its logo.
				if codex.Logo == "" {
					codex.Logo = native.Logo
				}
				if codex.Version == "" {
					codex.Version = "1.0.0"
				}
				c.TargetInfo["codex"] = codex
			}
		}
	}
	if info, ok := c.TargetInfo["claude"]; ok && info.Problem == "" {
		for _, target := range []string{"copilot", "grok", "antigravity-cli", "omp"} {
			if _, exists := c.TargetInfo[target]; !exists {
				c.TargetInfo[target] = info
			}
		}
	}
	// omp loads a Claude plugin's content and, from package.json, its own extension modules. Its
	// install records the version it finds itself, never the one in .omp-plugin/plugin.json.
	if info, ok := c.TargetInfo["omp"]; ok && info.Problem == "" {
		info.Components = slices.Clone(info.Components)
		info.Version = ompInstallVersion(root)
		if !validOMPName(c.Name) {
			info.block("plugins.problem.ompName", "OMP plugin names use only ASCII letters, digits, hyphens and dots, up to 64 characters", nil)
		} else if ompExtensions(root) && !slices.Contains(info.Components, "extensions") {
			info.Components = append(info.Components, "extensions")
			slices.Sort(info.Components)
		}
		c.TargetInfo["omp"] = info
	}
	if info, ok := c.TargetInfo["antigravity"]; ok {
		c.TargetInfo["antigravity-cli"] = info
	}
	c.collectTargets()
	if c.Name == "" {
		return c, fmt.Errorf("no valid supported plugin manifest found; choose a plugin or marketplace directory")
	}
	return c, nil
}

// claudeCatalogCandidate describes a Claude plugin that has no plugin.json: its catalog entry
// names it, and its components come from that entry and the default folders.
// ponytail: Claude only; other clients that read Claude plugins may still require the manifest.
func claudeCatalogCandidate(dir, name string, fields map[string]json.RawMessage) Candidate {
	var desc, version string
	_ = json.Unmarshal(fields["description"], &desc)
	_ = json.Unmarshal(fields["version"], &version)
	info := TargetPackage{Manifest: ".claude-plugin/marketplace.json", Version: version, Components: manifestComponents(dir, "claude", fields, []string{})}
	c := Candidate{Name: name, Description: desc, Version: version, Targets: []string{}, Components: info.Components, TargetInfo: map[string]TargetPackage{}}
	if len(info.Components) == 0 {
		c.block("plugins.problem.noManifest", name+": no plugin.json and no components found", nil)
		return c
	}
	c.TargetInfo["claude"] = info
	c.Targets = []string{"claude"}
	return c
}

// claudeEntryFields are the parts of a Claude catalog entry that define the plugin. With a
// plugin.json they are merged into it, so several entries can share one folder and differ here.
func claudeEntryFields(fields map[string]json.RawMessage) map[string]json.RawMessage {
	entry := map[string]json.RawMessage{}
	for _, key := range []string{"version", "description", "strict", "skills", "commands", "agents", "hooks", "mcpServers", "lspServers"} {
		if raw, ok := fields[key]; ok {
			entry[key] = raw
		}
	}
	return entry
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
	// Claude loads a root SKILL.md as the plugin's single skill when nothing else names skills.
	if _, err := os.Stat(filepath.Join(root, "SKILL.md")); err == nil && target == "claude" && !slices.Contains(components, "skills") {
		components = append(components, "skills")
	}
	slices.Sort(components)
	return components
}

const maxLogo = 1 << 20

var logoTypes = map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".svg": "image/svg+xml"}

// codexLogo returns interface.logo of a Codex manifest as a data: URI, "" when it is absent, not
// an image, too large, or outside the plugin. The path is opened through os.Root, so it cannot leave root.
func codexLogo(root string, raw json.RawMessage) string {
	var ui struct {
		Logo string `json:"logo"`
	}
	if json.Unmarshal(raw, &ui) != nil || ui.Logo == "" {
		return ""
	}
	mime, ok := logoTypes[strings.ToLower(filepath.Ext(ui.Logo))]
	if !ok {
		return ""
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return ""
	}
	defer r.Close()
	f, err := r.Open(filepath.FromSlash(strings.TrimPrefix(ui.Logo, "./")))
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxLogo+1))
	if err != nil || len(data) > maxLogo {
		return ""
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}
