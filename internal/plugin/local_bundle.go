package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gofrs/flock"

	"skillshare/internal/utils"
)

type cursorOwner struct {
	Config string `json:"config"`
	Digest string `json:"digest"`
}

// localRoot returns the plugin folder and the Agent folder (or project) it lives in.
func (s *Service) localRoot(target string) (string, string, error) {
	if target == "antigravity" && s.ProjectRoot != "" {
		hidden := filepath.Join(s.ProjectRoot, ".agents", "plugins")
		visible := filepath.Join(s.ProjectRoot, "_agents", "plugins")
		_, hErr := os.Lstat(hidden)
		_, vErr := os.Lstat(visible)
		if hErr == nil && vErr == nil {
			return "", "", fmt.Errorf("both .agents/plugins and _agents/plugins exist; consolidate before syncing")
		}
		if vErr == nil {
			return visible, s.ProjectRoot, nil
		}
		return hidden, s.ProjectRoot, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	if target == "antigravity" {
		return filepath.Join(home, ".gemini", "config", "plugins"), filepath.Join(home, ".gemini"), nil
	}
	return filepath.Join(home, ".cursor", "plugins", "local"), filepath.Join(home, ".cursor"), nil
}

// Native configuration and installation paths must not redirect writes elsewhere.
// root is the Agent folder or project the user chose, which may itself be reached
// through links (macOS /var, a linked home, CLAUDE_CONFIG_DIR); only path and the
// folders between it and root are checked.
func noSymlink(root, path string) error {
	root, path = filepath.Clean(root), filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("native path %s is outside %s", path, root)
	}
	if rel == "." {
		return nil
	}
	// Walk up exactly as many levels as rel has, rather than until p equals root:
	// Dir can spell root differently (a Windows UNC share root gains a trailing
	// separator), and an equality test would then never end.
	p := path
	for range strings.Split(rel, string(filepath.Separator)) {
		info, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && utils.IsLinkMode(p, info.Mode()) {
			return fmt.Errorf("refusing to modify symlinked native path: %s", p)
		}
		p = filepath.Dir(p)
	}
	return nil
}

func (s *Service) localInventory(target string) ([]Installed, string, error) {
	result := []Installed{}
	root, base, err := s.localRoot(target)
	if err != nil {
		return nil, "", err
	}
	if err = noSymlink(base, root); err != nil {
		return nil, "", err
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return result, hash(nil), nil
	}
	if err != nil {
		return nil, "", err
	}
	var fingerprint []byte
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		candidate, err := inspect(dir)
		if err != nil {
			return nil, "", fmt.Errorf("inspect local plugin %s: %w", entry.Name(), err)
		}
		if !slices.Contains(candidate.Targets, target) {
			continue
		}
		digest, err := treeDigest(dir)
		if err != nil {
			return nil, "", err
		}
		marker, _ := os.ReadFile(filepath.Join(root, ".skillshare-"+entry.Name()+".json"))
		fingerprint = append(fingerprint, []byte(entry.Name()+"\x00"+digest)...)
		fingerprint = append(fingerprint, marker...)
		// Only canonical directory names can be safely managed; other local installs
		// remain visible under their directory identity and cannot be imported.
		result = append(result, Installed{ID: entry.Name(), Name: candidate.Name, Version: candidate.Version, Scope: "user", Installed: true, Enabled: true})
	}
	return result, hash(fingerprint), nil
}

func (s *Service) applyLocal(c Change, b Binding, source string) error {
	target := c.Target
	root, base, err := s.localRoot(target)
	if err != nil {
		return err
	}
	dest := filepath.Join(root, b.ID)
	if err = noSymlink(base, dest); err != nil {
		return err
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(root, ".skillshare.lock"))
	if err = lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()
	marker := filepath.Join(root, ".skillshare-"+b.ID+".json")
	if err = noSymlink(base, marker); err != nil {
		return err
	}
	config, err := filepath.Abs(s.ConfigPath)
	if err != nil {
		return err
	}
	existed := false
	if _, err = os.Lstat(dest); err == nil {
		existed = true
		data, readErr := os.ReadFile(marker)
		var owner cursorOwner
		if readErr != nil || json.Unmarshal(data, &owner) != nil || owner.Config != config {
			return fmt.Errorf("Local plugin directory is not owned by this Skillshare configuration")
		}
		digest, err := treeDigest(dest)
		if err != nil {
			return err
		}
		if digest != owner.Digest {
			return fmt.Errorf("Local plugin files were edited locally; preserve those changes before syncing")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	remove := c.Action == "remove" || c.Action == "uninstall"
	if remove {
		if !existed {
			return nil
		}
		if err = os.RemoveAll(dest); err != nil {
			return err
		}
		return os.Remove(marker)
	}
	if source == "" {
		return fmt.Errorf("Local plugin requires a source")
	}
	temp, err := os.MkdirTemp(root, ".skillshare-copy-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if err = copyTree(source, temp); err != nil {
		return err
	}
	digest, err := treeDigest(temp)
	if err != nil {
		return err
	}
	backup := filepath.Join(root, ".skillshare-previous-"+b.ID)
	if _, err = os.Lstat(backup); err == nil {
		return fmt.Errorf("unfinished local plugin replacement at %s; inspect before retrying", backup)
	} else if !os.IsNotExist(err) {
		return err
	}
	if existed {
		if err = os.Rename(dest, backup); err != nil {
			return err
		}
	}
	if err = os.Rename(temp, dest); err != nil {
		if existed {
			_ = os.Rename(backup, dest)
		}
		return err
	}
	data, _ := json.Marshal(cursorOwner{Config: config, Digest: digest})
	if err = atomicNativeWrite(base, marker, data, 0600); err != nil {
		return fmt.Errorf("Plugin files copied but ownership recording failed; inspect %s before retrying: %w", dest, err)
	}
	if existed {
		return os.RemoveAll(backup)
	}
	return nil
}

func atomicNativeWrite(root, path string, data []byte, mode os.FileMode) error {
	if err := noSymlink(root, path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".skillshare-write-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
