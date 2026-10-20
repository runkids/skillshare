package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func ompCatalogFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, ".claude-plugin/marketplace.json", `{"name":"claude-market","plugins":[{"name":"kata","source":"./plugins/claude-code"}]}`)
	writeFile(t, root, ".omp-plugin/marketplace.json", `{"name":"omp-market","plugins":[{"name":"kata","source":"./plugins/oh-my-pi"}]}`)
	writeFile(t, root, "plugins/claude-code/.claude-plugin/plugin.json", `{"name":"kata","version":"0.1.0","hooks":"./hooks/hooks.json"}`)
	writeFile(t, root, "plugins/claude-code/hooks/hooks.json", `{}`)
	writeFile(t, root, "plugins/oh-my-pi/package.json", `{"name":"kata","version":"2.0.0","omp":{"extensions":["./kata.ts"]}}`)
	writeFile(t, root, "plugins/oh-my-pi/kata.ts", "export default function () {}")
	return root
}

func TestOMPDiscoverUsesItsOwnCatalogPackage(t *testing.T) {
	for _, pkg := range []string{
		`{"name":"kata","version":"2.0.0","omp":{"extensions":["./kata.ts"]}}`,
		`{"version":"2.0.0","omp":{"extensions":["./kata.ts"]}}`,
		`{"name":"@team/kata-omp","version":"2.0.0","omp":{"extensions":["./kata.ts"]}}`,
		`{"name":"@team/kata_omp","version":"2.0.0","omp":{"extensions":["./kata.ts"]}}`,
		`{"name":"kata","version":"2.0.0","pi":{"extensions":["./kata.ts"]}}`,
		`{"name":"@team/kata-pi","version":"2.0.0","pi":{"extensions":["./kata.ts"]}}`,
	} {
		t.Run(pkg, func(t *testing.T) {
			root := ompCatalogFixture(t)
			writeFile(t, root, "plugins/oh-my-pi/package.json", pkg)
			d, err := Discover(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if len(d.Candidates) != 1 {
				t.Fatalf("candidates: %+v", d.Candidates)
			}
			c := d.Candidates[0]
			omp := c.TargetInfo["omp"]
			if c.Problem != "" || !slices.Contains(c.Targets, "omp") || c.pathFor("omp") != "plugins/oh-my-pi" || omp.Manifest != "package.json" || omp.Version != "2.0.0" || !slices.Equal(omp.Components, []string{"extensions"}) {
				t.Fatalf("OMP did not select its native package: %+v", c)
			}
			if c.pathFor("claude") != "plugins/claude-code" || c.TargetInfo["claude"].Version != "0.1.0" || !slices.Equal(c.TargetInfo["claude"].Components, []string{"hooks"}) {
				t.Fatalf("Claude package changed: %+v", c)
			}
		})
	}
}

func TestOMPMergedBuildVersionWithoutTargets(t *testing.T) {
	for _, test := range []struct{ name, claudeVersion, want string }{
		{"different", "0.1.0", ""},
		{"same", "2.0.0", "2.0.0"},
		{"unspecified", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := ompCatalogFixture(t)
			writeFile(t, root, "plugins/claude-code/.claude-plugin/plugin.json", `{"name":"kata","version":"`+test.claudeVersion+`","hooks":"./hooks/hooks.json"}`)
			agents := &fakeAgents{}
			s := agents.service(t)
			r := Request{Action: "add", Source: root, Plugin: "kata"}
			p, err := s.Preview(context.Background(), r)
			if err != nil || len(p.Changes) != 1 || p.Changes[0].Binding.Version != test.want {
				t.Fatalf("ambiguous no-Agent version: %+v %v; want %q", p, err, test.want)
			}
			applyPluginRequest(t, s, r)
			d, err := s.load()
			if err != nil || d.packages["kata"].Version != test.want {
				t.Fatalf("recorded no-Agent version: %+v %v; want %q", d, err, test.want)
			}
		})
	}
}

func TestOMPCatalogVersionReachesNativeInstall(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "legacy"}[legacy], func(t *testing.T) {
			root := ompCatalogFixture(t)
			catalogPath := ".omp-plugin/marketplace.json"
			if legacy {
				if err := os.Remove(filepath.Join(root, catalogPath)); err != nil {
					t.Fatal(err)
				}
				catalogPath = ".claude-plugin/marketplace.json"
			}
			writeFile(t, root, catalogPath, `{"name":"market","plugins":[{"name":"kata","version":"3.0.0","source":"./plugins/oh-my-pi"}]}`)
			d, err := Discover(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if got := d.Candidates[0].TargetInfo["omp"].Version; got != "3.0.0" {
				t.Fatalf("OMP ignored catalog version: %q", got)
			}
			wantVersion := ""
			if legacy {
				wantVersion = "3.0.0"
			}
			if got := d.Candidates[0].Version; got != wantVersion {
				t.Fatalf("aggregate version: %q; want %q", got, wantVersion)
			}
			agents := &fakeAgents{version: "3.0.0"}
			s := agents.service(t)
			r := Request{Action: "add", Source: root, Plugin: "kata", Targets: []string{"omp"}}
			p, err := s.Preview(context.Background(), r)
			if err != nil || len(p.Changes) != 1 || p.Changes[0].Binding.Version != "3.0.0" {
				t.Fatalf("preview lost catalog version: %+v %v", p, err)
			}
			applyPluginRequest(t, s, r)
			for _, market := range agents.markets["omp"] {
				data, err := os.ReadFile(filepath.Join(market, ".claude-plugin/marketplace.json"))
				if err != nil {
					t.Fatal(err)
				}
				var catalog struct{ Plugins []struct{ Version string } }
				if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Plugins) != 1 || catalog.Plugins[0].Version != "3.0.0" {
					t.Fatalf("native catalog lost version: %s %v", data, err)
				}
			}
		})
	}
}

func TestOMPCatalogPreventsClaudeFallback(t *testing.T) {
	for _, catalog := range []string{
		`{"name":"omp-market","plugins":[]}`,
		`{"name":"omp-market","plugins":[{"name":"kata","source":"./missing"}]}`,
		`{"name":"omp-market","plugins":[{"name":"kata","source":"https://example.com/kata.git"}]}`,
		`{"name":"omp-market","plugins":[{"name":"kata","source":"../outside"}]}`,
		`{`,
	} {
		t.Run(catalog, func(t *testing.T) {
			root := ompCatalogFixture(t)
			writeFile(t, root, ".omp-plugin/marketplace.json", catalog)
			d, err := Discover(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			c := d.Candidates[0]
			if slices.Contains(c.Targets, "omp") || !slices.Contains(c.Targets, "claude") || c.Problem != "" {
				t.Fatalf("broken/missing native entry fell back to Claude: %+v", c)
			}
		})
	}
}

func TestOMPCatalogRejectsUnsupportedNativePackage(t *testing.T) {
	root := ompCatalogFixture(t)
	writeFile(t, root, "plugins/oh-my-pi/package.json", `{"name":"kata","pi":{"skills":["./skills"]}}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if c := d.Candidates[0]; slices.Contains(c.Targets, "omp") || !slices.Contains(c.Targets, "claude") {
		t.Fatalf("unsupported native package inherited Claude OMP metadata: %+v", c)
	}
}

func TestOMPCatalogAliasKeepsOtherTargetNameValidation(t *testing.T) {
	root := ompCatalogFixture(t)
	writeFile(t, root, "plugins/oh-my-pi/.codex-plugin/plugin.json", `{"name":"kata-codex","skills":"./skills"}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if c := d.Candidates[0]; !slices.Contains(c.Targets, "omp") || slices.Contains(c.Targets, "codex") {
		t.Fatalf("OMP alias bypassed another target's name validation: %+v", c)
	}
}

func TestOMPHybridPackagePreservesClaudeComponents(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, ".claude-plugin/plugin.json", `{"name":"demo","hooks":"./custom-hooks.json","mcpServers":{"demo":{"command":"demo"}}}`)
	writeFile(t, root, "custom-hooks.json", `{}`)
	writeFile(t, root, "package.json", `{"name":"demo","omp":{"extensions":["./extension.ts"]}}`)
	writeFile(t, root, "extension.ts", "export default function () {}")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if components := d.Candidates[0].TargetInfo["omp"].Components; !slices.Equal(components, []string{"extensions", "hooks", "mcpServers", "skills"}) {
		t.Fatalf("OMP lost Claude components: %v", components)
	}
}

func TestOMPAlongsideManifestlessClaudeCatalogEntry(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".claude-plugin/marketplace.json", `{"name":"market","plugins":[{"name":"demo","source":"./plugin","strict":false}]}`)
	writeFile(t, root, "plugin/package.json", `{"name":"demo","omp":{"extensions":["./extension.ts"]}}`)
	writeFile(t, root, "plugin/extension.ts", "export default function () {}")
	writeFile(t, root, "plugin/skills/demo/SKILL.md", "---\nname: demo\ndescription: Demo\n---\nHello")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if c := d.Candidates[0]; c.Problem != "" || !slices.Contains(c.Targets, "claude") || !slices.Contains(c.Targets, "omp") {
		t.Fatalf("OMP package hid manifestless Claude entry: %+v", c)
	}
}

func TestOMPSharedCatalogKeepsManifestlessClaudeEntry(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".omp-plugin/marketplace.json", `{"name":"omp-market","plugins":[{"name":"demo","source":"./plugin"}]}`)
	writeFile(t, root, ".claude-plugin/marketplace.json", `{"name":"claude-market","plugins":[{"name":"demo","source":"./plugin","strict":false}]}`)
	writeFile(t, root, "plugin/package.json", `{"name":"demo","omp":{"extensions":["./extension.ts"]}}`)
	writeFile(t, root, "plugin/extension.ts", "export default function () {}")
	writeFile(t, root, "plugin/skills/demo/SKILL.md", "---\nname: demo\ndescription: Demo\n---\nHello")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	c := d.Candidates[0]
	if c.Problem != "" || !slices.Contains(c.Targets, "claude") || !slices.Contains(c.Targets, "omp") || c.TargetInfo["claude"].Marketplace != "claude-market" || c.TargetInfo["omp"].Marketplace != "omp-market" || string(c.catalogEntry["strict"]) != "false" {
		t.Fatalf("shared folder lost native catalog metadata: %+v", c)
	}
}

func TestOMPPackageNameDoesNotOverrideNativeManifest(t *testing.T) {
	for _, native := range []struct{ path, target string }{
		{"plugin.json", "codex"},
		{".github/plugin/plugin.json", "copilot"},
	} {
		t.Run(native.target, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, native.path, `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","skills":"./skills"}`)
			writeFile(t, root, "package.json", `{"name":"@team/demo-omp","omp":{"extensions":["./extension.ts"]}}`)
			writeFile(t, root, "extension.ts", "export default function () {}")
			d, err := Discover(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if c := d.Candidates[0]; c.Name != "demo" || !slices.Contains(c.Targets, native.target) || !slices.Contains(c.Targets, "omp") {
				t.Fatalf("OMP npm name blocked native manifest: %+v", c)
			}
		})
	}
}

func TestOMPSeparateCatalogKeepsClaudeNativeAdoption(t *testing.T) {
	agents := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "kata@claude-market", Version: "0.1.0", Scope: "user", Installed: true, Enabled: true}}}}
	s := agents.service(t)
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: ompCatalogFixture(t), Plugin: "kata", Targets: []string{"claude"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Changes) != 1 || p.Changes[0].Action != "import" || p.Changes[0].ID != "kata@claude-market" {
		t.Fatalf("Claude native installation was not adopted: %+v", p)
	}
}

func TestOMPAddCatalogPackageMaterializesNativeSource(t *testing.T) {
	for _, test := range []struct {
		name, manifest string
		project        bool
	}{
		{"omp-global", "omp", false},
		{"omp-project", "omp", true},
		{"pi-global", "pi", false},
		{"pi-project", "pi", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			agents := &fakeAgents{version: "2.0.0"}
			s := agents.service(t)
			if test.project {
				s.ProjectRoot = t.TempDir()
			}
			root := ompCatalogFixture(t)
			writeFile(t, root, "plugins/oh-my-pi/package.json", `{"name":"kata","version":"2.0.0","`+test.manifest+`":{"extensions":["./kata.ts"]}}`)
			r := Request{Action: "add", Source: root, Plugin: "kata", Name: "kata", Targets: []string{"omp"}}
			p, err := s.Preview(context.Background(), r)
			if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "install" || p.Changes[0].Binding.Version != "2.0.0" || !slices.Equal(p.Changes[0].Components, []string{"extensions"}) {
				t.Fatalf("wrong preview: %+v %v", p, err)
			}
			applyPluginRequest(t, s, r)
			for _, market := range agents.markets["omp"] {
				data, err := os.ReadFile(filepath.Join(market, ".claude-plugin/marketplace.json"))
				if err != nil {
					t.Fatal(err)
				}
				var catalog struct {
					Plugins []struct{ Name, Source string }
				}
				if err := json.Unmarshal(data, &catalog); err != nil {
					t.Fatal(err)
				}
				if len(catalog.Plugins) != 1 || catalog.Plugins[0].Name != "kata" || catalog.Plugins[0].Source != "./content/plugins/oh-my-pi" {
					t.Fatalf("wrong native catalog: %s", data)
				}
				installed := filepath.Join(market, filepath.FromSlash(catalog.Plugins[0].Source))
				if _, err := os.Stat(filepath.Join(installed, "kata.ts")); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(filepath.Join(installed, "hooks")); !os.IsNotExist(err) {
					t.Fatalf("Claude build selected: %v", err)
				}
			}
		})
	}
}
