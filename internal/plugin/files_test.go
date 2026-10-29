package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFilesListsTheSnapshotAndReadsOnlyInsideIt(t *testing.T) {
	home := t.TempDir()
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	writePluginFile(t, home, "config.yaml", "plugins:\n  packages:\n    demo:\n      bindings:\n        claude:\n          id: demo@skillshare-abc\n          source: /src/demo\n          plugin: demo\n    imported:\n      bindings:\n        claude:\n          id: other@market\n")
	content := filepath.Join(s.snapshotPath(Binding{ID: "demo@skillshare-abc", Source: "/src/demo", Plugin: "demo"}, "claude"), "content")
	for path, body := range map[string]string{"skills/hi/SKILL.md": "# hi", "node_modules/dep/index.js": "x", "../secret": "no"} {
		full := filepath.Join(content, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	b := Binding{ID: "demo@skillshare-abc", Source: "/src/demo", Plugin: "demo"}
	b.Digest, _ = treeDigest(content)
	writePluginFile(t, filepath.Dir(content), ".skillshare-owner", b.Source+"\n"+b.Plugin)
	writePluginFile(t, filepath.Dir(content), ".skillshare-digest", b.Digest)
	seedLocalBinding(t, s, "demo", "claude", b)
	files, err := s.Files("demo")
	if err != nil || !slices.Equal(files, []string{"skills/hi/SKILL.md"}) {
		t.Fatalf("files = %v, %v", files, err)
	}
	if data, err := s.ReadFile("demo", "skills/hi/SKILL.md"); err != nil || string(data) != "# hi" {
		t.Fatalf("read = %q, %v", data, err)
	}
	if _, err := s.ReadFile("demo", "../secret"); err == nil {
		t.Fatal("a path outside the snapshot was read")
	}
	if files, err := s.Files("imported"); err != nil || len(files) != 0 {
		t.Fatalf("an imported plugin has no snapshot: %v, %v", files, err)
	}
}

func TestSaveKeepsTwoSpaceIndent(t *testing.T) {
	s := &Service{ConfigPath: filepath.Join(t.TempDir(), "config.yaml")}
	raw := []byte("ignore:\n  - '**/.git/**'\n")
	if err := os.WriteFile(s.ConfigPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	d, err := decodeDocument(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.intent["demo"] = packageDeclaration{Bindings: map[string]declaration{"claude": {Imported: true}}}
	if err := s.save(d); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(s.ConfigPath)
	want := "ignore:\n  - '**/.git/**'\nplugins:\n  version: 2\n  packages:\n    demo:\n      bindings:\n        claude:\n          imported: true\n"
	if !strings.HasPrefix(string(data), want) {
		t.Errorf("got:\n%s\nwant prefix:\n%s", data, want)
	}
}

// Opening a managed plugin asks which other Agents can take it. Its reviewed snapshot answers,
// so nothing is cloned: with no git on PATH, a clone would fail.
func TestDiscoverManagedReadsTheSnapshotWithoutGit(t *testing.T) {
	var calls []string
	s := accountPluginService(t, &calls)
	source := "https://example.invalid/demo.git"
	digest, err := treeDigest(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	b := Binding{ID: "demo@skillshare-abc", Source: source, Plugin: "demo", Digest: digest, Commit: "abc123"}
	if err := copyTree(fixture(t), filepath.Join(s.snapshotPath(b, "claude"), "content")); err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, filepath.Dir(s.ConfigPath), "config.yaml", "plugins:\n  packages:\n    demo:\n      bindings:\n        claude:\n          id: demo@skillshare-abc\n          source: "+source+"\n          plugin: demo\n          commit: abc123\n          digest: "+digest+"\n")
	writePluginFile(t, s.snapshotPath(b, "claude"), ".skillshare-owner", b.Source+"\n"+b.Plugin)
	writePluginFile(t, s.snapshotPath(b, "claude"), ".skillshare-digest", b.Digest)
	seedLocalBinding(t, s, "demo", "claude", b)
	t.Setenv("PATH", t.TempDir())
	d, err := s.DiscoverManaged(context.Background(), "demo", source, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Source != source || d.Commit != "abc123" || d.Digest != digest || len(d.Candidates) != 1 || !slices.Contains(d.Candidates[0].Targets, "claude-work") || len(calls) != 0 {
		t.Fatalf("discovery = %+v, calls = %v", d, calls)
	}
}

func TestLegacySnapshotReadersRequireLocalProof(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	s.Accounts = map[string]Account{"other": {Agent: "pi", Dir: t.TempDir()}}
	source, _ := repairGitSource(t, "legacy-files")
	applyPluginRequest(t, s, Request{Action: "add", Source: source, SourceRef: "main", Targets: []string{"pi"}})
	otherSource := hostStatePiSource(t)
	applyPluginRequest(t, s, Request{Action: "add", Name: "other", Source: otherSource, Targets: []string{"other"}})
	applyPluginRequest(t, s, Request{Action: "add", Name: "same-target", Source: otherSource, Targets: []string{"pi"}})
	b := readState(t, s).packages["demo"].Bindings["pi"]
	native, _ := s.piSettingsPath("pi")
	// Pi stores local sources relative to settings.json before preserving filters.
	relative, err := filepath.Rel(filepath.Dir(native), b.ID)
	if err != nil {
		t.Fatal(err)
	}
	otherID := readState(t, s).packages["same-target"].Bindings["pi"].ID
	otherRelative, err := filepath.Rel(filepath.Dir(native), otherID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"packages": []any{map[string]any{"source": relative, "extensions": []string{"-extensions/demo.ts"}}, otherRelative}})
	writePluginFile(t, filepath.Dir(native), filepath.Base(native), string(raw))
	applyPluginRequest(t, s, Request{Action: "import", Name: "demo", From: "pi", Plugin: b.ID})
	b = readState(t, s).packages["demo"].Bindings["pi"]
	if b.PiRegistration == "" {
		t.Fatal("fixture lacks private native registration proof")
	}
	if !slices.ContainsFunc(s.host(context.Background(), "pi").Installed, func(i Installed) bool { return i.ID == otherID }) {
		t.Fatal("fixture lacks the unrelated same-target native installation")
	}
	private, err := s.piRegistrationPath(b.PiRegistration)
	if err != nil {
		t.Fatal(err)
	}
	run := s.Run
	queries := 0
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		queries++
		return run(ctx, dir, env, bin, args...)
	}
	t.Setenv("PATH", t.TempDir()) // Current and proven legacy snapshots must work offline.
	checkReaders := func(t *testing.T, service *Service, want bool) {
		t.Helper()
		files, err := service.Files("demo")
		if err != nil || slices.Contains(files, "extensions/demo.ts") != want {
			t.Errorf("files=%v %v, want snapshot=%v", files, err, want)
		}
		body, err := service.ReadFile("demo", "extensions/demo.ts")
		if want {
			if err != nil || string(body) != "export default () => {}" {
				t.Errorf("read=%q %v", body, err)
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("unverified snapshot read: %q %v", body, err)
		}
		found, err := service.DiscoverManaged(context.Background(), "demo", source, "main", "")
		if want {
			if err != nil || found.Commit != b.Commit || found.Digest != b.Digest || len(found.Candidates) != 1 {
				t.Errorf("snapshot discovery=%+v %v", found, err)
			}
		} else if err == nil || found != nil {
			t.Errorf("unverified snapshot answered discovery: %+v %v", found, err)
		}
	}
	checkReaders(t, s, true)
	if queries != 0 {
		t.Fatal("current receipts caused native queries")
	}
	raw, err = yaml.Marshal(map[string]any{"plugins": map[string]any{"packages": readState(t, s).packages}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.ConfigPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(s.statePath()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Packages(); err != nil {
		t.Fatal(err)
	}
	if queries != 0 {
		t.Fatal("quick legacy Packages queried native hosts")
	}
	d := readState(t, s)
	s.adoptLegacy(d, func(target string) Host {
		if target != "pi" {
			return Host{Error: "outside fixture scope"}
		}
		return s.host(context.Background(), target)
	}, "demo")
	if got := d.packages["demo"].Bindings["pi"]; got.ID != b.ID || got.Digest != b.Digest {
		t.Fatal("fixture lacks verified local ownership")
	}
	if d.state.Bindings["same-target"]["pi"].Applied != nil || d.state.Bindings["other"]["other"].Applied != nil {
		t.Fatal("scoped proof adopted an unrelated package")
	}
	snapshot := s.snapshotPath(b, "pi")
	for _, mode := range []string{"valid", "absent", "foreign", "modified", "private"} {
		t.Run(mode, func(t *testing.T) {
			service := *s
			var altered string
			var replacement []byte
			switch mode {
			case "absent":
				altered = native
				replacement = []byte(`{"packages":[]}`)
			case "foreign":
				service.StateDir = t.TempDir()
				if err := copyTree(snapshot, service.snapshotPath(b, "pi")); err != nil {
					t.Fatal(err)
				}
			case "modified":
				altered = filepath.Join(snapshot, "content/extensions/demo.ts")
				replacement = []byte("local edit")
			case "private":
				altered = private
				replacement = []byte("modified")
			}
			if altered != "" {
				original, err := os.ReadFile(altered)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(altered, replacement, 0600); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.WriteFile(altered, original, 0600); err != nil {
						t.Error(err)
					}
				})
			}
			paths := []string{service.ConfigPath, native, private}
			for _, dir := range []string{snapshot, service.snapshotPath(b, "pi")} {
				paths = append(paths, filepath.Join(dir, ".skillshare-owner"), filepath.Join(dir, ".skillshare-digest"), filepath.Join(dir, "content/extensions/demo.ts"))
			}
			same := unchanged(t, paths...)
			info, _ := os.Stat(service.ConfigPath)
			before := len(commands)
			// Filtered Pi entries also check the supported-version gate. Measure
			// one normal scoped inventory read, rather than counting its commands.
			queries = 0
			_ = service.host(context.Background(), "pi")
			perHost := queries
			queries = 0
			checkReaders(t, &service, mode == "valid")
			same()
			after, _ := os.Stat(service.ConfigPath)
			if !info.ModTime().Equal(after.ModTime()) || len(commands) != before {
				t.Fatal("snapshot read changed shared/native state")
			}
			if queries > 3*perHost {
				t.Fatalf("snapshot readers queried unrelated or repeated hosts: %d", queries)
			}
		})
	}
}
