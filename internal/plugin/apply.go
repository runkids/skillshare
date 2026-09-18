package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	rel := "./content"
	if candidate.Path != "." {
		rel += "/" + candidate.Path
	}
	catalog := map[string]any{"name": market, "owner": map[string]string{"name": "skillshare"}, "plugins": []any{map[string]any{"name": candidate.Name, "source": rel, "policy": map[string]string{"installation": "AVAILABLE", "authentication": "ON_INSTALL"}, "category": "Productivity"}}}
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
	if c.Action == "import" || c.Action == "forget" || c.Action == "selection" {
		return nil
	}
	if c.Target != "claude" && c.Target != "codex" {
		return s.applyAdditional(ctx, c, b)
	}
	if (c.Action == "install" || c.Action == "update") && b.Source != "" {
		path, err := s.materialize(ctx, b, c.Target)
		if err != nil {
			return err
		}
		if c.Action == "install" {
			// Re-adding our own marketplace is unnecessary on retry. Query native
			// registration first, and refuse a same-name registration owned elsewhere.
			registered, err := s.registered(ctx, c.Target, b.ID, path)
			if err != nil {
				return err
			}
			if !registered {
				args := []string{"plugin", "marketplace", "add", path}
				if c.Target == "claude" {
					scope := "user"
					if s.ProjectRoot != "" {
						scope = "project"
					}
					args = append(args, "--scope", scope)
				} else {
					args = append(args, "--json")
				}
				if _, err := s.run(ctx, c.Target, args...); err != nil {
					return err
				}
			}
		}
		if c.Action == "update" {
			_, market, _ := strings.Cut(b.ID, "@")
			if _, err := s.run(ctx, c.Target, "plugin", "marketplace", "update", market); err != nil {
				return err
			}
		}
	}
	args, err := s.nativeArgs(c.Target, c.Action, b.ID)
	if err != nil {
		return err
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
			if c.Action == "update" && b.Version != "" && item.Version != b.Version {
				return fmt.Errorf("native update returned version %s; expected %s", item.Version, b.Version)
			}
			return nil
		}
	}
	if c.Action == "remove" || c.Action == "uninstall" {
		return nil
	}
	return fmt.Errorf("native operation completed but plugin was not found; inspect the native client before retrying")
}

func (s *Service) registered(ctx context.Context, target, id, path string) (bool, error) {
	data, err := s.run(ctx, target, "plugin", "marketplace", "list", "--json")
	if err != nil {
		return false, err
	}
	type entry struct {
		Name            string `json:"name"`
		Root            string `json:"root"`
		InstallLocation string `json:"installLocation"`
		Source          string `json:"source"`
	}
	var entries []entry
	if target == "codex" {
		var envelope struct {
			Marketplaces json.RawMessage `json:"marketplaces"`
		}
		if json.Unmarshal(data, &envelope) != nil || len(envelope.Marketplaces) == 0 {
			return false, fmt.Errorf("unrecognized native marketplace list")
		}
		err = json.Unmarshal(envelope.Marketplaces, &entries)
	} else {
		err = json.Unmarshal(data, &entries)
	}
	if err != nil || entries == nil {
		return false, fmt.Errorf("unrecognized native marketplace list")
	}
	_, market, _ := strings.Cut(id, "@")
	for _, e := range entries {
		if e.Name == market {
			root := e.Root
			if target == "claude" {
				root = e.InstallLocation
			}
			if filepath.Clean(root) != filepath.Clean(path) {
				return false, fmt.Errorf("marketplace name already registered at another path; resolve it in %s", target)
			}
			return true, nil
		}
	}
	return false, nil
}
