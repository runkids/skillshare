package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func (s *Service) materialize(ctx context.Context, b Binding, target string) (string, error) {
	expected := s.snapshotPath(b, target)
	market := filepath.Base(expected)
	if !strings.HasPrefix(market, "skillshare-") {
		return "", fmt.Errorf("invalid managed snapshot location; re-add the source")
	}
	if recorded, err := os.ReadFile(filepath.Join(expected, ".skillshare-digest")); err == nil {
		owner, err := os.ReadFile(filepath.Join(expected, ".skillshare-owner"))
		if err != nil || string(owner) != b.Source+"\n"+b.Plugin {
			return "", fmt.Errorf("snapshot ownership conflict")
		}
		actual, err := treeDigest(filepath.Join(expected, "content"))
		if err != nil || actual != string(recorded) {
			return "", fmt.Errorf("managed plugin snapshot was modified; preserve your edits before retrying")
		}
		if actual == b.Digest {
			return expected, nil
		}
	}
	ref := b.Commit
	if ref == "" {
		ref = b.SourceRef
	}
	root, normalized, cleanup, err := acquireRef(ctx, b.Source, ref)
	if err != nil {
		return "", err
	}
	defer cleanup()
	d, err := discoverRoot(root, normalized, b.Entry)
	if err != nil {
		return "", err
	}
	if d.Digest != b.Digest {
		return "", fmt.Errorf("plugin source changed; preview an update before synchronizing")
	}
	var candidate *Candidate
	for _, c := range d.Candidates {
		if c.Name == b.Plugin {
			cc := c
			candidate = &cc
		}
	}
	if candidate == nil || candidate.Problem != "" {
		return "", fmt.Errorf("plugin no longer available in this source")
	}
	parent := filepath.Dir(expected)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", err
	}
	temp, err := os.MkdirTemp(parent, ".snapshot-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)
	if err := copyTree(root, filepath.Join(temp, "content")); err != nil {
		return "", err
	}
	// Catch local edits during the copy. Hash the copy before adding our catalog.
	copied, err := treeDigest(filepath.Join(temp, "content"))
	if err != nil {
		return "", err
	}
	if copied != d.Digest {
		return "", fmt.Errorf("source changed while copying; preview again")
	}
	agent := s.agentOf(target)
	rel := "./content"
	if p := candidate.pathFor(agent); p != "." {
		rel += "/" + p
	}
	catalog := map[string]any{"name": market, "owner": map[string]string{"name": "skillshare"}, "plugins": []any{pluginEntry(*candidate, rel, agent)}}
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return "", err
	}
	for _, path := range []string{".claude-plugin/marketplace.json", ".agents/plugins/marketplace.json"} {
		p := filepath.Join(temp, path)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(p, data, 0644); err != nil {
			return "", err
		}
	}
	// Only replace directories owned by this source. Never delete unknown content.
	marker := []byte(b.Source + "\n" + b.Plugin)
	if existing, err := os.ReadFile(filepath.Join(expected, ".skillshare-owner")); err == nil {
		if string(existing) != string(marker) {
			return "", fmt.Errorf("snapshot ownership conflict")
		}
	} else if !os.IsNotExist(err) {
		return "", err
	} else if _, err := os.Stat(expected); err == nil {
		return "", fmt.Errorf("unowned snapshot directory")
	}
	if err := os.WriteFile(filepath.Join(temp, ".skillshare-owner"), marker, 0600); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(temp, ".skillshare-digest"), []byte(b.Digest), 0600); err != nil {
		return "", err
	}
	backup := expected + ".previous"
	if _, err := os.Stat(backup); err == nil {
		return "", fmt.Errorf("unfinished snapshot replacement at %s; inspect before retrying", backup)
	}
	hadOld := false
	if _, err := os.Stat(expected); err == nil {
		if err := os.Rename(expected, backup); err != nil {
			return "", err
		}
		hadOld = true
	}
	if err := os.Rename(temp, expected); err != nil {
		if hadOld {
			_ = os.Rename(backup, expected)
		}
		return "", err
	}
	// Keep the previous snapshot until the native operation succeeds.
	return expected, nil
}

func (s *Service) applyChange(ctx context.Context, c Change, b Binding) (resultErr error) {
	if b.Source != "" && (c.Action == "install" || c.Action == "update") {
		if _, err := os.Stat(s.snapshotPath(b, c.Target) + ".previous"); err == nil {
			return fmt.Errorf("unfinished snapshot replacement; inspect previous snapshot before retrying")
		}
		defer func() {
			path := s.snapshotPath(b, c.Target)
			backup := path + ".previous"
			if _, err := os.Stat(backup); err != nil {
				return
			}
			if resultErr == nil {
				_ = os.RemoveAll(backup)
				return
			}
			failed := path + ".failed"
			if _, err := os.Stat(failed); err == nil {
				return
			}
			if err := os.Rename(path, failed); err != nil {
				return
			}
			if err := os.Rename(backup, path); err != nil {
				_ = os.Rename(failed, path)
				return
			}
			_ = os.RemoveAll(failed)
		}()
	}
	if c.ompRemoval != nil && s.agentOf(c.Target) == "omp" {
		if err := s.applyOMPRemoval(c.ompRemoval); err != nil {
			return agentError{cause: err, key: "plugins.error.ompRemovalFailed", message: "OMP removal did not complete; shared cache was retained. Preview again before retrying: " + err.Error()}
		}
		return nil
	}
	if c.Action == "import" || c.Action == "forget" || c.Action == "selection" {
		return nil
	}
	agent := s.agentOf(c.Target)
	if !marketplaceAgent(agent) {
		return s.applyAdditional(ctx, c, b)
	}
	if agent == "omp" && c.Action == "install" {
		if err := ompEmptyRuntime(c.ompRuntimePath); err != nil {
			return agentError{cause: err, key: "plugins.error.ompRuntimeSafety", message: err.Error()}
		}
	}
	if (c.Action == "install" || c.Action == "update") && b.Source != "" {
		path, err := s.materialize(ctx, b, c.Target)
		if err != nil {
			return err
		}
		if c.Action == "install" || c.Action == "update" {
			// Re-adding our own marketplace is unnecessary on retry, and an update needs it back
			// if it went missing. Query native registration first, and refuse a same-name
			// registration owned elsewhere.
			registered, err := s.registered(ctx, c.Target, b.ID, path)
			if err != nil {
				return err
			}
			if !registered {
				// omp keeps one marketplace registry for both scopes and prints no JSON.
				args := []string{"plugin", "marketplace", "add", path}
				if agent == "claude" {
					scope := "user"
					if s.ProjectRoot != "" {
						scope = "project"
					}
					args = append(args, "--scope", scope)
				} else if agent == "codex" {
					args = append(args, "--json")
				}
				if _, err := s.run(ctx, c.Target, args...); err != nil {
					return err
				}
			}
		}
		// Codex reads a local marketplace in place; its marketplace upgrade is for Git ones.
		// omp installs from the catalog it cached when the marketplace was added.
		if c.Action == "update" && (agent == "claude" || agent == "omp") {
			_, market, _ := strings.Cut(b.ID, "@")
			if _, err := s.run(ctx, c.Target, "plugin", "marketplace", "update", market); err != nil {
				return err
			}
		}
	}
	if (c.Action == "remove" || c.Action == "uninstall") && b.Source != "" {
		// Removed natively already: only the marketplace Skillshare registered is left.
		if h := s.host(ctx, c.Target); h.Error == "" && !slices.ContainsFunc(h.Installed, func(i Installed) bool { return i.ID == b.ID }) {
			return s.removeMarketplace(ctx, c.Target, b)
		}
	}
	args, err := s.nativeArgs(c.Target, c.Action, b.ID)
	upgrade := agent == "codex" && c.Action == "update" && b.Source == ""
	if upgrade {
		_, market, _ := strings.Cut(b.ID, "@")
		args, err = []string{"plugin", "marketplace", "upgrade", market}, nil
	}
	if err != nil {
		return err
	}
	if agent == "omp" && c.Action == "install" {
		if err := ompEmptyRuntime(c.ompRuntimePath); err != nil {
			return agentError{cause: err, key: "plugins.error.ompRuntimeSafety", message: err.Error()}
		}
	}
	if _, err := s.run(ctx, c.Target, args...); err != nil {
		return err
	}
	h := s.host(ctx, c.Target)
	if h.Error != "" {
		return fmt.Errorf("native operation completed but verification failed: %s", h.Error)
	}
	for _, item := range h.Installed {
		if item.ID == b.ID {
			if c.Action == "remove" || c.Action == "uninstall" {
				return fmt.Errorf("native plugin is still installed")
			}
			if c.Action == "enable" && !item.Enabled {
				return fmt.Errorf("plugin is still disabled")
			}
			if c.Action == "disable" && item.Enabled {
				return fmt.Errorf("plugin is still enabled")
			}
			// An upgraded marketplace sets its own version, which Skillshare never reviewed.
			if c.Action == "update" && !upgrade && b.Version != "" && item.Version != b.Version {
				return fmt.Errorf("native update returned version %s; expected %s", item.Version, b.Version)
			}
			return nil
		}
	}
	if c.Action == "remove" || c.Action == "uninstall" {
		return s.removeMarketplace(ctx, c.Target, b)
	}
	return fmt.Errorf("native operation completed but plugin was not found; inspect the native client before retrying")
}

// removeMarketplace drops the marketplace Skillshare registered for one managed plugin; no
// other plugin uses it. An imported plugin's marketplace is not Skillshare's to remove.
func (s *Service) removeMarketplace(ctx context.Context, target string, b Binding) error {
	path := s.snapshotPath(b, target)
	market := filepath.Base(path)
	// omp's marketplace registry and plugin cache are shared by its user scope and every
	// project; what else refers to this marketplace cannot be seen from here, so it stays.
	if b.Source == "" || !strings.HasPrefix(market, "skillshare-") || s.agentOf(target) == "omp" {
		return nil
	}
	kept := func(cause error) error {
		return agentError{cause: cause, key: "plugins.error.marketplaceCleanup", message: fmt.Sprintf("native plugin removed, but Skillshare could not finish removing its marketplace %s; check it in the native client, then sync again: %v", market, cause), args: map[string]string{"market": market}}
	}
	registered, err := s.registered(ctx, target, b.ID, path)
	if err != nil {
		return kept(err)
	}
	if !registered {
		return nil
	}
	// No --scope: Claude refuses a scoped removal when only known_marketplaces.json records the
	// marketplace, for example after synced settings dropped its declaration. The name is unique
	// to this binding and its root was checked above, so every scope's copy is Skillshare's.
	if _, err := s.run(ctx, target, "plugin", "marketplace", "remove", market); err != nil {
		return kept(err)
	}
	// List again: a declaration the native client could not remove survives.
	if registered, err = s.registered(ctx, target, b.ID, path); err == nil && registered {
		err = fmt.Errorf("it is still registered, possibly in another settings scope")
	}
	if err != nil {
		return kept(err)
	}
	return nil
}

func (s *Service) registered(ctx context.Context, target, id, path string) (bool, error) {
	markets, err := s.marketplaces(ctx, target)
	if err != nil {
		return false, err
	}
	_, market, _ := strings.Cut(id, "@")
	root, ok := markets[market]
	if !ok {
		return false, nil
	}
	if root == "" {
		return false, fmt.Errorf("marketplace name is registered at several paths; resolve it in %s", target)
	}
	if filepath.Clean(root) != filepath.Clean(path) {
		return false, fmt.Errorf("marketplace name already registered at another path; resolve it in %s", target)
	}
	return true, nil
}

// marketplaces maps each marketplace the Agent knows to its root. Claude merges every
// settings scope into one list without saying which scope declared an entry; a name
// declared at several roots maps to "", since removing it would remove every copy.
func (s *Service) marketplaces(ctx context.Context, target string) (map[string]string, error) {
	agent := s.agentOf(target)
	if agent == "omp" {
		data, err := s.run(ctx, target, "plugin", "marketplace", "list")
		if err != nil {
			return nil, err
		}
		return parseOMPMarketplaces(data)
	}
	data, err := s.run(ctx, target, "plugin", "marketplace", "list", "--json")
	if err != nil {
		return nil, err
	}
	type entry struct {
		Name            string `json:"name"`
		Root            string `json:"root"`
		InstallLocation string `json:"installLocation"`
		Source          string `json:"source"`
	}
	var entries []entry
	if agent == "codex" {
		var envelope struct {
			Marketplaces json.RawMessage `json:"marketplaces"`
		}
		if json.Unmarshal(data, &envelope) != nil || len(envelope.Marketplaces) == 0 {
			return nil, fmt.Errorf("unrecognized native marketplace list")
		}
		err = json.Unmarshal(envelope.Marketplaces, &entries)
	} else {
		err = json.Unmarshal(data, &entries)
	}
	if err != nil || entries == nil {
		return nil, fmt.Errorf("unrecognized native marketplace list")
	}
	markets := map[string]string{}
	for _, e := range entries {
		root := e.Root
		if agent == "claude" {
			root = e.InstallLocation
		}
		if seen, ok := markets[e.Name]; ok && filepath.Clean(seen) != filepath.Clean(root) {
			root = ""
		}
		markets[e.Name] = root
	}
	return markets, nil
}

// pluginEntry is the one plugin in the catalog Skillshare writes next to a snapshot. For Claude,
// an entry that defines its own components (strict: false, skills, lspServers, ...) keeps them.
func pluginEntry(c Candidate, source, target string) map[string]any {
	entry := map[string]any{"name": c.Name, "source": source, "policy": map[string]string{"installation": "AVAILABLE", "authentication": "ON_INSTALL"}, "category": "Productivity"}
	if target != "claude" {
		return entry
	}
	for key, raw := range c.catalogEntry {
		entry[key] = raw
	}
	return entry
}
