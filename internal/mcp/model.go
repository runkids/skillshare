// Package mcp manages portable MCP declarations and native client configuration.
// It never starts servers, resolves credentials, or performs network requests.
package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Value is either a literal or a reference resolved by the receiving client.
type Value struct {
	Literal string `json:"-" yaml:"-"`
	FromEnv string `json:"fromEnv,omitempty" yaml:"fromEnv,omitempty"`
}

func (v *Value) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode && n.Tag == "!!str" {
		v.Literal = n.Value
		return nil
	}
	if n.Kind != yaml.MappingNode || len(n.Content) != 2 || n.Content[0].Value != "fromEnv" || n.Content[1].Tag != "!!str" {
		return fmt.Errorf("expected a string or {fromEnv: VARIABLE}")
	}
	v.FromEnv = n.Content[1].Value
	if !envName.MatchString(v.FromEnv) {
		return fmt.Errorf("invalid environment variable name")
	}
	return nil
}

func (v Value) MarshalYAML() (any, error) {
	if v.FromEnv != "" {
		return map[string]string{"fromEnv": v.FromEnv}, nil
	}
	return v.Literal, nil
}

func (v *Value) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, &v.Literal)
	}
	var ref struct {
		FromEnv string `json:"fromEnv"`
	}
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if err := d.Decode(&ref); err != nil {
		return fmt.Errorf("expected a string or environment reference")
	}
	if !envName.MatchString(ref.FromEnv) {
		return fmt.Errorf("invalid environment variable name")
	}
	v.FromEnv = ref.FromEnv
	return nil
}

func (v Value) MarshalJSON() ([]byte, error) {
	if v.FromEnv != "" {
		return json.Marshal(map[string]string{"fromEnv": v.FromEnv})
	}
	return json.Marshal(v.Literal)
}

// Server contains only settings that have explicit native adapter mappings.
type Server struct {
	Transport   string           `yaml:"transport,omitempty" json:"transport,omitempty"`
	Command     string           `yaml:"command,omitempty" json:"command,omitempty"`
	Args        []string         `yaml:"args,omitempty" json:"args,omitempty"`
	URL         string           `yaml:"url,omitempty" json:"url,omitempty"`
	Env         map[string]Value `yaml:"env,omitempty" json:"env,omitempty"`
	Headers     map[string]Value `yaml:"headers,omitempty" json:"headers,omitempty"`
	BearerToken *Value           `yaml:"bearerToken,omitempty" json:"bearerToken,omitempty"`
	Targets     []string         `yaml:"targets,omitempty" json:"targets,omitempty"`
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var serverName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)

// Targets are MCP clients, independently of installed skill targets.
var Targets = []string{"claude", "codex", "cursor", "vscode", "opencode", "grok", "antigravity", "amp", "claude-desktop", "cline", "copilot", "factory", "gemini", "goose", "junie", "kiro", "lmstudio", "warp", "windsurf"}

func hasInterpolation(value string) bool {
	return strings.Contains(value, "${") || strings.Contains(value, "{env:") || strings.Contains(value, "{file:")
}

func validTarget(target string) bool {
	for _, name := range Targets {
		if name == target {
			return true
		}
	}
	return false
}

func validateTargets(targets []string) error {
	seen := map[string]bool{}
	for _, target := range targets {
		if !validTarget(target) {
			return fmt.Errorf("unsupported MCP target %q", target)
		}
		if seen[target] {
			return fmt.Errorf("duplicate MCP target %q", target)
		}
		seen[target] = true
	}
	return nil
}

// Validate checks a portable server without executing or connecting to it.
func (s Server) Validate(name string) error {
	if s.Targets != nil && len(s.Targets) == 0 {
		return fmt.Errorf("MCP %s: select at least one target or omit targets to inherit defaults", name)
	}
	for key := range s.Env {
		if !envName.MatchString(key) {
			return fmt.Errorf("MCP %s: invalid environment variable name", name)
		}
	}
	if !serverName.MatchString(name) {
		return fmt.Errorf("invalid MCP name %q: use letters, digits, dots, underscores or hyphens", name)
	}
	if (s.Command == "") == (s.URL == "") {
		return fmt.Errorf("MCP %s requires exactly one of command or url", name)
	}
	if err := validateTargets(s.Targets); err != nil {
		return err
	}
	if s.Transport != "" && s.Transport != "stdio" && s.Transport != "streamable-http" {
		return fmt.Errorf("MCP %s: unsupported transport %q", name, s.Transport)
	}
	if s.URL != "" {
		u, err := url.Parse(s.URL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
			return fmt.Errorf("MCP %s requires an HTTP(S) URL without embedded credentials or fragment", name)
		}
		for key := range u.Query() {
			if sensitiveKey.MatchString(key) {
				return fmt.Errorf("MCP %s: put credentials in environment-backed headers, not URL parameters", name)
			}
		}
		if s.Transport == "stdio" || len(s.Args) > 0 || len(s.Env) > 0 {
			return fmt.Errorf("MCP %s: remote servers cannot have stdio settings", name)
		}
	} else if s.Transport == "streamable-http" || len(s.Headers) > 0 || s.BearerToken != nil {
		return fmt.Errorf("MCP %s: stdio servers cannot have HTTP settings", name)
	}
	if s.BearerToken != nil && !envName.MatchString(s.BearerToken.FromEnv) {
		return fmt.Errorf("MCP %s: bearerToken requires fromEnv", name)
	}
	for _, values := range []map[string]Value{s.Env, s.Headers} {
		for key, v := range values {
			if sensitiveKey.MatchString(key) && v.FromEnv == "" {
				return fmt.Errorf("MCP %s: sensitive settings require fromEnv", name)
			}
			if hasInterpolation(v.Literal) {
				return fmt.Errorf("MCP %s: use fromEnv instead of client-specific interpolation", name)
			}
			if v.FromEnv != "" && (!envName.MatchString(v.FromEnv) || v.Literal != "") {
				return fmt.Errorf("MCP %s: invalid environment reference", name)
			}
		}
	}
	for _, value := range append([]string{s.Command, s.URL}, s.Args...) {
		if hasInterpolation(value) {
			return fmt.Errorf("MCP %s: command, args and URL must be portable literals", name)
		}
	}
	for key, v := range s.Headers {
		if strings.EqualFold(key, "Authorization") {
			if s.BearerToken != nil {
				return fmt.Errorf("MCP %s: choose bearerToken or Authorization, not both", name)
			}
			if v.FromEnv == "" {
				return fmt.Errorf("MCP %s: Authorization requires an environment reference", name)
			}
		}
	}
	return nil
}
