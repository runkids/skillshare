package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"skillshare/internal/utils"
)

// SaveCandidate holds the exact serialized bytes and the fresh, expanded view
// decoded from them. Err mirrors the error returned by PrepareSave. Preparation
// validates decoding and expansion; callers apply their own semantic validation.
type SaveCandidate struct {
	Bytes    []byte
	Expanded *Config
	Err      error
	path     string
	private  bool
	check    func() error
	// then is written after this candidate; noop marks a candidate whose
	// files already hold Bytes, so Write does nothing.
	then *SaveCandidate
	noop bool
}

// Changed reports whether Write would change any file.
func (c *SaveCandidate) Changed() bool { return !c.noop }

// PrepareSave runs Save's serializer without changing the caller or filesystem.
// Existing comments, anchors and aliases participate in the effective result, so
// Expanded is decoded from Bytes rather than copied from next.
func PrepareSave(path string, next *Config) (*SaveCandidate, error) {
	c := &SaveCandidate{path: path}
	if next == nil {
		c.Err = fmt.Errorf("cannot prepare a nil config")
		return c, c.Err
	}
	data, err := serializeConfig(path, next)
	if err != nil {
		c.Err = err
		return c, c.Err
	}
	c.Bytes = data
	c.Expanded, c.Err = decodeConfig(path, data, false)
	return c, c.Err
}

func serializeConfig(path string, next *Config) ([]byte, error) {
	payload := next.withoutProjectTargets()
	if next.PreserveTildeOnSave {
		payload = payload.cloneForSave()
	}
	var value any = payload
	if payload.legacySkills.Kind != 0 {
		type plain Config
		value = struct {
			*plain `yaml:",inline"`
			Skills yaml.Node `yaml:"skills,omitempty"`
		}{(*plain)(payload), payload.legacySkills}
	}
	data, err := marshalConfigFile(path, value, schemaComment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	return data, nil
}

// Check refuses a stale local-project snapshot before any detach or write.
func (c *SaveCandidate) Check() error {
	if c.Err != nil {
		return c.Err
	}
	if c.check != nil {
		return c.check()
	}
	return nil
}

// Write atomically persists the already prepared Bytes without serializing again.
// Resolving the destination keeps a symlinked config linked to its shared file.
func (c *SaveCandidate) Write() error {
	if c.noop {
		return c.Check()
	}
	if err := c.write(); err != nil {
		return err
	}
	if c.then != nil {
		if err := c.then.write(); err != nil {
			return fmt.Errorf("saved %s, but not %s: %w", c.path, c.then.path, err)
		}
	}
	return nil
}

func (c *SaveCandidate) write() error {
	if err := c.Check(); err != nil {
		return err
	}
	dest, err := utils.ConfigWritePath(c.path, false)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}
	dir := filepath.Dir(dest)
	dirMode, fileMode := os.FileMode(0755), os.FileMode(0644)
	if c.private {
		dirMode, fileMode = 0700, 0600
	}
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	// Exclusive creation applies the process umask without changing global state.
	tmpPath := filepath.Join(dir, ".skillshare-config-"+rand.Text())
	tmp, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		return fmt.Errorf("failed to create config temporary file: %w", err)
	}
	defer os.Remove(tmp.Name())
	// Retain an existing config's permissions across atomic replacement. New
	// configs keep the creation mode above, including the process umask.
	if info, err := os.Stat(dest); err == nil && !c.private {
		if err := tmp.Chmod(info.Mode().Perm()); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("failed to set config permissions: %w", err)
		}
	}
	if _, err := tmp.Write(c.Bytes); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to sync config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close config: %w", err)
	}
	if err := c.Check(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}
