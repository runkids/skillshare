package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ConfigFileName is the conventional name for directory-level runbook config.
const ConfigFileName = "runbook.json"

// RunbookConfig holds lifecycle hooks and defaults for runbook execution.
type RunbookConfig struct {
	Build    string            `json:"build,omitempty"`    // command to run once before all runbooks
	Setup    string            `json:"setup,omitempty"`    // command to run before each runbook
	Teardown string            `json:"teardown,omitempty"` // command to run after each runbook
	Timeout  string            `json:"timeout,omitempty"`  // per-step timeout (e.g., "5m")
	Env      map[string]string `json:"env,omitempty"`      // environment variables seeded into all steps
}

// TimeoutDuration parses the timeout string into a time.Duration.
// Returns zero if empty or invalid.
func (c RunbookConfig) TimeoutDuration() time.Duration {
	if c.Timeout == "" {
		return 0
	}
	d, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return 0
	}
	return d
}

// loadConfig reads a runbook.json from the given directory.
// Returns an empty config (no error) if the file doesn't exist.
func loadConfig(dir string) (RunbookConfig, error) {
	path := filepath.Join(dir, ConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RunbookConfig{}, nil
		}
		return RunbookConfig{}, err
	}

	var cfg RunbookConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return RunbookConfig{}, err
	}
	return cfg, nil
}

// mergeConfig applies CLI flag overrides on top of file-based config.
// CLI flags take precedence when non-empty.
func mergeConfig(file RunbookConfig, cliBuild, cliSetup, cliTeardown string, cliTimeout time.Duration) RunbookConfig {
	merged := file
	if cliBuild != "" {
		merged.Build = cliBuild
	}
	if cliSetup != "" {
		merged.Setup = cliSetup
	}
	if cliTeardown != "" {
		merged.Teardown = cliTeardown
	}
	if cliTimeout != 0 {
		merged.Timeout = cliTimeout.String()
	}
	return merged
}
