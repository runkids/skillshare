package mcp

import (
	"slices"
	"strings"
)

// Normalize native dialects before applying the common import validation.
func normalizeClientImport(target string, entry map[string]any, c *Candidate) {
	if target == "pi" {
		if kind, exists := entry["transport"]; exists {
			entry["type"] = kind
			delete(entry, "transport")
		}
	}
	format, ok := clientFormats[target]
	if !ok {
		return
	}
	if target == "gemini" && entry["url"] != nil {
		c.Problems = append(c.Problems, "Gemini url uses SSE; only httpUrl (Streamable HTTP) is supported")
	}
	if format.urlKey != "url" {
		if value, exists := entry[format.urlKey]; exists {
			if entry["url"] != nil {
				c.Problems = append(c.Problems, "Multiple native URL fields are not supported")
			}
			entry["url"] = value
			delete(entry, format.urlKey)
		}
	}
	if target == "goose" {
		if value, ok := entry["cmd"]; ok {
			entry["command"] = value
			delete(entry, "cmd")
		}
		if value, ok := entry["envs"]; ok {
			entry["env"] = value
			delete(entry, "envs")
		}
		if keys, ok := entry["env_keys"]; ok {
			if list, ok := keys.([]any); !ok || len(list) > 0 {
				c.Problems = append(c.Problems, "Goose keychain env_keys cannot be imported as portable environment references")
			}
			delete(entry, "env_keys")
		}
		delete(entry, "name")
	}
	if kind, ok := entry["type"].(string); ok {
		if format.localType != "" && kind == format.localType {
			entry["type"] = "stdio"
		}
		if format.remoteType != "" && kind == format.remoteType {
			entry["type"] = "http"
		}
	}
	if format.stdioOnly && entry["url"] != nil {
		c.Problems = append(c.Problems, "Claude Desktop config files support only stdio servers")
	}
}

// importPiTools moves Pi's toolExposure into tools when writing that policy back gives Pi
// the same settings. Otherwise it stays in piOptions, for Pi only. exposure always stays
// in piOptions.
func importPiTools(c *Candidate) {
	if c.Server.PiOptions["toolExposure"] == nil {
		return
	}
	policy, ok := piImportPolicy(c.Server.PiOptions)
	if !ok {
		c.Warnings = append(c.Warnings, "Pi's toolExposure stays in piOptions and applies to Pi only; tools cannot express it exactly")
		return
	}
	delete(c.Server.PiOptions, "toolExposure")
	if len(c.Server.PiOptions) == 0 {
		c.Server.PiOptions = nil
	}
	c.Server.Tools = policy
}

// importCopilotTools reads Copilot's list of enabled tools into tools.allow. "*", Copilot's
// default, enables every tool, which is no policy.
func importCopilotTools(entry map[string]any, c *Candidate) {
	raw, set := entry["tools"]
	if !set {
		return
	}
	items, _ := raw.([]any)
	if slices.Contains(items, any("*")) {
		return
	}
	names := toolNames(items)
	policy := ToolPolicy{Allow: names}
	if len(names) == 0 || len(names) != len(items) || slices.ContainsFunc(names, isToolPattern) || policy.validate("") != nil {
		c.Warnings = append(c.Warnings, "Copilot tools not imported: only a list of exact tool names or \"*\" maps to tools.allow")
		return
	}
	c.Server.Tools = policy
}

// importCodexTools reads Codex's enabled_tools and disabled_tools into tools.allow and
// tools.deny. Both list exact tool names; an empty enabled_tools, no tool at all, has no
// equivalent.
func importCodexTools(entry map[string]any, c *Candidate) {
	var policy ToolPolicy
	for key, dest := range map[string]*[]string{"enabled_tools": &policy.Allow, "disabled_tools": &policy.Deny} {
		raw, set := entry[key]
		if !set {
			continue
		}
		items, _ := raw.([]any)
		names := toolNames(items)
		if len(names) == 0 || len(names) != len(items) || slices.ContainsFunc(names, isToolPattern) {
			c.Warnings = append(c.Warnings, "Codex "+key+" not imported: only a non-empty list of exact tool names maps to tools")
			continue
		}
		*dest = names
	}
	if err := policy.validate(""); err != nil {
		c.Warnings = append(c.Warnings, "Codex enabled_tools and disabled_tools not imported: "+strings.TrimPrefix(err.Error(), "MCP : "))
		return
	}
	c.Server.Tools = policy
}
