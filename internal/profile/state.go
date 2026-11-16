package profile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// State is local to one configuration and never belongs in shared config.
type State struct {
	Version int    `yaml:"version"`
	Active  string `yaml:"active,omitempty"`
	Pending bool   `yaml:"pending,omitempty"`
}

func ReadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{Version: 1}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read profile state %s: %w", path, err)
	}
	var state State
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&state); err != nil {
		return State{}, fmt.Errorf("invalid profile state %s: %w", path, err)
	}
	if err := dec.Decode(new(yaml.Node)); err != io.EOF {
		return State{}, fmt.Errorf("profile state %s must contain one YAML document", path)
	}
	if state.Version != 1 {
		return State{}, fmt.Errorf("unsupported profile state version %d in %s", state.Version, path)
	}
	return state, nil
}

// WriteState atomically replaces the state after writing a private temporary file.
func WriteState(path string, state State) error {
	if state.Version != 1 {
		return fmt.Errorf("unsupported profile state version %d", state.Version)
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".profile-state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
