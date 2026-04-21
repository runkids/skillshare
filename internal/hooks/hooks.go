package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"skillshare/internal/config"
	"skillshare/internal/tooling"
)

type Bundle struct {
	Name      string         `json:"name"`
	SourceDir string         `json:"source_dir"`
	Config    HookConfig     `json:"-"`
	Targets   map[string]int `json:"targets"`
	Warnings  []string       `json:"warnings,omitempty"`
	Issues    []string       `json:"issues,omitempty"`
}

type HookConfig struct {
	Name   string         `yaml:"name,omitempty"`
	Claude *TargetSection `yaml:"claude,omitempty"`
	Codex  *TargetSection `yaml:"codex,omitempty"`
}

type TargetSection struct {
	Events map[string][]HookEntry `yaml:"events,omitempty"`
}

type HookEntry struct {
	Matcher        any               `yaml:"matcher,omitempty"`
	Type           string            `yaml:"type,omitempty"`
	Command        string            `yaml:"command,omitempty"`
	URL            string            `yaml:"url,omitempty"`
	Prompt         string            `yaml:"prompt,omitempty"`
	Model          string            `yaml:"model,omitempty"`
	If             string            `yaml:"if,omitempty"`
	Shell          string            `yaml:"shell,omitempty"`
	Headers        map[string]string `yaml:"headers,omitempty"`
	AllowedEnvVars []string          `yaml:"allowed_env_vars,omitempty"`
	Timeout        int               `yaml:"timeout,omitempty"`
	StatusMessage  string            `yaml:"status_message,omitempty"`
	Async          bool              `yaml:"async,omitempty"`
	AsyncRewake    bool              `yaml:"async_rewake,omitempty"`
	Extra          map[string]any    `yaml:",inline"`
}

type ImportOptions struct {
	From      string
	Project   string
	All       bool
	OwnedOnly bool
}

type SyncResult struct {
	Name     string   `json:"name"`
	Target   string   `json:"target"`
	Root     string   `json:"root"`
	Merged   bool     `json:"merged"`
	Warnings []string `json:"warnings,omitempty"`
}

type claudeMatcherGroup struct {
	Matcher any
	Hooks   []map[string]any
	Extra   map[string]any
}

type importGroup struct {
	Name     string
	Root     string
	Section  *TargetSection
	Files    map[string]string
	Warnings []string
}

type localizedImportCommand struct {
	Command    string
	Owned      bool
	BundleName string
	BundleRoot string
	SourcePath string
	TargetRel  string
	Warning    string
}

var (
	placeholderRE = regexp.MustCompile(`\{([A-Z0-9_]+)\}`)

	supportedCodexEvents = map[string]bool{
		"PreToolUse":   true,
		"PostToolUse":  true,
		"Notification": true,
		"SessionStart": true,
		"SessionEnd":   true,
	}
	claudeToolScopedEvents = map[string]bool{
		"PreToolUse":         true,
		"PostToolUse":        true,
		"PostToolUseFailure": true,
		"PermissionRequest":  true,
	}
)

func Discover(sourceRoot string) ([]Bundle, error) {
	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []Bundle{}, nil
		}
		return nil, err
	}
	var out []Bundle
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(sourceRoot, entry.Name())
		cfg, warnings, err := readHookConfig(filepath.Join(dir, "hook.yaml"))
		if err != nil {
			continue
		}
		targets := map[string]int{}
		if cfg.Claude != nil {
			targets["claude"] = countEntries(cfg.Claude.Events)
		}
		if cfg.Codex != nil {
			targets["codex"] = countEntries(cfg.Codex.Events)
		}
		out = append(out, Bundle{Name: entry.Name(), SourceDir: dir, Config: cfg, Targets: targets, Warnings: warnings})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func SupportedTargets(bundle Bundle) []string {
	var out []string
	if supportsClaudeTarget(bundle.Config.Claude) {
		out = append(out, "claude")
	}
	if supportsCodexTarget(bundle.Config.Codex) {
		out = append(out, "codex")
	}
	return out
}

func SupportsTarget(bundle Bundle, target string) bool {
	for _, supported := range SupportedTargets(bundle) {
		if supported == target {
			return true
		}
	}
	return false
}

func RenderRoot(projectRoot, name, target string) string {
	if target == "codex" {
		return filepath.Join(config.CodexHooksRoot(projectRoot), name)
	}
	return filepath.Join(config.ClaudeHooksRoot(projectRoot), name)
}

func Import(sourceRoot string, opts ImportOptions) ([]Bundle, error) {
	if opts.All && opts.OwnedOnly {
		return nil, fmt.Errorf("--all and --owned-only cannot be used together")
	}
	switch opts.From {
	case "claude":
		return importClaudeHooks(sourceRoot, opts)
	case "codex":
		return importCodexHooks(sourceRoot, opts)
	default:
		return nil, fmt.Errorf("hooks import requires --from claude|codex")
	}
}

func SyncAll(sourceRoot, projectRoot, target string) ([]SyncResult, error) {
	bundles, err := Discover(sourceRoot)
	if err != nil {
		return nil, err
	}
	var results []SyncResult
	for _, bundle := range bundles {
		for _, one := range expandTargets(target) {
			res, err := SyncBundle(bundle, projectRoot, one)
			if err != nil {
				return results, err
			}
			results = append(results, res)
		}
	}
	return results, nil
}

func SyncBundle(bundle Bundle, projectRoot, target string) (SyncResult, error) {
	switch target {
	case "claude":
		if bundle.Config.Claude == nil {
			return SyncResult{Name: bundle.Name, Target: target, Warnings: []string{"no claude hooks defined"}}, nil
		}
		root := RenderRoot(projectRoot, bundle.Name, target)
		if err := os.MkdirAll(root, 0o755); err != nil {
			return SyncResult{}, err
		}
		if err := copyScripts(bundle.SourceDir, root); err != nil {
			return SyncResult{}, err
		}
		if err := mergeClaudeHooks(projectRoot, bundle.Name, root, bundle.Config.Claude); err != nil {
			return SyncResult{}, err
		}
		return SyncResult{Name: bundle.Name, Target: target, Root: root, Merged: true, Warnings: append([]string{}, bundle.Warnings...)}, nil
	case "codex":
		if bundle.Config.Codex == nil {
			return SyncResult{Name: bundle.Name, Target: target, Warnings: []string{"no codex hooks defined"}}, nil
		}
		root := RenderRoot(projectRoot, bundle.Name, target)
		if err := os.MkdirAll(root, 0o755); err != nil {
			return SyncResult{}, err
		}
		if err := copyScripts(bundle.SourceDir, root); err != nil {
			return SyncResult{}, err
		}
		warnings, err := mergeCodexHooks(projectRoot, bundle.Name, root, bundle.Config.Codex)
		if err != nil {
			return SyncResult{}, err
		}
		if err := enableCodexHooksFeature(); err != nil {
			return SyncResult{}, err
		}
		return SyncResult{Name: bundle.Name, Target: target, Root: root, Merged: true, Warnings: append(append([]string{}, bundle.Warnings...), warnings...)}, nil
	default:
		return SyncResult{}, fmt.Errorf("unsupported hook target %q", target)
	}
}

func readHookConfig(path string) (HookConfig, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return HookConfig{}, nil, err
	}
	var cfg HookConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return HookConfig{}, nil, err
	}
	var warnings []string
	for _, section := range []*TargetSection{cfg.Claude, cfg.Codex} {
		if section == nil {
			continue
		}
		for event, entries := range section.Events {
			if section == cfg.Codex && !supportedCodexEvents[event] {
				warnings = append(warnings, fmt.Sprintf("unsupported codex event %s", event))
			}
			for _, entry := range entries {
				for _, match := range placeholderRE.FindAllStringSubmatch(entry.Command, -1) {
					if len(match) > 1 && match[1] != "HOOK_ROOT" {
						return HookConfig{}, nil, fmt.Errorf("unsupported placeholder {%s} in %s", match[1], path)
					}
				}
			}
		}
	}
	return cfg, warnings, nil
}

func copyScripts(srcRoot, dstRoot string) error {
	src := filepath.Join(srcRoot, "scripts")
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	return tooling.ReplaceDir(src, filepath.Join(dstRoot, "scripts"))
}

func mergeClaudeHooks(projectRoot, name, root string, target *TargetSection) error {
	path := config.ClaudeSettingsPath(projectRoot)
	current := map[string]any{}
	if err := tooling.ReadJSON(path, &current); err != nil {
		return err
	}
	managedPrefix := filepath.ToSlash(RenderRoot(projectRoot, name, "claude"))
	existing := decodeClaudeHookMap(current["hooks"])
	filtered := removeManagedClaudeHandlers(existing, managedPrefix)
	current["hooks"] = encodeClaudeHookMap(mergeClaudeGroups(filtered, renderClaudeHookGroups(target.Events, root)))
	return tooling.WriteJSON(path, current)
}

func mergeCodexHooks(projectRoot, name, root string, target *TargetSection) ([]string, error) {
	path := config.CodexHooksConfigPath(projectRoot)
	current := map[string]any{}
	if err := tooling.ReadJSON(path, &current); err != nil {
		return nil, err
	}
	existing := decodeHookMap(current["hooks"])
	managedPrefix := filepath.ToSlash(RenderRoot(projectRoot, name, "codex"))
	replacements := map[string][]map[string]any{}
	var warnings []string
	for event, entries := range target.Events {
		if !supportedCodexEvents[event] {
			warnings = append(warnings, fmt.Sprintf("unsupported codex event %s not synced", event))
			continue
		}
		replacements[event] = renderHookEntries(map[string][]HookEntry{event: entries}, root)[event]
	}
	current["hooks"] = tooling.ManagedJSONMapMerge(existing, replacements, func(entry map[string]any) bool {
		command, _ := entry["command"].(string)
		return strings.Contains(filepath.ToSlash(command), managedPrefix)
	})
	return warnings, tooling.WriteJSON(path, current)
}

func renderClaudeHookGroups(events map[string][]HookEntry, root string) map[string][]claudeMatcherGroup {
	out := map[string][]claudeMatcherGroup{}
	for event, entries := range events {
		indexByMatcher := map[string]int{}
		for _, entry := range entries {
			matcher := entry.Matcher
			if matcher == nil {
				matcher = defaultClaudeMatcher(event)
			}
			key := matcherSignature(matcher)
			idx, ok := indexByMatcher[key]
			if !ok {
				idx = len(out[event])
				indexByMatcher[key] = idx
				out[event] = append(out[event], claudeMatcherGroup{Matcher: cloneMatcher(matcher)})
			}
			out[event][idx].Hooks = append(out[event][idx].Hooks, renderHandlerMap(entry, root))
		}
	}
	return out
}

func renderHookEntries(events map[string][]HookEntry, root string) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for event, entries := range events {
		for _, entry := range entries {
			out[event] = append(out[event], renderHandlerMap(entry, root))
		}
	}
	return out
}

func renderHandlerMap(entry HookEntry, root string) map[string]any {
	handler := map[string]any{}
	mergeAnyMap(handler, entry.Extra)
	for _, key := range []string{"matcher", "hooks"} {
		delete(handler, key)
	}
	typ := strings.TrimSpace(entry.Type)
	switch {
	case typ != "":
		handler["type"] = typ
	case entry.URL != "":
		handler["type"] = "http"
	case entry.Prompt != "":
		handler["type"] = "prompt"
	default:
		handler["type"] = "command"
	}
	if entry.Command != "" {
		handler["command"] = rewriteHookRoot(entry.Command, root)
	}
	if entry.URL != "" {
		handler["url"] = entry.URL
	}
	if entry.Prompt != "" {
		handler["prompt"] = entry.Prompt
	}
	if entry.Model != "" {
		handler["model"] = entry.Model
	}
	if entry.If != "" {
		handler["if"] = entry.If
	}
	if entry.Shell != "" {
		handler["shell"] = entry.Shell
	}
	if len(entry.Headers) > 0 {
		handler["headers"] = cloneStringMap(entry.Headers)
	}
	if len(entry.AllowedEnvVars) > 0 {
		handler["allowedEnvVars"] = append([]string{}, entry.AllowedEnvVars...)
	}
	if entry.Timeout > 0 {
		handler["timeout"] = entry.Timeout
	}
	if entry.StatusMessage != "" {
		handler["statusMessage"] = entry.StatusMessage
	}
	if entry.Async {
		handler["async"] = true
	}
	if entry.AsyncRewake {
		handler["asyncRewake"] = true
	}
	return handler
}

func decodeHookMap(raw any) map[string][]map[string]any {
	result := map[string][]map[string]any{}
	if raw == nil {
		return result
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return result
	}
	_ = json.Unmarshal(data, &result)
	return result
}

func decodeClaudeHookMap(raw any) map[string][]claudeMatcherGroup {
	result := map[string][]claudeMatcherGroup{}
	if raw == nil {
		return result
	}
	payload := map[string][]map[string]any{}
	data, err := json.Marshal(raw)
	if err != nil {
		return result
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return result
	}
	for event, groups := range payload {
		for _, rawGroup := range groups {
			group := claudeMatcherGroup{Extra: map[string]any{}}
			if hooksRaw, ok := rawGroup["hooks"]; ok {
				group.Matcher = rawGroup["matcher"]
				if group.Matcher == nil {
					group.Matcher = defaultClaudeMatcher(event)
				}
				for key, value := range rawGroup {
					if key == "hooks" || key == "matcher" {
						continue
					}
					group.Extra[key] = value
				}
				for _, hook := range decodeJSONArrayOfMaps(hooksRaw) {
					group.Hooks = append(group.Hooks, hook)
				}
			} else {
				group.Matcher = defaultClaudeMatcher(event)
				group.Hooks = append(group.Hooks, cloneAnyMap(rawGroup))
			}
			if len(group.Hooks) > 0 {
				result[event] = append(result[event], group)
			}
		}
	}
	return result
}

func encodeClaudeHookMap(groups map[string][]claudeMatcherGroup) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	events := sortedKeys(groups)
	for _, event := range events {
		for _, group := range groups[event] {
			if len(group.Hooks) == 0 {
				continue
			}
			rendered := map[string]any{
				"matcher": cloneMatcher(group.Matcher),
				"hooks":   cloneHandlerSlice(group.Hooks),
			}
			for key, value := range group.Extra {
				rendered[key] = value
			}
			out[event] = append(out[event], rendered)
		}
	}
	return out
}

func removeManagedClaudeHandlers(groups map[string][]claudeMatcherGroup, managedPrefix string) map[string][]claudeMatcherGroup {
	out := map[string][]claudeMatcherGroup{}
	for event, eventGroups := range groups {
		for _, group := range eventGroups {
			filtered := claudeMatcherGroup{Matcher: cloneMatcher(group.Matcher), Extra: cloneAnyMap(group.Extra)}
			for _, handler := range group.Hooks {
				if !isManagedCommandHandler(handler, managedPrefix) {
					filtered.Hooks = append(filtered.Hooks, cloneAnyMap(handler))
				}
			}
			if len(filtered.Hooks) > 0 {
				out[event] = append(out[event], filtered)
			}
		}
	}
	return out
}

func mergeClaudeGroups(existing, replacements map[string][]claudeMatcherGroup) map[string][]claudeMatcherGroup {
	out := map[string][]claudeMatcherGroup{}
	for event, groups := range existing {
		for _, group := range groups {
			out[event] = append(out[event], claudeMatcherGroup{
				Matcher: cloneMatcher(group.Matcher),
				Hooks:   cloneHandlerSlice(group.Hooks),
				Extra:   cloneAnyMap(group.Extra),
			})
		}
	}
	for event, groups := range replacements {
		for _, group := range groups {
			matched := false
			for idx := range out[event] {
				if matcherSignature(out[event][idx].Matcher) != matcherSignature(group.Matcher) {
					continue
				}
				out[event][idx].Hooks = append(out[event][idx].Hooks, cloneHandlerSlice(group.Hooks)...)
				matched = true
				break
			}
			if !matched {
				out[event] = append(out[event], claudeMatcherGroup{
					Matcher: cloneMatcher(group.Matcher),
					Hooks:   cloneHandlerSlice(group.Hooks),
					Extra:   cloneAnyMap(group.Extra),
				})
			}
		}
	}
	return out
}

func rewriteHookRoot(command, root string) string {
	return strings.ReplaceAll(command, "{HOOK_ROOT}", filepath.ToSlash(root))
}

func countEntries(events map[string][]HookEntry) int {
	total := 0
	for _, entries := range events {
		total += len(entries)
	}
	return total
}

func expandTargets(target string) []string {
	if target == "" || target == "all" {
		return []string{"claude", "codex"}
	}
	return []string{target}
}

func enableCodexHooksFeature() error {
	cfgPath := config.CodexConfigPath()
	data, err := os.ReadFile(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := tooling.EnsureManagedTOMLBool(string(data), []string{"features"}, "codex_hooks", true)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(cfgPath, []byte(content), 0o644)
}

func importClaudeHooks(sourceRoot string, opts ImportOptions) ([]Bundle, error) {
	path := config.ClaudeSettingsPath(opts.Project)
	current := map[string]any{}
	if err := tooling.ReadJSON(path, &current); err != nil {
		return nil, err
	}
	managedRoot := config.ClaudeHooksRoot(opts.Project)
	groups := groupImportedClaudeHooks(decodeClaudeHookMap(current["hooks"]), opts, managedRoot)
	legacyPath := filepath.Join(filepath.Dir(path), "hooks.json")
	if _, err := os.Stat(legacyPath); err == nil {
		legacy := map[string]any{}
		if err := tooling.ReadJSON(legacyPath, &legacy); err == nil {
			for name, group := range groupImportedClaudeHooks(decodeClaudeHookMap(legacy["hooks"]), opts, managedRoot) {
				groups[name] = mergeImportGroup(groups[name], group)
			}
		}
	}
	return writeImportedGroups(sourceRoot, groups, "claude")
}

func importCodexHooks(sourceRoot string, opts ImportOptions) ([]Bundle, error) {
	path := config.CodexHooksConfigPath(opts.Project)
	current := map[string]any{}
	if err := tooling.ReadJSON(path, &current); err != nil {
		return nil, err
	}
	groups := groupImportedFlatHooks(decodeHookMap(current["hooks"]), opts, config.CodexHooksRoot(opts.Project))
	return writeImportedGroups(sourceRoot, groups, "codex")
}

func groupImportedClaudeHooks(events map[string][]claudeMatcherGroup, opts ImportOptions, managedRoot string) map[string]importGroup {
	out := map[string]importGroup{}
	counter := 0
	for event, groups := range events {
		for _, group := range groups {
			for _, handler := range group.Hooks {
				entry, name, root, copyFile, warning, owned, ok := importedHookEntryFromHandler(event, group.Matcher, handler, managedRoot)
				if !ok {
					continue
				}
				if opts.OwnedOnly && !owned {
					continue
				}
				if name == "" {
					counter++
					name = fmt.Sprintf("imported-%d", counter)
				}
				current := out[name]
				if current.Name == "" {
					current = importGroup{
						Name:    name,
						Root:    root,
						Section: &TargetSection{Events: map[string][]HookEntry{}},
						Files:   map[string]string{},
					}
				}
				current.Section.Events[event] = append(current.Section.Events[event], entry)
				if copyFile.SourcePath != "" && copyFile.TargetRel != "" {
					current.Files[copyFile.SourcePath] = copyFile.TargetRel
				}
				if warning != "" {
					current.Warnings = append(current.Warnings, warning)
				}
				out[name] = current
			}
		}
	}
	return out
}

func groupImportedFlatHooks(events map[string][]map[string]any, opts ImportOptions, managedRoot string) map[string]importGroup {
	out := map[string]importGroup{}
	counter := 0
	for event, entries := range events {
		for _, handler := range entries {
			entry, name, root, copyFile, warning, owned, ok := importedHookEntryFromHandler(event, nil, handler, managedRoot)
			if !ok {
				continue
			}
			if opts.OwnedOnly && !owned {
				continue
			}
			if name == "" {
				counter++
				name = fmt.Sprintf("imported-%d", counter)
			}
			current := out[name]
			if current.Name == "" {
				current = importGroup{
					Name:    name,
					Root:    root,
					Section: &TargetSection{Events: map[string][]HookEntry{}},
					Files:   map[string]string{},
				}
			}
			current.Section.Events[event] = append(current.Section.Events[event], entry)
			if copyFile.SourcePath != "" && copyFile.TargetRel != "" {
				current.Files[copyFile.SourcePath] = copyFile.TargetRel
			}
			if warning != "" {
				current.Warnings = append(current.Warnings, warning)
			}
			out[name] = current
		}
	}
	return out
}

func importedHookEntryFromHandler(event string, matcher any, handler map[string]any, managedRoot string) (HookEntry, string, string, localizedImportCommand, string, bool, bool) {
	entry := HookEntry{Extra: map[string]any{}}
	for key, value := range handler {
		switch key {
		case "type":
			entry.Type, _ = value.(string)
		case "command":
			entry.Command, _ = value.(string)
		case "url":
			entry.URL, _ = value.(string)
		case "prompt":
			entry.Prompt, _ = value.(string)
		case "model":
			entry.Model, _ = value.(string)
		case "if":
			entry.If, _ = value.(string)
		case "shell":
			entry.Shell, _ = value.(string)
		case "headers":
			entry.Headers = anyStringMap(value)
		case "allowedEnvVars":
			entry.AllowedEnvVars = anyStringSlice(value)
		case "timeout":
			entry.Timeout = anyInt(value)
		case "statusMessage":
			entry.StatusMessage, _ = value.(string)
		case "async":
			entry.Async, _ = value.(bool)
		case "asyncRewake":
			entry.AsyncRewake, _ = value.(bool)
		default:
			if key != "matcher" && key != "hooks" {
				entry.Extra[key] = value
			}
		}
	}
	if matcher != nil {
		entry.Matcher = cloneMatcher(matcher)
	}
	if entry.Command != "" {
		localized := localizeImportedCommand(entry.Command, managedRoot)
		if localized.Command != "" {
			entry.Command = localized.Command
		}
		return entry, localized.BundleName, localized.BundleRoot, localized, localized.Warning, localized.Owned, true
	}
	return entry, "", "", localizedImportCommand{}, "", false, true
}

func localizeImportedCommand(command, managedRoot string) localizedImportCommand {
	command = strings.TrimSpace(command)
	if command == "" {
		return localizedImportCommand{}
	}
	if src, quote, rest, ok := parseDirectExecutableCommand(command); ok {
		return buildLocalizedImportCommand(command, managedRoot, src, quote, quote, rest)
	}
	if prefix, src, quote, suffix, ok := parseInterpreterScriptCommand(command); ok {
		return buildLocalizedImportCommand(prefix+src+suffix, managedRoot, src, prefix, quote, suffix)
	}
	return localizedImportCommand{
		Command: command,
		Warning: fmt.Sprintf("imported hook command verbatim; could not isolate a local script path from %q", command),
	}
}

func buildLocalizedImportCommand(original, managedRoot, srcPath, prefix, quote, suffix string) localizedImportCommand {
	info, err := os.Stat(srcPath)
	if err != nil || info.IsDir() {
		return localizedImportCommand{
			Command: original,
			Warning: fmt.Sprintf("imported hook command verbatim; local script %q was not found", srcPath),
		}
	}
	name, root, rel, owned := inferImportBundle(srcPath, managedRoot)
	rewritten := prefix + quote + "{HOOK_ROOT}/scripts/" + filepath.ToSlash(rel) + quote + suffix
	if prefix == quote && suffix == "" {
		rewritten = quote + "{HOOK_ROOT}/scripts/" + filepath.ToSlash(rel) + quote
	}
	return localizedImportCommand{
		Command:    rewritten,
		Owned:      owned,
		BundleName: name,
		BundleRoot: root,
		SourcePath: srcPath,
		TargetRel:  rel,
	}
}

func parseDirectExecutableCommand(command string) (string, string, string, bool) {
	if len(command) == 0 {
		return "", "", "", false
	}
	if command[0] == '"' || command[0] == '\'' {
		quote := string(command[0])
		end := strings.Index(command[1:], quote)
		if end < 0 {
			return "", "", "", false
		}
		path := command[1 : 1+end]
		if filepath.IsAbs(path) {
			return path, quote, command[1+end+1:], true
		}
		return "", "", "", false
	}
	if !filepath.IsAbs(strings.Fields(command)[0]) {
		return "", "", "", false
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", "", "", false
	}
	path := fields[0]
	rest := strings.TrimPrefix(command, path)
	return path, "", rest, true
}

func parseInterpreterScriptCommand(command string) (string, string, string, string, bool) {
	trimmed := strings.TrimSpace(command)
	fields := strings.Fields(trimmed)
	if len(fields) < 2 {
		return "", "", "", "", false
	}
	firstSpace := strings.IndexAny(trimmed, " \t")
	if firstSpace < 0 {
		return "", "", "", "", false
	}
	prefix := trimmed[:firstSpace+1]
	remaining := strings.TrimLeft(trimmed[firstSpace+1:], " \t")
	if remaining == "" {
		return "", "", "", "", false
	}
	if remaining[0] == '"' || remaining[0] == '\'' {
		quote := string(remaining[0])
		end := strings.Index(remaining[1:], quote)
		if end < 0 {
			return "", "", "", "", false
		}
		path := remaining[1 : 1+end]
		if !filepath.IsAbs(path) {
			return "", "", "", "", false
		}
		return prefix, path, quote, remaining[1+end+1:], true
	}
	second := strings.Fields(remaining)
	if len(second) == 0 || !filepath.IsAbs(second[0]) {
		return "", "", "", "", false
	}
	path := second[0]
	return prefix, path, "", strings.TrimPrefix(remaining, path), true
}

func inferImportBundle(srcPath, managedRoot string) (string, string, string, bool) {
	scriptPath := filepath.Clean(srcPath)
	managedRoot = filepath.Clean(managedRoot)
	if managedRoot != "" {
		prefix := managedRoot + string(filepath.Separator)
		if strings.HasPrefix(scriptPath, prefix) {
			relToManaged, _ := filepath.Rel(managedRoot, scriptPath)
			parts := strings.Split(filepath.ToSlash(relToManaged), "/")
			if len(parts) >= 2 {
				bundleName := parts[0]
				root := filepath.Join(managedRoot, bundleName)
				if parts[1] == "scripts" {
					return bundleName, root, filepath.ToSlash(filepath.Join(parts[2:]...)), true
				}
				return bundleName, root, filepath.Base(scriptPath), true
			}
		}
	}
	dir := filepath.Dir(scriptPath)
	name := filepath.Base(dir)
	root := dir
	if filepath.Base(filepath.Dir(scriptPath)) == "scripts" {
		root = filepath.Dir(filepath.Dir(scriptPath))
		name = filepath.Base(root)
		rel, _ := filepath.Rel(filepath.Join(root, "scripts"), scriptPath)
		return name, root, filepath.ToSlash(rel), false
	}
	if name == "hooks" || name == ".claude" || name == ".codex" || name == "" || name == "." {
		base := strings.TrimSuffix(filepath.Base(scriptPath), filepath.Ext(scriptPath))
		if base != "" {
			name = base
		}
	}
	return name, root, filepath.Base(scriptPath), false
}

func mergeImportGroup(dst, src importGroup) importGroup {
	if dst.Name == "" {
		return src
	}
	for event, entries := range src.Section.Events {
		dst.Section.Events[event] = append(dst.Section.Events[event], entries...)
	}
	if dst.Files == nil {
		dst.Files = map[string]string{}
	}
	for srcPath, rel := range src.Files {
		dst.Files[srcPath] = rel
	}
	dst.Warnings = append(dst.Warnings, src.Warnings...)
	return dst
}

func writeImportedGroups(sourceRoot string, groups map[string]importGroup, target string) ([]Bundle, error) {
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	var bundles []Bundle
	for _, name := range names {
		group := groups[name]
		dir := filepath.Join(sourceRoot, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		cfg := HookConfig{Name: name}
		switch target {
		case "claude":
			cfg.Claude = group.Section
		case "codex":
			cfg.Codex = group.Section
		}
		data, err := yaml.Marshal(&cfg)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(dir, "hook.yaml"), data, 0o644); err != nil {
			return nil, err
		}
		if err := copyImportedHookFiles(dir, group.Files); err != nil {
			return nil, err
		}
		discovered, ok, err := discoverImportedBundle(dir, name)
		if err != nil {
			return nil, err
		}
		if ok {
			discovered.Warnings = append(discovered.Warnings, group.Warnings...)
			bundles = append(bundles, discovered)
		}
	}
	return bundles, nil
}

func copyImportedHookFiles(dir string, files map[string]string) error {
	for srcPath, rel := range files {
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, "scripts", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		info, err := os.Stat(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, info.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func discoverImportedBundle(dir, name string) (Bundle, bool, error) {
	cfg, warnings, err := readHookConfig(filepath.Join(dir, "hook.yaml"))
	if err != nil {
		return Bundle{}, false, err
	}
	targets := map[string]int{}
	if cfg.Claude != nil {
		targets["claude"] = countEntries(cfg.Claude.Events)
	}
	if cfg.Codex != nil {
		targets["codex"] = countEntries(cfg.Codex.Events)
	}
	return Bundle{Name: name, SourceDir: dir, Config: cfg, Targets: targets, Warnings: warnings}, true, nil
}

func supportsClaudeTarget(section *TargetSection) bool {
	return section != nil && countEntries(section.Events) > 0
}

func supportsCodexTarget(section *TargetSection) bool {
	if section == nil {
		return false
	}
	for event, entries := range section.Events {
		if supportedCodexEvents[event] && len(entries) > 0 {
			return true
		}
	}
	return false
}

func defaultClaudeMatcher(event string) any {
	if claudeToolScopedEvents[event] {
		return "*"
	}
	return ""
}

func matcherSignature(matcher any) string {
	data, err := json.Marshal(cloneMatcher(matcher))
	if err != nil {
		return fmt.Sprintf("%v", matcher)
	}
	return string(data)
}

func cloneMatcher(matcher any) any {
	data, err := json.Marshal(matcher)
	if err != nil {
		return matcher
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return matcher
	}
	return out
}

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func mergeAnyMap(dst map[string]any, src map[string]any) {
	for key, value := range src {
		dst[key] = value
	}
}

func decodeJSONArrayOfMaps(raw any) []map[string]any {
	var items []map[string]any
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	_ = json.Unmarshal(data, &items)
	return items
}

func cloneHandlerSlice(src []map[string]any) []map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make([]map[string]any, 0, len(src))
	for _, item := range src {
		dst = append(dst, cloneAnyMap(item))
	}
	return dst
}

func isManagedCommandHandler(handler map[string]any, managedPrefix string) bool {
	command, _ := handler["command"].(string)
	return command != "" && strings.Contains(filepath.ToSlash(command), managedPrefix)
}

func anyStringMap(raw any) map[string]string {
	items, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(items))
	for key, value := range items {
		if str, ok := value.(string); ok {
			out[key] = str
		}
	}
	return out
}

func anyStringSlice(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		if typed, ok := raw.([]string); ok {
			return append([]string{}, typed...)
		}
		return nil
	}
	out := make([]string, 0, len(list))
	for _, value := range list {
		if str, ok := value.(string); ok {
			out = append(out, str)
		}
	}
	return out
}

func anyInt(raw any) int {
	switch value := raw.(type) {
	case float64:
		return int(value)
	case int:
		return value
	case int64:
		return int(value)
	case json.Number:
		i, _ := value.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(value)
		return i
	default:
		return 0
	}
}

func sortedKeys[V any](items map[string]V) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
