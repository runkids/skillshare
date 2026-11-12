package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The dashboard groups Agents by remedy, so a missing CLI has to stay distinguishable
// from a reachable Agent that failed for its own reasons, without reading the message.
func TestHostStatusSeparatesAMissingCLIFromOtherFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
		key  string
	}{
		{"cli not on path", agentError{cause: ErrCLIMissing, key: "plugins.error.cliMissing", message: "claude CLI is not installed"}, HostMissing, "plugins.error.cliMissing"},
		{"agent refused", errors.New("claude command failed; open the native client"), HostBlocked, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var h Host
			h.fail(tc.err)
			if h.Status != tc.want {
				t.Fatalf("status = %q, want %q", h.Status, tc.want)
			}
			if h.Error != tc.err.Error() {
				t.Fatalf("message was rewritten: %q", h.Error)
			}
			if h.ErrorKey != tc.key {
				t.Fatalf("key = %q, want %q", h.ErrorKey, tc.key)
			}
		})
	}
}

// A host whose inventory fails still has to list nothing, not null, or the dashboard
// cannot draw the page for the Agents that answered.
func TestFailedInventoryStillEncodesAnEmptyList(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("OPENCODE_CONFIG", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("OPENCODE_CONFIG_CONTENT", "")
	writePluginFile(t, home, ".config/opencode/opencode.json", "{}")
	writePluginFile(t, home, ".config/opencode/opencode.jsonc", "{}")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		return []byte("1.18.32"), nil
	}
	h := s.host(context.Background(), "opencode")
	if h.Status != HostBlocked {
		t.Fatalf("status = %q (error %q), want %q", h.Status, h.Error, HostBlocked)
	}
	data, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"installed":[]`) {
		t.Fatalf("installed not an empty list: %s", data)
	}
}

func TestRunCommandMarksAnAbsentBinary(t *testing.T) {
	_, err := runCommand(context.Background(), t.TempDir(), nil, "skillshare-no-such-binary", "--version")
	if !errors.Is(err, exec.ErrNotFound) && !errors.Is(err, ErrCLIMissing) {
		t.Fatalf("err = %v, want it to match ErrCLIMissing", err)
	}
	if !errors.Is(err, ErrCLIMissing) {
		t.Fatalf("err = %v, want ErrCLIMissing", err)
	}
}

func TestGitFailureNamesTheCauseWithoutItsOutput(t *testing.T) {
	for stderr, key := range map[string]string{
		"fatal: unable to access 'https://x/': Could not resolve host: github.com":       "plugins.error.network",
		"fatal: unable to access 'https://x/': Failed to connect to github.com port 443": "plugins.error.network",
		"remote: Repository not found.\nfatal: repository 'https://x/' not found":        "plugins.error.sourceNotFound",
		"fatal: couldn't find remote ref nope":                                           "plugins.error.sourceNotFound",
		"fatal: repository 'https://x/' not found":                                       "plugins.error.sourceNotFound",
	} {
		failure, ok := gitFailure(stderr)
		if !ok || failure.key != key || strings.Contains(failure.message, "https://x/") {
			t.Errorf("%q: key = %q, message = %q, want %s", stderr, failure.key, failure.message, key)
		}
	}
	// Git words other failures the same way; a wrong cause would send the user to the wrong fix.
	for _, stderr := range []string{
		"fatal: something else",
		"git-lfs filter-process: git-lfs: command not found",
		"fatal: unable to access 'https://x/': SSL certificate problem: self signed certificate",
		"fatal: unable to access 'https://x/': The requested URL returned error: 403",
	} {
		if failure, ok := gitFailure(stderr); ok {
			t.Errorf("%q was classified as %s", stderr, failure.key)
		}
	}
}

func TestRunCommandReadsVersionFromStderr(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pi"), []byte("#!/bin/sh\necho 0.73.1 >&2\n"), 0755); err != nil {
		t.Fatal(err)
	}
	out, err := runCommand(context.Background(), dir, nil, filepath.Join(dir, "pi"), "--version")
	if err != nil || strings.TrimSpace(string(out)) != "0.73.1" {
		t.Fatalf("version = %q %v", out, err)
	}
}

// A CLI that is a shim starting the real client keeps its output open in the child; a cancelled
// command still has to return, or the check never marks the Agent as timed out.
func TestRunCommandReturnsWhenCancelledWhileAChildStillHoldsItsOutput(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\nsleep 60\n"), 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := runCommand(ctx, dir, nil, filepath.Join(dir, "claude"), "--version")
	if key, _ := ErrorKey(err); key != "plugins.error.timeout" || time.Since(start) > 15*time.Second {
		t.Fatalf("returned %v after %s", err, time.Since(start))
	}
}

// A CLI that answers and exits while a helper it started keeps its output open still answered.
func TestRunCommandKeepsAnAnswerWhenAHelperOutlivesTheCLI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pi"), []byte("#!/bin/sh\necho 0.73.1\nsleep 5 &\n"), 0755); err != nil {
		t.Fatal(err)
	}
	out, err := runCommand(context.Background(), dir, nil, filepath.Join(dir, "pi"), "--version")
	if err != nil || strings.TrimSpace(string(out)) != "0.73.1" {
		t.Fatalf("version = %q %v", out, err)
	}
}

// Git exports GIT_DIR to hooks; a plugin clone, or a native CLI that runs git itself,
// must not act on the hook's repository (#852).
func TestRunCommandDropsTheCallersRepository(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\necho \"[$GIT_DIR]\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_DIR", "/caller/.git")
	out, err := runCommand(context.Background(), dir, nil, filepath.Join(dir, "claude"), "--version")
	if err != nil || strings.TrimSpace(string(out)) != "[]" {
		t.Fatalf("child saw GIT_DIR = %q %v", out, err)
	}
}

func TestHostReportsReadyWhenTheCLIAnswers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		if args[0] == "--version" {
			return []byte("2.1.276"), nil
		}
		return []byte(`[]`), nil
	}
	if h := s.host(context.Background(), "claude"); h.Status != HostReady {
		t.Fatalf("status = %q (error %q), want %q", h.Status, h.Error, HostReady)
	}
}

// A fixed sentence without a key silently stays English in the dashboard, so every
// message that is a constant has to carry one.
func TestFixedMessagesCarryATranslationKey(t *testing.T) {
	for _, d := range TargetDefinitions() {
		if d.Reason != "" && d.ReasonKey == "" {
			t.Errorf("%s: reason has no key", d.Target)
		}
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		if args[0] == "--version" {
			return []byte("1.0.0"), nil
		}
		return []byte(`[]`), nil
	}
	for _, target := range Targets {
		h := s.host(context.Background(), target)
		if h.Note != "" && h.NoteKey == "" {
			t.Errorf("%s: note has no key", target)
		}
		if automationProblem(target) != "" && h.ErrorKey == "" {
			t.Errorf("%s: automation problem has no key", target)
		}
	}
}

// The dashboard draws the plugin list from Packages before any Agent has answered, so it
// must not wait on one. Inventory asks them all at once and still lists them in order.
func TestPackagesSkipsTheAgentsAndInventoryKeepsTheirOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var asked atomic.Int32
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		asked.Add(1)
		if args[0] == "--version" {
			return []byte("1.0.0"), nil
		}
		return []byte(`[]`), nil
	}
	quick, err := s.Packages()
	if err != nil {
		t.Fatal(err)
	}
	if asked.Load() != 0 || len(quick.Hosts) != 0 {
		t.Fatalf("Packages asked %d Agents and returned %d hosts, want none", asked.Load(), len(quick.Hosts))
	}
	full, err := s.Inventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i, target := range Targets {
		if full.Hosts[i].Target != target {
			t.Fatalf("hosts[%d] = %q, want %q", i, full.Hosts[i].Target, target)
		}
	}
}
