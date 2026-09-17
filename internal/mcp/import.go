package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/tailscale/hujson"
)

// Candidate is a portable import draft. Problems block saving, warnings describe
// credentials replaced with references and never contain their original values.
type Candidate struct {
	Name     string   `json:"name"`
	Server   Server   `json:"server"`
	Problems []string `json:"problems"`
	Warnings []string `json:"warnings"`
	From     string   `json:"from,omitempty"`
}

var sensitiveKey = regexp.MustCompile(`(?i)(token|secret|password|api.?key|authorization|credential)`)
var variableReference = regexp.MustCompile(`^\$\{(?:env:)?([A-Za-z_][A-Za-z0-9_]*)\}$`)
var openCodeReference = regexp.MustCompile(`^\{env:([A-Za-z_][A-Za-z0-9_]*)\}$`)

// hasURLPassword reports a URL with an embedded password, such as a database DSN.
func hasURLPassword(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.User == nil {
		return false
	}
	_, ok := u.User.Password()
	return ok
}

func importValue(key, value string, warnings *[]string) Value {
	if match := variableReference.FindStringSubmatch(value); match != nil {
		return Value{FromEnv: match[1]}
	}
	if sensitiveKey.MatchString(key) || hasURLPassword(value) {
		name := strings.ToUpper(regexp.MustCompile(`[^A-Za-z0-9_]`).ReplaceAllString(key, "_"))
		if !envName.MatchString(name) {
			name = "MCP_" + name
		}
		*warnings = append(*warnings, "Set "+name+" in the Agent environment; the imported secret was not saved")
		return Value{FromEnv: name}
	}
	return Value{Literal: value}
}

// detectJSONFormat picks the client whose top-level MCP key pasted JSON uses.
// A bare server object reads as Claude's single-entry shape. TOML is ambiguous
// between Codex and Grok, so callers must name that format explicitly.
func detectJSONFormat(data []byte) string {
	v, err := hujson.Parse(data)
	if err != nil {
		return "claude" // ParseNative reports the syntax error.
	}
	v.Standardize()
	var document map[string]json.RawMessage
	_ = json.Unmarshal(v.Pack(), &document)
	for _, target := range []string{"claude", "vscode", "opencode"} {
		if _, ok := document[nativeKey(target)]; ok {
			return target
		}
	}
	return "claude"
}

// Import parses a native file or a single JSON entry without persisting it.
func Import(target string, data []byte, singleName string) ([]Candidate, error) {
	if target == "" {
		target = detectJSONFormat(data)
	}
	native, err := ParseNative(target, data)
	if err != nil {
		return nil, err
	}
	if len(native.Entries) == 0 && singleName != "" && !isTOMLTarget(target) {
		v := native.json.Clone()
		v.Standardize()
		var entry map[string]any
		if json.Unmarshal(v.Pack(), &entry) != nil {
			return nil, fmt.Errorf("invalid server JSON")
		}
		if entry["command"] == nil && entry["url"] == nil {
			return nil, fmt.Errorf("no MCP entries found")
		}
		native.Entries[singleName] = entry
	}
	if len(native.Entries) == 0 {
		return nil, fmt.Errorf("no MCP entries found; select the matching client format")
	}
	out := []Candidate{}
	for _, name := range sortedKeys(native.Entries) {
		entry := native.Entries[name]
		c := Candidate{Name: name, Problems: []string{}, Warnings: []string{}}
		for key, active := range map[string]any{"enabled": true, "disabled": false} {
			if value, exists := entry[key]; exists {
				if value != active {
					c.Problems = append(c.Problems, "Disabled servers cannot be imported as active connections")
				}
				delete(entry, key)
			}
		}
		if target == "opencode" {
			if command, exists := entry["command"]; exists {
				var words []string
				data, _ := json.Marshal(command)
				if json.Unmarshal(data, &words) != nil || len(words) == 0 {
					c.Problems = append(c.Problems, "command must be a non-empty array of strings")
				} else {
					entry["command"] = words[0]
					entry["args"] = words[1:]
				}
			}
			if environment, exists := entry["environment"]; exists {
				entry["env"] = environment
				delete(entry, "environment")
			}
			switch entry["type"] {
			case "local":
				entry["type"] = "stdio"
			case "remote":
				entry["type"] = "http"
			default:
				c.Problems = append(c.Problems, "OpenCode type must be local or remote")
			}
		}
		allowed := map[string]bool{"type": true, "command": true, "args": true, "url": true, "env": true, "headers": true}
		if target == "codex" {
			allowed = map[string]bool{"command": true, "args": true, "url": true, "env": true, "env_vars": true, "http_headers": true, "env_http_headers": true, "bearer_token_env_var": true}
		}
		// Agent-only fields such as timeouts stay in existing Agent entries on sync.
		for _, key := range sortedKeys(entry) {
			if !allowed[key] {
				c.Warnings = append(c.Warnings, "Agent-specific field not imported: "+key)
			}
		}
		for key, dest := range map[string]*string{"command": &c.Server.Command, "url": &c.Server.URL} {
			if value, ok := entry[key]; ok {
				text, ok := value.(string)
				if !ok {
					c.Problems = append(c.Problems, key+" must be a string")
				} else {
					*dest = text
				}
			}
		}
		if value, ok := entry["type"]; ok {
			switch value {
			case "stdio":
				c.Server.Transport = "stdio"
			case "http", "streamable-http":
				c.Server.Transport = "streamable-http"
			default:
				c.Problems = append(c.Problems, "Unsupported native transport")
			}
		}
		if value, ok := entry["args"]; ok {
			data, _ := json.Marshal(value)
			if json.Unmarshal(data, &c.Server.Args) != nil {
				c.Problems = append(c.Problems, "args must be an array of strings")
			}
		}
		for _, key := range []string{"env", "headers", "http_headers"} {
			raw, ok := entry[key]
			if !ok {
				continue
			}
			values, ok := raw.(map[string]any)
			if !ok {
				c.Problems = append(c.Problems, key+" must be an object")
				continue
			}
			converted := map[string]Value{}
			for _, k := range sortedKeys(values) {
				value, ok := values[k].(string)
				if !ok {
					c.Problems = append(c.Problems, key+" values must be strings")
					continue
				}
				if target == "opencode" {
					prefix := ""
					candidate := value
					if strings.EqualFold(k, "Authorization") && strings.HasPrefix(value, "Bearer ") {
						prefix = "Bearer "
						candidate = strings.TrimPrefix(value, prefix)
					}
					if match := openCodeReference.FindStringSubmatch(candidate); match != nil {
						value = prefix + "${" + match[1] + "}"
					} else if strings.Contains(value, "{env:") || strings.Contains(value, "{file:") {
						c.Problems = append(c.Problems, "OpenCode interpolation must use a single environment reference")
						continue
					}
				}
				if key != "env" && strings.EqualFold(k, "Authorization") && strings.HasPrefix(value, "Bearer ") {
					v := importValue("MCP_"+strings.ToUpper(strings.ReplaceAll(name, "-", "_"))+"_TOKEN", strings.TrimPrefix(value, "Bearer "), &c.Warnings)
					c.Server.BearerToken = &v
					continue
				}
				converted[k] = importValue(k, value, &c.Warnings)
			}
			if key == "env" {
				c.Server.Env = converted
			} else {
				c.Server.Headers = converted
			}
		}
		if raw, ok := entry["env_vars"]; ok {
			var names []string
			data, _ := json.Marshal(raw)
			if json.Unmarshal(data, &names) != nil {
				c.Problems = append(c.Problems, "Only local env_vars string entries are supported")
			} else {
				if c.Server.Env == nil {
					c.Server.Env = map[string]Value{}
				}
				for _, key := range names {
					c.Server.Env[key] = Value{FromEnv: key}
				}
			}
		}
		if raw, ok := entry["env_http_headers"]; ok {
			var headers map[string]string
			data, _ := json.Marshal(raw)
			if json.Unmarshal(data, &headers) != nil {
				c.Problems = append(c.Problems, "Invalid env_http_headers")
			} else {
				if c.Server.Headers == nil {
					c.Server.Headers = map[string]Value{}
				}
				for key, value := range headers {
					c.Server.Headers[key] = Value{FromEnv: value}
				}
			}
		}
		if raw, ok := entry["bearer_token_env_var"]; ok {
			value, ok := raw.(string)
			if !ok {
				c.Problems = append(c.Problems, "Invalid bearer token reference")
			} else {
				c.Server.BearerToken = &Value{FromEnv: value}
			}
		}
		for _, arg := range c.Server.Args {
			if strings.Contains(arg, "${") {
				c.Problems = append(c.Problems, "Argument interpolation must be converted to a portable literal before importing")
				break
			}
		}
		// Arguments have no portable reference syntax, so credentials there can only be flagged.
		for i, arg := range c.Server.Args {
			key, value, assigned := strings.Cut(strings.TrimLeft(arg, "-"), "=")
			flagWithValue := !assigned && strings.HasPrefix(arg, "-") && i+1 < len(c.Server.Args) && !strings.HasPrefix(c.Server.Args[i+1], "-")
			if hasURLPassword(arg) || hasURLPassword(value) || sensitiveKey.MatchString(key) && (value != "" || flagWithValue) {
				c.Warnings = append(c.Warnings, "An argument looks like a credential and is saved as plain text; use an environment variable if the server supports one")
				break
			}
		}
		for _, values := range []map[string]Value{c.Server.Env, c.Server.Headers} {
			for _, value := range values {
				if strings.Contains(value.Literal, "${") {
					c.Problems = append(c.Problems, "Client-specific input or interpolation must be replaced with an environment reference")
				}
			}
		}
		if err := c.Server.Validate(name); err != nil {
			c.Problems = append(c.Problems, err.Error())
		}
		if len(c.Problems) > 0 {
			c.Server = Server{}
		}
		out = append(out, c)
	}
	return out, nil
}

// ImportClient inspects only the selected native configuration and scope.
func (s *Service) ImportClient(target string) ([]Candidate, error) {
	path, err := s.nativePath(target)
	if err != nil {
		return nil, err
	}
	data, exists, _, err := safeRead(path)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("no MCP configuration found for %s in this scope", target)
	}
	items, err := Import(target, data, "")
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].From = target
	}
	return items, nil
}
