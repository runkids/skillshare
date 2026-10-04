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
	"slices"
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

// ErrorKey is the translation key and arguments of a fixed failure, "" when the message
// was assembled at runtime.
func ErrorKey(err error) (string, map[string]string) {
	var keyed agentError
	if errors.As(err, &keyed) {
		return keyed.key, keyed.args
	}
	return "", nil
}

// gitFailure sorts a git failure by the words in its stderr, so the dashboard can say
// whether the network or the source is at fault. Only the key leaves this function:
// stderr can echo a URL with credentials. LC_ALL=C keeps the words English.
func gitFailure(stderr string) (agentError, bool) {
	lower := strings.ToLower(stderr)
	// GitHub asks for a username when the repository does not exist or is private. A bare
	// "not found" is not enough: a missing git-lfs says "command not found".
	missing := strings.Contains(lower, "fatal: repository '") && strings.Contains(lower, "' not found")
	for _, s := range []string{"repository not found", "does not appear to be a git repository", "couldn't find remote ref", "returned error: 404", "could not read username"} {
		missing = missing || strings.Contains(lower, s)
	}
	if missing {
		return agentError{key: "plugins.error.sourceNotFound", message: "plugin source not found; check the repository address and branch"}, true
	}
	// Not "unable to access": git opens every HTTPS failure with it, a bad certificate or a
	// 403 included, and those are not the network's fault.
	for _, s := range []string{"could not resolve host", "failed to connect", "connection timed out", "connection refused", "operation timed out", "network is unreachable", "resolving timed out"} {
		if strings.Contains(lower, s) {
			return agentError{key: "plugins.error.network", message: "could not reach the plugin source; check the network connection and try again"}, true
		}
	}
	return agentError{}, false
}

// missingPath reports whether bin is a path that does not exist, which is as missing as a
// name not on PATH. A missing working directory fails with the same error, so check bin.
func missingPath(bin string, err error) bool {
	if !filepath.IsAbs(bin) || !errors.Is(err, os.ErrNotExist) {
		return false
	}
	_, statErr := os.Stat(bin)
	return errors.Is(statErr, os.ErrNotExist)
}

func runCommand(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	// No shell, inherited stdin, or automatic native trust/command confirmation.
	cmd.Env = append(append(os.Environ(), "GIT_TERMINAL_PROMPT=0"), env...)
	if bin == "git" {
		cmd.Env = append(cmd.Env, "LC_ALL=C")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) || missingPath(bin, err) {
			return nil, agentError{cause: ErrCLIMissing, key: "plugins.error.cliMissing", message: fmt.Sprintf("%s CLI is not installed or not on PATH on the machine running Skillshare; install it there before syncing plugins", bin)}
		}
		if ctx.Err() != nil {
			return nil, agentError{key: "plugins.error.timeout", message: fmt.Sprintf("%s timed out or was cancelled; inspect native status before retrying", bin)}
		}
		if bin == "git" {
			if failure, ok := gitFailure(stderr.String()); ok {
				return nil, failure
			}
		}
		// Native output can contain credentials or command-source scripts.
		return nil, agentError{key: "plugins.error.commandFailed", message: fmt.Sprintf("%s command failed; open the native client to resolve authentication, trust, or configuration", bin)}
	}
	// Some CLIs (Pi) print their version to stderr.
	if stdout.Len() == 0 && slices.Equal(args, []string{"--version"}) {
		return stderr.Bytes(), nil
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
	// An account runs its Agent's CLI against the account's own config directory.
	var env []string
	bin := s.agentOf(target)
	if bin == "antigravity-cli" {
		bin = "agy"
	}
	explicit := false
	if account, ok := s.account(target); ok {
		env = account.env()
		if account.CLI != "" {
			bin, explicit = account.CLI, true
		}
	}
	// Only the default codex is searched for; an account's cli, even "codex", is kept.
	if bin != "codex" || explicit {
		return run(ctx, dir, env, bin, args...)
	}
	find := s.findCodex
	if find == nil {
		find = newCodexFinder().find
	}
	bin, looked := find()
	out, err := run(ctx, dir, env, bin, args...)
	if errors.Is(err, ErrCLIMissing) {
		where := strings.Join(looked, ", ")
		return nil, agentError{cause: ErrCLIMissing, key: "plugins.error.codexMissing", args: map[string]string{"looked": where},
			message: fmt.Sprintf("Codex CLI not found on the machine running Skillshare (looked in: %s); install it there, or set %s to its path", where, codexCLIEnv)}
	}
	return out, err
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
		// Filter by scope before validating: Claude also lists other scopes' plugins
		// (e.g. "(suppressed)@skills-dir" for the folder it runs in), and an entry this
		// scope never uses must not block the whole target.
		if target == "claude" {
			if project == "" && item.Scope != "user" {
				continue
			}
			if project != "" && (item.Scope != "project" || filepath.Clean(item.ProjectPath) != filepath.Clean(project)) {
				continue
			}
		}
		if !validID(item.ID) {
			return nil, fmt.Errorf("native plugin list contains an unsupported plugin identifier")
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
	agent := s.agentOf(target)
	if agent != "claude" && agent != "codex" {
		return s.additionalHost(ctx, target)
	}
	h := Host{Target: target, Status: HostReady, Installed: []Installed{}}
	if agent == "codex" && s.ProjectRoot != "" {
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
	h.Installed, err = parseInventory(agent, data, s.ProjectRoot)
	if err != nil {
		h.fail(err)
		return h
	}
	// Marketplaces stays nil when the list cannot be read: planning then keeps a removal
	// pending instead of guessing that nothing is left to clean up.
	if markets, err := s.marketplaces(ctx, target); err == nil {
		h.Marketplaces = markets
		for name, root := range markets {
			// A name also declared elsewhere ("") stays claimed, so removal reports the clash
			// instead of forgetting the binding.
			if strings.HasPrefix(name, "skillshare-") && (root == "" || filepath.Clean(root) == filepath.Join(s.managedRoot(), name)) {
				h.ManagedMarketplaces = append(h.ManagedMarketplaces, name)
			}
		}
		slices.Sort(h.ManagedMarketplaces)
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
	return &Inventory{TargetDefinitions: s.TargetDefinitions(), Packages: cfg.packages, Hosts: []Host{}}, nil
}

func (s *Service) Inventory(ctx context.Context) (*Inventory, error) {
	result, err := s.Packages()
	if err != nil {
		return nil, err
	}
	// Each Agent answers through its own CLI and none depends on another, so the slowest
	// one sets the wait instead of the sum of all of them.
	targets := s.targets()
	result.Hosts = make([]Host, len(targets))
	var wg sync.WaitGroup
	for i, target := range targets {
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
	agent := s.agentOf(target)
	if agent == "codex" {
		if s.ProjectRoot != "" {
			return nil, fmt.Errorf("Codex project plugin operations are unsupported")
		}
		switch action {
		// Codex has no update command; add replaces an installed plugin with the
		// marketplace's current copy.
		case "install", "update":
			return []string{"plugin", "add", id, "--json"}, nil
		case "remove":
			return []string{"plugin", "remove", id, "--json"}, nil
		}
		return nil, fmt.Errorf("Codex has no verified native %s command; manage this operation in Codex", action)
	}
	if agent != "claude" {
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
	if agent := s.agentOf(target); agent != "claude" && agent != "codex" {
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
