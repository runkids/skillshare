package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const AutoUpdateCheckEnv = "SKILLSHARE_AUTO_UPDATE_CHECK"

type UpdatesConfig struct {
	AutoCheck *bool `yaml:"auto_check,omitempty"`
}

// UpdatePolicy keeps the saved preference separate from its process-local override.
type UpdatePolicy struct {
	Enabled     bool `json:"enabled"`
	Configured  bool `json:"configured"`
	EnvOverride bool `json:"envOverride"`
}

func (c UpdatesConfig) Policy() UpdatePolicy {
	configured := c.AutoCheck == nil || *c.AutoCheck
	p := UpdatePolicy{Enabled: configured, Configured: configured}
	if value, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(AutoUpdateCheckEnv))); err == nil {
		p.Enabled, p.EnvOverride = value, true
	}
	return p
}

// ReadUpdatePolicy reads only the global preference, without migrations or source discovery.
func ReadUpdatePolicy() (UpdatePolicy, error) {
	var cfg struct {
		Updates UpdatesConfig `yaml:"updates"`
	}
	data, err := os.ReadFile(ReadOnlyConfigPath())
	if err != nil && !os.IsNotExist(err) {
		return UpdatePolicy{}, err
	}
	if err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return UpdatePolicy{}, err
		}
	}
	return cfg.Updates.Policy(), nil
}

// AutoUpdateCheckEnabled fails quiet when the config cannot be read.
func AutoUpdateCheckEnabled() bool {
	p, err := ReadUpdatePolicy()
	return err == nil && p.Enabled
}

func SetAutoUpdateCheck(enabled bool) error {
	path := ReadOnlyConfigPath()
	cfg, err := LoadReadOnlyPath(path)
	if err != nil {
		return err
	}
	cfg.Updates.AutoCheck = &enabled
	candidate, err := PrepareSave(path, cfg)
	if err != nil {
		return fmt.Errorf("failed to prepare config: %w", err)
	}
	return candidate.Write()
}
