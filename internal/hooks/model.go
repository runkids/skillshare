// Package hooks manages Agent hook declarations and their native configuration.
// It never executes hook commands or extension code and never changes native trust.
package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Entry is one named hook under hooks.entries.
type Entry struct {
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Enabled defaults to true. A disabled entry keeps its definition and publishes nothing.
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	// Bindings are keyed by canonical Agent name or a global account target. An empty map publishes nothing.
	Bindings map[string]Binding `yaml:"bindings" json:"bindings"`
}

// Project is what a project's own config would hold under hooks, declared in the
// global config under hooks.projects so one sync reaches every root.
type Project struct {
	Entries map[string]Entry `yaml:"entries" json:"entries"`
}

// Binding is what one Agent receives. Command Agents use Events and Files, code Agents Code.
type Binding struct {
	// Events is the Agent's own event map, written without translation.
	Events map[string]any `yaml:"events,omitempty" json:"events,omitempty"`
	// Code is one standalone native TypeScript/JavaScript extension or plugin.
	Code string `yaml:"code,omitempty" json:"code,omitempty"`
	// Files are script files written to <Agent config dir>/hooks/skillshare/<entry>/.
	Files map[string]string `yaml:"files,omitempty" json:"files,omitempty"`
	// Commands are Git's native hook.<name> definitions.
	Commands map[string]GitCommand `yaml:"commands,omitempty" json:"commands,omitempty"`
	// Scripts is reserved for Git whole-file hooks, which are not supported yet.
	Scripts map[string]string `yaml:"scripts,omitempty" json:"scripts,omitempty"`
}

// GitCommand describes one friendly name, which Git may run for several events.
type GitCommand struct {
	Events   []string `yaml:"events" json:"events"`
	Command  string   `yaml:"command" json:"command"`
	Parallel *bool    `yaml:"parallel,omitempty" json:"parallel,omitempty"`
}

// IsEnabled reports the effective enabled state.
func (e Entry) IsEnabled() bool { return e.Enabled == nil || *e.Enabled }

// Target kinds.
const (
	KindCommand = "command"
	KindCode    = "code"
	KindGit     = "git"
)

// TargetDef describes one supported Agent.
type TargetDef struct {
	Name  string `json:"name"`
	Agent string `json:"agent,omitempty"`
	Kind  string `json:"kind"`
	// Note is static loading and trust guidance; Skillshare never infers loaded state.
	Note string `json:"note"`
}

// Targets are the supported Agents in display order.
var Targets = []TargetDef{
	{Name: "git", Kind: KindGit, Note: "Git 2.54+ runs config-based hooks (hook.<name>). Skillshare writes its own included file and helper scripts. Git reads configuration on every command; management never executes hooks."},
	{Name: "claude", Kind: KindCommand, Note: "Claude Code runs settings.json hooks only after the workspace trust dialog is accepted; /hooks lists them read-only."},
	{Name: "codex", Kind: KindCommand, Note: "Codex loads hooks.json together with inline [hooks] in config.toml, which Skillshare leaves untouched and lists as an additional source. Review and trust each new or changed hook in /hooks; project hooks load only when the project's .codex folder is trusted."},
	{Name: "gemini", Kind: KindCommand, Note: "Gemini CLI reads hooks from settings.json. Project hooks are fingerprinted and warn until trusted again after any change; manage them with /hooks panel."},
	{Name: "copilot", Kind: KindCommand, Note: "GitHub Copilot loads every JSON file in its hooks folder (COPILOT_HOME/hooks, or .github/hooks in a project, which loads only in a trusted folder). Skillshare writes one skillshare-<entry>.json per hook."},
	{Name: "cursor", Kind: KindCommand, Note: "Cursor reads hooks.json (version 1, lowerCamelCase events) and reloads it when it changes; project hooks need a trusted workspace."},
	{Name: "droid", Kind: KindCommand, Note: "Factory Droid reads hooks.json, and settings.json hooks only while hooks.json is absent, so Skillshare never creates hooks.json over inline hooks. Droid snapshots hooks at startup; review changes in /hooks."},
	{Name: "qwen", Kind: KindCommand, Note: "Qwen Code reads hooks from settings.json; project hooks load only in trusted folders. Opening /hooks reloads the definitions."},
	{Name: "antigravity", Kind: KindCommand, Note: "Antigravity and its CLI (agy) share hooks.json: ~/.gemini/config/hooks.json globally and .agents/hooks.json in a project, where hooks load only in a trusted folder. Each hook is a named block; Skillshare writes one block per hook, named after it. The CLI can also read hooks from ~/.gemini/antigravity-cli/settings.json, which Skillshare leaves untouched."},
	{Name: "pi", Kind: KindCode, Note: "Pi discovers extensions at startup; project extensions load only after project trust is granted. The code must use the extension API of your installed Pi version."},
	{Name: "omp", Kind: KindCode, Note: "Oh My Pi discovers extensions at startup from its agent directory (~/.omp/agent/extensions, or PI_CODING_AGENT_DIR, which Pi reads too) and from .omp/extensions in a project. OMP has no project trust gate, so the next startup can execute the code. Its own hooks/pre and hooks/post factories are left untouched and listed as additional sources. The code must use the extension API of your installed OMP version."},
	{Name: "amp", Kind: KindCode, Note: "Amp runs plugins from its plugins folder with Bun; run 'plugins: reload' after sync. The code must use your Amp version's plugin API."},
	{Name: "opencode", Kind: KindCode, Note: "OpenCode loads plugins from its plugins folder. v1 plugins are named exports and v2 plugins export default Plugin.define(...); Skillshare writes your code as given and never converts between them."},
}

// targetAliases are accepted spellings of canonical Agent names.
var targetAliases = map[string]string{"factory": "droid", "antigravity-cli": "antigravity", "agy": "antigravity"}

func targetDef(name string) (TargetDef, bool) {
	for _, t := range Targets {
		if t.Name == name {
			return t, true
		}
	}
	return TargetDef{}, false
}

// canonicalTarget resolves aliases; unknown names are returned unchanged.
func canonicalTarget(name string) string {
	if alias, ok := targetAliases[name]; ok {
		return alias
	}
	return name
}

var (
	entryName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
	fileName  = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
	eventName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
)

// Errors that the HTTP layer maps to 409.
var (
	ErrStaleRevision = errors.New("hooks configuration changed since preview; preview again")
	ErrConflict      = errors.New("hooks conflicts found; no files changed")
)

// ParseEntry reads one Entry from a JSON or YAML document and validates it.
func ParseEntry(data []byte) (Entry, error) {
	var entry Entry
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		d := json.NewDecoder(bytes.NewReader(trimmed))
		d.DisallowUnknownFields()
		if err := d.Decode(&entry); err != nil {
			return Entry{}, fmt.Errorf("invalid hook entry JSON: %w", err)
		}
		if d.Decode(new(any)) != io.EOF {
			return Entry{}, fmt.Errorf("hook entry must contain one JSON document")
		}
	} else {
		d := yaml.NewDecoder(bytes.NewReader(trimmed))
		d.KnownFields(true)
		if err := d.Decode(&entry); err != nil {
			return Entry{}, fmt.Errorf("invalid hook entry YAML: %w", err)
		}
		if d.Decode(new(yaml.Node)) != io.EOF {
			return Entry{}, fmt.Errorf("hook entry must contain one YAML document")
		}
	}
	if err := entry.normalize(); err != nil {
		return Entry{}, err
	}
	return entry, entry.validate("entry", nil, validateParse)
}

// normalize resolves aliases and gives events their JSON shape, so YAML integers and
// maps compare and hash the same as JSON ones.
func (e *Entry) normalize() error {
	if e.Bindings == nil {
		e.Bindings = map[string]Binding{}
	}
	bindings := map[string]Binding{}
	for name, b := range e.Bindings {
		canonical := canonicalTarget(name)
		if _, dup := bindings[canonical]; dup {
			return fmt.Errorf("duplicate binding for %s", canonical)
		}
		if b.Events != nil {
			data, err := json.Marshal(b.Events)
			if err != nil {
				return fmt.Errorf("%s events: %w", canonical, err)
			}
			b.Events = map[string]any{}
			if err := json.Unmarshal(data, &b.Events); err != nil {
				return err
			}
		}
		bindings[canonical] = b
	}
	e.Bindings = bindings
	return nil
}

// Validate checks an entry against each Agent's native schema without executing it.
type validationScope int

const (
	validateGlobal validationScope = iota
	validateProject
	validateParse
)

var accountName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func (e Entry) Validate(name string) error { return e.validate(name, nil, validateGlobal) }

func (e Entry) validate(name string, accounts map[string]string, scope validationScope) error {
	if !entryName.MatchString(name) {
		return fmt.Errorf("invalid hook name %q: use letters, digits, dots, underscores or hyphens", name)
	}
	for _, target := range sortedKeys(e.Bindings) {
		agent := target
		def, ok := targetDef(agent)
		if !ok {
			if a := accounts[target]; a != "" && slices.Contains(accountAgents, a) {
				if scope == validateProject {
					return fmt.Errorf("hook %s: %s is an account of %s, and every account reads the same project files; bind %s", name, target, a, a)
				}
				agent = a
				def, ok = targetDef(agent)
			}
			if !ok {
				if scope == validateParse && accountName.MatchString(target) {
					continue
				}
				return fmt.Errorf("hook %s: unsupported Agent %q: it is neither an Agent nor a target with agent and config_dir", name, target)
			}
		}
		b := e.Bindings[target]
		if def.Kind == KindGit {
			if err := validateGit(b); err != nil {
				return fmt.Errorf("hook %s: git: %w", name, err)
			}
			continue
		}
		if b.Commands != nil || b.Scripts != nil {
			return fmt.Errorf("hook %s: %s does not accept Git commands or scripts", name, target)
		}
		if def.Kind == KindCode {
			if len(b.Events) > 0 || len(b.Files) > 0 {
				return fmt.Errorf("hook %s: %s takes code only, not events or files", name, target)
			}
			if strings.TrimSpace(b.Code) == "" {
				return fmt.Errorf("hook %s: %s requires code", name, target)
			}
			continue
		}
		if b.Code != "" {
			return fmt.Errorf("hook %s: %s takes events, not code", name, target)
		}
		if len(b.Events) == 0 {
			return fmt.Errorf("hook %s: %s requires at least one event", name, target)
		}
		if err := validateEvents(agent, b.Events); err != nil {
			return fmt.Errorf("hook %s: %s: %w", name, target, err)
		}
		for file := range b.Files {
			if !fileName.MatchString(file) || !filepath.IsLocal(file) {
				return fmt.Errorf("hook %s: %s: file %q must be a plain file name", name, target, file)
			}
		}
	}
	return nil
}

// validateEvents checks the structure every Agent documents, leaving event names,
// matchers, timeouts and payloads exactly as the Agent defines them.
func validateEvents(target string, events map[string]any) error {
	for _, event := range sortedKeys(events) {
		if !eventName.MatchString(event) {
			return fmt.Errorf("invalid event name %q", event)
		}
		if target == "cursor" && (event[0] < 'a' || event[0] > 'z') {
			return fmt.Errorf("event %q: Cursor uses lowerCamelCase event names", event)
		}
		items, ok := events[event].([]any)
		if !ok || len(items) == 0 {
			return fmt.Errorf("event %s must be a non-empty array", event)
		}
		for i, item := range items {
			obj, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("event %s[%d] must be an object", event, i)
			}
			if err := validateElement(target, event, obj); err != nil {
				return fmt.Errorf("event %s[%d]: %w", event, i, err)
			}
		}
	}
	return nil
}

func validateElement(target, event string, obj map[string]any) error {
	switch target {
	case "cursor":
		if kind, set := obj["type"]; set && kind != "command" && kind != "prompt" {
			return fmt.Errorf("type must be command or prompt")
		}
		if obj["type"] == "prompt" {
			return requireString(obj, "prompt")
		}
		return requireString(obj, "command")
	case "copilot":
		return validateCopilot(event, obj)
	case "antigravity":
		return validateAntigravity(obj)
	}
	// Matcher groups: Claude, Codex, Gemini, Qwen and Droid.
	if matcher, set := obj["matcher"]; set {
		if _, ok := matcher.(string); !ok {
			return fmt.Errorf("matcher must be a string")
		}
	}
	handlers, ok := obj["hooks"].([]any)
	if !ok || len(handlers) == 0 {
		return fmt.Errorf("requires a non-empty hooks array")
	}
	for i, h := range handlers {
		handler, ok := h.(map[string]any)
		if !ok {
			return fmt.Errorf("hooks[%d] must be an object", i)
		}
		if err := requireString(handler, "type"); err != nil {
			return fmt.Errorf("hooks[%d]: %w", i, err)
		}
		if handler["type"] == "command" {
			if err := requireString(handler, "command"); err != nil {
				return fmt.Errorf("hooks[%d]: %w", i, err)
			}
		}
		if timeout, set := handler["timeout"]; set {
			if n, ok := timeout.(float64); !ok || n <= 0 {
				return fmt.Errorf("hooks[%d]: timeout must be a positive number", i)
			}
		}
	}
	return nil
}

// validateAntigravity checks a matcher group (PreToolUse, PostToolUse) or, for the other
// events, a handler listed directly under the event. type defaults to command.
func validateAntigravity(obj map[string]any) error {
	handlers := []any{obj}
	if _, grouped := obj["hooks"]; grouped {
		if matcher, set := obj["matcher"]; set {
			if _, ok := matcher.(string); !ok {
				return fmt.Errorf("matcher must be a string")
			}
		}
		list, ok := obj["hooks"].([]any)
		if !ok || len(list) == 0 {
			return fmt.Errorf("requires a non-empty hooks array")
		}
		handlers = list
	}
	for i, h := range handlers {
		handler, ok := h.(map[string]any)
		if !ok {
			return fmt.Errorf("hooks[%d] must be an object", i)
		}
		if kind, set := handler["type"]; set && kind != "command" {
			return fmt.Errorf("hooks[%d]: type must be command", i)
		}
		if err := requireString(handler, "command"); err != nil {
			return fmt.Errorf("hooks[%d]: %w", i, err)
		}
		if timeout, set := handler["timeout"]; set {
			if n, ok := timeout.(float64); !ok || n <= 0 {
				return fmt.Errorf("hooks[%d]: timeout must be a positive number", i)
			}
		}
	}
	return nil
}

// validateCopilot checks the required fields and exclusive forms of Copilot's flat
// handlers, leaving every field as written.
func validateCopilot(event string, obj map[string]any) error {
	for _, key := range []string{"timeoutSec", "timeout"} {
		if value, set := obj[key]; set {
			if n, ok := value.(float64); !ok || n <= 0 {
				return fmt.Errorf("%s must be a positive number", key)
			}
		}
	}
	switch obj["type"] {
	case "http":
		if err := requireString(obj, "url"); err != nil {
			return err
		}
		u, err := url.Parse(obj["url"].(string))
		if err != nil || u.Host == "" || u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("url must be an http:// or https:// URL")
		}
		_, envVars := obj["allowedEnvVars"]
		if u.Scheme != "https" && (envVars || event == "preToolUse" || event == "permissionRequest" || event == "PreToolUse" || event == "PermissionRequest") {
			return fmt.Errorf("url must use https:// here")
		}
		return nil
	case "prompt":
		if !strings.EqualFold(event, "sessionStart") {
			return fmt.Errorf("prompt hooks are only supported on sessionStart")
		}
		return requireString(obj, "prompt")
	case nil, "command":
	default:
		return fmt.Errorf("type must be command, http or prompt")
	}
	shell := false
	for _, key := range []string{"bash", "powershell", "command"} {
		if _, set := obj[key]; set {
			if err := requireString(obj, key); err != nil {
				return err
			}
			shell = true
		}
	}
	if _, set := obj["exec"]; set {
		if shell {
			return fmt.Errorf("exec cannot be combined with bash, powershell or command")
		}
		if err := requireString(obj, "exec"); err != nil {
			return err
		}
		if args, set := obj["args"]; set {
			list, ok := args.([]any)
			for _, arg := range list {
				if _, isString := arg.(string); !isString {
					ok = false
				}
			}
			if !ok {
				return fmt.Errorf("args must be an array of strings")
			}
		}
		return nil
	}
	if _, set := obj["args"]; set {
		return fmt.Errorf("args are passed to exec only")
	}
	if !shell {
		return fmt.Errorf("requires bash, powershell, command or exec")
	}
	return nil
}

func requireString(obj map[string]any, key string) error {
	if s, ok := obj[key].(string); !ok || strings.TrimSpace(s) == "" {
		return fmt.Errorf("%s must be a non-empty string", key)
	}
	return nil
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
