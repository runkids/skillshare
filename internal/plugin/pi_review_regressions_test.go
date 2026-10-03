package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPiProjectDefaultDoesNotRevealShadowedResources(t *testing.T) {
	for _, earlier := range []string{"replacement", "delta", "unknown"} {
		t.Run(earlier, func(t *testing.T) {
			f, root := projectFixture(t)
			source := f.pkg
			first := map[string]any{"source": source, "extensions": []string{"-extensions/b.ts"}, "skills": []string{}, "prompts": []string{}, "themes": []string{}}
			if earlier == "delta" {
				first["autoload"] = false
			}
			if earlier == "unknown" {
				first["source"] = "https://example.invalid/org/pkg.git?opaque=dummy"
			}
			f.global(map[string]any{"packages": []any{source}})
			f.writeJSON(f.projectFile(root), map[string]any{"packages": []any{first, map[string]any{"source": source, "autoload": false, "extensions": []string{"-extensions/a.ts"}}}})
			before := f.view("pi")
			assertRows(t, selections(before.Packages[1]), "extensions/a.ts:skipped", "extensions/b.ts:loads", "extensions/c.ts:loads")
			settingsSame := unchanged(t, f.settingsPath(), filepath.Join(f.agentDir, "trust.json"))
			change := PiExtensionChange{Scope: "project", Index: 1, Source: source, Path: "extensions/a.ts", Action: "default"}
			plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{change})
			if err != nil {
				t.Fatal(err)
			}
			if plan.Entries[0].Removed {
				t.Fatal("preview would expose an earlier entry's resource filters")
			}
			if len(plan.Rows) != 1 || plan.Rows[0].Path != change.Path {
				t.Fatalf("preview changed an unrequested row: %+v", plan.Rows)
			}
			if _, err = f.svc.ApplyPiExtensions(context.Background(), "pi", []PiExtensionChange{change}, plan.Revision); err != nil {
				t.Fatal(err)
			}
			settingsSame()
			got := readJSON(t, f.projectFile(root))["packages"].([]any)
			encoded, _ := json.Marshal(first)
			var wantFirst any
			_ = json.Unmarshal(encoded, &wantFirst)
			if len(got) != 2 || !reflect.DeepEqual(got[0], wantFirst) || !reflect.DeepEqual(got[1], map[string]any{"source": source, "autoload": false}) {
				t.Fatalf("precedence or other resources changed: %v", got)
			}
			assertRows(t, selections(f.view("pi").Packages[1]), "extensions/a.ts:loads", "extensions/b.ts:loads", "extensions/c.ts:loads")
		})
	}
}

func TestPiAutoloadUsesOnlyExplicitFalse(t *testing.T) {
	for _, value := range []string{"null", "true", "false", `"false"`, "0", "[]", "{}", " false "} {
		for _, scope := range []string{"global", "project"} {
			t.Run(scope+"/"+value, func(t *testing.T) {
				f, root := projectFixture(t)
				source, _ := json.Marshal(f.pkg)
				entry := `{"source":` + string(source) + `,"autoload":` + value + `,"extensions":["+extensions/a.ts"]}`
				file := f.settingsPath()
				if scope == "project" {
					f.global(map[string]any{"packages": []any{f.pkg}})
					file = f.projectFile(root)
				}
				writeTree(t, filepath.Dir(file), map[string]string{"settings.json": `{"packages":[` + entry + `]}`})
				pkg := findPackage(t, f.view("pi"), scope, f.pkg)
				if strings.TrimSpace(value) == "false" {
					if scope == "global" && pkg.Problem != "autoloadGlobal" {
						t.Fatalf("explicit false ignored: %+v", pkg)
					}
					if scope == "project" && pkg.Shape != "delta" {
						t.Fatalf("explicit false ignored: %+v", pkg)
					}
				} else {
					if pkg.Problem != "" || pkg.Shape == "delta" || pkg.Shape == "deltaOnly" {
						t.Fatalf("non-false interpreted as an override: %+v", pkg)
					}
					assertRows(t, selections(pkg), "extensions/a.ts:loads", "extensions/b.ts:loads", "extensions/c.ts:loads")
				}
			})
		}
	}
}

func TestPiFilteredBatchRestoreAcceptsOnlyItsOwnWrites(t *testing.T) {
	for _, scope := range []string{"global", "project", "account"} {
		for _, mode := range []string{"normal", "external", "native-failure"} {
			t.Run(fmt.Sprintf("%s/%s", scope, mode), func(t *testing.T) {
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
				alpha := `{"source":"npm:alpha","extensions":["-a.ts"],"skills":[],"opaque":{"keep":9007199254740993}}`
				beta := `{"source":"npm:beta","extensions":["-b.ts"],"prompts":[],"opaque":"kept"}`
				writeTree(t, filepath.Dir(file), map[string]string{"settings.json": `{"top":{"keep":9007199254740993},"packages":[` + alpha + `,` + beta + `,"npm:other"]}`})
				for _, id := range []string{"npm:alpha", "npm:beta"} {
					applyPluginRequest(t, f.svc, Request{Action: "import", From: target, Plugin: id})
				}
				absent := `{"top":{"keep":9007199254740993},"packages":["npm:other"]}`
				if err := os.WriteFile(file, []byte(absent), 0600); err != nil {
					t.Fatal(err)
				}
				var installs []string
				f.svc.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
					if args[0] == "--version" {
						return []byte("1.0.0"), nil
					}
					if len(args) > 1 && args[1] == "--help" {
						return []byte("install remove update --local"), nil
					}
					if args[0] != "install" {
						return nil, fmt.Errorf("unexpected command %v", args)
					}
					installs = append(installs, args[1])
					raw, _, entries, err := readPackageConfig(filepath.Dir(file), file, "packages")
					if err != nil {
						return nil, err
					}
					want := alpha
					if args[1] == "npm:beta" {
						want = beta
					}
					if string(entries[len(entries)-1]) != want {
						return nil, fmt.Errorf("object not restored before install: %s", raw)
					}
					if mode == "native-failure" && len(installs) == 1 {
						return nil, fmt.Errorf("simulated native failure")
					}
					if mode == "external" && len(installs) == 1 {
						return nil, os.WriteFile(file, []byte(strings.Replace(string(raw), `"top":`, `"external":true,"top":`, 1)), 0600)
					}
					return nil, nil
				}
				r := Request{Action: "sync"}
				p, err := f.svc.Preview(context.Background(), r)
				if err != nil || p.Blocked {
					t.Fatalf("preview %+v %v", p, err)
				}
				_, err = f.svc.Apply(context.Background(), r, p.Revision)
				raw, _ := os.ReadFile(file)
				if mode == "external" {
					if err == nil || len(installs) != 1 || !strings.Contains(string(raw), `"external":true`) || strings.Contains(string(raw), beta) {
						t.Fatalf("external write accepted: err=%v installs=%v raw=%s", err, installs, raw)
					}
				} else if (mode == "normal" && err != nil || mode == "native-failure" && (err == nil || !strings.Contains(err.Error(), "simulated native failure"))) || len(installs) != 2 || string(raw) != strings.Replace(absent, `["npm:other"]`, `["npm:other",`+alpha+`,`+beta+`]`, 1) {
					t.Fatalf("batch treated its own write as stale: %v %v %s", err, installs, raw)
				}
				if mode == "native-failure" {
					d, loadErr := f.svc.load()
					if loadErr != nil || d.packages["alpha"].Bindings[target].Pending != "install" || d.packages["beta"].Bindings[target].Pending != "" {
						t.Fatalf("partial failure lost retry or successful result: %+v %v", d, loadErr)
					}
				}
			})
		}
	}
}
