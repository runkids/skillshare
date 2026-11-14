package plugin

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func declareReviewCommands(t *testing.T, root, commands string) {
	t.Helper()
	writePluginFile(t, root, "plugin.json", `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","version":"1.0.0","extensions":{"cc.runkids.skillshare":{"commands":`+commands+`}}}`)
}

func TestPluginCommandsAliasIsolation(t *testing.T) {
	s, root, bin := commandFixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: root})
	writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf second\n")
	declareReviewCommands(t, root, `{"alias-command":"bin/one"}`)
	applyPluginRequest(t, s, Request{Action: "add", Source: root, Name: "second"})
	commandOutput(t, bin, "demo-one", "old-one")
	writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf updated-second\n")
	applyPluginRequest(t, s, Request{Action: "update", Name: "second"})
	commandOutput(t, bin, "demo-one", "old-one")
	commandOutput(t, bin, "alias-command", "updated-second")
	// Files must select the same package-specific reviewed snapshot as its command.
	dir, err := s.snapshotDir("demo")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "bin/one"))
	if err != nil || string(raw) != "#!/bin/sh\nprintf old-one\n" {
		t.Fatalf("alias Files changed: %q %v", raw, err)
	}
}

func TestPluginCommandsInterruptedEvidence(t *testing.T) {
	if home := os.Getenv("SKILLSHARE_COMMAND_CRASH_HOME"); home != "" {
		s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
		mode := os.Getenv("SKILLSHARE_COMMAND_CRASH_MODE")
		s.commandPublish = func(path, old, next string, exists bool) error {
			if mode != "publish" {
				os.Exit(72)
			} // Pending has been saved, but no second mutation yet.
			if err := replaceCommand(path, old, next, exists); err != nil {
				return err
			}
			if filepath.Base(path) == "aaa-new" {
				os.Exit(71)
			}
			return nil
		}
		action := "update"
		if mode == "remove" {
			action = "remove"
		}
		r := Request{Action: action, Name: "demo"}
		p, err := s.Preview(context.Background(), r)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.Apply(context.Background(), r, p.Revision)
		t.Fatalf("expected interruption: %v", err)
	}
	for _, retry := range []string{"remove", "update"} {
		t.Run(retry, func(t *testing.T) {
			s, root, bin := commandFixture(t)
			applyPluginRequest(t, s, Request{Action: "add", Source: root})
			crash := func(mode string, want int) {
				t.Helper()
				cmd := exec.Command(os.Args[0], "-test.run=^TestPluginCommandsInterruptedEvidence$")
				cmd.Env = append(os.Environ(), "SKILLSHARE_COMMAND_CRASH_HOME="+filepath.Dir(s.ConfigPath), "SKILLSHARE_COMMAND_CRASH_MODE="+mode)
				raw, err := cmd.CombinedOutput()
				if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != want {
					t.Fatalf("interruption %s: %v %s", mode, err, raw)
				}
				if err := os.Remove(s.operationLockPath()); err != nil {
					t.Fatal(err)
				} // Exact dead child's existing operation lock.
			}
			declareReviewCommands(t, root, `{"aaa-new":"bin/one"}`)
			crash("publish", 71)
			if _, err := os.Lstat(filepath.Join(bin, "aaa-new")); err != nil {
				t.Fatal(err)
			}
			if retry == "update" {
				declareReviewCommands(t, root, `{"bbb-next":"bin/two"}`)
			}
			crash(retry, 72)
			// A second process must still prove the first interrupted publication after Pending changed.
			d, err := s.load()
			if err != nil {
				t.Fatal(err)
			}
			if len(commandOwners(d.state.Commands["demo"])["aaa-new"]) == 0 {
				t.Fatal("second interruption discarded old pending ownership")
			}
			if retry == "update" {
				applyPluginRequest(t, s, Request{Action: "update", Name: "demo"})
				commandOutput(t, bin, "bbb-next", "old-two")
			}
			applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
			for _, name := range []string{"aaa-new", "bbb-next", "demo-one", "demo-two"} {
				if _, err := os.Lstat(filepath.Join(bin, name)); !os.IsNotExist(err) {
					t.Fatalf("removed package left %s: %v", name, err)
				}
			}
		})
	}
}

func TestPluginCommandsDiscoverAfterDeselection(t *testing.T) {
	s, root, bin := commandFixture(t)
	writePluginFile(t, root, "plugin.json", `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"}`)
	applyPluginRequest(t, s, Request{Action: "add", Source: root, Targets: []string{"cursor"}})
	applyPluginRequest(t, s, Request{Action: "disable", Name: "demo", Targets: []string{"cursor"}})
	declareReviewCommands(t, root, `{"demo-one":"bin/one"}`)
	p, err := s.Preview(context.Background(), Request{Action: "check", Name: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range p.Changes {
		if c.Commands && c.Action == "update-available" {
			found = true
		}
	}
	if !found {
		t.Errorf("check omitted newly declared commands: %+v", p.Changes)
	}
	applyPluginRequest(t, s, Request{Action: "update", All: true})
	commandOutput(t, bin, "demo-one", "old-one")
	inv, err := s.Packages()
	if err != nil || inv.Packages["demo"].Bindings["cursor"].Selected() {
		t.Fatalf("native selection changed: %+v %v", inv, err)
	}
}

func TestPluginCommandsVersionObservation(t *testing.T) {
	s, root, _ := commandFixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: root})
	for _, path := range []string{".claude-plugin/plugin.json", ".codex-plugin/plugin.json"} {
		writePluginFile(t, root, path, `{"name":"demo","version":"2.0.0","skills":"./skills"}`)
	}
	writePluginFile(t, root, "plugin.json", `{"name":"demo","version":"2.0.0","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","extensions":{"cc.runkids.skillshare":{"commands":{"demo-one":"bin/one"}}}}`)
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo"})
	inv, err := s.Packages()
	if err != nil {
		t.Fatal(err)
	}
	if p := inv.Packages["demo"]; p.Version != "2.0.0" || p.Commands.Binding.Version != p.Version {
		t.Fatalf("package/command version mismatch: %+v", p)
	}
}

func TestPluginCommandsFreshDiscoveryFailure(t *testing.T) {
	s, root, _ := commandFixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: root})
	raw, err := os.ReadFile(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	fresh := &Service{ConfigPath: filepath.Join(t.TempDir(), "config.yaml"), StateDir: t.TempDir(), ProjectRoot: t.TempDir()}
	writePluginFile(t, filepath.Dir(fresh.ConfigPath), "config.yaml", string(raw))
	if err := os.Rename(root, root+"-offline"); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(root+"-offline", root)
	for _, action := range []string{"check", "sync"} {
		r := Request{Action: action}
		p, err := fresh.Preview(context.Background(), r)
		if err != nil {
			t.Fatal(err)
		}
		failed := false
		for _, c := range p.Changes {
			if c.Commands && c.Failed {
				failed = true
			}
		}
		if !failed {
			t.Errorf("%s silently reconciled unavailable source: %+v", action, p.Changes)
		}
		if action == "sync" {
			if result, err := fresh.Apply(context.Background(), r, p.Revision); err == nil {
				t.Errorf("sync succeeded: %+v", result)
			}
		}
	}
}

func TestPluginCommandsLocalizedFailure(t *testing.T) {
	s, root, bin := commandFixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: root})
	if err := os.Remove(filepath.Join(bin, "demo-one")); err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, bin, "demo-one", "operator")
	r := Request{Action: "update", All: true}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range p.Changes {
		if c.Commands && (c.MessageKey != "plugins.error.commandConflict" || c.MessageArgs["name"] != "demo-one" || c.MessageArgs["bin"] != bin) {
			t.Errorf("preview lost localized identity: %+v", c)
		}
	}
	result, err := s.Apply(context.Background(), r, p.Revision)
	if err == nil {
		t.Fatal("conflict succeeded")
	}
	for _, o := range result.Results {
		if o.Commands && (o.MessageKey != "plugins.error.commandConflict" || o.MessageArgs["bin"] != bin) {
			t.Errorf("outcome lost localized identity: %+v", o)
		}
	}
	declareReviewCommands(t, root, `{"demo-one":"../escape"}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	// Exercise discovery serialization: catalog/API transport must preserve the validation key.
	raw, _ := json.Marshal(d)
	var restored Discovery
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	c := restored.Candidates[0]
	raw, _ = json.Marshal(c)
	var fields map[string]any
	_ = json.Unmarshal(raw, &fields)
	if fields["commandProblemKey"] != "plugins.error.commandPath" {
		t.Fatalf("validation identity lost: %s", raw)
	}
}
