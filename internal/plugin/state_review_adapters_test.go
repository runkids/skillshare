package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedRefReconcileAndAccountChangeAcrossAdapters(t *testing.T) {
	for _, agent := range []string{"codex", "opencode", "omp"} {
		t.Run(agent, func(t *testing.T) {
			native := &fakeAgents{version: "1.0.0"}
			s := isolatedAgentService(t, native)
			run := s.Run
			s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
				if bin == "opencode" && len(args) == 1 && args[0] == "--version" {
					return []byte("1.18.32"), nil
				}
				return run(ctx, dir, env, bin, args...)
			}
			target := "work"
			if agent == "omp" {
				target = "omp"
			} else {
				s.Accounts = map[string]Account{target: {Agent: agent, Dir: t.TempDir()}}
			}
			source := fixture(t)
			writePluginFile(t, source, "package.json", `{"name":"demo","version":"1.0.0","main":"index.js","dependencies":{"@opencode-ai/plugin":"1"}}`)
			writePluginFile(t, source, "index.js", "export default async () => ({})")
			git := func(args ...string) string {
				t.Helper()
				cmd := exec.Command("git", args...)
				cmd.Dir = source
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %s %v", args, out, err)
				}
				return strings.TrimSpace(string(out))
			}
			git("init", "-b", "main")
			git("add", ".")
			git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "one")
			git("checkout", "-b", "next")
			writePluginFile(t, source, "skills/demo/SKILL.md", "---\nname: demo\ndescription: Changed fixture\n---\nChanged.")
			git("add", ".")
			git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "two")
			next := git("rev-parse", "HEAD")
			url := "https://fixture.invalid/" + agent + ".git"
			cfg := filepath.Join(t.TempDir(), "gitconfig")
			writePluginFile(t, filepath.Dir(cfg), filepath.Base(cfg), "[url \"file://"+source+"\"]\n  insteadOf = "+url+"\n")
			t.Setenv("GIT_CONFIG_GLOBAL", cfg)
			applyPluginRequest(t, s, Request{Action: "add", Source: url, SourceRef: "main", Targets: []string{target}})
			original := hostPackages(t, s)["demo"].Bindings[target]
			raw, err := os.ReadFile(s.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			// The bytes produced by a remote update --source-ref, without its local state.
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), strings.ReplaceAll(string(raw), "source_ref: main", "source_ref: next"))
			same := unchanged(t, s.ConfigPath)
			if b := hostPackages(t, s)["demo"].Bindings[target]; !b.RefChanged || b.ID != original.ID {
				t.Fatalf("lost local ref projection: %+v", b)
			}
			p, err := s.Preview(context.Background(), Request{Action: "sync"})
			if err != nil {
				t.Fatal(err)
			}
			if agent == "omp" {
				if !p.Blocked || len(p.Changes) != 1 || p.Changes[0].MessageKey != "plugins.error.ompNativeDestructive" {
					t.Fatalf("OMP lost its non-destructive update guard: %+v", p)
				}
				if _, err := s.Apply(context.Background(), Request{Action: "sync"}, p.Revision); err == nil {
					t.Fatal("OMP destructive reconcile applied")
				}
				if b := hostPackages(t, s)["demo"].Bindings[target]; b.ID != original.ID || b.Commit != original.Commit || !b.RefChanged {
					t.Fatalf("blocked reconcile replaced OMP receipt: %+v", b)
				}
			} else {
				if p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "update" {
					t.Fatalf("ref not reconciled: %+v", p)
				}
				applyPluginRequest(t, s, Request{Action: "sync"})
				if b := hostPackages(t, s)["demo"].Bindings[target]; b.RefChanged || b.SourceRef != "next" || b.Commit != next {
					t.Fatalf("ref receipt not updated: %+v", b)
				}
			}
			same()

			before := len(native.commands)
			if agent == "omp" {
				t.Setenv("HOME", t.TempDir())
			} else {
				s.Accounts[target] = Account{Agent: agent, Dir: t.TempDir()}
			}
			if b := hostPackages(t, s)["demo"].Bindings[target]; !b.ContextChanged {
				t.Fatal("account change not projected")
			}
			req := Request{Action: "sync"}
			p, err = s.Preview(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if !p.Blocked || len(p.Changes) != 1 || p.Changes[0].MessageKey != "plugins.note.accountChanged" {
				t.Fatalf("account change not guarded: %+v", p)
			}
			if _, err := s.Apply(context.Background(), req, p.Revision); err == nil {
				t.Fatal("account change applied")
			}
			same()
			if len(native.commands) != before {
				t.Fatal("account guard ran a native mutation")
			}
		})
	}
}

func TestImportsMatchLocalPathsAndDefaultRecipeCandidate(t *testing.T) {
	for _, agent := range []string{"claude", "pi", "opencode", "opencode-nested", "opencode-explicit", "opencode-unrelated-entry", "opencode-missing-entry"} {
		t.Run(agent, func(t *testing.T) {
			native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
			s := isolatedAgentService(t, native)
			source := fixture(t)
			id := "demo@team"
			explicit := ""
			missing := agent == "opencode-missing-entry"
			reject := agent == "opencode-unrelated-entry" || missing
			switch agent {
			case "pi":
				var calls []string
				s = fakePiNpm(t, &calls)
				source = hostStatePiSource(t)
				id = source
				file, _ := s.piSettingsPath("pi")
				writePluginFile(t, filepath.Dir(file), filepath.Base(file), `{"packages":["`+id+`"]}`)
			case "opencode", "opencode-nested", "opencode-explicit", "opencode-unrelated-entry", "opencode-missing-entry":
				noSDK := agent == "opencode-explicit"
				nested := agent == "opencode-nested" || noSDK
				agent = "opencode"
				entry := "index.js"
				if nested {
					entry = "dist/plugin.js"
				}
				if noSDK {
					source = t.TempDir()
					explicit = entry
					writePluginFile(t, source, "package.json", `{"name":"demo","main":"`+entry+`"}`)
				} else {
					writePluginFile(t, source, "package.json", `{"name":"demo","main":"`+entry+`","dependencies":{"@opencode-ai/plugin":"1"}}`)
				}
				writePluginFile(t, source, entry, "export default async () => ({})")
				id = fileURL(filepath.Join(source, entry))
				if reject {
					if !missing {
						writePluginFile(t, source, "plugins/demo.js", "export default async () => ({})")
					}
					id = fileURL(filepath.Join(source, "plugins/demo.js"))
				}
				file, _ := s.openCodeConfigPath("opencode")
				writePluginFile(t, filepath.Dir(file), filepath.Base(file), `{"plugin":["`+id+`"]}`)
				run := s.Run
				s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
					if bin == "opencode" && len(args) == 1 && args[0] == "--version" {
						return []byte("1.18.32"), nil
					}
					return run(ctx, dir, env, bin, args...)
				}
			}
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  version: 2\n  packages:\n    alias:\n      source: "+source+"\n      entry: "+explicit+"\n      bindings:\n        "+agent+": {}\n")
			same := unchanged(t, s.ConfigPath)
			req := Request{Action: "import", From: agent, Plugin: id, Name: "alias"}
			p, err := s.Preview(context.Background(), req)
			if reject {
				if err == nil {
					t.Fatal("native file without a verified package entry was accepted as its package")
				}
				same()
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.Blocked || len(p.Changes) != 1 || p.Changes[0].MessageKey != "plugins.note.localImportRecorded" {
				t.Fatalf("matching recipe import rejected: %+v", p)
			}
			applyPluginRequest(t, s, req)
			same()
			if b := hostPackages(t, s)["alias"].Bindings[agent]; !b.Imported || b.ID != id {
				t.Fatalf("import not recorded: %+v", b)
			}
		})
	}
}

func TestHostStateAgentFixtureIsolatesAmbientOpenCodeConfig(t *testing.T) {
	for _, override := range []string{"xdg", "file", "directory", "inline"} {
		t.Run(override, func(t *testing.T) {
			ambient := t.TempDir()
			sentinel := filepath.Join(ambient, "opencode", "opencode.json")
			marker := `{"sentinel":"ambient config must remain untouched"}`
			writePluginFile(t, filepath.Dir(sentinel), filepath.Base(sentinel), marker)
			t.Setenv("XDG_CONFIG_HOME", ambient)
			for _, key := range []string{"OPENCODE_CONFIG", "OPENCODE_CONFIG_DIR", "OPENCODE_CONFIG_CONTENT"} {
				t.Setenv(key, "")
			}
			switch override {
			case "file":
				t.Setenv("OPENCODE_CONFIG", sentinel)
			case "directory":
				t.Setenv("OPENCODE_CONFIG_DIR", filepath.Dir(sentinel))
			case "inline":
				t.Setenv("OPENCODE_CONFIG_CONTENT", `{"plugin":[]}`)
			}
			s := isolatedAgentService(t, &fakeAgents{})
			file, err := s.openCodeConfigPath("opencode")
			if err != nil {
				t.Fatalf("fixture inherited ambient OpenCode overrides: %v", err)
			}
			want := filepath.Join(filepath.Dir(s.ConfigPath), ".config", "opencode", "opencode.json")
			if file != want {
				t.Fatalf("fixture native config escaped its isolated HOME: got %s, want %s", file, want)
			}
			writePluginFile(t, filepath.Dir(file), filepath.Base(file), `{"plugin":[]}`)
			raw, err := os.ReadFile(sentinel)
			if err != nil {
				t.Fatal(err)
			}
			if string(raw) != marker {
				t.Fatal("fixture changed the ambient config sentinel")
			}
		})
	}
}
