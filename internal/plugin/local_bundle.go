package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gofrs/flock"
)

type cursorOwner struct {
	Config string `json:"config"`
	Digest string `json:"digest"`
}

func (s *Service) localRoot(target string) (string, error) {
	if target == "antigravity" && s.ProjectRoot != "" {
		hidden := filepath.Join(s.ProjectRoot, ".agents", "plugins")
		visible := filepath.Join(s.ProjectRoot, "_agents", "plugins")
		_, hErr := os.Lstat(hidden)
		_, vErr := os.Lstat(visible)
		if hErr == nil && vErr == nil {
			return "", fmt.Errorf("both .agents/plugins and _agents/plugins exist; consolidate before syncing")
		}
		if vErr == nil {
			return visible, nil
		}
		return hidden, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if target == "antigravity" {
		return filepath.Join(home, ".gemini", "config", "plugins"), nil
	}
	return filepath.Join(home, ".cursor", "plugins", "local"), nil
}

// Native configuration and installation paths must not redirect writes elsewhere.
func noSymlink(path string) error {
	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to modify symlinked native path: %s", p)
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
	}
	return nil
}

func (s *Service) localInventory(target string) ([]Installed, string, error) {
	result := []Installed{}
	root, err := s.localRoot(target)
	if err != nil {
		return nil, "", err
	}
	if err = noSymlink(root); err != nil {
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
	root, err := s.localRoot(target)
	if err != nil {
		return err
	}
	dest := filepath.Join(root, b.ID)
	if err = noSymlink(dest); err != nil {
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
	if err = noSymlink(marker); err != nil {
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
	if err = atomicNativeWrite(marker, data, 0600); err != nil {
		return fmt.Errorf("Plugin files copied but ownership recording failed; inspect %s before retrying: %w", dest, err)
	}
	if existed {
		return os.RemoveAll(backup)
	}
	return nil
}

func atomicNativeWrite(path string, data []byte, mode os.FileMode) error {
	if err := noSymlink(path); err != nil {
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
