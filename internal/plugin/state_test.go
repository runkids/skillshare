package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/jsonschema-go/jsonschema"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPluginHostStateStaleApprovalAndCorruption(t *testing.T) {
	for _, change := range []string{"local", "account", "shared", "missing", "unknown", "corrupt"} {
		t.Run(change, func(t *testing.T) {
			var commands []string
			s := fakePiNpm(t, &commands)
			s.Accounts = map[string]Account{"work": {Agent: "pi", Dir: t.TempDir()}}
			applyPluginRequest(t, s, Request{Action: "add", Source: hostStatePiSource(t), Targets: []string{"work"}})
			req := Request{Action: "sync"}
			p, err := s.Preview(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "local":
				raw, _ := os.ReadFile(s.statePath())
				if err := os.WriteFile(s.statePath(), append(raw, ' '), 0600); err != nil {
					t.Fatal(err)
				}
			case "account":
				s.Accounts["work"] = Account{Agent: "pi", Dir: t.TempDir()}
			case "shared":
				raw, _ := os.ReadFile(s.ConfigPath)
				_ = os.WriteFile(s.ConfigPath, append(raw, '\n'), 0600)
			case "missing":
				_ = os.Remove(s.ConfigPath)
			case "unknown":
				_ = os.WriteFile(s.statePath(), []byte(`{"version":2,"bindings":{}}`), 0600)
			case "corrupt":
				_ = os.WriteFile(s.statePath(), []byte(`{broken`), 0600)
			}
			before := len(commands)
			if _, err := s.Apply(context.Background(), req, p.Revision); err == nil {
				t.Fatal("stale or corrupt state accepted")
			}
			if len(commands) != before {
				t.Fatal("stale state ran native command")
			}
		})
	}
}

func TestPluginHostStateMidApplyIntentEditStopsRemainingTargets(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	s.Accounts = map[string]Account{"work": {Agent: "pi", Dir: t.TempDir()}}
	source := hostStatePiSource(t)
	req := Request{Action: "add", Source: source, Targets: []string{"pi", "work"}}
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		raw, err := run(ctx, dir, env, bin, args...)
		if args[0] == "install" && args[1] != "--help" {
			cfg, _ := os.ReadFile(s.ConfigPath)
			_ = os.WriteFile(s.ConfigPath, append(cfg, []byte("# external intent edit\n")...), 0600)
		}
		return raw, err
	}
	result := applyFailure(t, s, req)
	if len(result.Results) != 1 || result.Results[0].Status != "installed" || len(commands) != 1 {
		t.Fatalf("continued after external edit: %+v %q", result, commands)
	}
	if local := hostPackages(t, s)["demo"].Bindings["pi"]; local.ID == "" || local.Pending != "" {
		t.Fatal("completed result not retained")
	}
}

func TestPluginHostStateLockIsLocal(t *testing.T) {
	s := &Service{ConfigPath: filepath.Join(t.TempDir(), "missing", "config.yaml"), StateDir: t.TempDir()}
	unlock, err := s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := s.Lock(); err == nil {
		t.Fatal("same local state did not contend")
	}
	other := *s
	other.StateDir = t.TempDir()
	release, err := other.Lock()
	if err != nil {
		t.Fatal(err)
	}
	release()
	if _, err := os.Stat(filepath.Dir(s.ConfigPath)); !os.IsNotExist(err) {
		t.Fatal("local lock touched shared config directory")
	}
}

func TestPluginHostStateIntentEditBetweenNativeSteps(t *testing.T) {
	native := &fakeAgents{}
	s := native.service(t)
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		out, err := run(ctx, dir, env, bin, args...)
		if len(args) > 2 && args[0] == "plugin" && args[1] == "marketplace" && args[2] == "add" {
			raw, _ := os.ReadFile(s.ConfigPath)
			_ = os.WriteFile(s.ConfigPath, append(raw, []byte("# external intent edit\n")...), 0600)
		}
		return out, err
	}
	r := Request{Action: "add", Source: fixture(t), Targets: []string{"claude"}}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Apply(context.Background(), r, p.Revision)
	if err == nil || slices.ContainsFunc(native.commands, func(c string) bool { return strings.Contains(c, "plugin install") }) {
		t.Fatalf("continued native install after changed intent: %q %v", native.commands, err)
	}
}

func TestPluginDeclarationSchemas(t *testing.T) {
	for _, file := range []string{"config.schema.json", "project-config.schema.json"} {
		t.Run(file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", file))
			if err != nil {
				t.Fatal(err)
			}
			var schema struct {
				Properties map[string]json.RawMessage `json:"properties"`
			}
			if err := json.Unmarshal(raw, &schema); err != nil {
				t.Fatal(err)
			}
			var plugins jsonschema.Schema
			if err := json.Unmarshal(schema.Properties["plugins"], &plugins); err != nil {
				t.Fatal(err)
			}
			resolved, err := plugins.Resolve(nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, sample := range []struct {
				raw   string
				valid bool
			}{
				{`{"version":2,"packages":{"demo":{"source":"https://example.invalid/demo.git","bindings":{"pi":{},"omp":{"imported":true}}}}}`, true},
				{`{"packages":{"demo":{"bindings":{"pi":{"id":"npm:demo"}}}}}`, true},
				{`{"version":2,"packages":{"demo":{"bindings":{"pi":{"id":"foreign"}}}}}`, false},
			} {
				var value any
				_ = json.Unmarshal([]byte(sample.raw), &value)
				if err := resolved.Validate(value); (err == nil) != sample.valid {
					t.Fatalf("validation %s: %v", sample.raw, err)
				}
			}
		})
	}
}

func TestPluginHostStateNativeErrorKeepsCause(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	cause := errors.New("disk full")
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if args[0] == "install" && args[1] != "--help" {
			return nil, cause
		}
		return run(ctx, dir, env, bin, args...)
	}
	r := Request{Action: "add", Source: "npm:demo", Targets: []string{"pi"}}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Apply(context.Background(), r, p.Revision)
	if !errors.Is(err, cause) || strings.Contains(err.Error(), "project packages require trust") {
		t.Fatalf("misleading native failure: %v", err)
	}
}

func TestPluginHostStateResultWriteFailureStopsNativeWork(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	s.Accounts = map[string]Account{"work": {Agent: "pi", Dir: t.TempDir()}}
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		out, err := run(ctx, dir, env, bin, args...)
		if args[0] == "install" && args[1] != "--help" {
			if err := os.Rename(s.statePath(), s.statePath()+".pending"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(s.statePath(), 0700); err != nil {
				t.Fatal(err)
			}
		}
		return out, err
	}
	result := applyFailure(t, s, Request{Action: "add", Source: hostStatePiSource(t), Targets: []string{"pi", "work"}})
	if len(commands) != 1 || len(result.Results) != 1 || !strings.Contains(result.Results[0].Message, "recording its result failed") {
		t.Fatalf("continued after result write failure: %+v %q", result, commands)
	}
	if err := os.Remove(s.statePath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(s.statePath()+".pending", s.statePath()); err != nil {
		t.Fatal(err)
	}
	if got := hostPackages(t, s)["demo"].Bindings["pi"]; got.Pending == "" {
		t.Fatal("durable pending lost")
	}
}

func TestPluginHostStateRepeatAddPreservesSelection(t *testing.T) {
	for _, kind := range []string{"source", "npm"} {
		for _, receipt := range []string{"present", "missing", "adopt"} {
			if kind == "source" && receipt == "adopt" {
				continue
			}
			t.Run(kind+"/"+receipt, func(t *testing.T) {
				var commands []string
				s := fakePiNpm(t, &commands)
				source := "npm:demo"
				if kind == "source" {
					source = hostStatePiSource(t)
				}
				req := Request{Action: "add", Source: source, Targets: []string{"pi"}}
				applyPluginRequest(t, s, req)
				applyPluginRequest(t, s, Request{Action: "disable", Name: "demo", Targets: []string{"pi"}})
				var other *Service
				var otherCommands []string
				if receipt == "present" {
					shared, _ := os.ReadFile(s.ConfigPath)
					other = fakePiNpm(t, &otherCommands)
					writePluginFile(t, filepath.Dir(other.ConfigPath), filepath.Base(other.ConfigPath), string(shared))
					p, err := other.Preview(context.Background(), Request{Action: "sync"})
					if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "noop" {
						t.Fatalf("disabled on second host: %+v %v", p, err)
					}
					repairHost(t, s)
				}
				if receipt != "present" {
					if err := os.Remove(s.statePath()); err != nil {
						t.Fatal(err)
					}
					if receipt == "missing" {
						file, _ := s.piSettingsPath("pi")
						writePluginFile(t, filepath.Dir(file), filepath.Base(file), `{"packages":[]}`)
					}
				}
				applyPluginRequest(t, s, req)
				if b := readState(t, s).intent["demo"].Bindings["pi"]; b.Sync == nil || *b.Sync {
					t.Fatal("repeat add changed shared selection")
				}
				if other != nil {
					shared, _ := os.ReadFile(s.ConfigPath)
					writePluginFile(t, filepath.Dir(other.ConfigPath), filepath.Base(other.ConfigPath), string(shared))
					repairHost(t, other)
					applyPluginRequest(t, other, Request{Action: "sync"})
					if len(otherCommands) != 0 {
						t.Fatal("repeat add made another host install a disabled target")
					}
				}
			})
		}
	}
	native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
	s := native.service(t)
	source := fixture(t)
	writePluginFile(t, source, ".claude-plugin/marketplace.json", `{"name":"team","plugins":[{"name":"demo","source":"."}]}`)
	req := Request{Action: "add", Source: source, Targets: []string{"claude"}}
	applyPluginRequest(t, s, req)
	applyPluginRequest(t, s, Request{Action: "disable", Name: "demo", Targets: []string{"claude"}})
	applyPluginRequest(t, s, req)
	if b := readState(t, s).intent["demo"].Bindings["claude"]; b.Sync == nil || *b.Sync {
		t.Fatal("adoption changed selection")
	}
}
