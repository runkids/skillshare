package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func (s *Service) codexHome(target string) (string, error) {
	dir := os.Getenv("CODEX_HOME")
	if account, ok := s.account(target); ok {
		dir = account.Dir
	}
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".codex")
	}
	return filepath.Abs(dir)
}

type codexNativeConfig struct {
	Marketplaces map[string]struct {
		SourceType string   `toml:"source_type"`
		Source     string   `toml:"source"`
		Ref        string   `toml:"ref_name"`
		Sparse     []string `toml:"sparse_paths"`
	} `toml:"marketplaces"`
	Plugins map[string]struct {
		Enabled *bool `toml:"enabled"`
	} `toml:"plugins"`
}

func readCodexConfig(home string) ([]byte, codexNativeConfig, error) {
	var cfg codexNativeConfig
	path := filepath.Join(home, "config.toml")
	if err := codexPlainPath(home, path); err != nil {
		return nil, cfg, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, cfg, err
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, cfg, fmt.Errorf("cannot read native Codex configuration")
	}
	return data, cfg, nil
}

// Refuse filesystem indirection before backing up or restoring native state.
// The explicitly selected home may itself be a symlink; its descendants may not.
func codexPlainPath(home, path string) error {
	rel, err := filepath.Rel(home, path)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("Codex state path escapes its configuration directory")
	}
	current := home
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("Codex state contains a symlink; resolve it in the native client before updating")
		}
	}
	return nil
}

func (s *Service) codexMarketplace(ctx context.Context, target, id string) (string, error) {
	data, err := s.run(ctx, target, "plugin", "marketplace", "list", "--json")
	if err != nil {
		return "", err
	}
	var response struct {
		Marketplaces []struct {
			Name string `json:"name"`
			Root string `json:"root"`
		} `json:"marketplaces"`
	}
	if json.Unmarshal(data, &response) != nil || response.Marketplaces == nil {
		return "", fmt.Errorf("unrecognized Codex marketplace list schema")
	}
	_, market, _ := strings.Cut(id, "@")
	root := ""
	for _, entry := range response.Marketplaces {
		if entry.Name != market {
			continue
		}
		if root != "" || !filepath.IsAbs(entry.Root) {
			return "", fmt.Errorf("ambiguous Codex marketplace registration")
		}
		root = filepath.Clean(entry.Root)
	}
	if root == "" {
		return "", fmt.Errorf("Codex marketplace is not registered; resolve it in Codex")
	}
	return root, nil
}

func codexCandidate(d *Discovery, name string) (*Candidate, error) {
	for _, c := range d.Candidates {
		if c.Name == name && c.Problem == "" && slices.Contains(c.Targets, "codex") {
			if c.TargetInfo["codex"].Version == "" {
				return nil, fmt.Errorf("Codex update requires a versioned plugin manifest")
			}
			return &c, nil
		}
	}
	return nil, fmt.Errorf("updated source has no compatible Codex plugin; install it in Codex to resolve its native source")
}

// A native Git catalogue carries its own metadata marker, absent in a source checkout.
// Hash the reviewed plugin content independently of that native bookkeeping.
func codexContentDigest(root string) (string, error) {
	temp, err := os.MkdirTemp("", "skillshare-codex-content-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)
	copy := filepath.Join(temp, "content")
	if err := copyTree(root, copy); err != nil {
		return "", err
	}
	if err := os.Remove(filepath.Join(copy, ".codex-marketplace-install.json")); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return treeDigest(copy)
}

func codexCatalogDigest(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, ".agents/plugins/marketplace.json"))
	if err != nil {
		return "", fmt.Errorf("Codex update requires its native marketplace catalog")
	}
	return hash(data), nil
}

func (s *Service) previewCodexUpdate(ctx context.Context, c *Change, installed Installed, override string) error {
	if !installed.Installed || !installed.EnabledKnown || installed.Version == "" {
		return fmt.Errorf("Codex update requires a verified installed version and enabled state")
	}
	home, err := s.codexHome(c.Target)
	if err != nil {
		return err
	}
	data, cfg, err := readCodexConfig(home)
	if err != nil {
		return err
	}
	setting, ok := cfg.Plugins[c.ID]
	if !ok || setting.Enabled == nil || *setting.Enabled != installed.Enabled {
		return fmt.Errorf("Codex enabled state differs from its user configuration; resolve overrides in Codex before updating")
	}
	if _, err := codexEnabledConfig(data, c.ID); err != nil {
		return err
	}
	name, market, _ := strings.Cut(c.ID, "@")
	root, err := s.codexMarketplace(ctx, c.Target, c.ID)
	if err != nil {
		return err
	}
	u := &CodexUpdate{OldVersion: installed.Version, Enabled: installed.Enabled, MarketplaceRoot: root, ConfigDigest: hash(data), Affected: []string{}, Operations: [][]string{}}
	if c.Binding.Source == "" {
		if override != "" {
			return fmt.Errorf("imported Codex marketplace refs belong to native configuration; change the ref in Codex")
		}
		m, ok := cfg.Marketplaces[market]
		if !ok {
			return fmt.Errorf("Codex marketplace source is not in the selected user configuration")
		}
		if m.SourceType != "local" && m.SourceType != "git" {
			return fmt.Errorf("Codex marketplace source cannot be reviewed; update it in the native client")
		}
		u.Source, u.SourceRef, u.Refresh = m.Source, m.Ref, m.SourceType == "git"
		if u.Refresh {
			expected := filepath.Join(home, ".tmp/marketplaces", market)
			if root != expected {
				return fmt.Errorf("Codex Git marketplace uses an unsupported native snapshot location")
			}
			if err := codexPlainPath(home, root); err != nil {
				return err
			}
			u.MarketplaceDigest, err = codexStateDigest(root)
			if err != nil {
				return err
			}
			u.Operations = append(u.Operations, []string{"plugin", "marketplace", "upgrade", market, "--json"})
		} else if filepath.Clean(m.Source) != root {
			return fmt.Errorf("Codex marketplace source differs from its registration")
		}
	} else {
		u.Source, u.SourceRef = c.Binding.Source, c.Binding.SourceRef
		if root != s.snapshotPath(c.Binding, c.Target) {
			return fmt.Errorf("managed Codex marketplace registration points to another source")
		}
	}
	ref := u.SourceRef
	if c.Binding.Source != "" && c.Binding.Commit != "" {
		ref = c.Binding.Commit
	}
	source, normalized, cleanup, err := acquireCodexSource(ctx, u.Source, ref, u.Refresh)
	if err != nil {
		return err
	}
	defer cleanup()
	d, err := discoverRoot(source, normalized)
	if err != nil {
		return err
	}
	candidate, err := codexCandidate(d, name)
	if err != nil {
		return err
	}
	if c.Binding.Source == "" && candidate.Marketplace != market {
		return fmt.Errorf("native marketplace identity changed in the source")
	}
	u.PluginPath = candidate.pathFor("codex")
	u.ContentDigest, err = codexContentDigest(filepath.Join(source, filepath.FromSlash(u.PluginPath)))
	if err != nil {
		return err
	}
	if c.Binding.Source == "" {
		u.CatalogDigest, err = codexCatalogDigest(source)
		if err != nil {
			return err
		}
	}
	if u.Refresh || strings.HasPrefix(normalized, "https://") {
		commit, err := runCommand(ctx, source, nil, "git", "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		u.Commit = strings.TrimSpace(string(commit))
	}
	cache := filepath.Join(home, "plugins/cache", market)
	if err := codexPlainPath(home, cache); err != nil {
		return err
	}
	version := installed.Version
	if filepath.Base(version) != version || version == "." || version == ".." {
		return fmt.Errorf("native Codex cache is missing or unsupported; reinstall the plugin in Codex")
	}
	if info, err := os.Lstat(filepath.Join(cache, name, version)); err != nil || !info.IsDir() {
		return fmt.Errorf("native Codex cache is missing or unsupported; reinstall the plugin in Codex")
	}
	u.CacheDigest, err = codexStateDigest(cache)
	if err != nil {
		return fmt.Errorf("cannot verify installed Codex cache: %w", err)
	}
	for id := range cfg.Plugins {
		_, m, valid := strings.Cut(id, "@")
		if valid && m == market {
			u.Affected = append(u.Affected, id)
		}
	}
	slices.Sort(u.Affected)
	// Expose the supported command and the private reviewed-source override used at apply.
	marketKey, _ := json.Marshal(market)
	u.Operations = append(u.Operations, []string{"plugin", "add", c.ID, "--json", "-c", "marketplaces." + string(marketKey) + ".source_type=\"local\"", "-c", "marketplaces." + string(marketKey) + ".source=\"<reviewed-catalog>\""})
	c.CodexUpdate = u
	previousVersion := c.Binding.Version
	previousDigest := c.Binding.Digest
	c.Binding.Version = candidate.TargetInfo["codex"].Version
	if c.Binding.Source == "" {
		c.Binding.Digest = u.ContentDigest
		c.Binding.Commit = u.Commit
	}
	c.Components = candidate.TargetInfo["codex"].Components
	if c.Action == "native-check" {
		c.Action = "update-available"
		if installed.Version == c.Binding.Version && previousDigest == u.ContentDigest {
			c.Action = "noop"
		}
	}
	if c.Action == "update" && c.Binding.Pending == "" && c.Binding.Source == "" && previousVersion == c.Binding.Version && installed.Version == c.Binding.Version && previousDigest == u.ContentDigest {
		c.Action = "noop"
	}
	if c.Action == "update" {
		if err := s.verifyCodexUpdateCommands(ctx, c.Target, u.Refresh); err != nil {
			return err
		}
	}
	commands := []string{}
	for _, args := range u.Operations {
		commands = append(commands, "codex "+strings.Join(args, " "))
	}
	c.Message = fmt.Sprintf("%s -> %s; preserve enabled=%t. %s", installed.Version, c.Binding.Version, installed.Enabled, strings.Join(commands, "; "))
	if u.Refresh {
		c.Message += ". Native marketplace refresh may reinstall " + strings.Join(u.Affected, ", ") + "; restore unselected caches before completing."
	}
	return nil
}

// Native Git registrations can use a local Git directory. Clone it read-only
// just like an HTTPS source so the approved revision is immutable during apply.
func acquireCodexSource(ctx context.Context, source, ref string, git bool) (string, string, func(), error) {
	if !git {
		return acquireRef(ctx, source, ref)
	}
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		return acquireRef(ctx, source, ref)
	}
	cleanup := func() {}
	if strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, "\x00\r\n ") {
		return "", "", cleanup, fmt.Errorf("invalid native Git ref")
	}
	root, err := os.MkdirTemp("", "skillshare-codex-git-")
	if err != nil {
		return "", "", cleanup, err
	}
	cleanup = func() { _ = os.RemoveAll(root) }
	if _, err = runCommand(ctx, "", nil, "git", "-c", "core.hooksPath=/dev/null", "clone", "--no-hardlinks", "--", source, root); err == nil && ref != "" {
		_, err = runCommand(ctx, root, nil, "git", "-c", "core.hooksPath=/dev/null", "checkout", "--detach", ref)
	}
	if err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("resolve native Git source: %w", err)
	}
	return root, source, cleanup, nil
}

func (s *Service) verifyCodexUpdateCommands(ctx context.Context, target string, refresh bool) error {
	add, err := s.run(ctx, target, "plugin", "add", "--help")
	if err != nil {
		return err
	}
	if !bytes.Contains(add, []byte("--json")) || (!bytes.Contains(add, []byte("--config")) && !bytes.Contains(add, []byte("-c"))) {
		return fmt.Errorf("installed Codex does not expose verified JSON reinstall/configuration options; upgrade the native CLI")
	}
	if refresh {
		help, err := s.run(ctx, target, "plugin", "marketplace", "upgrade", "--help")
		if err != nil {
			return err
		}
		if !bytes.Contains(help, []byte("--json")) {
			return fmt.Errorf("installed Codex does not expose verified JSON marketplace upgrade; upgrade the native CLI")
		}
	}
	return nil
}
