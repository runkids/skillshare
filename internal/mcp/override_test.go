package mcp

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sharedDocs() Server {
	return Server{
		URL:       "https://docs.example/mcp",
		Targets:   TargetList{"claude", "pi"},
		Tools:     ToolPolicy{Deny: []string{"delete_*"}},
		PiOptions: PiOptions{"exposure": "codemode", "timeout": 30},
		Options:   map[string]map[string]any{"codex": {"default_tools_approval_mode": "prompt", "startup_timeout_sec": 10}},
		Global:    new(bool),
	}
}

func TestOverrideApply_MergesPiOptionsKeyByKey(t *testing.T) {
	got := Override{PiOptions: PiOptions{"exposure": "direct"}}.apply(sharedDocs())
	want := PiOptions{"exposure": "direct", "timeout": 30}
	if !reflect.DeepEqual(got.PiOptions, want) {
		t.Fatalf("piOptions = %v, want %v", got.PiOptions, want)
	}
}

func TestOverrideApply_MergesOptionsPerAgentAndKey(t *testing.T) {
	got := Override{Options: map[string]map[string]any{
		"codex": {"default_tools_approval_mode": "approve"},
		"goose": {"timeout": 5},
	}}.apply(sharedDocs())
	want := map[string]map[string]any{
		"codex": {"default_tools_approval_mode": "approve", "startup_timeout_sec": 10},
		"goose": {"timeout": 5},
	}
	if !reflect.DeepEqual(got.Options, want) {
		t.Fatalf("options = %v, want %v", got.Options, want)
	}
}

func TestOverrideApply_ReplacesTargetsAndTools(t *testing.T) {
	got := Override{Targets: TargetList{"pi"}, Tools: ToolPolicy{Allow: []string{"search"}}}.apply(sharedDocs())
	if !reflect.DeepEqual(got.Targets, TargetList{"pi"}) || !reflect.DeepEqual(got.Tools, ToolPolicy{Allow: []string{"search"}}) {
		t.Fatalf("targets = %v, tools = %v", got.Targets, got.Tools)
	}
}

func TestOverrideApply_KeepsTheConnection(t *testing.T) {
	got := Override{PiOptions: PiOptions{"exposure": "direct"}}.apply(sharedDocs())
	if got.URL != "https://docs.example/mcp" || !got.Shared() {
		t.Fatalf("url = %q, shared = %v", got.URL, got.Shared())
	}
}

func TestOverrideApply_LeavesTheSharedServerAlone(t *testing.T) {
	shared := sharedDocs()
	Override{
		PiOptions: PiOptions{"exposure": "direct"},
		Options:   map[string]map[string]any{"codex": {"default_tools_approval_mode": "approve"}},
	}.apply(shared)
	if !reflect.DeepEqual(shared, sharedDocs()) {
		t.Fatalf("shared server changed: %+v", shared)
	}
}

func TestOverrideApply_EmptyOverrideChangesNothing(t *testing.T) {
	if got := (Override{}).apply(sharedDocs()); !reflect.DeepEqual(got, sharedDocs()) {
		t.Fatalf("empty override changed the server: %+v", got)
	}
}

const overrideConfig = `mcp:
  targets: [pi]
  servers:
    docs:
      global: false
      url: https://docs.example.com/mcp
      piOptions: {exposure: codemode, timeout: 30}
  projects:
    $TMP/api:
      use: [docs]
    $TMP/billing:
      use: [docs]
      overrides:
        docs:
          piOptions: {exposure: direct}
      servers:
        local:
          command: local-server
    $TMP/blog: {}
`

func loadOverrides(t *testing.T, s *Service, root string) map[string]Override {
	t.Helper()
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	return source.Projects[root].Overrides
}

func TestOverrideReachesOnlyItsRoot(t *testing.T) {
	s, tmp := projectsService(t, overrideConfig)
	applyProjects(t, s)
	billing := readFile(t, filepath.Join(tmp, "billing", ".pi", "mcp.json"))
	if !strings.Contains(billing, `"direct"`) || !strings.Contains(billing, "30") {
		t.Fatalf("billing does not load its override over the shared settings:\n%s", billing)
	}
	if api := readFile(t, filepath.Join(tmp, "api", ".pi", "mcp.json")); strings.Contains(api, `"direct"`) {
		t.Fatalf("api took billing's override:\n%s", api)
	}
}

func TestRenderNativeShowsEachRootsOverride(t *testing.T) {
	s, tmp := projectsService(t, overrideConfig)
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range s.RenderNative("docs", source.Servers["docs"]) {
		direct := strings.Contains(r.Content, `"direct"`)
		if billing := strings.HasPrefix(r.Path, filepath.Join(tmp, "billing")); direct != billing {
			t.Fatalf("%s: direct = %v\n%s", r.Path, direct, r.Content)
		}
	}
}

func TestOverrideRules(t *testing.T) {
	for name, tc := range map[string]struct{ config, want string }{
		"not used": {`mcp:
  targets: [pi]
  servers:
    docs: {url: https://docs.example.com/mcp, global: false}
  projects:
    $TMP/api:
      overrides:
        docs: {piOptions: {exposure: direct}}
`, "does not use"},
		"bad exposure": {`mcp:
  targets: [pi]
  servers:
    docs: {url: https://docs.example.com/mcp, global: false}
  projects:
    $TMP/api:
      use: [docs]
      overrides:
        docs: {piOptions: {exposure: sideways}}
`, "exposure"},
		"account target": {`targets:
  claude-work: {agent: claude, config_dir: ~/.claude-work}
mcp:
  targets: [pi]
  servers:
    docs: {url: https://docs.example.com/mcp, global: false}
  projects:
    $TMP/api:
      use: [docs]
      overrides:
        docs: {targets: [claude-work]}
`, "claude-work"},
		"connection field": {`mcp:
  targets: [pi]
  servers:
    docs: {url: https://docs.example.com/mcp, global: false}
  projects:
    $TMP/api:
      use: [docs]
      overrides:
        docs: {url: https://other.example.com/mcp}
`, "url"},
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := projectsService(t, tc.config)
			_, err := s.Preview()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestStoppingUseDropsTheOverride(t *testing.T) {
	s, tmp := projectsService(t, overrideConfig)
	if _, err := s.Mutate(Mutation{Project: tmp + "/billing", Name: "docs", Using: new(false)}, "", false); err != nil {
		t.Fatal(err)
	}
	if got := loadOverrides(t, s, filepath.Join(tmp, "billing")); got != nil {
		t.Fatalf("override kept for a server the root no longer uses: %v", got)
	}
}

func TestRemovingOrGlobalizingASharedServerDropsItsOverrides(t *testing.T) {
	for name, m := range map[string]Mutation{
		"remove": {Name: "docs", Remove: true},
		"global": {Name: "docs", Server: &Server{URL: "https://docs.example.com/mcp", Global: new(true)}, Replace: true},
	} {
		t.Run(name, func(t *testing.T) {
			s, tmp := projectsService(t, overrideConfig)
			if _, err := s.Mutate(m, "", false); err != nil {
				t.Fatal(err)
			}
			if got := loadOverrides(t, s, filepath.Join(tmp, "billing")); got != nil {
				t.Fatalf("override kept: %v", got)
			}
		})
	}
}

func TestOverrideMutationSetsAndClearsIt(t *testing.T) {
	s, tmp := projectsService(t, overrideConfig)
	api := filepath.Join(tmp, "api")
	set := Override{PiOptions: PiOptions{"exposure": "direct"}}
	if _, err := s.Mutate(Mutation{Project: api, Name: "docs", Override: &set}, "", false); err != nil {
		t.Fatal(err)
	}
	if got := loadOverrides(t, s, api); !reflect.DeepEqual(got, map[string]Override{"docs": set}) {
		t.Fatalf("overrides = %v", got)
	}
	if _, err := s.Mutate(Mutation{Project: api, Name: "docs", Override: &Override{}}, "", false); err != nil {
		t.Fatal(err)
	}
	if got := loadOverrides(t, s, api); got != nil {
		t.Fatalf("an empty override was kept: %v", got)
	}
}

func TestOverrideMutationNeedsTheServerInUse(t *testing.T) {
	s, tmp := projectsService(t, overrideConfig)
	m := Mutation{Project: tmp + "/blog", Name: "docs", Override: &Override{PiOptions: PiOptions{"exposure": "direct"}}}
	if _, err := s.Mutate(m, "", false); err == nil || !strings.Contains(err.Error(), "does not use") {
		t.Fatalf("want a refusal, got %v", err)
	}
}

func TestUsingAndOverrideInOneMutation(t *testing.T) {
	s, tmp := projectsService(t, overrideConfig)
	set := Override{PiOptions: PiOptions{"exposure": "direct"}}
	m := Mutation{Project: tmp + "/blog", Name: "docs", Using: new(true), Override: &set}
	if _, err := s.Mutate(m, "", false); err != nil {
		t.Fatal(err)
	}
	if got := loadOverrides(t, s, filepath.Join(tmp, "blog")); !reflect.DeepEqual(got, map[string]Override{"docs": set}) {
		t.Fatalf("overrides = %v", got)
	}
}

func TestProjectSettingsKeepTheOverride(t *testing.T) {
	s, tmp := projectsService(t, overrideConfig)
	billing := filepath.Join(tmp, "billing")
	m := Mutation{Project: billing, Settings: &Settings{Targets: []string{"pi"}}, Replace: true}
	if _, err := s.Mutate(m, "", false); err != nil {
		t.Fatal(err)
	}
	if got := loadOverrides(t, s, billing); got["docs"].PiOptions["exposure"] != "direct" {
		t.Fatalf("overrides = %v", got)
	}
}

func TestCheckPassesARootWithAnOverride(t *testing.T) {
	s, _ := projectsService(t, overrideConfig)
	report, err := s.Check(offlineCheck())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"docs", "local"} {
		for _, f := range findings(t, report, name) {
			if f.Level == "error" {
				t.Fatalf("%s reported as broken: %+v", name, f)
			}
		}
	}
}

func TestSharedTargetsFollowEachRootsOverride(t *testing.T) {
	s, _ := projectsService(t, `mcp:
  targets: [cursor]
  servers:
    docs: {url: https://docs.example.com/mcp, global: false}
  projects:
    $TMP/api:
      use: [docs]
    $TMP/billing:
      use: [docs]
      overrides:
        docs: {targets: [pi]}
`)
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := source.SharedTargets("docs"); !reflect.DeepEqual(got, TargetList{"cursor", "pi"}) {
		t.Fatalf("shared targets = %v", got)
	}
}

func TestSharedTargetsAddAnOverrideToTheServersOwn(t *testing.T) {
	s, _ := projectsService(t, `mcp:
  servers:
    docs: {url: https://docs.example.com/mcp, global: false, targets: [cursor]}
  projects:
    $TMP/api:
      use: [docs]
    $TMP/billing:
      use: [docs]
      overrides:
        docs: {targets: [pi]}
`)
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := source.SharedTargets("docs"); !reflect.DeepEqual(got, TargetList{"cursor", "pi"}) {
		t.Fatalf("shared targets = %v", got)
	}
}

func TestCheckRefusingOneRootsOverrideTargetLeavesTheOtherRoots(t *testing.T) {
	// Claude reserves the name workspace, so only billing's override target refuses it.
	for name, own := range map[string]string{"inherited": "", "own targets": ", targets: [cursor]"} {
		t.Run(name, func(t *testing.T) {
			s, _ := projectsService(t, `mcp:
  targets: [cursor]
  servers:
    workspace: {url: https://example.com/mcp, global: false`+own+`}
  projects:
    $TMP/api:
      use: [workspace]
    $TMP/billing:
      use: [workspace]
      overrides:
        workspace: {targets: [claude]}
`)
			report, err := s.Check(offlineCheck())
			if err != nil {
				t.Fatal(err)
			}
			list := findings(t, report, "workspace")
			if !hasFinding(list, "error", "client-rule", "claude") || hasFinding(list, "error", "client-rule", "cursor") {
				t.Fatalf("only billing's claude must be refused: %+v", list)
			}
			if !hasFinding(list, "warning", "sync", "cursor") {
				t.Fatalf("api's cursor entry must still be planned: %+v", list)
			}
		})
	}
}

func TestCheckToolPolicyFollowsEachRootsOverride(t *testing.T) {
	s, _ := projectsService(t, `mcp:
  targets: [pi]
  servers:
    docs: {url: https://docs.example.com/mcp, global: false}
  projects:
    $TMP/api:
      use: [docs]
    $TMP/billing:
      use: [docs]
      overrides:
        docs: {targets: [cursor], tools: {allow: [search]}}
`)
	report, err := s.Check(offlineCheck())
	if err != nil {
		t.Fatal(err)
	}
	if list := findings(t, report, "docs"); !hasFinding(list, "warning", "tools", "cursor") {
		t.Fatalf("billing's tool policy reaches cursor, which cannot hold allow: %+v", list)
	}
}

func TestOverrideWithAnUnknownTargetIsRefusedAndNotSaved(t *testing.T) {
	s, tmp := projectsService(t, overrideConfig)
	api := filepath.Join(tmp, "api")
	m := Mutation{Project: api, Name: "docs", Override: &Override{Targets: TargetList{"codx"}}}
	if _, err := s.Mutate(m, "", false); err == nil || !strings.Contains(err.Error(), "codx") {
		t.Fatalf("want a refusal naming codx, got %v", err)
	}
	if got := loadOverrides(t, s, api); got != nil {
		t.Fatalf("a refused override was saved: %v", got)
	}
}

func TestOverrideMutationShapeIsChecked(t *testing.T) {
	s, tmp := projectsService(t, overrideConfig)
	billing := filepath.Join(tmp, "billing")
	set := &Override{PiOptions: PiOptions{"exposure": "codemode"}}
	for name, m := range map[string]Mutation{
		"with remove":    {Project: billing, Name: "docs", Remove: true, Override: set},
		"with a server":  {Project: billing, Name: "docs", Server: &Server{Command: "x"}, Override: set},
		"while stopping": {Project: billing, Name: "docs", Using: new(false), Override: set},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := s.Mutate(m, "", false); err == nil {
				t.Fatal("accepted")
			}
			if got := loadOverrides(t, s, billing); got["docs"].PiOptions["exposure"] != "direct" {
				t.Fatalf("overrides changed: %v", got)
			}
		})
	}
}

func TestOverrideWithNoTargetsKeepsTheServerOutOfThatRoot(t *testing.T) {
	s, tmp := projectsService(t, strings.Replace(overrideConfig, "piOptions: {exposure: direct}", "targets: []", 1))
	billing := filepath.Join(tmp, "billing")
	if _, err := s.Mutate(Mutation{Project: billing, Settings: &Settings{Targets: []string{"pi"}}, Replace: true}, "", false); err != nil {
		t.Fatal(err)
	}
	if got := loadOverrides(t, s, billing); got["docs"].Targets == nil {
		t.Fatalf("an empty targets override was lost on save: %v", got)
	}
	applyProjects(t, s)
	if got := readFile(t, filepath.Join(billing, ".pi", "mcp.json")); strings.Contains(got, "docs") {
		t.Fatalf("billing received docs:\n%s", got)
	}
}

func TestSavingOneRootsOverrideLeavesTheOtherRootsAsWritten(t *testing.T) {
	s, tmp := projectsService(t, strings.Replace(overrideConfig, "      overrides:\n", "      # billing goes direct\n      overrides:\n", 1))
	set := Override{PiOptions: PiOptions{"exposure": "direct"}}
	if _, err := s.Mutate(Mutation{Project: filepath.Join(tmp, "api"), Name: "docs", Override: &set}, "", false); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, s.ConfigPath); !strings.Contains(got, "# billing goes direct") {
		t.Fatalf("billing's root was rewritten:\n%s", got)
	}
}

func TestOverrideExposureWorksWithASharedToolPolicy(t *testing.T) {
	s, _ := projectsService(t, strings.Replace(overrideConfig, "piOptions: {exposure: codemode, timeout: 30}", "tools: {deny: [delete_*]}", 1))
	if _, err := s.Preview(); err != nil {
		t.Fatal(err)
	}
}

func TestSharedTargetsOfAServerNoProjectUses(t *testing.T) {
	for own, want := range map[string]TargetList{"": {}, ", targets: [cursor]": {"cursor"}} {
		s, _ := projectsService(t, `mcp:
  targets: [pi]
  servers:
    docs: {url: https://docs.example.com/mcp, global: false`+own+`}
  projects:
    $TMP/blog: {}
`)
		source, err := LoadSource(s.ConfigPath)
		if err != nil {
			t.Fatal(err)
		}
		if got := source.SharedTargets("docs"); !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: shared targets = %v, want %v", own, got, want)
		}
	}
}
