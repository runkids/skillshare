package plugin

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

// A CLI target filter scopes native bindings, but a default-recipe update still
// reviews package commands. Explicit API selections remain authoritative.
func TestPluginCommandsTargetedDefaultUpdate(t *testing.T) {
	for _, mode := range []string{"default", "checked", "alternate", "api-native"} {
		t.Run(mode, func(t *testing.T) {
			s, root, bin := commandFixture(t)
			applyPluginRequest(t, s, Request{Action: "add", Source: root, Targets: []string{"cursor"}})
			target := "cursor"
			if mode == "alternate" {
				other := fixture(t)
				writePluginFile(t, other, "plugin.json", `{"name":"demo","version":"1"}`)
				applyPluginRequest(t, s, Request{Action: "add", Name: "demo", Source: other, Targets: []string{"antigravity"}})
				target = "antigravity"
			}
			writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf targeted\n")
			r := Request{Action: "update", Name: "demo", Targets: []string{target}}
			if mode == "api-native" {
				no := false
				r.Commands = &no
			}
			if mode == "checked" {
				p, err := s.Preview(context.Background(), Request{Action: "check", Name: "demo", Targets: r.Targets})
				if err != nil {
					t.Fatal(err)
				}
				seen := map[string]Seen{}
				for _, c := range p.Changes {
					seen[c.Target] = checkedObservation(t, c.Binding)
				}
				if _, ok := seen[""]; !ok {
					t.Fatal("targeted check omitted default commands")
				}
				item := UpdateItem{Name: "demo", Targets: r.Targets, Commands: true, Seen: seen}
				delete(item.Seen, "")
				if _, result, err := s.updateLocked(context.Background(), item, func(*Plan) {}); !errors.Is(err, ErrSourceMoved) || result != nil {
					t.Fatalf("unapproved command update: %+v %v", result, err)
				}
				for _, c := range p.Changes {
					seen[c.Target] = checkedObservation(t, c.Binding)
				}
				if _, _, err := s.updateLocked(context.Background(), item, func(*Plan) {}); err != nil {
					t.Fatal(err)
				}
			} else {
				applyPluginRequest(t, s, r)
			}
			want := "targeted"
			if mode == "alternate" || mode == "api-native" {
				want = "old-one"
			}
			commandOutput(t, bin, "demo-one", want)
			applyPluginRequest(t, s, Request{Action: "remove", Name: "demo", Targets: []string{target}})
			commandOutput(t, bin, "demo-one", want)
		})
	}
}

func TestPluginCommandsTargetedRefKeepsDefault(t *testing.T) {
	s, _, bin := commandFixture(t)
	source, root, commit := remoteFixture(t)
	declareReviewCommands(t, root, `{"demo-one":"bin/one"}`)
	writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf default\n")
	if err := os.Chmod(filepath.Join(root, "bin/one"), 0755); err != nil {
		t.Fatal(err)
	}
	commit()
	applyPluginRequest(t, s, Request{Action: "add", Source: source, SourceRef: "main", Targets: []string{"cursor"}})
	cmd := exec.Command("git", "checkout", "-b", "next")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("branch: %s %v", out, err)
	}
	writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf alternate-ref\n")
	commit()
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo", Targets: []string{"cursor"}, SourceRef: "next"})
	inventory, err := s.Packages()
	if err != nil {
		t.Fatal(err)
	}
	pack := inventory.Packages["demo"]
	if pack.SourceRef != "main" || pack.Commands.Binding.SourceRef != "main" || pack.Bindings["cursor"].SourceRef != "next" {
		t.Fatalf("target ref replaced default: %+v", pack)
	}
	commandOutput(t, bin, "demo-one", "default")
	p, err := s.Preview(context.Background(), Request{Action: "check", Name: "demo", Targets: []string{"cursor"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range p.Changes {
		if c.Commands {
			t.Fatalf("alternate-ref check selected default commands: %+v", c)
		}
	}
}

func TestPluginCommandsSyncIntent(t *testing.T) {
	for _, mode := range []string{"restored-main", "reviewed-next", "pending-only", "restored-removal", "last-command-removal"} {
		t.Run(mode, func(t *testing.T) {
			s, _, bin := commandFixture(t)
			url, root, commit := remoteFixture(t)
			declareReviewCommands(t, root, `{"demo-one":"bin/one"}`)
			writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf main\n")
			if err := os.Chmod(filepath.Join(root, "bin/one"), 0755); err != nil {
				t.Fatal(err)
			}
			commit()
			if mode != "pending-only" {
				applyPluginRequest(t, s, Request{Action: "add", Source: url, SourceRef: "main"})
			}
			mainConfig, err := os.ReadFile(s.ConfigPath)
			if mode != "pending-only" && err != nil {
				t.Fatal(err)
			}
			git := func(args ...string) {
				t.Helper()
				cmd := exec.Command("git", args...)
				cmd.Dir = root
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %s %v", args, out, err)
				}
			}
			r := Request{Action: "update", Name: "demo", SourceRef: "next"}
			if mode == "restored-removal" {
				r = Request{Action: "remove", Name: "demo"}
			} else if mode == "last-command-removal" {
				for _, path := range []string{"plugin.json", ".claude-plugin/plugin.json", ".codex-plugin/plugin.json"} {
					writePluginFile(t, root, path, `{"name":"demo","version":"2.0.0","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"}`)
				}
				commit()
				r.SourceRef = ""
			} else {
				git("checkout", "-b", "next")
				declareReviewCommands(t, root, `{"demo-next":"bin/one"}`)
				writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf reviewed-next\n")
				commit()
				if mode == "pending-only" {
					r = Request{Action: "add", Source: url, SourceRef: "next"}
				}
			}
			p, err := s.Preview(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			s.commandPublish = func(string, string, string, bool) error { return errors.New("fixture publication failure") }
			result, err := s.Apply(context.Background(), r, p.Revision)
			if err == nil || result == nil {
				t.Fatalf("publication fault not reported: %+v %v", result, err)
			}
			s.commandPublish = nil
			if mode == "restored-main" || mode == "restored-removal" {
				// Restore a declaration produced by the successful main add, as another machine can.
				writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), string(mainConfig))
			}
			if mode == "reviewed-next" || mode == "pending-only" {
				writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf unchecked-later\n")
				commit()
			}
			if mode == "restored-removal" || mode == "last-command-removal" {
				if err := os.Rename(root, root+"-offline"); err != nil {
					t.Fatal(err)
				}
				defer os.Rename(root+"-offline", root)
			}
			inv, err := s.Packages()
			if err != nil {
				t.Fatal(err)
			}
			if mode == "restored-removal" && inv.Packages["demo"].Commands.Binding.Pending == "remove" {
				t.Error("restored identical declaration still advertises removal")
			}
			applyPluginRequest(t, s, Request{Action: "sync"})
			want, absent := "demo-one", "demo-next"
			if mode == "reviewed-next" || mode == "pending-only" {
				want, absent = absent, want
				commandOutput(t, bin, want, "reviewed-next")
			} else if mode != "last-command-removal" {
				commandOutput(t, bin, want, "main")
			}
			if mode == "last-command-removal" {
				absent = "demo-one"
				inv, err := s.Packages()
				if err != nil || inv.Packages["demo"].Commands != nil || inv.Packages["demo"].Version != "2.0.0" {
					t.Fatalf("offline last-command removal lost version: %+v %v", inv, err)
				}
			}
			if _, err := os.Lstat(filepath.Join(bin, absent)); !os.IsNotExist(err) {
				t.Fatalf("obsolete command %s survived: %v", absent, err)
			}
			d, err := s.load()
			if err != nil || d.state.Commands["demo"].Pending != nil || len(d.state.Commands["demo"].Owned) != 0 {
				t.Fatalf("reconciliation retained pending ownership: %+v %v", d, err)
			}
		})
	}
}

func TestPluginCommandsSyncPackageErrors(t *testing.T) {
	for _, commands := range []bool{true, false} {
		t.Run(map[bool]string{true: "commands", false: "commandless"}[commands], func(t *testing.T) {
			s, _, bin := commandFixture(t)
			aaa := fixture(t)
			applyPluginRequest(t, s, Request{Action: "add", Source: aaa, Name: "aaa"})
			url, root, commit := remoteFixture(t)
			if commands {
				declareReviewCommands(t, root, `{"demo-one":"bin/one"}`)
				writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf healthy\n")
				if err := os.Chmod(filepath.Join(root, "bin/one"), 0755); err != nil {
					t.Fatal(err)
				}
				commit()
				writePluginFile(t, bin, "demo-one", "operator")
			} else {
				writePluginFile(t, root, "plugin.json", `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"}`)
				commit()
			}
			r := Request{Action: "add", Source: url, Name: "zzz", Targets: []string{"cursor"}}
			p, err := s.Preview(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			result, err := s.Apply(context.Background(), r, p.Revision)
			if (err != nil) != commands || result == nil {
				t.Fatalf("initial add: %+v %v", result, err)
			}
			inv, err := s.Packages()
			if err != nil || inv.Packages["zzz"].Bindings["cursor"].ID == "" {
				t.Fatalf("native installation did not complete: %+v %v", inv, err)
			}
			if commands {
				if err := os.Remove(filepath.Join(bin, "demo-one")); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Rename(aaa, aaa+"-offline"); err != nil {
				t.Fatal(err)
			}
			defer os.Rename(aaa+"-offline", aaa)
			r = Request{Action: "sync"}
			p, err = s.Preview(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range p.Changes {
				if c.Name == "zzz" && c.Failed {
					t.Errorf("healthy package inherited aaa discovery error: %+v", c)
				}
			}
			result, err = s.Apply(context.Background(), r, p.Revision)
			if err == nil || result == nil || !slices.ContainsFunc(result.Results, func(o Outcome) bool { return o.Name == "aaa" && o.Commands && o.Status == "failed" }) {
				t.Fatalf("unavailable aaa failure hidden: %+v %v", result, err)
			}
			if commands {
				commandOutput(t, bin, "demo-one", "healthy")
			}
		})
	}
}
