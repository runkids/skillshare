package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// remoteFixture serves fixture(t) as a Git repository at an HTTPS address, and returns that
// address and a function that commits the working tree.
func remoteFixture(t *testing.T) (string, string, func()) {
	t.Helper()
	root := fixture(t)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
	}
	commit := func() {
		git("add", ".")
		git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "change")
	}
	git("init", "-b", "main")
	commit()
	url := "https://fixture.invalid/demo.git"
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	writePluginFile(t, filepath.Dir(cfg), filepath.Base(cfg), "[url \"file://"+root+"\"]\n  insteadOf = "+url+"\n")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	return url, root, commit
}

func addPlugin(t *testing.T, s *Service, r Request) {
	t.Helper()
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(context.Background(), r, p.Revision); err != nil {
		t.Fatal(err)
	}
}

func TestCheckDoesNotDownloadASourceStillAtTheInstalledCommit(t *testing.T) {
	url, root, commit := remoteFixture(t)
	s, _, _ := fakeClaude(t)
	addPlugin(t, s, Request{Action: "add", Source: url, Plugin: "demo", Targets: []string{"claude"}})
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)

	p, err := s.Preview(context.Background(), Request{Action: "check"})
	if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "noop" || !slices.Equal(p.Changes[0].Components, []string{"skills"}) {
		t.Fatalf("unchanged source: %+v %v", p, err)
	}
	if raw, _ := os.ReadFile(trace); strings.Contains(string(raw), " clone ") {
		t.Fatal("check downloaded a source whose commit had not moved")
	}

	writePluginFile(t, root, ".claude-plugin/plugin.json", `{"name":"demo","version":"1.1.0"}`)
	commit()
	p, err = s.Preview(context.Background(), Request{Action: "check"})
	if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "update-available" || p.Changes[0].Binding.Version != "1.1.0" {
		t.Fatalf("moved source: %+v %v", p, err)
	}
}

func TestCheckAsksEachAgentOnceForAllPlugins(t *testing.T) {
	s, _, _ := fakeClaude(t)
	addPlugin(t, s, Request{Action: "add", Name: "one", Source: fixture(t), Plugin: "demo", Targets: []string{"claude"}})
	addPlugin(t, s, Request{Action: "add", Name: "two", Source: fixture(t), Plugin: "demo", Targets: []string{"claude"}})
	var mu sync.Mutex
	lists := 0
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "plugin list --json" {
			mu.Lock()
			lists++
			mu.Unlock()
		}
		return run(ctx, dir, env, bin, args...)
	}

	var checked []string
	if err := s.Check(context.Background(), Request{Action: "check"}, func(name string, p *Plan, err error) {
		if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "noop" {
			t.Errorf("%s: %+v %v", name, p, err)
		}
		checked = append(checked, name)
	}); err != nil {
		t.Fatal(err)
	}
	if slices.Sort(checked); !slices.Equal(checked, []string{"one", "two"}) || lists != 1 {
		t.Fatalf("checked %v, listed Claude's plugins %d times", checked, lists)
	}
}

// An Agent whose CLI does not answer holds up only the plugins it has.
func TestCheckReportsAPluginWithoutWaitingForAnotherPluginsAgent(t *testing.T) {
	s, _, _ := fakeClaude(t)
	work := t.TempDir()
	s.Accounts = map[string]Account{"claude-work": {Agent: "claude", Dir: work}}
	addPlugin(t, s, Request{Action: "add", Name: "one", Source: fixture(t), Plugin: "demo", Targets: []string{"claude"}})
	addPlugin(t, s, Request{Action: "add", Name: "two", Source: fixture(t), Plugin: "demo", Targets: []string{"claude-work"}})
	oneDone := make(chan struct{})
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if slices.Contains(env, "CLAUDE_CONFIG_DIR="+work) {
			select {
			case <-oneDone:
				return nil, agentError{key: "plugins.error.timeout", message: "claude timed out or was cancelled; inspect native status before retrying"}
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return run(ctx, dir, env, bin, args...)
	}

	var order []string
	err := s.Check(t.Context(), Request{Action: "check"}, func(name string, p *Plan, err error) {
		order = append(order, name)
		if name == "one" {
			if err != nil || p.Changes[0].Action != "noop" {
				t.Errorf("one: %+v %v", p, err)
			}
			close(oneDone)
			return
		}
		if err != nil || p.Changes[0].Action != "blocked" || p.Changes[0].MessageKey != "plugins.error.timeout" {
			t.Errorf("two: %+v %v", p, err)
		}
	})
	if err != nil || !slices.Equal(order, []string{"one", "two"}) {
		t.Fatalf("order %v: %v", order, err)
	}
}

// Every plugin is checked at the same time, so plugins waiting on slow Agents never keep
// another plugin from starting.
func TestCheckChecksEveryPluginAtOnce(t *testing.T) {
	s, _, _ := fakeClaude(t)
	s.Accounts = map[string]Account{}
	const plugins = 9
	for i := range plugins {
		target := fmt.Sprintf("claude-%d", i)
		s.Accounts[target] = Account{Agent: "claude", Dir: t.TempDir()}
		addPlugin(t, s, Request{Action: "add", Name: fmt.Sprintf("p%d", i), Source: fixture(t), Plugin: "demo", Targets: []string{target}})
	}
	// Each Agent answers only once every plugin's Agent has been asked.
	var asked sync.WaitGroup
	asked.Add(plugins)
	all := make(chan struct{})
	go func() { asked.Wait(); close(all) }()
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "--version" {
			asked.Done()
			select {
			case <-all:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return run(ctx, dir, env, bin, args...)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	checked := 0
	if err := s.Check(ctx, Request{Action: "check"}, func(name string, p *Plan, err error) {
		if err != nil || p.Changes[0].Action != "noop" {
			t.Errorf("%s: %+v %v", name, p, err)
		}
		checked++
	}); err != nil || checked != plugins || ctx.Err() != nil {
		t.Fatalf("checked %d of %d: %v %v", checked, plugins, err, ctx.Err())
	}
}

// A plugin bound to several Agents asks them all at once, not one after another.
func TestCheckAsksEveryAgentOfAPluginAtOnce(t *testing.T) {
	s, _, _ := fakeClaude(t)
	s.Accounts = map[string]Account{"claude-a": {Agent: "claude", Dir: t.TempDir()}, "claude-b": {Agent: "claude", Dir: t.TempDir()}}
	addPlugin(t, s, Request{Action: "add", Name: "demo", Source: fixture(t), Plugin: "demo", Targets: []string{"claude-a", "claude-b"}})
	var asked sync.WaitGroup
	asked.Add(2)
	both := make(chan struct{})
	go func() { asked.Wait(); close(both) }()
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "--version" {
			asked.Done()
			select {
			case <-both:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return run(ctx, dir, env, bin, args...)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err := s.Check(ctx, Request{Action: "check"}, func(name string, p *Plan, err error) {
		if err != nil || len(p.Changes) != 2 || p.Changes[0].Action != "noop" || p.Changes[1].Action != "noop" {
			t.Errorf("%s: %+v %v", name, p, err)
		}
	}); err != nil || ctx.Err() != nil {
		t.Fatalf("%v %v", err, ctx.Err())
	}
}

// Proving another plugin's leftover install on an Agent that does not answer holds up only that plugin.
func TestCheckDoesNotWaitOnAnotherPluginsLeftoverInstall(t *testing.T) {
	s, _, _ := fakeClaude(t)
	work := t.TempDir()
	s.Accounts = map[string]Account{"claude-work": {Agent: "claude", Dir: work}}
	addPlugin(t, s, Request{Action: "add", Name: "one", Source: fixture(t), Plugin: "demo", Targets: []string{"claude"}})
	addPlugin(t, s, Request{Action: "add", Name: "two", Source: fixture(t), Plugin: "demo", Targets: []string{"claude"}})
	d, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	d.putLocal("two", "claude-work", localBinding{Legacy: &receipt{Intent: "legacy", Store: s.storeIdentity("claude-work"), Binding: Binding{ID: "demo@elsewhere"}}})
	if err := s.saveState(d); err != nil {
		t.Fatal(err)
	}
	oneDone := make(chan struct{})
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if slices.Contains(env, "CLAUDE_CONFIG_DIR="+work) {
			select {
			case <-oneDone:
				return nil, agentError{key: "plugins.error.timeout", message: "claude timed out or was cancelled; inspect native status before retrying"}
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return run(ctx, dir, env, bin, args...)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var order []string
	if err := s.Check(ctx, Request{Action: "check"}, func(name string, p *Plan, err error) {
		order = append(order, name)
		if name == "one" {
			close(oneDone)
		}
	}); err != nil || ctx.Err() != nil || len(order) != 2 || order[0] != "one" {
		t.Fatalf("order %v: %v %v", order, err, ctx.Err())
	}
}
