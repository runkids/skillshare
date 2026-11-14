package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func commandFixture(t *testing.T) (*Service, string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR", "OMP_CONFIG_DIR"} {
		t.Setenv(key, "")
	}
	root := fixture(t)
	writePluginFile(t, root, "plugin.json", `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","version":"1.0.0","extensions":{"cc.runkids.skillshare":{"commands":{"demo-one":"bin/one","demo-two":"bin/two"}}}}`)
	for _, name := range []string{"one", "two"} {
		writePluginFile(t, root, "bin/"+name, "#!/bin/sh\nprintf old-"+name+"\n")
		if err := os.Chmod(filepath.Join(root, "bin", name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	return s, root, filepath.Join(home, ".local", "bin")
}

func commandOutput(t *testing.T, bin, name, want string) {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-c", name)
	cmd.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin")
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != want {
		t.Fatalf("published command %s: output=%q error=%v, want %q", name, out, err, want)
	}
}

func TestPluginCommandsLifecycle(t *testing.T) {
	s, root, bin := commandFixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: root})
	commandOutput(t, bin, "demo-one", "old-one")
	commandOutput(t, bin, "demo-two", "old-two")
	writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf new-one\n")
	applyPluginRequest(t, s, Request{Action: "update", All: true})
	commandOutput(t, bin, "demo-one", "new-one")
	if err := os.Rename(root, root+"-offline"); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(root+"-offline", root)
	applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
	for _, name := range []string{"demo-one", "demo-two"} {
		if _, err := os.Lstat(filepath.Join(bin, name)); !os.IsNotExist(err) {
			t.Fatalf("command survived removal: %s %v", name, err)
		}
	}
	plain := fixture(t)
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: plain})
	if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "record" {
		t.Fatalf("metadata-free add changed: %+v %v", p, err)
	}
}

// These cases catch overwriting operator entries, losing missing-link repair,
// and leaving obsolete command names published after a same-version update.
func TestPluginCommandsOwnership(t *testing.T) {
	for _, foreign := range []string{"file", "dangling"} {
		for _, after := range []bool{false, true} {
			t.Run(foreign+"/"+map[bool]string{false: "before", true: "after"}[after], func(t *testing.T) {
				s, root, bin := commandFixture(t)
				if after {
					applyPluginRequest(t, s, Request{Action: "add", Source: root})
				}
				if err := os.MkdirAll(bin, 0755); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(bin, "demo-one")
				if after {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
				if foreign == "file" {
					writePluginFile(t, bin, "demo-one", "operator")
				} else if err := os.Symlink("/missing/operator", path); err != nil {
					t.Fatal(err)
				}
				r := Request{Action: "add", Source: root}
				if after {
					r = Request{Action: "update", Name: "demo"}
				}
				p, err := s.Preview(context.Background(), r)
				if err != nil {
					t.Fatal(err)
				}
				result, err := s.Apply(context.Background(), r, p.Revision)
				if err == nil {
					t.Fatalf("operator conflict accepted: %+v", result)
				}
				if after {
					applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
				}
				if foreign == "file" {
					raw, err := os.ReadFile(path)
					if err != nil || string(raw) != "operator" {
						t.Fatalf("operator file changed: %q %v", raw, err)
					}
				} else {
					link, err := os.Readlink(path)
					if err != nil || link != "/missing/operator" {
						t.Fatalf("foreign link changed: %q %v", link, err)
					}
				}
			})
		}
	}
	t.Run("repair-and-rename", func(t *testing.T) {
		s, root, bin := commandFixture(t)
		applyPluginRequest(t, s, Request{Action: "add", Source: root})
		if err := os.Remove(filepath.Join(bin, "demo-one")); err != nil {
			t.Fatal(err)
		}
		applyPluginRequest(t, s, Request{Action: "sync"})
		commandOutput(t, bin, "demo-one", "old-one")
		writePluginFile(t, root, "plugin.json", `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","extensions":{"cc.runkids.skillshare":{"commands":{"demo-new":"bin/one"}}}}`)
		applyPluginRequest(t, s, Request{Action: "update", Name: "demo"})
		commandOutput(t, bin, "demo-new", "old-one")
		for _, name := range []string{"demo-one", "demo-two"} {
			if _, err := os.Lstat(filepath.Join(bin, name)); !os.IsNotExist(err) {
				t.Fatalf("obsolete command kept: %s %v", name, err)
			}
		}
	})
	t.Run("second-package", func(t *testing.T) {
		s, root, bin := commandFixture(t)
		applyPluginRequest(t, s, Request{Action: "add", Source: root})
		p, err := s.Preview(context.Background(), Request{Action: "add", Source: root, Name: "second"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Apply(context.Background(), Request{Action: "add", Source: root, Name: "second"}, p.Revision); err == nil {
			t.Fatal("second package acquired commands")
		}
		commandOutput(t, bin, "demo-one", "old-one")
	})
}

func TestPluginCommandsScopeAndApproval(t *testing.T) {
	s, root, globalBin := commandFixture(t)
	s.ProjectRoot = t.TempDir()
	projectBin := filepath.Join(s.ProjectRoot, ".skillshare", "bin")
	r := Request{Action: "add", Source: root}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.StateDir); !os.IsNotExist(err) {
		t.Fatal("preview wrote state")
	}
	if _, err := os.Stat(projectBin); !os.IsNotExist(err) {
		t.Fatal("preview wrote bin")
	}
	writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf changed\n")
	if _, err := s.Apply(context.Background(), r, p.Revision); err == nil {
		t.Fatal("changed source accepted")
	}
	applyPluginRequest(t, s, r)
	commandOutput(t, projectBin, "demo-one", "changed")
	if _, err := os.Stat(globalBin); !os.IsNotExist(err) {
		t.Fatal("project published global commands")
	}
	if err := os.Remove(filepath.Join(projectBin, "demo-one")); err != nil {
		t.Fatal(err)
	}
	r = Request{Action: "sync"}
	p, err = s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, projectBin, "demo-one", "operator")
	if _, err := s.Apply(context.Background(), r, p.Revision); err == nil {
		t.Fatal("changed destination accepted")
	}
}

func TestPluginCommandsRecovery(t *testing.T) {
	s, root, bin := commandFixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: root})
	// Sync must pin the reviewed snapshot, even while the local source moves.
	writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf unreviewed\n")
	if err := os.Remove(filepath.Join(bin, "demo-one")); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, Request{Action: "sync"})
	commandOutput(t, bin, "demo-one", "old-one")
	// A shared deletion leaves cleanup work locally, including for zero bindings.
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  version: 2\n  packages: {}\n")
	if err := os.Rename(root, root+"-offline"); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(root+"-offline", root)
	applyPluginRequest(t, s, Request{Action: "sync"})
	if _, err := os.Lstat(filepath.Join(bin, "demo-one")); !os.IsNotExist(err) {
		t.Fatal("orphan command survived")
	}
	// Missing configuration is never interpreted as removing every package.
	other := fixture(t)
	writePluginFile(t, other, "plugin.json", `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","extensions":{"cc.runkids.skillshare":{"commands":{"demo-one":"script"}}}}`)
	writePluginFile(t, other, "script", "#!/bin/sh\ntrue\n")
	if err := os.Chmod(filepath.Join(other, "script"), 0755); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, Request{Action: "add", Source: other})
	if err := os.Remove(s.ConfigPath); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Preview(context.Background(), Request{Action: "sync"}); err == nil {
		t.Fatal("missing config discarded command ownership")
	}
}

func TestPluginCommandsPublicationRecovery(t *testing.T) {
	for _, failure := range []string{"second-link", "state-save"} {
		t.Run(failure, func(t *testing.T) {
			s, root, bin := commandFixture(t)
			applyPluginRequest(t, s, Request{Action: "add", Source: root})
			writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf new-one\n")
			writePluginFile(t, root, "bin/two", "#!/bin/sh\nprintf new-two\n")
			writePluginFile(t, root, "plugin.json", `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","extensions":{"cc.runkids.skillshare":{"commands":{"demo-one":"bin/two","demo-two":"bin/one"}}}}`)
			calls := 0
			s.commandPublish = func(path, previous, next string, exists bool) error {
				calls++
				if calls == 2 && failure == "second-link" {
					return errors.New("fixture publication failure")
				}
				if err := replaceCommand(path, previous, next, exists); err != nil {
					return err
				}
				if calls == 2 && failure == "state-save" {
					if err := os.Rename(s.statePath(), s.statePath()+".pending"); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(s.statePath(), 0700); err != nil {
						t.Fatal(err)
					}
				}
				return nil
			}
			r := Request{Action: "update", Name: "demo"}
			p, err := s.Preview(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			result, err := s.Apply(context.Background(), r, p.Revision)
			if err == nil || !slices.ContainsFunc(result.Results, func(o Outcome) bool { return o.Commands && o.Status == "failed" }) {
				t.Fatalf("fault not reported: %+v %v", result, err)
			}
			commandOutput(t, bin, "demo-one", "old-one")
			commandOutput(t, bin, "demo-two", "old-two")
			statePath := s.statePath()
			if failure == "state-save" {
				statePath += ".pending"
			}
			raw, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			var state struct {
				Commands map[string]struct {
					Pending json.RawMessage `json:"pending"`
				} `json:"commands"`
			}
			if err := json.Unmarshal(raw, &state); err != nil || len(state.Commands["demo"].Pending) == 0 {
				t.Fatalf("pending recovery lost: %s %v", raw, err)
			}
			if failure == "state-save" {
				if err := os.Remove(s.statePath()); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(statePath, s.statePath()); err != nil {
					t.Fatal(err)
				}
			}
			s.commandPublish = nil
			applyPluginRequest(t, s, Request{Action: "sync"})
			commandOutput(t, bin, "demo-one", "new-two")
			commandOutput(t, bin, "demo-two", "new-one")
			if err := os.Rename(root, root+"-offline"); err != nil {
				t.Fatal(err)
			}
			defer os.Rename(root+"-offline", root)
			applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
		})
	}
}

func TestPluginCommandsNativeIndependence(t *testing.T) {
	s, root, bin := commandFixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: root, Targets: []string{"cursor"}})
	other := fixture(t)
	writePluginFile(t, other, "plugin.json", `{"name":"demo","version":"1"}`)
	applyPluginRequest(t, s, Request{Action: "add", Name: "demo", Source: other, Targets: []string{"antigravity"}})
	writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf moving\n")
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo", Targets: []string{"cursor"}})
	commandOutput(t, bin, "demo-one", "old-one")
	applyPluginRequest(t, s, Request{Action: "remove", Name: "demo", Targets: []string{"antigravity", "cursor"}})
	commandOutput(t, bin, "demo-one", "old-one")
	applyPluginRequest(t, s, Request{Action: "update", All: true})
	commandOutput(t, bin, "demo-one", "moving")
	files, err := s.Files("demo")
	if err != nil || !slices.Contains(files, "bin/one") {
		t.Fatalf("targetless command snapshot unavailable: %q %v", files, err)
	}
	second := *s
	second.ConfigPath = filepath.Join(filepath.Dir(s.ConfigPath), "second.yaml")
	r := Request{Action: "add", Source: root}
	p, err := second.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Apply(context.Background(), r, p.Revision); err == nil {
		t.Fatal("second config acquired first config's command")
	}
}

func TestPluginCommandsDefaultRecipeReplacement(t *testing.T) {
	a, first, bin := commandFixture(t)
	applyPluginRequest(t, a, Request{Action: "add", Source: first})
	b := &Service{ConfigPath: filepath.Join(t.TempDir(), "config.yaml"), StateDir: filepath.Join(t.TempDir(), "state"), ProjectRoot: t.TempDir()}
	raw, err := os.ReadFile(a.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, filepath.Dir(b.ConfigPath), filepath.Base(b.ConfigPath), string(raw))
	applyPluginRequest(t, b, Request{Action: "remove", Name: "demo"})
	second := fixture(t)
	writePluginFile(t, second, "plugin.json", `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","extensions":{"cc.runkids.skillshare":{"commands":{"demo-one":"script"}}}}`)
	writePluginFile(t, second, "script", "#!/bin/sh\nprintf replacement\n")
	if err := os.Chmod(filepath.Join(second, "script"), 0755); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, b, Request{Action: "add", Source: second})
	raw, err = os.ReadFile(b.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, filepath.Dir(a.ConfigPath), filepath.Base(a.ConfigPath), string(raw))
	applyPluginRequest(t, a, Request{Action: "sync"})
	commandOutput(t, bin, "demo-one", "replacement")
	if _, err := os.Lstat(filepath.Join(bin, "demo-two")); !os.IsNotExist(err) {
		t.Fatal("old provider command kept")
	}
	applyPluginRequest(t, a, Request{Action: "remove", Name: "demo"})
	applyPluginRequest(t, b, Request{Action: "remove", Name: "demo"})
}
