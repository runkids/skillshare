package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"skillshare/internal/config"
	"skillshare/internal/tooling"
)

type Bundle struct {
	Name             string   `json:"name"`
	SourceDir        string   `json:"source_dir"`
	HasClaude        bool     `json:"has_claude"`
	HasCodex         bool     `json:"has_codex"`
	GeneratedTargets []string `json:"generated_targets,omitempty"`
	RenderedTargets  []string `json:"rendered_targets,omitempty"`
	InstalledTargets []string `json:"installed_targets,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
	Issues           []string `json:"issues,omitempty"`
}

type ImportOptions struct {
	From string
}

type SyncResult struct {
	Name      string   `json:"name"`
	Target    string   `json:"target"`
	Rendered  string   `json:"rendered"`
	Installed bool     `json:"installed"`
	Generated bool     `json:"generated,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

type translationMeta struct {
	Shared map[string]any `yaml:"shared,omitempty"`
}

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
		bundle, ok, err := discoverBundle(dir, entry.Name())
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		out = append(out, bundle)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func discoverBundle(dir, fallbackName string) (Bundle, bool, error) {
	claudeManifest := filepath.Join(dir, ".claude-plugin", "plugin.json")
	codexManifest := filepath.Join(dir, ".codex-plugin", "plugin.json")
	metaPath := filepath.Join(dir, "skillshare.plugin.yaml")
	bundle := Bundle{
		Name:      fallbackName,
		SourceDir: dir,
		HasClaude: fileExists(claudeManifest),
		HasCodex:  fileExists(codexManifest),
	}
	if !bundle.HasClaude && !bundle.HasCodex && !fileExists(metaPath) {
		return Bundle{}, false, nil
	}
	if bundle.HasClaude {
		if name, err := manifestName(claudeManifest); err == nil && name != "" {
			bundle.Name = name
		}
	}
	if bundle.Name == fallbackName && bundle.HasCodex {
		if name, err := manifestName(codexManifest); err == nil && name != "" {
			bundle.Name = name
		}
	}
	if _, warnings, _, err := loadSharedMetadata(dir, bundle.Name); err == nil {
		bundle.Warnings = append(bundle.Warnings, warnings...)
	}
	bundle.GeneratedTargets = GeneratedTargets(bundle)
	return bundle, true, nil
}

func SupportedTargets(bundle Bundle) []string {
	var out []string
	if bundle.HasClaude || canGenerateTarget(bundle, "claude") {
		out = append(out, "claude")
	}
	if bundle.HasCodex || canGenerateTarget(bundle, "codex") {
		out = append(out, "codex")
	}
	return out
}

func GeneratedTargets(bundle Bundle) []string {
	var out []string
	if !bundle.HasClaude && canGenerateTarget(bundle, "claude") {
		out = append(out, "claude")
	}
	if !bundle.HasCodex && canGenerateTarget(bundle, "codex") {
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
		return filepath.Join(config.CodexMarketplaceRoot(projectRoot), name)
	}
	return filepath.Join(config.ClaudeMarketplaceRoot(projectRoot), "plugins", name)
}

func Import(sourceRoot, ref string, opts ImportOptions) (Bundle, error) {
	resolved, err := resolveImportSource(ref, opts.From)
	if err != nil {
		return Bundle{}, err
	}
	name, err := bundleNameFromSource(resolved, ref)
	if err != nil {
		return Bundle{}, err
	}
	dst := filepath.Join(sourceRoot, name)
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		return Bundle{}, err
	}
	if err := tooling.MergeDir(resolved, dst); err != nil {
		return Bundle{}, err
	}
	bundle, ok, err := discoverBundle(dst, name)
	if err != nil {
		return Bundle{}, err
	}
	if !ok {
		return Bundle{}, fmt.Errorf("imported plugin %q is missing native manifests and translation metadata", name)
	}
	return bundle, nil
}

func SyncAll(sourceRoot, projectRoot, target string, install bool) ([]SyncResult, error) {
	bundles, err := Discover(sourceRoot)
	if err != nil {
		return nil, err
	}
	var results []SyncResult
	for _, bundle := range bundles {
		for _, one := range expandTargets(target) {
			res, err := SyncBundle(bundle, projectRoot, one, install)
			if err != nil {
				return results, err
			}
			results = append(results, res)
		}
	}
	return results, nil
}

func SyncBundle(bundle Bundle, projectRoot, target string, install bool) (SyncResult, error) {
	if !SupportsTarget(bundle, target) {
		return SyncResult{}, fmt.Errorf("plugin %q cannot sync to %s", bundle.Name, target)
	}
	stagedDir, generated, warnings, err := stageBundle(bundle, target)
	if err != nil {
		return SyncResult{}, err
	}
	defer os.RemoveAll(stagedDir)

	switch target {
	case "claude":
		renderRoot := config.ClaudeMarketplaceRoot(projectRoot)
		renderedDir := RenderRoot(projectRoot, bundle.Name, target)
		if err := tooling.ReplaceDir(stagedDir, renderedDir); err != nil {
			return SyncResult{}, err
		}
		if err := writeClaudeMarketplace(renderRoot); err != nil {
			return SyncResult{}, err
		}
		if install {
			scope := "user"
			if projectRoot != "" {
				scope = "project"
			}
			if err := runClaudePluginFlow(bundle.Name, renderRoot, scope); err != nil {
				return SyncResult{}, err
			}
		}
		return SyncResult{Name: bundle.Name, Target: target, Rendered: renderedDir, Installed: install, Generated: generated, Warnings: warnings}, nil
	case "codex":
		renderRoot := config.CodexMarketplaceRoot(projectRoot)
		renderedDir := RenderRoot(projectRoot, bundle.Name, target)
		if err := tooling.ReplaceDir(stagedDir, renderedDir); err != nil {
			return SyncResult{}, err
		}
		if err := writeCodexMarketplace(renderRoot); err != nil {
			return SyncResult{}, err
		}
		if install {
			cacheDir := filepath.Join(config.CodexPluginCacheRoot(), bundle.Name, "local")
			if err := tooling.ReplaceDir(stagedDir, cacheDir); err != nil {
				return SyncResult{}, err
			}
			if err := enableCodexPlugin(bundle.Name); err != nil {
				return SyncResult{}, err
			}
		}
		return SyncResult{Name: bundle.Name, Target: target, Rendered: renderedDir, Installed: install, Generated: generated, Warnings: warnings}, nil
	default:
		return SyncResult{}, fmt.Errorf("unsupported plugin target %q", target)
	}
}

func stageBundle(bundle Bundle, target string) (string, bool, []string, error) {
	tempDir, err := os.MkdirTemp("", "skillshare-plugin-*")
	if err != nil {
		return "", false, nil, err
	}
	generated := false
	if err := copySharedFiles(bundle.SourceDir, tempDir); err != nil {
		os.RemoveAll(tempDir)
		return "", false, nil, err
	}
	switch target {
	case "claude":
		if bundle.HasClaude {
			if err := tooling.MergeDir(filepath.Join(bundle.SourceDir, ".claude-plugin"), filepath.Join(tempDir, ".claude-plugin")); err != nil {
				os.RemoveAll(tempDir)
				return "", false, nil, err
			}
		} else {
			warnings, err := generateManifest(tempDir, bundle.SourceDir, bundle.Name, target)
			if err != nil {
				os.RemoveAll(tempDir)
				return "", false, nil, err
			}
			generated = true
			return tempDir, generated, append([]string{"generated .claude-plugin/plugin.json"}, warnings...), nil
		}
	case "codex":
		if bundle.HasCodex {
			if err := tooling.MergeDir(filepath.Join(bundle.SourceDir, ".codex-plugin"), filepath.Join(tempDir, ".codex-plugin")); err != nil {
				os.RemoveAll(tempDir)
				return "", false, nil, err
			}
		} else {
			warnings, err := generateManifest(tempDir, bundle.SourceDir, bundle.Name, target)
			if err != nil {
				os.RemoveAll(tempDir)
				return "", false, nil, err
			}
			generated = true
			return tempDir, generated, append([]string{"generated .codex-plugin/plugin.json"}, warnings...), nil
		}
	default:
		os.RemoveAll(tempDir)
		return "", false, nil, fmt.Errorf("unsupported plugin target %q", target)
	}
	return tempDir, generated, nil, nil
}

func copySharedFiles(srcRoot, dstRoot string) error {
	entries, err := os.ReadDir(srcRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == ".claude-plugin" || name == ".codex-plugin" || name == "skillshare.plugin.yaml" {
			continue
		}
		src := filepath.Join(srcRoot, name)
		dst := filepath.Join(dstRoot, name)
		if entry.IsDir() {
			if err := tooling.MergeDir(src, dst); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func generateManifest(bundleDir, sourceDir, name, target string) ([]string, error) {
	meta, warnings, ok, err := loadSharedMetadata(sourceDir, name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("plugin %q cannot be synced to %s: no shared metadata available", name, target)
	}
	switch target {
	case "claude":
		if err := tooling.WriteJSON(filepath.Join(bundleDir, ".claude-plugin", "plugin.json"), meta); err != nil {
			return nil, err
		}
	case "codex":
		if err := tooling.WriteJSON(filepath.Join(bundleDir, ".codex-plugin", "plugin.json"), meta); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported plugin target %q", target)
	}
	return warnings, nil
}

func loadSharedMetadata(bundleDir, name string) (map[string]any, []string, bool, error) {
	var sourceManifest string
	switch {
	case fileExists(filepath.Join(bundleDir, ".claude-plugin", "plugin.json")):
		sourceManifest = filepath.Join(bundleDir, ".claude-plugin", "plugin.json")
	case fileExists(filepath.Join(bundleDir, ".codex-plugin", "plugin.json")):
		sourceManifest = filepath.Join(bundleDir, ".codex-plugin", "plugin.json")
	}
	meta := map[string]any{
		"name":    name,
		"version": "0.1.0",
	}
	if sourceManifest != "" {
		data, err := os.ReadFile(sourceManifest)
		if err != nil {
			return nil, nil, false, err
		}
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, nil, false, err
		}
	}
	if fileExists(filepath.Join(bundleDir, "skillshare.plugin.yaml")) {
		var sidecar translationMeta
		data, err := os.ReadFile(filepath.Join(bundleDir, "skillshare.plugin.yaml"))
		if err != nil {
			return nil, nil, false, err
		}
		if err := yaml.Unmarshal(data, &sidecar); err != nil {
			return nil, nil, false, err
		}
		for k, v := range sidecar.Shared {
			meta[k] = v
		}
	}
	ok := sourceManifest != "" || fileExists(filepath.Join(bundleDir, "skillshare.plugin.yaml"))
	if !ok {
		for _, sharedDir := range []string{"skills", "assets", "vendor"} {
			if dirExists(filepath.Join(bundleDir, sharedDir)) {
				ok = true
				break
			}
		}
	}
	var warnings []string
	for _, skipped := range []string{"commands", "agents", "hooks", ".lsp.json", "monitors", "bin", ".app.json"} {
		path := filepath.Join(bundleDir, skipped)
		if fileExists(path) || dirExists(path) {
			warnings = append(warnings, fmt.Sprintf("skipped %s during cross-target translation", skipped))
		}
	}
	return meta, warnings, ok, nil
}

func writeClaudeMarketplace(root string) error {
	pluginsDir := filepath.Join(root, "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return err
	}
	type pluginEntry struct {
		Name string `json:"name"`
		Ref  string `json:"ref"`
	}
	var plugins []pluginEntry
	for _, entry := range entries {
		if entry.IsDir() {
			plugins = append(plugins, pluginEntry{Name: entry.Name(), Ref: entry.Name() + "@skillshare"})
		}
	}
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Name < plugins[j].Name })
	return tooling.WriteJSON(filepath.Join(root, ".claude-plugin", "marketplace.json"), map[string]any{"plugins": plugins})
}

func writeCodexMarketplace(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	type pluginEntry struct {
		Name string `json:"name"`
		Ref  string `json:"ref"`
	}
	var plugins []pluginEntry
	for _, entry := range entries {
		if entry.IsDir() {
			plugins = append(plugins, pluginEntry{Name: entry.Name(), Ref: entry.Name() + "@skillshare"})
		}
	}
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Name < plugins[j].Name })
	return tooling.WriteJSON(filepath.Join(root, "marketplace.json"), map[string]any{"plugins": plugins})
}

func runClaudePluginFlow(name, renderRoot, scope string) error {
	verb := "install"
	if isClaudePluginInstalled(name + "@skillshare") {
		verb = "update"
	}
	cmds := [][]string{
		{"plugin", "marketplace", "add", renderRoot, "--scope", scope},
		{"plugin", verb, name + "@skillshare", "--scope", scope},
		{"plugin", "enable", name + "@skillshare"},
	}
	for _, args := range cmds {
		cmd := exec.Command("claude", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("claude %s failed: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}

func isClaudePluginInstalled(ref string) bool {
	installed, _ := readClaudeInstalledPlugins()
	if _, ok := installed[ref]; ok {
		return true
	}
	name := pluginRefName(ref)
	home, _ := os.UserHomeDir()
	for _, candidate := range []string{
		filepath.Join(home, ".claude", "plugins", name),
		filepath.Join(home, ".claude", "plugins", ref),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return true
		}
	}
	return false
}

func enableCodexPlugin(name string) error {
	cfgPath := config.CodexConfigPath()
	data, err := os.ReadFile(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(data)
	content = tooling.EnsureManagedTOMLBool(content, []string{"plugins", fmt.Sprintf("%q", name+"@skillshare")}, "enabled", true)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(cfgPath, []byte(content), 0o644)
}

func resolveImportSource(ref, from string) (string, error) {
	if info, err := os.Stat(ref); err == nil && info.IsDir() {
		return ref, nil
	}
	switch from {
	case "claude":
		return resolveClaudeImportPath(ref)
	case "codex":
		return resolveCodexImportPath(ref)
	default:
		return "", fmt.Errorf("plugin import requires --from claude|codex when %q is not a local directory", ref)
	}
}

func resolveClaudeImportPath(ref string) (string, error) {
	installed, _ := readClaudeInstalledPlugins()
	resolvedRef := ref
	if len(installed) > 0 {
		var err error
		resolvedRef, err = resolvePluginRef(ref, installed, "claude")
		if err != nil {
			return "", err
		}
	}
	if records, ok := installed[resolvedRef]; ok {
		for _, record := range records {
			if info, err := os.Stat(record.InstallPath); err == nil && info.IsDir() {
				return record.InstallPath, nil
			}
		}
	}
	name := pluginRefName(resolvedRef)
	home, _ := os.UserHomeDir()
	for _, candidate := range []string{
		filepath.Join(home, ".claude", "plugins", name),
		filepath.Join(home, ".claude", "plugins", resolvedRef),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("claude plugin %q not found in local state", ref)
}

func resolveCodexImportPath(ref string) (string, error) {
	installed, _ := discoverCodexInstalledPlugins()
	resolvedRef := ref
	if len(installed) > 0 {
		var err error
		resolvedRef, err = resolvePluginRef(ref, installed, "codex")
		if err != nil {
			return "", err
		}
	}
	if candidates, ok := installed[resolvedRef]; ok {
		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate, nil
			}
		}
	}
	name := pluginRefName(resolvedRef)
	for _, candidate := range []string{
		filepath.Join(config.CodexMarketplaceRoot(""), name),
		filepath.Join(config.CodexMarketplaceRoot(""), "plugins", name),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("codex plugin %q not found in local state", ref)
}

func bundleNameFromSource(sourceDir, ref string) (string, error) {
	for _, manifest := range []string{
		filepath.Join(sourceDir, ".claude-plugin", "plugin.json"),
		filepath.Join(sourceDir, ".codex-plugin", "plugin.json"),
	} {
		if fileExists(manifest) {
			name, err := manifestName(manifest)
			if err != nil {
				return "", err
			}
			if name != "" {
				return name, nil
			}
		}
	}
	return pluginRefName(filepath.Base(ref)), nil
}

func manifestName(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	if name, _ := payload["name"].(string); name != "" {
		return name, nil
	}
	return "", nil
}

func pluginRefName(ref string) string {
	base := filepath.Base(ref)
	if idx := strings.Index(base, "@"); idx >= 0 {
		return base[:idx]
	}
	return base
}

func expandTargets(target string) []string {
	if target == "" || target == "all" {
		return []string{"claude", "codex"}
	}
	return []string{target}
}

type claudeInstalledPluginRecord struct {
	InstallPath string `json:"installPath"`
}

func canGenerateTarget(bundle Bundle, target string) bool {
	_, _, ok, err := loadSharedMetadata(bundle.SourceDir, bundle.Name)
	return err == nil && ok
}

func readClaudeInstalledPlugins() (map[string][]claudeInstalledPluginRecord, error) {
	var payload struct {
		Plugins map[string][]claudeInstalledPluginRecord `json:"plugins"`
	}
	if err := tooling.ReadJSON(config.ClaudeInstalledPluginsPath(), &payload); err != nil {
		return nil, err
	}
	if payload.Plugins == nil {
		return map[string][]claudeInstalledPluginRecord{}, nil
	}
	return payload.Plugins, nil
}

func resolvePluginRef[T any](ref string, installed map[string][]T, ecosystem string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("%s plugin reference cannot be empty", ecosystem)
	}
	if strings.Contains(ref, "@") {
		if _, ok := installed[ref]; ok {
			return ref, nil
		}
		return ref, fmt.Errorf("%s plugin %q not found in local state", ecosystem, ref)
	}
	var matches []string
	for fullRef := range installed {
		if pluginRefName(fullRef) == ref {
			matches = append(matches, fullRef)
		}
	}
	sort.Strings(matches)
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("%s plugin %q not found in local state", ecosystem, ref)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("%s plugin %q is ambiguous; use full ref", ecosystem, ref)
	}
}

func discoverCodexInstalledPlugins() (map[string][]string, error) {
	refs := readCodexConfiguredPluginRefs()
	cacheBase := config.CodexPluginCacheBase()
	pattern := filepath.Join(cacheBase, "*", "*", "*")
	matches, _ := filepath.Glob(pattern)
	out := map[string][]string{}
	for _, dir := range matches {
		if !dirExists(dir) || !fileExists(filepath.Join(dir, ".codex-plugin", "plugin.json")) {
			continue
		}
		provider := filepath.Base(filepath.Dir(filepath.Dir(dir)))
		name := filepath.Base(filepath.Dir(dir))
		if filepath.Base(dir) == "local" {
			name = filepath.Base(filepath.Dir(dir))
			provider = "skillshare"
		}
		ref := name + "@" + provider
		out[ref] = append(out[ref], dir)
	}
	for _, ref := range refs {
		if _, ok := out[ref]; ok {
			continue
		}
		name, provider := splitPluginRef(ref)
		pattern := filepath.Join(cacheBase, provider, name, "*")
		candidates, _ := filepath.Glob(pattern)
		for _, candidate := range candidates {
			if dirExists(candidate) && fileExists(filepath.Join(candidate, ".codex-plugin", "plugin.json")) {
				out[ref] = append(out[ref], candidate)
			}
		}
	}
	return out, nil
}

func readCodexConfiguredPluginRefs() []string {
	data, err := os.ReadFile(config.CodexConfigPath())
	if err != nil {
		return nil
	}
	re := regexp.MustCompile(`(?m)^\[plugins\.(?:"([^"]+)"|'([^']+)')\]`)
	var refs []string
	for _, match := range re.FindAllStringSubmatch(string(data), -1) {
		ref := match[1]
		if ref == "" {
			ref = match[2]
		}
		if ref != "" {
			refs = append(refs, ref)
		}
	}
	return refs
}

func splitPluginRef(ref string) (string, string) {
	name := pluginRefName(ref)
	if idx := strings.Index(ref, "@"); idx >= 0 {
		return name, ref[idx+1:]
	}
	return name, ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
