package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/tailscale/hujson"
	"gopkg.in/yaml.v3"
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
		var doc yaml.Node
		if parseYAML(data, &doc) == nil && field(&doc, "extensions") != nil {
			return "goose"
		}
		return "claude" // ParseNative reports the syntax error.
	}
	v.Standardize()
	var document map[string]json.RawMessage
	_ = json.Unmarshal(v.Pack(), &document)
	var schema string
	_ = json.Unmarshal(document["$schema"], &schema)
	if document["disabledServers"] != nil || document["enabledServers"] != nil || strings.Contains(schema, "/can1357/oh-my-pi/") {
		return "omp"
	}
	if document["exposure"] != nil || document["toolExposure"] != nil || document["directTools"] != nil {
		return "pi"
	}
	if _, ok := document["transport"]; ok {
		return "pi"
	}
	if _, ok := document["serverUrl"]; ok {
		return "antigravity"
	}
	if _, ok := document["httpUrl"]; ok {
		return "gemini"
	}
	var servers map[string]map[string]any
	if json.Unmarshal(document["mcpServers"], &servers) == nil {
		for _, name := range sortedKeys(servers) {
			entry := servers[name]
			if entry["exposure"] != nil || entry["toolExposure"] != nil || entry["directTools"] != nil {
				return "pi"
			}
			if _, ok := entry["transport"]; ok {
				return "pi"
			}
			if entry["type"] == "streamableHttp" {
				return "cline"
			}
			if entry["type"] == "local" {
				return "copilot"
			}
			if _, ok := entry["httpUrl"]; ok {
				return "gemini"
			}
			if _, ok := entry["serverUrl"]; ok {
				return "antigravity"
			}
		}
	}
	var kind string
	_ = json.Unmarshal(document["type"], &kind)
	if kind == "streamableHttp" {
		return "cline"
	}
	if kind == "local" && document["command"] != nil {
		return "copilot"
	}
	for _, target := range []string{"goose", "amp", "claude", "vscode", "opencode"} {
		if _, ok := document[nativeKey(target)]; ok {
			return target
		}
	}
	return "claude"
}

// Import parses a native file or a single JSON entry without persisting it.
func Import(target string, data []byte, singleName string) ([]Candidate, error) {
	return importNative(target, data, singleName, false, false)
}

// DetectImportFormat identifies a native JSON shape; ambiguous connections use Claude's shape.
func DetectImportFormat(data []byte) string {
	return detectJSONFormat(data)
}

// importNative converts native entries to portable drafts. adapter marks pi-mcp-adapter's
// mcp-adapter.json, whose !! escapes a literal beginning with !. piProject marks a project's
// .pi/mcp.json, the only file where Pi reads a connection-less entry as an override.
func importNative(target string, data []byte, singleName string, adapter, piProject bool) ([]Candidate, error) {
	if target == "" {
		target = detectJSONFormat(data)
	}
	native, err := ParseNative(target, data)
	if err != nil {
		return nil, err
	}
	if len(native.Entries) == 0 && singleName != "" && !isTOMLTarget(target) && target != "goose" {
		v := native.json.Clone()
		v.Standardize()
		var entry PiOptions
		if json.Unmarshal(v.Pack(), &entry) != nil {
			return nil, fmt.Errorf("invalid server JSON")
		}
		if entry["command"] == nil && entry["url"] == nil && entry["serverUrl"] == nil && entry["httpUrl"] == nil {
			return nil, fmt.Errorf("no MCP entries found")
		}
		native.Entries[singleName] = entry
	}
	if len(native.Entries) == 0 {
		return nil, fmt.Errorf("no MCP entries found; select the matching client format")
	}
	var ompLists struct {
		Disabled []string `json:"disabledServers"`
		Enabled  []string `json:"enabledServers"`
	}
	if target == "omp" {
		v := native.json.Clone()
		v.Standardize()
		if err := json.Unmarshal(v.Pack(), &ompLists); err != nil {
			return nil, fmt.Errorf("invalid OMP server lists: %w", err)
		}
	}
	out := []Candidate{}
	for _, name := range sortedKeys(native.Entries) {
		entry := native.Entries[name]
		c := Candidate{Name: name, Problems: []string{}, Warnings: []string{}}
		if target == "omp" {
			if slices.Contains(ompLists.Disabled, name) {
				c.Problems = append(c.Problems, "OMP disabledServers hides this server; it cannot be imported as an active connection")
			} else if slices.Contains(ompLists.Enabled, name) && entry["enabled"] == false {
				delete(entry, "enabled")
			}
		}
		if piProject && piOverride(entry) {
			c.Problems = append(c.Problems, "a Pi project override of the global server with this name, not a server; there is nothing to import")
			out = append(out, c)
			continue
		}
		normalizeClientImport(target, entry, &c)
		if target == "antigravity" {
			if value, exists := entry["serverUrl"]; exists {
				if entry["url"] != nil {
					c.Problems = append(c.Problems, "Antigravity requires serverUrl, not both url and serverUrl")
				}
				entry["url"] = value
				delete(entry, "serverUrl")
			} else if entry["url"] != nil {
				c.Problems = append(c.Problems, "Antigravity remote servers require serverUrl")
			}
		}
		for key, active := range map[string]any{"enabled": true, "disabled": false} {
			if value, exists := entry[key]; exists {
				if value != active && !(target == "pi" && key == "enabled") {
					c.Problems = append(c.Problems, "Disabled servers cannot be imported as active connections")
				}
				if !(target == "pi" && key == "enabled" && value == false) {
					delete(entry, key)
				}
			}
		}
		if openCodeFormat(target) {
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
		if target == "codex" {
			importCodexTools(entry, &c)
			allowed["enabled_tools"], allowed["disabled_tools"] = true, true
		}
		if target == "copilot" {
			importCopilotTools(entry, &c)
			allowed["tools"] = true
		}
		if target == "pi" {
			// pi-mcp-adapter's settings: its tool lists convert, the rest Pi does not read.
			list := func(key string) []any {
				value, set := entry[key]
				if !set {
					return nil
				}
				if items, ok := value.([]any); ok && items != nil {
					return items
				}
				return []any{}
			}
			directTools, include, exclude := entry["directTools"], list("includeTools"), list("excludeTools")
			for _, key := range append([]string{"directTools", "includeTools", "excludeTools"}, adapterPiOptions...) {
				if value, set := entry[key]; set && (key == "directTools" || key == "includeTools" || key == "excludeTools" || adapterPiOption(key, isObject(value))) {
					allowed[key] = true
				}
			}
			for key, value := range entry {
				if !allowed[key] {
					if c.Server.PiOptions == nil {
						c.Server.PiOptions = PiOptions{}
					}
					c.Server.PiOptions[key] = importPiOption(name, key, value, &c.Warnings)
					allowed[key] = true
				}
			}
			importPiTools(&c)
			if directTools != nil || include != nil || exclude != nil {
				before, _ := json.Marshal(c.Server)
				dropped := c.Server.adoptAdapterTools(directTools, include, exclude)
				if after, _ := json.Marshal(c.Server); string(after) != string(before) {
					c.Warnings = append(c.Warnings, "pi-mcp-adapter's directTools, includeTools and excludeTools are converted to Pi's exposure settings and tools")
				}
				for _, key := range dropped {
					c.Warnings = append(c.Warnings, "pi-mcp-adapter setting not imported: "+key+"; the entry already sets that part of its tool exposure, or it is not a list of tool names")
				}
			}
			for _, key := range adapterPiOptions {
				if value, set := entry[key]; set && adapterPiOption(key, isObject(value)) {
					c.Warnings = append(c.Warnings, "pi-mcp-adapter setting not imported, because Pi's built-in MCP does not read it: "+key)
				}
			}
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
				if target == "pi" && adapter && strings.HasPrefix(value, "!!") {
					value = strings.TrimPrefix(value, "!")
				} else if (target == "pi" || target == "omp") && strings.HasPrefix(value, "!") {
					c.Problems = append(c.Problems, map[string]string{"pi": "Pi", "omp": "OMP"}[target]+" command-based credentials must remain in the client; use an environment reference to import this connection")
					continue
				}
				if target == "omp" && value == k && envName.MatchString(value) {
					c.Warnings = append(c.Warnings, "OMP same-name environment reference imported as fromEnv; set the variable before connecting")
					value = "${" + value + "}"
				}
				if openCodeFormat(target) {
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

// importClient resolves the client an import reads: an Agent, or an account of one, whose
// file is its Agent's own format. A project has no accounts: every account reads its files.
func (s *Service) importClient(target string) (*Service, string, error) {
	if validTarget(target) {
		if source, err := LoadSource(s.ConfigPath); err == nil {
			return s.withAccounts(source.Accounts), target, nil
		}
		return s, target, nil
	}
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		return nil, "", err
	}
	if _, ok := source.Accounts[target]; !ok || s.ProjectRoot != "" {
		return nil, "", fmt.Errorf("unsupported MCP target %q", target)
	}
	scoped := s.withAccounts(source.Accounts)
	account, agent := scoped.forTarget(target)
	return account, agent, nil
}

// ImportFormat is the native format a target's own file is written in: an Agent's own,
// or, for an account, its Agent's. It is what --file data has to be read as.
func (s *Service) ImportFormat(target string) (string, error) {
	if target == "" {
		return "", nil
	}
	_, format, err := s.importClient(target)
	return format, err
}

// ImportClient inspects only the selected native configuration and scope. Candidates
// report the name that was asked for, so an account keeps its own.
func (s *Service) ImportClient(target string) ([]Candidate, error) {
	return s.ImportClientMode(target, "")
}

// ImportClientMode reads the target's file. Pi's pi-mcp-adapter file is still read, never
// written, so servers written there by hand can move to Pi's built-in MCP: with no piFile,
// both files, mcp.json first, and "pi-mcp-adapter" reads only the adapter's file. Any other
// piFile, which named a Pi MCP mode before 0.23.0, reads only mcp.json.
func (s *Service) ImportClientMode(target, piFile string) ([]Candidate, error) {
	client, format, err := s.importClient(target)
	if err != nil {
		return nil, err
	}
	if piFile != "" && (format != "pi" || (piFile != "builtin" && piFile != "pi-mcp-adapter" && piFile != "pi-mcp-extension")) {
		return nil, fmt.Errorf("unsupported Pi MCP file %q for %s", piFile, target)
	}
	path, err := client.nativePath(format)
	if err != nil {
		return nil, err
	}
	paths := []string{path}
	if format == "pi" {
		switch piFile {
		case "":
			paths = append(paths, piAdapterPath(path))
		case "pi-mcp-adapter":
			paths = []string{piAdapterPath(path)}
		}
	}
	items, found := []Candidate{}, false
	seen := map[string]bool{}
	for _, path := range paths {
		data, exists, _, err := safeRead(path)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		found = true
		adapter := format == "pi" && path == piAdapterPath(path)
		read, err := importNative(format, data, "", adapter, format == "pi" && !adapter && client.ProjectRoot != "")
		if err != nil {
			return nil, err
		}
		for _, item := range read {
			// mcp.json comes first and wins a name both files define.
			if seen[item.Name] {
				continue
			}
			seen[item.Name] = true
			item.From = target
			if adapter {
				item.Warnings = append(item.Warnings, "Imported from pi-mcp-adapter's mcp-adapter.json; sync writes it to Pi's built-in mcp.json and leaves the adapter's file as it is")
			}
			items = append(items, item)
		}
	}
	if !found {
		return nil, fmt.Errorf("no MCP configuration found for %s in this scope", target)
	}
	return items, nil
}

// ImportProjectClient is ImportClient reading a target's file in one mcp.projects root.
func (s *Service) ImportProjectClient(root, target string) ([]Candidate, error) {
	return s.ImportProjectClientMode(root, target, "")
}

// ImportProjectClientMode selects a Pi file within a configured project root.
func (s *Service) ImportProjectClientMode(root, target, piFile string) ([]Candidate, error) {
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	if _, ok := source.Projects[root]; !ok || s.ProjectRoot != "" {
		return nil, fmt.Errorf("%w: %s", ErrUnknownProject, root)
	}
	scoped := *s
	scoped.ProjectRoot = root
	return scoped.ImportClientMode(target, piFile)
}

// ImportSource is one selectable native file. For Pi, PiExtension says which of its files:
// builtin for mcp.json, or pi-mcp-adapter for the adapter's file, read-only since 0.23.0.
type ImportSource struct {
	Target      string `json:"target"`
	Path        string `json:"path"`
	PiExtension string `json:"piExtension,omitempty"`
}

// ImportSources exposes actual read paths in this scope (the empty key) and
// configured projects. Accounts exist only in the global scope.
func (s *Service) ImportSources(source *Source) map[string][]ImportSource {
	s = s.withAccounts(source.Accounts)
	scopes := map[string]*Service{"": s}
	for root := range source.Projects {
		scoped := *s
		scoped.ProjectRoot = root
		scopes[root] = &scoped
	}
	result := map[string][]ImportSource{}
	for root, scope := range scopes {
		paths := scope.ClientPaths()
		for target, path := range scope.AccountPaths(source.Accounts) {
			paths[target] = path
		}
		result[root] = []ImportSource{}
		for _, target := range sortedKeys(paths) {
			path := paths[target]
			if target == "pi" || source.Accounts[target].Agent == "pi" {
				result[root] = append(result[root], ImportSource{Target: target, Path: path, PiExtension: "builtin"}, ImportSource{Target: target, Path: piAdapterPath(path), PiExtension: "pi-mcp-adapter"})
			} else {
				result[root] = append(result[root], ImportSource{Target: target, Path: path})
			}
		}
	}
	return result
}

// Unmanaged is one Agent file's servers that no Skillshare config manages and the source
// does not define: ones added to the Agent directly, which an import can take over.
type Unmanaged struct {
	Target  string   `json:"target"`
	Project string   `json:"project,omitempty"`
	Path    string   `json:"path"`
	Names   []string `json:"names"`
}

// FindUnmanaged reads the Agent files of this scope, its accounts and every mcp.projects
// root. A file that cannot be read or parsed is skipped; sync reports it when it is used.
func (s *Service) FindUnmanaged(source *Source) []Unmanaged {
	s = s.withAccounts(source.Accounts)
	state, _, err := s.loadLedger()
	if err != nil {
		return nil
	}
	found := []Unmanaged{}
	scanFile := func(target, format, path, root string, defined map[string]Server) {
		data, exists, _, err := safeRead(path)
		if err != nil || !exists {
			return
		}
		native, err := ParseNative(format, data)
		if err != nil {
			return
		}
		var names []string
		for _, name := range sortedKeys(native.Entries) {
			_, owned := state.Entries[ownershipKey(format, path, name)]
			if _, ok := defined[name]; !ok && !owned && isServerEntry(native.Entries[name]) {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			found = append(found, Unmanaged{Target: target, Project: root, Path: path, Names: names})
		}
	}
	// ImportClient still reads pi-mcp-adapter's file next to Pi's own.
	scan := func(target, format, path, root string, defined map[string]Server) {
		scanFile(target, format, path, root, defined)
		if format == "pi" {
			scanFile(target, format, piAdapterPath(path), root, defined)
		}
	}
	paths := s.ClientPaths()
	for _, target := range Targets {
		if path, ok := paths[target]; ok {
			scan(target, target, path, "", source.Servers)
		}
	}
	accounts := s.AccountPaths(source.Accounts)
	for _, name := range sortedKeys(accounts) {
		scan(name, source.Accounts[name].Agent, accounts[name], "", source.Servers)
	}
	for _, root := range sortedKeys(source.Projects) {
		scoped := *s
		scoped.ProjectRoot = root
		paths := scoped.ClientPaths()
		for _, target := range Targets {
			if path, ok := paths[target]; ok {
				scan(target, target, path, root, source.Projects[root].Servers)
			}
		}
	}
	return found
}

// isServerEntry leaves out what has nothing to connect to, such as Goose's built-in
// extensions or a lone switch turning a global server off.
func isServerEntry(entry map[string]any) bool {
	for _, key := range []string{"command", "cmd", "url", "uri", "serverUrl", "httpUrl"} {
		if entry[key] != nil {
			return true
		}
	}
	return false
}
