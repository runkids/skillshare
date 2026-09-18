package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// These clients do not yet expose a verified non-interactive lifecycle contract.
// Discovery still reports their native manifests without promising installation.
func automationProblem(target string) string {
	switch target {
	case "kimi":
		return "Kimi plugin discovery only: install with /plugins in Kimi; a non-interactive lifecycle adapter is not yet verified."
	case "hermes":
		return "Hermes plugin discovery only: native profile inventory and capability-consent handling are not yet verified. Use hermes plugins."
	case "devin":
		return "Devin plugin discovery only: local inventory, trust, and personal-cloud removal are not yet verified for automation. Use devin plugins."
	}
	return ""
}

func commandTarget(target string) bool {
	return target == "copilot" || target == "antigravity-cli" || target == "grok"
}

func (s *Service) commandHost(ctx context.Context, target string) Host {
	h := Host{Target: target, Status: HostReady, Installed: []Installed{}, NoteKey: "plugins.note.native", Note: "Plugin registrations only; resource loading and native trust are verified in the Agent."}
	if s.ProjectRoot != "" {
		h.block("plugins.error.globalScopeOnly", "This plugin adapter supports global scope only; project operations never fall back to global.")
		return h
	}
	version, err := s.run(ctx, target, "--version")
	h.Version = strings.TrimSpace(string(version))
	if err != nil {
		h.fail(err)
		return h
	}
	{
		args := []string{"plugin", "list", "--json"}
		if target == "antigravity-cli" {
			args = []string{"plugin", "list"}
		}
		var data []byte
		data, err = s.run(ctx, target, args...)
		if err == nil {
			h.Installed, err = parseCommandInventory(target, data)
			h.Fingerprint = hash(data)
		}
	}
	if err != nil {
		h.fail(err)
	}
	return h
}

func parseCommandInventory(target string, data []byte) ([]Installed, error) {
	if target == "antigravity-cli" {
		if strings.TrimSpace(string(data)) == "No imported plugins." {
			return []Installed{}, nil
		}
		var envelope struct {
			Imports json.RawMessage `json:"imports"`
		}
		if json.Unmarshal(data, &envelope) != nil || envelope.Imports == nil {
			return nil, fmt.Errorf("unrecognized Antigravity CLI plugin inventory")
		}
		data = envelope.Imports
	}
	var rows []struct {
		Name        string `json:"name"`
		Marketplace string `json:"marketplace"`
		Version     string `json:"version"`
		Enabled     *bool  `json:"enabled"`
		Source      string `json:"source"`
	}
	if json.Unmarshal(data, &rows) != nil || rows == nil {
		return nil, fmt.Errorf("unrecognized %s plugin inventory", target)
	}
	result := []Installed{}
	for _, row := range rows {
		if !namePattern.MatchString(row.Name) {
			return nil, fmt.Errorf("invalid native plugin name")
		}
		id := row.Name
		if target == "copilot" && row.Marketplace != "" {
			id += "@" + row.Marketplace
		}
		if !validTargetID(target, id) {
			return nil, fmt.Errorf("invalid native plugin identifier")
		}
		item := Installed{ID: id, Name: row.Name, Version: row.Version, Installed: true, Scope: "user", EnabledKnown: row.Enabled != nil}
		if row.Enabled != nil {
			item.Enabled = *row.Enabled
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *Service) verifyNativeTarget(ctx context.Context, target, action, id string) error {
	if s.ProjectRoot != "" {
		return fmt.Errorf("%s project plugin operations are unsupported", target)
	}
	if !validTargetID(target, id) {
		return fmt.Errorf("invalid plugin identifier")
	}
	if target == "grok" && (action == "install" || action == "update") {
		return fmt.Errorf("Grok requires native trust confirmation; install or update in Grok, then import. Skillshare never supplies --trust automatically")
	}
	command := action
	if action == "remove" || action == "uninstall" {
		command = "uninstall"
	}
	if action == "update" && target == "antigravity-cli" {
		command = "install"
	}
	// agy handles --help on subcommands as ordinary arguments; its parent help is safe.
	args := []string{"plugin", command, "--help"}
	if target == "antigravity-cli" {
		args = []string{"plugin", "--help"}
	}
	_, err := s.run(ctx, target, args...)
	return err
}

func (s *Service) applyNativeTarget(ctx context.Context, c Change, b Binding, root string) error {
	remove := c.Action == "remove" || c.Action == "uninstall"
	command, argument := "install", root
	if remove {
		command, argument = "uninstall", b.ID
	}
	if !remove && root == "" {
		return fmt.Errorf("this imported plugin has no reviewed reinstall source; install in the native client and import again")
	}
	// Reinstallation may reset a disabled native plugin. Require the native client
	// to update those installations rather than silently changing enablement.
	if c.Action == "update" {
		for _, item := range s.host(ctx, c.Target).Installed {
			if item.ID == b.ID && (!item.EnabledKnown || !item.Enabled) {
				return fmt.Errorf("update this plugin in the native client to preserve its native enabled state")
			}
		}
	}
	if _, err := s.run(ctx, c.Target, "plugin", command, argument); err != nil {
		return err
	}
	h := s.host(ctx, c.Target)
	if h.Error != "" {
		return fmt.Errorf("native operation completed but verification failed: %s", h.Error)
	}
	for _, item := range h.Installed {
		if item.ID == b.ID {
			if remove {
				return fmt.Errorf("plugin remains installed")
			}
			return nil
		}
	}
	if remove {
		return nil
	}
	return fmt.Errorf("plugin was not found after native installation")
}
