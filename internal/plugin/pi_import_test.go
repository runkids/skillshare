package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiFilteredImportRefusesUnsupportedEntriesWithoutWrites(t *testing.T) {
	for _, entry := range []string{
		`{"source":"npm:demo","skills":null}`,
		`{"source":"npm:demo","themes":[3]}`,
		`{"source":"npm:demo","autoload":"false"}`,
		`{"source":"npm:demo","extensions":["\ud800"]}`,
		`{"source":"npm:demo","source":"npm:demo"}`,
		`{"source":"https://example.invalid/org/demo.git?credential=dummy","extensions":[]}`,
		`{"source":"./relative","extensions":[]}`,
	} {
		t.Run(fmt.Sprintf("entry-%d", len(entry)), func(t *testing.T) {
			f := newPiFixture(t)
			file := filepath.Join(f.agentDir, "settings.json")
			writeTree(t, f.agentDir, map[string]string{"settings.json": `{"packages":[` + entry + `]}`})
			id := parsePiEntry(0, []byte(entry)).source
			id = piRegistrationID(id, file)
			check := unchanged(t, file, f.svc.ConfigPath, filepath.Join(f.agentDir, "trust.json"))
			p, err := f.svc.Preview(context.Background(), Request{Action: "import", From: "pi", Plugin: id})
			if err == nil && !p.Blocked {
				t.Fatal("unsafe entry importable")
			}
			check()
			if _, err := os.Stat(f.svc.StateDir); !os.IsNotExist(err) {
				t.Fatal("preview wrote private state")
			}
		})
	}
}

func TestPiFilteredImportBindsPreviewAndPrivateRecord(t *testing.T) {
	f := newPiFixture(t)
	f.global(map[string]any{"packages": []any{map[string]any{"source": "npm:demo", "extensions": []string{"-a.ts"}}}})
	r := Request{Action: "import", From: "pi", Plugin: "npm:demo"}
	p, err := f.svc.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(f.agentDir, "settings.json")
	f.global(map[string]any{"packages": []any{map[string]any{"source": "npm:demo", "extensions": []string{"-b.ts"}}}})
	check := unchanged(t, file, f.svc.ConfigPath)
	if _, err := f.svc.Apply(context.Background(), r, p.Revision); err == nil {
		t.Fatal("stale import applied")
	}
	check()
	applyPluginRequest(t, f.svc, r)
	d, err := f.svc.load()
	if err != nil {
		t.Fatal(err)
	}
	b := d.packages["demo"].Bindings["pi"]
	blob, err := f.svc.piRegistrationPath(b.PiRegistration)
	if err != nil {
		t.Fatal(err)
	}
	assertPiRecordPrivate(t, blob)
	f.svc.Accounts = map[string]Account{"pi-work": {Agent: "pi", Dir: filepath.Join(f.home, "account")}}
	if _, err := f.svc.readPiRegistration(b.PiRegistration, "pi-work", b.ID); err == nil {
		t.Fatal("account borrowed another target's record")
	}
	if err := os.WriteFile(blob, []byte("modified"), 0o600); err != nil {
		t.Fatal(err)
	}
	check = unchanged(t, file, f.svc.ConfigPath, blob)
	if _, err := f.svc.Apply(context.Background(), r, mustPreviewRevision(t, f.svc, r)); err == nil {
		t.Fatal("modified record overwritten")
	}
	check()
}

func mustPreviewRevision(t *testing.T, s *Service, r Request) string {
	t.Helper()
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	return p.Revision
}

func TestPiFilteredReinstallRetriesNativeFailureAndRefusesLateChanges(t *testing.T) {
	for _, late := range []bool{false, true} {
		t.Run(fmt.Sprintf("late-%t", late), func(t *testing.T) {
			f := newPiFixture(t)
			f.global(map[string]any{"packages": []any{map[string]any{"source": "npm:demo", "extensions": []string{"-a.ts"}}}})
			applyPluginRequest(t, f.svc, Request{Action: "import", From: "pi", Plugin: "npm:demo"})
			// Simulate an absent registration after a completed uninstall.
			f.global(map[string]any{"packages": []any{"npm:other"}})
			attempts := 0
			f.svc.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
				if args[0] == "--version" {
					return []byte("1.0.0"), nil
				}
				if len(args) > 1 && args[1] == "--help" {
					return nil, nil
				}
				attempts++
				if attempts == 1 {
					return nil, fmt.Errorf("simulated native install failure")
				}
				return nil, nil
			}
			r := Request{Action: "sync"}
			p, err := f.svc.Preview(context.Background(), r)
			if err != nil || p.Blocked {
				t.Fatalf("preview: %v %+v", err, p)
			}
			file := filepath.Join(f.agentDir, "settings.json")
			if late {
				piBeforeRegistrationWrite = func(string) { f.global(map[string]any{"packages": []any{"npm:external"}, "external": true}) }
				t.Cleanup(func() { piBeforeRegistrationWrite = func(string) {} })
			}
			if _, err := f.svc.Apply(context.Background(), r, p.Revision); err == nil {
				t.Fatal("failed apply reported success")
			}
			if late {
				raw, _ := os.ReadFile(file)
				if attempts != 0 || !strings.Contains(string(raw), "npm:external") || strings.Contains(string(raw), "npm:demo") {
					t.Fatalf("overwrote late change or ran install: %s %d", raw, attempts)
				}
				return
			}
			p, err = f.svc.Preview(context.Background(), r)
			if err != nil || p.Blocked || p.Changes[0].Action != "install" {
				t.Fatalf("native failure lost its pending retry: %+v %v", p, err)
			}
			applyPluginRequest(t, f.svc, r)
			if attempts != 2 {
				t.Fatalf("install attempts=%d", attempts)
			}
		})
	}
}

func TestPiFilteredImportPreservesLifecycle(t *testing.T) {
	for _, scope := range []string{"global", "project", "account"} {
		t.Run(scope, func(t *testing.T) {
			f := newPiFixture(t)
			target := "pi"
			if scope == "project" {
				f.svc.ProjectRoot = t.TempDir()
			}
			if scope == "account" {
				target = "pi-work"
				f.svc.Accounts = map[string]Account{target: {Agent: "pi", Dir: filepath.Join(f.home, "account")}}
			}
			file, err := f.svc.piSettingsPath(target)
			if err != nil {
				t.Fatal(err)
			}
			entry := `{"source":"npm:demo@1.2.3", "extensions":["-extensions/a.ts"], "skills":[], "prompts":["!prompts/*.md"], "themes":[], "autoload":false, "opaque":{"integer":9007199254740993,"private":"dummy-private","escaped":"\u0061"}}`
			// Global autoload:false is not a meaningful install; use ordinary global entries.
			if scope != "project" {
				entry = strings.Replace(entry, `"autoload":false, `, "", 1)
			}
			writeTree(t, filepath.Dir(file), map[string]string{filepath.Base(file): `{"top":{"keep":9007199254740993}, "packages":[` + entry + `,"npm:other"]}`})
			var commands []string
			f.svc.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
				if len(args) == 1 && args[0] == "--version" {
					return []byte("1.0.0"), nil
				}
				if len(args) > 1 && args[1] == "--help" {
					return []byte("install remove update --local"), nil
				}
				commands = append(commands, strings.Join(args, " "))
				raw, _, entries, err := readPackageConfig(filepath.Dir(file), file, "packages")
				if err != nil {
					return nil, err
				}
				switch args[0] {
				case "remove":
					if len(entries) != 2 {
						return nil, fmt.Errorf("unexpected entries: %s", raw)
					}
					removed := string(raw)
					if parsePiEntry(0, entries[0]).source == "npm:demo@1.2.3" {
						removed = strings.Replace(removed, string(entries[0])+",", "", 1)
					} else {
						removed = strings.Replace(removed, ","+string(entries[1]), "", 1)
					}
					return nil, os.WriteFile(file, []byte(removed), 0o644)
				case "install":
					// The preserved object must precede native installation; no default-enabled window.
					if len(entries) != 2 || string(entries[1]) != entry {
						return nil, fmt.Errorf("filters were not restored before install: %s", raw)
					}
				case "update":
					// Native update leaves package registrations untouched.
				default:
					return nil, fmt.Errorf("unexpected command %v", args)
				}
				return nil, nil
			}
			r := Request{Action: "import", From: target, Plugin: "npm:demo@1.2.3"}
			before := unchanged(t, file, filepath.Join(f.agentDir, "trust.json"))
			p, err := f.svc.Preview(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			before()
			shown, _ := json.Marshal(p)
			if strings.Contains(string(shown), "dummy-private") || strings.Contains(string(shown), "9007199254740993") {
				t.Fatalf("opaque values reached preview: %s", shown)
			}
			applyPluginRequest(t, f.svc, r)
			before()
			if len(commands) != 0 {
				t.Fatalf("import ran lifecycle commands: %v", commands)
			}
			config, _ := os.ReadFile(f.svc.ConfigPath)
			if strings.Contains(string(config), "dummy-private") {
				t.Fatal("private native fields copied into shared config")
			}
			applyPluginRequest(t, f.svc, Request{Action: "sync"})
			before()
			applyPluginRequest(t, f.svc, Request{Action: "update", Name: "demo"})
			before()
			// Capture current native edits at uninstall, not the old import snapshot.
			rawBefore, _ := os.ReadFile(file)
			latestEntry := strings.Replace(entry, "-extensions/a.ts", "+extensions/b.ts", 1)
			if err := os.WriteFile(file, []byte(strings.Replace(string(rawBefore), entry, latestEntry, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			entry = latestEntry
			before = unchanged(t, file, filepath.Join(f.agentDir, "trust.json"))
			applyPluginRequest(t, f.svc, Request{Action: "disable", Name: "demo"})
			before()
			applyPluginRequest(t, f.svc, Request{Action: "sync"})
			applyPluginRequest(t, f.svc, Request{Action: "enable", Name: "demo"})
			applyPluginRequest(t, f.svc, Request{Action: "sync"})
			raw, _, entries, err := readPackageConfig(filepath.Dir(file), file, "packages")
			if err != nil || len(entries) != 2 || string(entries[1]) != entry || !strings.Contains(string(raw), `"top":{"keep":9007199254740993}`) {
				t.Fatalf("lossy restore: %s %v", raw, err)
			}
			applyPluginRequest(t, f.svc, Request{Action: "remove", Name: "demo"})
		})
	}
}
