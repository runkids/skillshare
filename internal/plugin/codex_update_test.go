package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func codexUpdateFixture(t *testing.T, enabled bool) (*Service, string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	native := filepath.Join(home, "custom-codex")
	t.Setenv("CODEX_HOME", native)
	source := fixture(t)
	writeFile(t, source, ".agents/plugins/marketplace.json", `{"name":"team","owner":{"name":"test"},"plugins":[{"name":"demo","source":"./","policy":{"installation":"AVAILABLE","authentication":"ON_INSTALL"}}]}`)
	writeFile(t, source, ".codex-plugin/plugin.json", `{"name":"demo","version":"1.0.1","skills":"./skills"}`)
	writeFile(t, native, "config.toml", fmt.Sprintf("# preserve layout\n[marketplaces.team]\nsource_type = \"local\"\nsource = %q\n\n[plugins.\"demo@team\"]\nenabled = %t # native choice\n", source, enabled))
	writeFile(t, native, "plugins/cache/team/demo/1.0.0/.codex-plugin/plugin.json", `{"name":"demo","version":"1.0.0"}`)
	writeFile(t, native, "plugins/cache/team/demo/1.0.0/skills/demo/SKILL.md", "old content")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	writeFile(t, home, "config.yaml", "plugins:\n  packages:\n    demo:\n      bindings:\n        codex:\n          id: demo@team\n          version: 1.0.0\n")
	s.Run = func(_ context.Context, _ string, _ []string, bin string, args ...string) ([]byte, error) {
		if bin != "codex" {
			return nil, fmt.Errorf("unexpected binary %s", bin)
		}
		command := strings.Join(args, " ")
		if command == "--version" {
			return []byte("codex-cli 0.159.3"), nil
		}
		if strings.HasSuffix(command, "--help") {
			return []byte("plugin add upgrade --json --config -c"), nil
		}
		if command == "plugin marketplace list --json" {
			return json.Marshal(map[string]any{"marketplaces": []any{map[string]any{"name": "team", "root": source, "marketplaceSource": map[string]string{"sourceType": "local", "source": source}}}})
		}
		if command == "plugin list --json" {
			var cfg struct {
				Plugins map[string]struct{ Enabled bool }
			}
			data, err := os.ReadFile(filepath.Join(native, "config.toml"))
			if err != nil {
				return nil, err
			}
			if err := toml.Unmarshal(data, &cfg); err != nil {
				return nil, err
			}
			entries, err := os.ReadDir(filepath.Join(native, "plugins/cache/team/demo"))
			if err != nil {
				return nil, err
			}
			version := ""
			for _, entry := range entries {
				if entry.IsDir() {
					version = entry.Name()
				}
			}
			return json.Marshal(map[string]any{"installed": []any{map[string]any{"pluginId": "demo@team", "installed": true, "enabled": cfg.Plugins["demo@team"].Enabled, "version": version}}})
		}
		return nil, fmt.Errorf("unexpected native mutation: %s", command)
	}
	return s, source, native
}

func TestCodexUpdatePreviewImported(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			s, _, native := codexUpdateFixture(t, enabled)
			before, _ := os.ReadFile(filepath.Join(native, "config.toml"))
			plan, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo", Targets: []string{"codex"}})
			if err != nil {
				t.Fatal(err)
			}
			if plan.Blocked || len(plan.Changes) != 1 || plan.Changes[0].Action != "update" {
				t.Fatalf("update blocked: %+v", plan)
			}
			if plan.Changes[0].Binding.Version != "1.0.1" || plan.Changes[0].Binding.Source != "" {
				t.Fatalf("wrong imported binding: %+v", plan.Changes[0])
			}
			if !strings.Contains(plan.Changes[0].Message, "plugin add") {
				t.Fatalf("native operation missing from preview: %+v", plan.Changes[0])
			}
			after, _ := os.ReadFile(filepath.Join(native, "config.toml"))
			if string(before) != string(after) {
				t.Fatal("preview mutated config")
			}
		})
	}
}

func TestCodexUpdatePreviewRejectsImportedRefOverride(t *testing.T) {
	s, _, _ := codexUpdateFixture(t, true)
	p, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo", SourceRef: "other", Targets: []string{"codex"}})
	if err != nil || p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "skip" {
		t.Fatalf("imported native ref override was not skipped: %+v %v", p, err)
	}
}

func TestCodexUpdatePreviewCheckImported(t *testing.T) {
	s, _, _ := codexUpdateFixture(t, true)
	p, err := s.Preview(context.Background(), Request{Action: "check", Name: "demo", Targets: []string{"codex"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Blocked || p.Changes[0].Action != "update-available" || p.Changes[0].Binding.Version != "1.0.1" {
		t.Fatalf("imported change not discovered: %+v", p)
	}
}

func TestCodexUpdatePreviewSameVersionContent(t *testing.T) {
	s, _, native := codexUpdateFixture(t, true)
	if err := os.Rename(filepath.Join(native, "plugins/cache/team/demo/1.0.0"), filepath.Join(native, "plugins/cache/team/demo/1.0.1")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Dir(s.ConfigPath), "config.yaml", "plugins:\n  packages:\n    demo:\n      bindings:\n        codex:\n          id: demo@team\n          version: 1.0.1\n")
	p, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo", Targets: []string{"codex"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Blocked || p.Changes[0].Action != "update" {
		t.Fatalf("unreviewed same-version content treated as unchanged: %+v", p)
	}
	if p.Changes[0].Binding.Digest == "" {
		t.Fatal("reviewed imported content has no digest")
	}
}

func TestCodexUpdatePreviewAccountAndStaleEvidence(t *testing.T) {
	s, _, native := codexUpdateFixture(t, false)
	s.Accounts = map[string]Account{"codex-work": {Agent: "codex", Dir: native}}
	data, _ := os.ReadFile(s.ConfigPath)
	if err := os.WriteFile(s.ConfigPath, []byte(strings.ReplaceAll(string(data), "        codex:", "        codex-work:")), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "unrelated"))
	r := Request{Action: "update", Name: "demo", Targets: []string{"codex-work"}}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if p.Blocked {
		t.Fatalf("account blocked: %+v", p)
	}
	writeFile(t, native, "plugins/cache/team/demo/1.0.0/notes.txt", "external edit")
	if _, err := s.Apply(context.Background(), r, p.Revision); err == nil || !strings.Contains(err.Error(), "state changed") {
		t.Fatalf("stale cache accepted: %v", err)
	}
	p, err = s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	config, _ := os.ReadFile(filepath.Join(native, "config.toml"))
	writeFile(t, native, "config.toml", string(config)+"\n# concurrent comment\n")
	if _, err := s.Apply(context.Background(), r, p.Revision); err == nil || !strings.Contains(err.Error(), "state changed") {
		t.Fatalf("stale config accepted: %v", err)
	}
}

func TestCodexUpdatePreviewUnknownNativeState(t *testing.T) {
	for _, field := range []string{"enabled", "installed", "version"} {
		t.Run(field, func(t *testing.T) {
			s, _, _ := codexUpdateFixture(t, false)
			original := s.Run
			s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
				data, err := original(ctx, dir, env, bin, args...)
				if strings.Join(args, " ") == "plugin list --json" && err == nil {
					var d map[string][]map[string]any
					if err := json.Unmarshal(data, &d); err != nil {
						return nil, err
					}
					delete(d["installed"][0], field)
					return json.Marshal(d)
				}
				return data, err
			}
			p, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo", Targets: []string{"codex"}})
			if err != nil || p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "skip" {
				t.Fatalf("missing %s did not leave the update skipped: %+v %v", field, p, err)
			}
		})
	}
}

func TestCodexUpdatePreviewRejectsNativeInlineRewrite(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			s, source, home := codexUpdateFixture(t, enabled)
			config := fmt.Sprintf("plugins = { \"demo@team\" = { enabled = %t } }\n[marketplaces.team]\nsource_type = 'local'\nsource = %q\n", enabled, source)
			writeFile(t, home, "config.toml", config)
			p, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo", Targets: []string{"codex"}})
			if err != nil || p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "skip" {
				t.Fatalf("native inline rewrite did not leave the update skipped: %+v %v", p, err)
			}
			after, _ := os.ReadFile(filepath.Join(home, "config.toml"))
			if string(after) != config {
				t.Fatal("preview changed native config")
			}
		})
	}
}

func TestCodexUpdatePreviewRejectsMissingInstalledCache(t *testing.T) {
	for _, missing := range []string{"marketplace", "plugin", "version"} {
		t.Run(missing, func(t *testing.T) {
			s, _, home := codexUpdateFixture(t, false)
			writeFile(t, home, "plugins/cache/team/other/2.0.0/keep", "unrelated")
			path := filepath.Join(home, "plugins/cache/team")
			if missing != "marketplace" {
				path = filepath.Join(path, "demo")
			}
			if missing == "version" {
				path = filepath.Join(path, "1.0.0")
			}
			if err := os.RemoveAll(path); err != nil {
				t.Fatal(err)
			}
			original := s.Run
			s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
				if strings.Join(args, " ") == "plugin list --json" {
					return []byte(`{"installed":[{"pluginId":"demo@team","installed":true,"enabled":false,"version":"1.0.0"}]}`), nil
				}
				return original(ctx, dir, env, bin, args...)
			}
			p, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo", Targets: []string{"codex"}})
			if err != nil || p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "skip" || !strings.Contains(p.Changes[0].Message, "cache is missing") {
				t.Fatalf("inconsistent native cache was not skipped: %+v %v", p, err)
			}
		})
	}
}
