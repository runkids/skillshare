package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func runCommand(ctx context.Context, dir, bin string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	// No shell, inherited stdin, or automatic native trust/command confirmation.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("%s CLI is not installed or not on PATH on the machine running Skillshare; install it there before syncing plugins", bin)
		}
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%s timed out or was cancelled; inspect native status before retrying", bin)
		}
		// Native output can contain credentials or command-source scripts.
		return nil, fmt.Errorf("%s command failed; open the native client to resolve authentication, trust, or configuration", bin)
	}
	return stdout.Bytes(), nil
}

func (s *Service) run(ctx context.Context, target string, args ...string) ([]byte, error) {
	run := s.Run
	if run == nil {
		run = runCommand
	}
	dir := s.ProjectRoot
	if dir == "" {
		var err error
		dir, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}
	return run(ctx, dir, target, args...)
}

func parseInventory(target string, data []byte, project string) ([]Installed, error) {
	result := []Installed{}
	if target == "codex" {
		var envelope struct {
			Installed json.RawMessage `json:"installed"`
		}
		if json.Unmarshal(data, &envelope) != nil || len(envelope.Installed) == 0 {
			return nil, fmt.Errorf("unrecognized Codex plugin list schema")
		}
		if err := json.Unmarshal(envelope.Installed, &result); err != nil {
			return nil, fmt.Errorf("invalid Codex installed list")
		}
		for i := range result {
			r := &result[i]
			r.ID = r.PluginID
			if r.ID == "" && r.Name != "" && r.Marketplace != "" {
				r.ID = r.Name + "@" + r.Marketplace
			}
			r.Scope = "user"
		}
	} else {
		if err := json.Unmarshal(data, &result); err != nil || result == nil {
			return nil, fmt.Errorf("unrecognized Claude plugin list schema")
		}
	}
	filtered := []Installed{}
	for _, item := range result {
		if !validID(item.ID) {
			return nil, fmt.Errorf("native plugin list contains an unsupported plugin identifier")
		}
		if target == "claude" {
			if project == "" && item.Scope != "user" {
				continue
			}
			if project != "" && (item.Scope != "project" || filepath.Clean(item.ProjectPath) != filepath.Clean(project)) {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	return filtered, nil
}

func validID(id string) bool {
	name, market, ok := strings.Cut(id, "@")
	return ok && namePattern.MatchString(name) && namePattern.MatchString(market)
}

func (s *Service) host(ctx context.Context, target string) Host {
	if target != "claude" && target != "codex" {
		return s.additionalHost(ctx, target)
	}
	h := Host{Target: target, Installed: []Installed{}}
	if target == "codex" && s.ProjectRoot != "" {
		h.Error = "Codex native plugin installation is user-scoped; use global mode. Project operations never fall back to global."
		return h
	}
	version, err := s.run(ctx, target, "--version")
	if err != nil {
		h.Error = err.Error()
		return h
	}
	h.Version = strings.TrimSpace(string(version))
	data, err := s.run(ctx, target, "plugin", "list", "--json")
	if err != nil {
		h.Error = err.Error()
		return h
	}
	h.Installed, err = parseInventory(target, data, s.ProjectRoot)
	if err != nil {
		h.Error = err.Error()
	}
	return h
}

func (s *Service) Inventory(ctx context.Context) (*Inventory, error) {
	cfg, err := s.load()
	if err != nil {
		return nil, err
	}
	result := &Inventory{Packages: cfg.packages, Hosts: []Host{}}
	for _, target := range Targets {
		result.Hosts = append(result.Hosts, s.host(ctx, target))
	}
	return result, nil
}

func (s *Service) nativeArgs(target, action, id string) ([]string, error) {
	if action == "uninstall" {
		action = "remove"
	}
	if !validID(id) {
		return nil, fmt.Errorf("invalid plugin identity")
	}
	if target == "codex" {
		if s.ProjectRoot != "" {
			return nil, fmt.Errorf("Codex project plugin operations are unsupported")
		}
		switch action {
		case "install":
			return []string{"plugin", "add", id, "--json"}, nil
		case "remove":
			return []string{"plugin", "remove", id, "--json"}, nil
		}
		return nil, fmt.Errorf("Codex has no verified native %s command; manage this operation in Codex", action)
	}
	if target != "claude" {
		return nil, fmt.Errorf("unsupported plugin target %q", target)
	}
	if action == "install" || action == "update" || action == "remove" {
		command := action
		if action == "remove" {
			command = "uninstall"
		}
		scope := "user"
		if s.ProjectRoot != "" {
			scope = "project"
		}
		args := []string{"plugin", command, id, "--scope", scope}
		if action == "install" || action == "update" {
			args = append(args, "--json")
		}
		return args, nil
	}
	return nil, fmt.Errorf("unsupported plugin action %q", action)
}

func (s *Service) verifyCommand(ctx context.Context, target, action, id string) error {
	if target != "claude" && target != "codex" {
		return s.verifyAdditional(ctx, target, action, id)
	}
	args, err := s.nativeArgs(target, action, id)
	if err != nil {
		return err
	}
	help, err := s.run(ctx, target, "plugin", args[1], "--help")
	if err != nil {
		return err
	}
	for _, flag := range args {
		if strings.HasPrefix(flag, "--") && !strings.Contains(string(help), flag) {
			return fmt.Errorf("installed %s does not support %s for %s; upgrade the native CLI", target, flag, action)
		}
	}
	return nil
}
