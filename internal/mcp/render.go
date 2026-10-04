package mcp

import (
	"fmt"
	"sort"
)

// renderDisabled writes only the switch. That works where the Agent merges its project
// file over its global one field by field, so the global command or url survives. Agents
// that replace the whole entry, or reject one without a transport, would break instead.
func renderDisabled(target string, s Server) (map[string]any, error) {
	switch {
	case openCodeFormat(target):
		return map[string]any{"enabled": false}, nil
	case target == "claude":
		// No entry to write: destination sends the name to Claude Code's per-project off list.
		return map[string]any{}, nil
	case target == "codex":
		// Codex merges the project file by field, so the switch works where the global config
		// defines the server. Where it does not, the merged entry has no command or url and
		// Codex fails its whole config load with "invalid transport". The project file is
		// usually committed, so one person's switch would break Codex for a teammate.
		return nil, fmt.Errorf("codex cannot turn off a global server from a project file: on a machine whose global config lacks the server, Codex stops loading its whole config; set enabled = false in ~/.codex/config.toml instead")
	case target == "pi":
		// Since Pi 1.0.1 a project entry without command, url or type overrides only enabled,
		// exposure and toolExposure of the global server with that name, which keeps its args,
		// env and credentials. Pi's /mcp writes the same entry. Older Pi reports it as invalid.
		return map[string]any{"enabled": false}, nil
	}
	return nil, fmt.Errorf("%s cannot turn off a global server from a project file; disabled supports claude, opencode, kilocode and pi", target)
}

// Render converts a portable definition to a native entry without reading env.
func Render(target string, s Server) (map[string]any, error) {
	if !validTarget(target) {
		return nil, fmt.Errorf("unsupported MCP target %q", target)
	}
	if err := s.Validate("server"); err != nil {
		return nil, err
	}
	if s.Disabled {
		return renderDisabled(target, s)
	}
	if target == "pi" {
		return renderPi(s)
	}
	if format, ok := clientFormats[target]; ok {
		return renderAdditionalClient(target, format, s)
	}
	out := map[string]any{}
	if target == "antigravity" {
		if s.BearerToken != nil {
			return nil, fmt.Errorf("Antigravity: bearerToken/fromEnv is not supported; complete OAuth authentication in Antigravity")
		}
		for _, values := range []map[string]Value{s.Env, s.Headers} {
			for _, value := range values {
				if value.FromEnv != "" {
					return nil, fmt.Errorf("Antigravity: fromEnv is not supported because native environment interpolation is not documented")
				}
			}
		}
	}
	resolve := func(v Value) string {
		if v.FromEnv == "" {
			return v.Literal
		}
		if openCodeFormat(target) {
			return "{env:" + v.FromEnv + "}"
		}
		if target == "claude" || target == "grok" {
			return "${" + v.FromEnv + "}"
		}
		return "${env:" + v.FromEnv + "}"
	}
	if s.Command != "" {
		out["command"] = s.Command
		if target != "codex" && target != "grok" && target != "antigravity" {
			out["type"] = "stdio"
		}
		if len(s.Args) > 0 {
			out["args"] = s.Args
		}
		env := map[string]string{}
		var forwarded []string
		for key, v := range s.Env {
			if target == "codex" && v.FromEnv != "" {
				if key != v.FromEnv {
					return nil, fmt.Errorf("Codex requires environment reference %s to use the same variable name", key)
				}
				forwarded = append(forwarded, key)
			} else {
				env[key] = resolve(v)
			}
		}
		if len(env) > 0 {
			out["env"] = env
		}
		if len(forwarded) > 0 {
			sort.Strings(forwarded)
			out["env_vars"] = forwarded
		}
		if openCodeFormat(target) {
			out["type"] = "local"
			out["command"] = append([]string{s.Command}, s.Args...)
			delete(out, "args")
			if len(env) > 0 {
				out["environment"] = env
				delete(out, "env")
			}
		}
	} else {
		out["url"] = s.URL
		if target == "antigravity" {
			out["serverUrl"] = s.URL
			delete(out, "url")
		}
		if openCodeFormat(target) {
			out["type"] = "remote"
		}
		if target == "claude" || target == "vscode" {
			out["type"] = "http"
		}
		headers := map[string]string{}
		envHeaders := map[string]string{}
		for key, v := range s.Headers {
			if target == "codex" && v.FromEnv != "" {
				envHeaders[key] = v.FromEnv
			} else {
				headers[key] = resolve(v)
			}
		}
		if s.BearerToken != nil {
			if target == "codex" {
				out["bearer_token_env_var"] = s.BearerToken.FromEnv
			} else {
				headers["Authorization"] = "Bearer " + resolve(*s.BearerToken)
			}
		}
		if len(headers) > 0 {
			if target == "codex" {
				out["http_headers"] = headers
			} else {
				out["headers"] = headers
			}
		}
		if len(envHeaders) > 0 {
			out["env_http_headers"] = envHeaders
		}
	}
	if target == "codex" {
		// Codex applies disabled_tools after enabled_tools; both list exact tool names.
		allow, deny, _ := namedTools(s.Tools, true)
		if allow != nil {
			out["enabled_tools"] = allow
		}
		if deny != nil {
			out["disabled_tools"] = deny
		}
	}
	return out, nil
}

// Rendered is one server as one Agent's config file would hold it.
type Rendered struct {
	Target  string `json:"target"`
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
	// ToolGaps are the parts of the server's tool policy this Agent cannot hold, as the
	// plan's notices name them, so an editor can say so before the server is saved.
	ToolGaps []string `json:"toolGaps,omitempty"`
}

// RenderNative writes the server into an empty file per target, so the wrapper key and the
// format (JSON, TOML, YAML) come from the code that writes the real file, not from a guess
// made in the dashboard. Existing Pi-only settings are included with native secrets redacted; nothing is executed or written.
func (s *Service) RenderNative(name string, server Server) []Rendered {
	// An unreadable config has no accounts.
	if source, err := LoadSource(s.ConfigPath); err == nil {
		s = s.withAccounts(source.Accounts)
	}
	out := make([]Rendered, 0, len(server.Targets))
	for _, target := range server.Targets {
		r := Rendered{Target: target}
		client, agent := s.forTarget(target)
		r.ToolGaps = toolPolicyGaps(agent, server.Tools)
		path, native, err := client.destination(agent, server)
		r.Path = path
		if err == nil {
			err = client.checkScope(name, agent, server)
		}
		var entry map[string]any
		if err == nil {
			entry, err = Render(agent, server)
		}
		if err == nil && agent == "pi" {
			data, _, _, readErr := safeRead(path)
			var current *Native
			if readErr == nil {
				current, readErr = ParseNative("pi", data)
			}
			if readErr != nil {
				err = readErr
			} else {
				before, want := current.Entries[name], entry
				entry = withAgentFields("pi", before, want)
				// Sync removes the fields it wrote earlier that the server no longer sets.
				state, _, stateErr := s.loadLedger()
				if stateErr != nil {
					err = stateErr
				} else if own := state.Entries[ownershipKey("pi", path, name)]; own.Owner == s.ConfigPath {
					for key, hash := range own.PiFields {
						if _, set := want[key]; set {
							continue
						}
						if value, exists := before[key]; exists {
							if piFieldHash(value) != hash {
								err = fmt.Errorf("Pi setting changed since sync: %s", key)
								break
							}
							delete(entry, key)
						}
					}
				}
				// The preview may include manually configured OAuth/custom fields. Never
				// copy their secret values into an API response.
				for key, value := range entry {
					if _, specified := server.PiOptions[key]; !specified && key != "command" && key != "args" && key != "env" && key != "url" && key != "headers" {
						entry[key] = redactPiOption(key, value)
					}
				}
			}
		}
		var n *Native
		if err == nil {
			if target == "goose" {
				entry["name"] = name
			}
			n, err = ParseNative(native, nil)
		}
		var data []byte
		if err == nil {
			data, err = n.Edit(map[string]map[string]any{name: entry})
		}
		if err != nil {
			r.Error = err.Error()
		} else {
			r.Content = string(data)
		}
		out = append(out, r)
	}
	return out
}
