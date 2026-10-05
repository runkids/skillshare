package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// NormalizeTarget resolves the Antigravity CLI nickname without changing stored keys.
func NormalizeTarget(target string) string {
	if target == "agy" {
		return "antigravity"
	}
	return target
}

// ProjectSupported never falls back to a user-scoped installation.
func ProjectSupported(target string) bool {
	for _, d := range TargetDefinitions() {
		if d.Target == target {
			return d.Project
		}
	}
	return false
}

// validTargetID checks an identifier for the Agent that installs it. An empty target is
// an account whose Agent the caller could not resolve; every Agent that can have one
// writes its plugins as name@marketplace.
func validTargetID(target, id string) bool {
	switch target {
	case "":
		return validID(id)
	case "claude", "codex", "omp":
		return validID(id)
	case "cursor", "antigravity", "antigravity-cli", "grok", "kimi", "hermes", "devin":
		return namePattern.MatchString(id)
	case "copilot":
		return namePattern.MatchString(id) || validID(id)
	case "pi", "opencode":
		return id != "" && !strings.HasPrefix(id, "-") && !strings.ContainsAny(id, "\x00\r\n")
	}
	return false
}

func (s *Service) snapshotPath(b Binding, target string) string {
	market := "skillshare-" + hash([]byte(s.ConfigPath + "\x00" + b.Source + "\x00" + b.Plugin + "\x00" + target))[:16]
	if marketplaceAgent(s.agentOf(target)) {
		_, market, _ = strings.Cut(b.ID, "@")
	}
	return filepath.Join(s.managedRoot(), market)
}

// Require an explicit, existing entry. Do not execute package code to discover it.
func openCodeEntry(root string, explicit ...string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return "", err
	}
	var m struct {
		Main    string          `json:"main"`
		Exports json.RawMessage `json:"exports"`
	}
	if err = json.Unmarshal(data, &m); err != nil {
		return "", err
	}
	entry := m.Main
	if len(m.Exports) > 0 && (len(explicit) == 0 || explicit[0] == "") {
		if json.Unmarshal(m.Exports, &entry) != nil {
			var exports map[string]json.RawMessage
			if json.Unmarshal(m.Exports, &exports) != nil || json.Unmarshal(exports["."], &entry) != nil {
				return "", agentError{key: "plugins.problem.opencodeExports", message: "OpenCode package needs an unambiguous string root export; conditional exports are not supported"}
			}
		}
	}
	if len(explicit) > 0 && explicit[0] != "" {
		entry = explicit[0]
	}
	if entry == "" {
		entry = "index.js"
	}
	clean := filepath.Clean(entry)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", agentError{key: "plugins.problem.opencodeEscapes", message: "OpenCode package entry escapes its directory"}
	}
	ext := filepath.Ext(clean)
	if ext != ".js" && ext != ".mjs" && ext != ".ts" {
		return "", agentError{key: "plugins.problem.opencodeEntryType", message: "OpenCode package requires a built JavaScript or TypeScript entry"}
	}
	info, err := os.Lstat(filepath.Join(root, clean))
	if err != nil || !info.Mode().IsRegular() {
		return "", agentError{key: "plugins.problem.opencodeEntryMissing", message: fmt.Sprintf("OpenCode package entry %s is missing; build the package before adding it", clean), args: map[string]string{"entry": clean}}
	}
	return clean, nil
}

func (s *Service) additionalHost(ctx context.Context, target string) Host {
	agent := s.agentOf(target)
	h := Host{Target: target, Status: HostReady, Installed: []Installed{}}
	if problem := automationProblem(agent); problem != "" {
		h.block("plugins.problem."+agent, problem)
		return h
	}
	if commandTarget(agent) {
		return s.commandHost(ctx, target)
	}
	if s.ProjectRoot != "" && !ProjectSupported(agent) {
		h.block("plugins.error.userScoped", agent+" plugins are user-scoped; use global mode. Project operations never fall back to global.")
		return h
	}
	var err error
	switch agent {
	case "cursor":
		h.NoteKey = "plugins.note.cursor"
		h.Note = "Local plugin files only. Reload Cursor and allow local plugin imports; marketplace installations take precedence."
		h.Installed, h.Fingerprint, err = s.localInventory(agent)
	case "antigravity":
		h.NoteKey = "plugins.note.antigravity"
		h.Note = "Custom plugin files for Antigravity desktop/workspaces; verify loading in Antigravity. The standalone agy CLI uses a separate plugin store."
		h.Installed, h.Fingerprint, err = s.localInventory(agent)
	case "pi":
		var data []byte
		data, err = s.run(ctx, target, "--version")
		h.Version = strings.TrimSpace(string(data))
		if err == nil {
			h.Installed, h.Fingerprint, err = s.piInventory(target)
		}
		if err == nil && filepath.IsAbs(s.StateDir) && slices.ContainsFunc(h.Installed, func(i Installed) bool { return i.Filtered }) {
			if _, why := s.piGate(ctx, target); why == "" {
				for i := range h.Installed {
					if h.Installed[i].Filtered {
						_, _, _, entryErr := s.piRegistrationEntry(target, h.Installed[i].ID)
						h.Installed[i].Importable = entryErr == nil
					}
				}
			}
		}
		h.NoteKey = "plugins.note.pi"
		h.Note = "Package registrations from Pi settings; resource loading is verified in Pi."
	case "opencode":
		var data []byte
		data, err = s.run(ctx, target, "--version")
		h.Version = strings.TrimSpace(string(data))
		if err == nil {
			h.Installed, h.Fingerprint, err = s.openCodeInventory(h.Version)
		}
		h.NoteKey = "plugins.note.opencode"
		h.Note = "Plugin registrations only. OpenCode loads code and resolves dependencies on startup."
	default:
		err = fmt.Errorf("unsupported plugin target %q", target)
	}
	if err != nil {
		h.fail(err)
	}
	return h
}

func (s *Service) verifyAdditional(ctx context.Context, target, action, id string) error {
	agent := s.agentOf(target)
	if commandTarget(agent) {
		return s.verifyNativeTarget(ctx, target, action, id)
	}
	if s.ProjectRoot != "" && !ProjectSupported(agent) {
		return fmt.Errorf("%s project plugins are unsupported", target)
	}
	if !validTargetID(agent, id) {
		return fmt.Errorf("invalid native plugin identifier")
	}
	if !slices.Contains([]string{"install", "update", "remove", "uninstall"}, action) {
		return fmt.Errorf("unsupported plugin action %s", action)
	}
	if agent == "cursor" || agent == "antigravity" || agent == "opencode" {
		return nil
	}
	command := action
	if action == "remove" || action == "uninstall" {
		command = "remove"
	}
	args := []string{command, "--help"}
	_, err := s.run(ctx, target, args...)
	return err
}

func (s *Service) applyAdditional(ctx context.Context, c Change, b Binding) error {
	var root string
	if (c.Action == "install" || c.Action == "update") && b.Source != "" {
		snapshot, err := s.materialize(ctx, b, c.Target)
		if err != nil {
			return err
		}
		d, err := discoverRoot(filepath.Join(snapshot, "content"), b.Source, b.Entry)
		if err != nil {
			return err
		}
		for _, candidate := range d.Candidates {
			if candidate.Name == b.Plugin {
				root = filepath.Join(snapshot, "content", candidate.pathFor(s.agentOf(c.Target)))
			}
		}
		if root == "" {
			return fmt.Errorf("plugin missing from snapshot")
		}
	}
	if commandTarget(s.agentOf(c.Target)) {
		return s.applyNativeTarget(ctx, c, b, root)
	}
	remove := c.Action == "remove" || c.Action == "uninstall"
	switch s.agentOf(c.Target) {
	case "cursor", "antigravity":
		return s.applyLocal(c, b, root)
	case "opencode":
		return s.applyOpenCode(ctx, c, b)
	case "pi":
		if c.Action == "update" {
			// An imported package updates natively; pi update keeps its settings entry.
			// Local Pi packages load directly from the managed snapshot.
			if root == "" {
				if _, err := s.run(ctx, c.Target, "update", b.ID); err != nil {
					return fmt.Errorf("Pi update failed: %w", err)
				}
			}
		} else {
			if c.Action == "install" && b.PiRegistration != "" {
				if err := s.restorePiRegistration(c, b); err != nil {
					return err
				}
			}
			command := "install"
			if remove {
				command = "remove"
			}
			args := []string{command, b.ID}
			if s.ProjectRoot != "" {
				args = append(args, "--local")
			}
			if _, err := s.run(ctx, c.Target, args...); err != nil {
				return fmt.Errorf("Pi operation failed; project packages require trust established in Pi: %w", err)
			}
		}
	}
	h := s.host(ctx, c.Target)
	if h.Error != "" {
		return fmt.Errorf("native verification failed: %s", h.Error)
	}
	for _, i := range h.Installed {
		if i.ID == b.ID {
			if remove {
				return fmt.Errorf("plugin remains registered")
			}

			return nil
		}
	}
	if remove {
		return nil
	}
	return fmt.Errorf("plugin not found after native operation")
}

func fileURL(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}
