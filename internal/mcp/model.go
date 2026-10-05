// Package mcp manages portable MCP declarations and native client configuration.
// It never starts servers, resolves credentials, or performs network requests, except
// for the live probe that `mcp check --live` asks for explicitly.
package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
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
	Targets     TargetList       `yaml:"targets,omitempty" json:"targets,omitzero"`
	// Tools is which of the server's tools reach the model, translated per Agent.
	Tools ToolPolicy `yaml:"tools,omitempty" json:"tools,omitzero"`
	// PiOptions are Pi built-in fields Skillshare has no setting for, such as timeout.
	// They are written into Pi's entry as given.
	PiOptions PiOptions `yaml:"piOptions,omitempty" json:"piOptions,omitempty"`
	// Disabled is the whole entry: it turns off, for one project, a server that the
	// Agent's global config defines. Unselecting an Agent already covers a server
	// Skillshare defines, so a disabled server carries no command or url.
	Disabled bool `yaml:"disabled,omitempty" json:"disabled,omitempty"`
}

// legacyServerFields chose Pi's MCP extension and opted in to pruning Pi fields, before
// 0.23.0 made Pi use its built-in MCP and always prune. A config that still has them loads,
// and saving it drops them.
var legacyServerFields = []string{"piExtension", "piOptionsPrune"}

// UnmarshalJSON ignores legacyServerFields and converts directTools as loading does, so a
// dashboard that still sends them can save.
func (s *Server) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, key := range legacyServerFields {
		delete(fields, key)
	}
	var directTools any
	if raw, ok := fields["directTools"]; ok {
		_ = json.Unmarshal(raw, &directTools)
		delete(fields, "directTools")
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	type plain Server
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode((*plain)(s)); err != nil {
		return err
	}
	s.adoptAdapterTools(directTools, nil, nil)
	return nil
}

// adapterPiOptions are pi-mcp-adapter server fields that Pi's built-in MCP (0.99.0) does
// not read. Before 0.23.0 they reached the adapter's file through piOptions.
var adapterPiOptions = []string{"approveTools", "auth", "bearerToken", "bearerTokenEnv", "bearerTokenStore", "caFile", "debug", "exposeResources", "idleTimeout", "inheritEnv", "lifecycle", "protocolVersion", "requestHeadersCommand", "requestTimeoutMs", "searchKeywords", "socket", "tasks", "toolPrefix", "trace"}

// adapterPiOption reports a pi-mcp-adapter field. The adapter's auth is a string; since
// 0.99.2, Pi reads its own auth, an object naming a provider.
func adapterPiOption(key string, object bool) bool {
	return slices.Contains(adapterPiOptions, key) && !(key == "auth" && object)
}

func isObject(value any) bool {
	_, ok := value.(map[string]any)
	return ok
}

func deref(n *yaml.Node) *yaml.Node {
	if n != nil && n.Kind == yaml.AliasNode {
		return n.Alias
	}
	return n
}

// migration is what migrateServers needs to know about a servers mapping's scope.
type migration struct {
	// piAccounts are the account targets whose Agent is Pi.
	piAccounts []string
	// directTools is the scope's pi-mcp-adapter default, mcp.directTools or a project's,
	// for the servers that reach Pi and set none; defaults are the scope's targets.
	directTools any
	defaults    []string
}

// reachesPi reports a server whose targets, its own or the scope's, include Pi.
func (m migration) reachesPi(server *yaml.Node) bool {
	targets := m.defaults
	if n := deref(field(server, "targets")); n != nil {
		targets = nil
		_ = n.Decode(&targets)
	}
	return slices.ContainsFunc(targets, func(t string) bool { return t == "pi" || slices.Contains(m.piAccounts, t) })
}

// migrateServers rewrites, in place, what 0.23.0 retired in every server of a servers
// mapping, and names the servers each change touched, keyed by what changed:
//   - legacyServerFields are dropped; a piExtension that chose pi-mcp-adapter or
//     pi-mcp-extension is also keyed "piExtension.<extension>";
//   - pi-mcp-adapter piOptions are dropped, keyed "piOptions.<field>";
//   - directTools and piOptions includeTools/excludeTools become Pi exposure settings or
//     tools (adoptAdapterTools), keyed "piTools", or are dropped, keyed "piTools.<field>".
func migrateServers(servers *yaml.Node, m migration) map[string][]string {
	found := map[string][]string{}
	servers = deref(servers)
	if servers == nil || servers.Kind != yaml.MappingNode {
		return found
	}
	for i := 0; i+1 < len(servers.Content); i += 2 {
		name, server := servers.Content[i].Value, deref(servers.Content[i+1])
		if server.Kind != yaml.MappingNode {
			continue
		}
		for _, key := range legacyServerFields {
			if n := deref(field(server, key)); n != nil {
				if key == "piExtension" && (n.Value == "pi-mcp-adapter" || n.Value == "pi-mcp-extension") {
					found[key+"."+n.Value] = append(found[key+"."+n.Value], name)
				}
				drop(server, key)
				found[key] = append(found[key], name)
			}
		}
		if options := deref(field(server, "piOptions")); options != nil && options.Kind == yaml.MappingNode {
			for _, key := range adapterPiOptions {
				if n := deref(field(options, key)); n != nil && adapterPiOption(key, n.Kind == yaml.MappingNode) {
					drop(options, key)
					found["piOptions."+key] = append(found["piOptions."+key], name)
				}
			}
		}
		if dropped, ok := migrateAdapterTools(server, m); ok {
			found["piTools"] = append(found["piTools"], name)
			for _, key := range dropped {
				found["piTools."+key] = append(found["piTools."+key], name)
			}
		}
		if options := deref(field(server, "piOptions")); options != nil && options.Kind == yaml.MappingNode && len(options.Content) == 0 {
			drop(server, "piOptions")
		}
	}
	return found
}

// migrateAdapterTools converts one server's directTools, or the scope's when it reaches Pi
// and sets none, and its piOptions includeTools/excludeTools. It reports whether the server
// had any, and which of them it dropped.
func migrateAdapterTools(server *yaml.Node, m migration) ([]string, bool) {
	var directTools any
	var include, exclude []any
	had := false
	disabled := deref(field(server, "disabled"))
	if n := field(server, "directTools"); n != nil {
		_ = n.Decode(&directTools)
		drop(server, "directTools")
		had = true
	} else if m.directTools != nil && m.directTools != false && disabled == nil && m.reachesPi(server) {
		directTools = m.directTools
	}
	options := deref(field(server, "piOptions"))
	if options != nil && options.Kind == yaml.MappingNode {
		for key, list := range map[string]*[]any{"includeTools": &include, "excludeTools": &exclude} {
			if n := field(options, key); n != nil {
				if n.Decode(list) != nil || *list == nil {
					*list = []any{}
				}
				drop(options, key)
				had = true
			}
		}
	}
	if !had && directTools == nil {
		return nil, false
	}
	// Only what decides the conversion is decoded; the rest of the node stays as written.
	var draft Server
	if n := field(server, "tools"); n != nil && n.Decode(&draft.Tools) != nil {
		draft.Tools.Deny = []string{"unreadable"} // still counts as set
	}
	for _, key := range []string{"exposure", "toolExposure"} {
		if options != nil && field(options, key) != nil {
			if draft.PiOptions == nil {
				draft.PiOptions = PiOptions{}
			}
			draft.PiOptions[key] = true
		}
	}
	draft.Disabled = disabled != nil && disabled.Value == "true"
	before := draft
	dropped := draft.adoptAdapterTools(directTools, include, exclude)
	if before.Tools.IsZero() && !draft.Tools.IsZero() {
		_ = put(server, "tools", draft.Tools)
	}
	for _, key := range []string{"exposure", "toolExposure"} {
		if value, set := draft.PiOptions[key]; set && before.PiOptions[key] == nil {
			if options == nil || options.Kind != yaml.MappingNode {
				_ = put(server, "piOptions", map[string]any{})
				options = field(server, "piOptions")
			}
			_ = put(options, key, value)
		}
	}
	return dropped, true
}

// TargetList tells a missing list from an empty one. Missing inherits mcp.targets; empty
// keeps the server in Skillshare and writes it to no Agent, so it has to survive a save.
type TargetList []string

// IsZero makes yaml's omitempty drop only a missing list.
func (t TargetList) IsZero() bool { return t == nil }

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var serverName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)

// Targets are MCP clients, independently of installed skill targets.
var Targets = []string{"claude", "codex", "cursor", "vscode", "opencode", "kilocode", "grok", "antigravity", "amp", "claude-desktop", "cline", "copilot", "factory", "gemini", "goose", "junie", "kiro", "lmstudio", "warp", "windsurf", "pi", "omp"}

func hasInterpolation(value string) bool {
	return strings.Contains(value, "${") || strings.Contains(value, "{env:") || strings.Contains(value, "{file:")
}

// openCodeFormat reports the clients that read OpenCode's config shape: Kilo Code is an
// OpenCode fork and keeps the same "mcp" section, entry fields and {env:} references.
func openCodeFormat(target string) bool { return target == "opencode" || target == "kilocode" }

func validTarget(target string) bool {
	for _, name := range Targets {
		if name == target {
			return true
		}
	}
	return false
}

// Account is a target that is another config directory of a built-in Agent, such as a
// second Claude account. Its servers are written in that Agent's format, into Dir.
type Account struct {
	Agent string `json:"agent"`
	Dir   string `json:"configDir"`
}

// accountAgents are the Agents whose MCP file follows their config directory.
var accountAgents = []string{"claude", "codex", "pi", "omp"}

var accountName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// validateTargets knows the built-in Agents only. Another name may be an account, which
// the config's targets section declares, so Source.checkTargets settles it.
func validateTargets(targets []string) error {
	seen := map[string]bool{}
	for _, target := range targets {
		if !validTarget(target) && !accountName.MatchString(target) {
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
	for key := range s.Env {
		if !envName.MatchString(key) {
			return fmt.Errorf("MCP %s: invalid environment variable name", name)
		}
	}
	if !serverName.MatchString(name) {
		return fmt.Errorf("invalid MCP name %q: use letters, digits, dots, underscores or hyphens", name)
	}
	if err := s.validatePiOptions(name); err != nil {
		return err
	}
	if !s.Tools.IsZero() {
		switch {
		case s.Disabled:
			return fmt.Errorf("MCP %s: tools cannot be set on a disabled entry; it only switches the server off", name)
		case s.PiOptions["toolExposure"] != nil:
			return fmt.Errorf("MCP %s: tools and piOptions.toolExposure both set which tools Pi shows; keep it in tools", name)
		}
		if err := s.Tools.validate(name); err != nil {
			return err
		}
	}
	if s.Disabled {
		if s.Command != "" || s.URL != "" || s.Transport != "" || s.BearerToken != nil || len(s.Args)+len(s.Env)+len(s.Headers) > 0 {
			return fmt.Errorf("MCP %s: disabled turns off a server the Agent already has; leave out its command, url and settings", name)
		}
		return validateTargets(s.Targets)
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

// usesEnv reports whether any value is read from the environment when the Agent starts.
func (s Server) usesEnv() bool {
	if s.BearerToken != nil && s.BearerToken.FromEnv != "" {
		return true
	}
	for _, values := range []map[string]Value{s.Env, s.Headers} {
		for _, v := range values {
			if v.FromEnv != "" {
				return true
			}
		}
	}
	return false
}
