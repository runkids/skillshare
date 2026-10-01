package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Opt-in native filesystem contract. No model requests, credentials, or existing
// user configuration are needed; run in the repository devcontainer.
func TestCodexUpdateNativeE2E(t *testing.T) {
	if os.Getenv("SKILLSHARE_CODEX_UPDATE_E2E") != "1" {
		t.Skip("set SKILLSHARE_CODEX_UPDATE_E2E=1 with native Codex installed")
	}
	if _, err := exec.LookPath("codex"); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"local", "git", "managed", "git-rollback", "managed-policy", "git-ref-race"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			native := filepath.Join(home, "codex")
			t.Setenv("CODEX_HOME", native)
			if err := os.MkdirAll(native, 0700); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(home, "source")
			writeFile(t, source, ".agents/plugins/marketplace.json", `{"name":"team","owner":{"name":"test"},"plugins":[{"name":"demo","source":"./demo","policy":{"installation":"AVAILABLE","authentication":"ON_INSTALL"}},{"name":"other","source":"./other","policy":{"installation":"AVAILABLE","authentication":"ON_INSTALL"}},{"name":"uncached","source":"./uncached","policy":{"installation":"AVAILABLE","authentication":"ON_INSTALL"}}]}`)
			manifest := func(v string) {
				for _, name := range []string{"demo", "other", "uncached"} {
					writeFile(t, source, name+"/.codex-plugin/plugin.json", fmt.Sprintf(`{"name":%q,"version":%q,"skills":"./skills"}`, name, v))
					writeFile(t, source, name+"/skills/hello/SKILL.md", "---\nname: hello\ndescription: Test fixture\n---\nVersion "+v)
				}
			}
			manifest("1.0.0")
			command := func(bin string, args ...string) []byte {
				t.Helper()
				cmd := exec.Command(bin, args...)
				cmd.Dir = source
				data, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("%s %v: %v\n%s", bin, args, err, data)
				}
				return data
			}
			isGit := strings.HasPrefix(kind, "git")
			if isGit {
				command("git", "init", "-b", "main")
				command("git", "add", ".")
				command("git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "initial fixture")
			}
			s := &Service{ConfigPath: filepath.Join(home, "skillshare.yaml"), StateDir: filepath.Join(home, "state")}
			id := "demo@team"
			if strings.HasPrefix(kind, "managed") {
				r := Request{Action: "add", Name: "demo", Plugin: "demo", Source: source, Targets: []string{"codex"}}
				p, err := s.Preview(context.Background(), r)
				if err != nil || p.Blocked {
					t.Fatalf("managed add preview %+v %v", p, err)
				}
				if _, err := s.Apply(context.Background(), r, p.Revision); err != nil {
					t.Fatal(err)
				}
				id = p.Changes[0].ID
			} else {
				if isGit {
					writeFile(t, native, "config.toml", fmt.Sprintf("[marketplaces.team]\nsource_type = 'git'\nsource = %q\nref_name = 'main'\n", source))
					command("codex", "plugin", "marketplace", "upgrade", "team", "--json")
				} else {
					command("codex", "plugin", "marketplace", "add", source, "--json")
				}
				command("codex", "plugin", "add", id, "--json")
				command("codex", "plugin", "add", "other@team", "--json")
				r := Request{Action: "import", Name: "demo", From: "codex", Plugin: id}
				p, err := s.Preview(context.Background(), r)
				if err != nil || p.Blocked {
					t.Fatalf("import preview %+v %v", p, err)
				}
				if _, err := s.Apply(context.Background(), r, p.Revision); err != nil {
					t.Fatal(err)
				}
			}
			cfgPath := filepath.Join(native, "config.toml")
			cfg, _ := os.ReadFile(cfgPath)
			// Native Codex has no disable command: its existing user configuration
			// is the documented enablement source, edited only in this isolated home.
			cfg = []byte(strings.Replace(string(cfg), "enabled = true", "enabled = false", 1))
			if isGit {
				cfg = append(cfg, []byte("\n[plugins.\"uncached@team\"]\nenabled = false\n")...)
			}
			cfg = append(cfg, []byte("\n[[flags]]\nenabled = false # unrelated array table\n")...)
			if err := os.WriteFile(cfgPath, cfg, 0600); err != nil {
				t.Fatal(err)
			}
			sibling := filepath.Join(native, "plugins/cache/team/other")
			beforeSibling, err := codexStateDigest(sibling)
			if err != nil {
				t.Fatal(err)
			}
			manifest("1.0.1")
			if kind == "managed-policy" {
				data, err := os.ReadFile(filepath.Join(source, ".agents/plugins/marketplace.json"))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, source, ".agents/plugins/marketplace.json", strings.Replace(string(data), "AVAILABLE", "NOT_AVAILABLE", 1))
			}
			if isGit {
				command("git", "add", ".")
				command("git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "update fixture")
			}
			_, market, _ := strings.Cut(id, "@")
			cache := filepath.Join(native, "plugins/cache", market)
			beforeCache, err := codexStateDigest(cache)
			if err != nil {
				t.Fatal(err)
			}
			marketRoot := filepath.Join(native, ".tmp/marketplaces/team")
			beforeRoot, err := codexStateDigest(marketRoot)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "git-rollback" || kind == "git-ref-race" {
				s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
					if kind == "git-rollback" && len(args) > 2 && args[0] == "plugin" && args[1] == "add" && args[2] == id {
						return nil, fmt.Errorf("injected native add failure after marketplace upgrade")
					}
					if kind == "git-ref-race" && strings.Join(args, " ") == "plugin marketplace upgrade team --json" {
						writeFile(t, source, "notes.txt", "ref advanced with unchanged selected plugin and catalog")
						command("git", "add", ".")
						command("git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "advance during update")
					}
					return runCommand(ctx, dir, env, bin, args...)
				}
			}
			r := Request{Action: "update", Name: "demo", Targets: []string{"codex"}}
			p, err := s.Preview(context.Background(), r)
			if err != nil || p.Blocked {
				t.Fatalf("update preview %+v %v", p, err)
			}
			_, err = s.Apply(context.Background(), r, p.Revision)
			if kind == "git-rollback" || kind == "managed-policy" || kind == "git-ref-race" {
				if err == nil {
					t.Fatal("native failure or denied installation policy accepted")
				}
				afterCache, e := codexStateDigest(cache)
				if e != nil || afterCache != beforeCache {
					t.Fatalf("cache rollback differs: %v; update %v", e, err)
				}
				afterRoot, e := codexStateDigest(marketRoot)
				if e != nil || afterRoot != beforeRoot {
					t.Fatalf("marketplace rollback differs: %v; update %v", e, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err := s.verifyCodexInstalled(context.Background(), "codex", id, "1.0.1", false); err != nil {
					t.Fatal(err)
				}
			}
			afterSibling, err := codexStateDigest(sibling)
			if err != nil || afterSibling != beforeSibling {
				t.Fatalf("unselected native cache changed: %v", err)
			}
			if isGit {
				if _, err := os.Stat(filepath.Join(native, "plugins/cache/team/uncached")); !os.IsNotExist(err) {
					t.Fatalf("uncached sibling was materialized by update: %v", err)
				}
			}
			afterCfg, _ := os.ReadFile(cfgPath)
			if string(cfg) != string(afterCfg) {
				t.Fatalf("native config bytes changed:\nbefore %s\nafter %s", cfg, afterCfg)
			}
		})
	}
}
