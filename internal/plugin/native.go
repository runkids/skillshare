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
	"sync"
	"time"
)

// ErrCLIMissing marks the one host failure fixed by installing something rather than
// by opening the Agent. Hosts carrying it are grouped together in the dashboard, so the
// same sentence is not repeated once per Agent.
var ErrCLIMissing = errors.New("native CLI not on PATH")

// agentError names a fixed failure template so the dashboard can show it in the reader's
// language. message stays the English the CLI prints and is the fallback; cause is
// ErrCLIMissing when installing something, not opening the Agent, is the remedy. args fill
// the {placeholders} of the translated sentence, for the part only known at runtime.
type agentError struct {
	cause   error
	key     string
	message string
	args    map[string]string
}

func (e agentError) Error() string { return e.message }
func (e agentError) Unwrap() error { return e.cause }

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
			return nil, agentError{cause: ErrCLIMissing, key: "plugins.error.cliMissing", message: fmt.Sprintf("%s CLI is not installed or not on PATH on the machine running Skillshare; install it there before syncing plugins", bin)}
		}
		if ctx.Err() != nil {
			return nil, agentError{key: "plugins.error.timeout", message: fmt.Sprintf("%s timed out or was cancelled; inspect native status before retrying", bin)}
		}
		// Native output can contain credentials or command-source scripts.
		return nil, agentError{key: "plugins.error.commandFailed", message: fmt.Sprintf("%s command failed; open the native client to resolve authentication, trust, or configuration", bin)}
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
	if target == "antigravity-cli" {
		target = "agy"
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
		item.EnabledKnown = true
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
	h := Host{Target: target, Status: HostReady, Installed: []Installed{}}
	if target == "codex" && s.ProjectRoot != "" {
		h.block("plugins.error.userScoped", "Codex native plugin installation is user-scoped; use global mode. Project operations never fall back to global.")
		return h
	}
	version, err := s.run(ctx, target, "--version")
	if err != nil {
		h.fail(err)
		return h
	}
	h.Version = strings.TrimSpace(string(version))
	data, err := s.run(ctx, target, "plugin", "list", "--json")
	if err != nil {
		h.fail(err)
		return h
	}
	h.Installed, err = parseInventory(target, data, s.ProjectRoot)
	if err != nil {
		h.fail(err)
	}
	return h
}

// Packages is the part of the inventory that config alone answers, so the dashboard can
// draw the plugin list while Inventory is still waiting on each Agent's CLI.
func (s *Service) Packages() (*Inventory, error) {
	cfg, err := s.load()
	if err != nil {
		return nil, err
	}
	return &Inventory{TargetDefinitions: TargetDefinitions(), Packages: cfg.packages, Hosts: []Host{}}, nil
}

func (s *Service) Inventory(ctx context.Context) (*Inventory, error) {
	result, err := s.Packages()
	if err != nil {
		return nil, err
	}
	// Each Agent answers through its own CLI and none depends on another, so the slowest
	// one sets the wait instead of the sum of all of them.
	result.Hosts = make([]Host, len(Targets))
	var wg sync.WaitGroup
	for i, target := range Targets {
		wg.Go(func() { result.Hosts[i] = s.host(ctx, target) })
	}
	wg.Wait()
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
