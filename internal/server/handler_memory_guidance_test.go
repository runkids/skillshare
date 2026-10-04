package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/memory"
)

type guidanceStatus struct {
	Instructions map[string]string `json:"instructions"`
	Targets      []guidanceTarget  `json:"targets"`
}

func guidanceState(t *testing.T, s *Server) map[string]guidanceTarget {
	t.Helper()
	rr := instructionsRequest(t, s, http.MethodGet, "/api/extras/memory/guidance", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("guidance: %d %s", rr.Code, rr.Body)
	}
	out := map[string]guidanceTarget{}
	for _, target := range decodeBody[guidanceStatus](t, rr).Targets {
		out[target.Name] = target
	}
	return out
}

func planGuidanceFor(t *testing.T, s *Server, targets string) guidancePlan {
	t.Helper()
	rr := instructionsRequest(t, s, http.MethodPost, "/api/extras/memory/guidance/plan", `{"targets":`+targets+`}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("plan: %d %s", rr.Code, rr.Body)
	}
	return decodeBody[guidancePlan](t, rr)
}

func applyGuidance(t *testing.T, s *Server, targets, token string) map[string]any {
	t.Helper()
	rr := instructionsRequest(t, s, http.MethodPost, "/api/extras/memory/guidance/apply", `{"targets":`+targets+`,"token":"`+token+`"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", rr.Code, rr.Body)
	}
	return decodeBody[map[string]any](t, rr)
}

func TestMemoryGuidance_AppendsToOwnFilesAndIsIdempotent(t *testing.T) {
	s, home := newInstructionsServer(t, "claude", "codex")
	codex := writeHome(t, home, ".codex/AGENTS.md", "codex own\n")
	claude := writeHome(t, home, ".claude/CLAUDE.md", "# Me\n@~/.claude/rules/a.md\n")
	if got := guidanceState(t, s); got["codex"].State != "unconfigured" || got["codex"].File != codex {
		t.Fatalf("before = %+v", got)
	}
	plan := planGuidanceFor(t, s, `["claude","codex"]`)
	if len(plan.Changes) != 2 {
		t.Fatalf("plan = %+v", plan)
	}
	if res := applyGuidance(t, s, `["claude","codex"]`, plan.Token); res["success"] != true {
		t.Fatalf("apply = %v", res)
	}
	if got := readFile(t, codex); !strings.HasPrefix(got, "codex own\n\n<!-- skillshare:memory scope=global") {
		t.Errorf("codex = %q", got)
	}
	if got := readFile(t, claude); !strings.HasPrefix(got, "# Me\n@~/.claude/rules/a.md\n\n<!-- skillshare:memory") {
		t.Errorf("claude = %q", got)
	}
	for name, target := range guidanceState(t, s) {
		if target.State != "configured" {
			t.Errorf("%s = %+v", name, target)
		}
	}
	if again := planGuidanceFor(t, s, `["claude","codex"]`); len(again.Changes) != 0 || len(again.Skipped) != 2 || again.Skipped[0].Reason != "configured" {
		t.Errorf("second plan = %+v", again)
	}
}

func TestMemoryGuidance_WritesSharedSourceAndKeepsRouting(t *testing.T) {
	s, home := newInstructionsServer(t, "claude", "codex")
	writeHome(t, home, ".codex/AGENTS.md", "codex own\n")
	claude := writeHome(t, home, ".claude/CLAUDE.md", "# Me\n")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"personal\n"}`)
	if rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude","codex"],"extras":["personal"]}`); rr.Code != http.StatusOK {
		t.Fatalf("assign: %d %s", rr.Code, rr.Body)
	}
	claudeBefore := readFile(t, claude)
	cfgBefore := readFile(t, s.configPath())

	plan := planGuidanceFor(t, s, `["codex"]`)
	if len(plan.Changes) != 1 || len(plan.Warnings) != 1 || plan.Warnings[0].Code != "also_read_by" || plan.Warnings[0].Targets[0] != "claude" {
		t.Fatalf("plan = %+v", plan)
	}
	shared := plan.Changes[0].Path
	applyGuidance(t, s, `["codex"]`, plan.Token)
	if got := readFile(t, shared); !strings.HasPrefix(got, "personal\n\n<!-- skillshare:memory") {
		t.Errorf("shared = %q", got)
	}
	if info, err := os.Lstat(filepath.Join(home, ".codex", "AGENTS.md")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("codex is no longer a link: %v", err)
	}
	if readFile(t, claude) != claudeBefore || readFile(t, s.configPath()) != cfgBefore {
		t.Error("connecting changed the import file or config")
	}
	if got := guidanceState(t, s); got["claude"].State != "configured" || got["codex"].State != "configured" {
		t.Errorf("after = %+v", got)
	}
}

func TestMemoryGuidance_PreservesUserChangedAndManualGuidance(t *testing.T) {
	s, home := newInstructionsServer(t, "codex")
	root, _ := memory.GlobalRoot(s.cfg)
	block := memory.Instructions(root, "", memory.ModePassive)
	edited := "own\n" + strings.Replace(block, "Open only", "Always open", 1)
	codex := writeHome(t, home, ".codex/AGENTS.md", edited)
	if got := guidanceState(t, s)["codex"]; got.State != "broken" || got.Detail != "modified" {
		t.Fatalf("modified = %+v", got)
	}
	if plan := planGuidanceFor(t, s, `["codex"]`); len(plan.Changes) != 0 || plan.Skipped[0].Reason != "modified" {
		t.Fatalf("plan = %+v", plan)
	}
	manual := "## Shared memory\n\nShared notes directory: `" + root + "`\n"
	writeHome(t, home, ".codex/AGENTS.md", manual)
	if got := guidanceState(t, s)["codex"]; got.State != "unconfigured" {
		t.Fatalf("manual = %+v", got)
	}
	plan := planGuidanceFor(t, s, `["codex"]`)
	applyGuidance(t, s, `["codex"]`, plan.Token)
	if got := readFile(t, codex); !strings.HasPrefix(got, manual) {
		t.Errorf("manual guidance changed: %q", got)
	}
}

func TestMemoryGuidance_RejectsStalePlans(t *testing.T) {
	s, home := newInstructionsServer(t, "codex")
	codex := writeHome(t, home, ".codex/AGENTS.md", "own\n")
	plan := planGuidanceFor(t, s, `["codex"]`)
	writeHome(t, home, ".codex/AGENTS.md", "edited elsewhere\n")
	rr := instructionsRequest(t, s, http.MethodPost, "/api/extras/memory/guidance/apply", `{"targets":["codex"],"token":"`+plan.Token+`"}`)
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "memory_guidance_stale") || readFile(t, codex) != "edited elsewhere\n" {
		t.Fatalf("file change: %d %s", rr.Code, rr.Body)
	}

	plan = planGuidanceFor(t, s, `["codex"]`)
	s.cfg.Extras = append(s.cfg.Extras, config.ExtraConfig{Name: "memory", Source: t.TempDir()})
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}
	rr = instructionsRequest(t, s, http.MethodPost, "/api/extras/memory/guidance/apply", `{"targets":["codex"],"token":"`+plan.Token+`"}`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("source change: %d %s", rr.Code, rr.Body)
	}
}

func TestMemoryGuidance_UpdatesOutdatedBlockInPlace(t *testing.T) {
	s, home := newInstructionsServer(t, "codex")
	codex := writeHome(t, home, ".codex/AGENTS.md", "top\n")
	plan := planGuidanceFor(t, s, `["codex"]`)
	applyGuidance(t, s, `["codex"]`, plan.Token)
	writeHome(t, home, ".codex/AGENTS.md", readFile(t, codex)+"bottom\n")

	moved := t.TempDir()
	s.cfg.Extras = append(s.cfg.Extras, config.ExtraConfig{Name: "memory", Source: moved})
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := guidanceState(t, s)["codex"]; got.State != "outdated" {
		t.Fatalf("after move = %+v", got)
	}
	plan = planGuidanceFor(t, s, `["codex"]`)
	applyGuidance(t, s, `["codex"]`, plan.Token)
	got := readFile(t, codex)
	if !strings.HasPrefix(got, "top\n\n<!--") || !strings.HasSuffix(got, "-->\nbottom\n") || !strings.Contains(got, filepath.ToSlash(moved)) || strings.Count(got, "scope=global") != 1 {
		t.Errorf("codex = %q", got)
	}
	if state := guidanceState(t, s)["codex"].State; state != "configured" {
		t.Errorf("state = %s", state)
	}
}

func TestMemoryGuidance_UnsyncedSharedFileIsNotConfigured(t *testing.T) {
	s, home := newInstructionsServer(t, "codex")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"personal\n"}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["codex"],"extras":["personal"]}`)
	plan := planGuidanceFor(t, s, `["codex"]`)
	applyGuidance(t, s, `["codex"]`, plan.Token)
	codex := filepath.Join(home, ".codex", "AGENTS.md")
	os.Remove(codex)
	writeHome(t, home, ".codex/AGENTS.md", "replaced by hand\n")

	if got := guidanceState(t, s)["codex"]; got.State != "broken" || got.Detail != "not_synced" {
		t.Fatalf("state = %+v", got)
	}
	if plan := planGuidanceFor(t, s, `["codex"]`); len(plan.Changes) != 0 || plan.Skipped[0].Reason != "not_synced" {
		t.Fatalf("plan = %+v", plan)
	}
	if readFile(t, codex) != "replaced by hand\n" {
		t.Error("hand-made file changed")
	}
}

func TestMemoryGuidance_ProjectUsesAgentsFileAndRelativePath(t *testing.T) {
	s, root := newTestProjectServerWithExtras(t, nil)
	s.projectCfg.Targets = []config.ProjectTargetEntry{{Name: "claude"}, {Name: "codex"}}
	if err := s.projectCfg.Save(root); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("project rules\n"), 0644)
	os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("own\n"), 0644)

	plan := planGuidanceFor(t, s, `["claude","codex"]`)
	if len(plan.Changes) != 2 {
		t.Fatalf("plan = %+v", plan)
	}
	applyGuidance(t, s, `["claude","codex"]`, plan.Token)
	agents := readFile(t, filepath.Join(root, "AGENTS.md"))
	if !strings.HasPrefix(agents, "project rules\n\n<!-- skillshare:memory scope=project") || !strings.Contains(agents, "`.skillshare/extras/memory` (relative to the project root)") || strings.Contains(agents, root) {
		t.Errorf("AGENTS.md = %q", agents)
	}
	if got := readFile(t, filepath.Join(root, "CLAUDE.md")); !strings.HasPrefix(got, "own\n\n<!-- skillshare:memory scope=project") {
		t.Errorf("CLAUDE.md = %q", got)
	}
	for name, target := range guidanceState(t, s) {
		if target.State != "configured" {
			t.Errorf("%s = %+v", name, target)
		}
	}
}

func TestMemoryGuidance_ProjectReaderOfAgentsGetsNoOwnFile(t *testing.T) {
	s, root := newTestProjectServerWithExtras(t, nil)
	s.projectCfg.Targets = []config.ProjectTargetEntry{{Name: "claude"}}
	if err := s.projectCfg.Save(root); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("rules\n"), 0644)
	os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("@AGENTS.md\n"), 0644)

	plan := planGuidanceFor(t, s, `["claude"]`)
	if len(plan.Changes) != 1 || plan.Changes[0].Path != filepath.Join(root, "AGENTS.md") {
		t.Fatalf("plan = %+v", plan)
	}
	applyGuidance(t, s, `["claude"]`, plan.Token)
	if readFile(t, filepath.Join(root, "CLAUDE.md")) != "@AGENTS.md\n" {
		t.Error("CLAUDE.md changed")
	}
}

func TestMemoryGuidance_SyncsCopiesOfChangedSharedFile(t *testing.T) {
	s, home := newInstructionsServer(t, "codex")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"personal\n"}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["codex"],"extras":["personal"]}`)
	if rr := instructionsRequest(t, s, http.MethodPut, "/api/instructions/personal/targets/codex/mode", `{"mode":"copy"}`); rr.Code != http.StatusOK {
		t.Fatalf("mode: %d %s", rr.Code, rr.Body)
	}
	plan := planGuidanceFor(t, s, `["codex"]`)
	if len(plan.Changes) != 1 || strings.Contains(plan.Changes[0].Path, ".codex") {
		t.Fatalf("plan = %+v", plan)
	}
	applyGuidance(t, s, `["codex"]`, plan.Token)
	if got := readFile(t, filepath.Join(home, ".codex", "AGENTS.md")); !strings.HasPrefix(got, "personal\n\n<!-- skillshare:memory") {
		t.Errorf("codex copy = %q", got)
	}
	if state := guidanceState(t, s)["codex"].State; state != "configured" {
		t.Errorf("state = %s", state)
	}
}

func TestMemoryGuidance_ProjectAgentsLinkWritesItsRealFile(t *testing.T) {
	s, root := newTestProjectServerWithExtras(t, nil)
	s.projectCfg.Targets = []config.ProjectTargetEntry{{Name: "codex"}}
	if err := s.projectCfg.Save(root); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, "docs"), 0755)
	real := filepath.Join(root, "docs", "RULES.md")
	os.WriteFile(real, []byte("rules\n"), 0644)
	agents := filepath.Join(root, "AGENTS.md")
	os.Symlink("docs/RULES.md", agents)

	plan := planGuidanceFor(t, s, `["codex"]`)
	if len(plan.Changes) != 1 || plan.Changes[0].Path != real || plan.Changes[0].Created {
		t.Fatalf("plan = %+v", plan)
	}
	other := filepath.Join(root, "docs", "OTHER.md")
	os.WriteFile(other, []byte("rules\n"), 0644)
	os.Remove(agents)
	os.Symlink("docs/OTHER.md", agents)
	rr := instructionsRequest(t, s, http.MethodPost, "/api/extras/memory/guidance/apply", `{"targets":["codex"],"token":"`+plan.Token+`"}`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("relinked AGENTS.md: %d %s", rr.Code, rr.Body)
	}

	plan = planGuidanceFor(t, s, `["codex"]`)
	applyGuidance(t, s, `["codex"]`, plan.Token)
	if dest, err := os.Readlink(agents); err != nil || dest != "docs/OTHER.md" {
		t.Errorf("AGENTS.md link = %q, %v", dest, err)
	}
	if got := readFile(t, other); !strings.HasPrefix(got, "rules\n\n<!-- skillshare:memory scope=project") || readFile(t, real) != "rules\n" {
		t.Errorf("OTHER.md = %q, RULES.md = %q", got, readFile(t, real))
	}
}

func TestMemoryGuidance_RejectsPlanAfterRoutingChange(t *testing.T) {
	s, _ := newInstructionsServer(t, "codex")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"personal\n"}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["codex"],"extras":["personal"]}`)
	plan := planGuidanceFor(t, s, `["codex"]`)
	if rr := instructionsRequest(t, s, http.MethodPut, "/api/instructions/personal/targets/codex/mode", `{"mode":"copy"}`); rr.Code != http.StatusOK {
		t.Fatalf("mode: %d %s", rr.Code, rr.Body)
	}
	if again := planGuidanceFor(t, s, `["codex"]`); again.Changes[0].Before != plan.Changes[0].Before || again.Changes[0].After != plan.Changes[0].After {
		t.Fatalf("reviewed text changed; the test needs equal text: %+v", again)
	}
	rr := instructionsRequest(t, s, http.MethodPost, "/api/extras/memory/guidance/apply", `{"targets":["codex"],"token":"`+plan.Token+`"}`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("apply after mode change: %d %s", rr.Code, rr.Body)
	}
}

func TestMemoryGuidance_WarnsLimitOfUnselectedReader(t *testing.T) {
	s, _ := newInstructionsServer(t, "codex", "windsurf")
	long := strings.Repeat("x", 5990)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"`+long+`\n"}`)
	if rr := instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["codex","windsurf"],"extras":["personal"]}`); rr.Code != http.StatusOK {
		t.Fatalf("assign: %d %s", rr.Code, rr.Body)
	}
	plan := planGuidanceFor(t, s, `["codex"]`)
	var limited bool
	for _, w := range plan.Warnings {
		limited = limited || w.Code == "over_limit" && w.Target == "windsurf" && w.Limit == 6000
	}
	if !limited {
		t.Fatalf("warnings = %+v", plan.Warnings)
	}
}

func TestMemoryGuidance_CreatedFlagForNewFile(t *testing.T) {
	s, _ := newInstructionsServer(t, "codex")
	plan := planGuidanceFor(t, s, `["codex"]`)
	if len(plan.Changes) != 1 || !plan.Changes[0].Created || plan.Changes[0].Before != "" {
		t.Fatalf("plan = %+v", plan)
	}
	if res := applyGuidance(t, s, `["codex"]`, plan.Token); res["success"] != true {
		t.Fatalf("apply = %v", res)
	}
	if got := readFile(t, plan.Changes[0].Path); got != plan.Changes[0].After {
		t.Fatalf("created guidance = %q", got)
	}
}

func TestMemoryGuidance_SkipsNonUTF8Files(t *testing.T) {
	latin1 := []byte("caf\xe9 rules\n")
	cases := map[string]func(t *testing.T) (*Server, string, string){
		"global own file": func(t *testing.T) (*Server, string, string) {
			s, home := newInstructionsServer(t, "codex")
			return s, writeHome(t, home, ".codex/AGENTS.md", string(latin1)), "codex"
		},
		"global shared source": func(t *testing.T) (*Server, string, string) {
			s, _ := newInstructionsServer(t, "codex")
			instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"p\n"}`)
			instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["codex"],"extras":["personal"]}`)
			extra, _ := s.sharedExtra("personal")
			source := filepath.Join(s.extrasSourceDir(extra), extra.File)
			os.WriteFile(source, latin1, 0644)
			return s, source, "codex"
		},
		"project AGENTS.md": func(t *testing.T) (*Server, string, string) {
			s, root := newTestProjectServerWithExtras(t, nil)
			s.projectCfg.Targets = []config.ProjectTargetEntry{{Name: "codex"}}
			if err := s.projectCfg.Save(root); err != nil {
				t.Fatal(err)
			}
			agents := filepath.Join(root, "AGENTS.md")
			os.WriteFile(agents, latin1, 0644)
			return s, agents, "codex"
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			s, file, target := setup(t)
			if got := guidanceState(t, s)[target]; got.State != "broken" || got.Detail != "unsupported" || got.File != file {
				t.Fatalf("status = %+v", got)
			}
			plan := planGuidanceFor(t, s, `["`+target+`"]`)
			if len(plan.Changes) != 0 || len(plan.Skipped) != 1 || plan.Skipped[0].Reason != "unsupported" {
				t.Fatalf("plan = %+v", plan)
			}
			if res := applyGuidance(t, s, `["`+target+`"]`, plan.Token); len(res["applied"].([]any)) != 0 {
				t.Fatalf("apply = %v", res)
			}
			if got, _ := os.ReadFile(file); string(got) != string(latin1) {
				t.Errorf("bytes changed: %q", got)
			}
		})
	}
}

func TestMemoryGuidance_RechecksEachReviewedFile(t *testing.T) {
	for _, change := range []string{"edited", "deleted", "created"} {
		t.Run(change, func(t *testing.T) {
			s, home := newInstructionsServer(t, "claude", "codex")
			writeHome(t, home, ".claude/CLAUDE.md", "claude own\n")
			codex := filepath.Join(home, ".codex/AGENTS.md")
			if change != "created" {
				writeHome(t, home, ".codex/AGENTS.md", "codex own\n")
			}
			plan := planGuidanceFor(t, s, `["claude","codex"]`)
			if len(plan.Changes) != 2 {
				t.Fatalf("plan = %+v", plan)
			}
			if err := writeGuidanceChange(plan.Changes[0]); err != nil {
				t.Fatal(err)
			}
			// An external editor changes the later file while the earlier file is applied.
			if change == "deleted" {
				if err := os.Remove(codex); err != nil {
					t.Fatal(err)
				}
			} else {
				content := "external edit\n"
				if change == "created" {
					content = ""
				}
				writeHome(t, home, ".codex/AGENTS.md", content)
			}
			if err := writeGuidanceChange(plan.Changes[1]); !errors.Is(err, errGuidanceStale) {
				t.Fatalf("expected stale conflict, got %v", err)
			}
			if change == "deleted" {
				if _, err := os.Stat(codex); !os.IsNotExist(err) {
					t.Fatal("deleted instructions recreated")
				}
			} else {
				want := "external edit\n"
				if change == "created" {
					want = ""
				}
				if got := readFile(t, codex); got != want {
					t.Fatalf("external edit lost: %q", got)
				}
			}
		})
	}
}

func TestMemoryGuidance_CreateIsExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	change := guidanceChange{Path: path, Created: true, After: "reviewed guidance"}
	if err := checkGuidanceChange(change); err != nil {
		t.Fatal(err)
	}
	// A different process creates the file after the final review check.
	if err := os.WriteFile(path, []byte("external instructions"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := commitGuidanceChange(change); !errors.Is(err, errGuidanceStale) {
		t.Fatalf("expected stale plan, got %v", err)
	}
	if got := readFile(t, path); got != "external instructions" {
		t.Fatalf("external instructions lost: %q", got)
	}
}

func TestMemoryGuidance_SwitchesModeForEveryReaderOfTheFile(t *testing.T) {
	s, home := newInstructionsServer(t, "claude", "codex")
	writeHome(t, home, ".claude/CLAUDE.md", "# Me\n")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"personal\n"}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude","codex"],"extras":["personal"]}`)
	plan := planGuidanceFor(t, s, `["codex"]`)
	applyGuidance(t, s, `["codex"]`, plan.Token)
	if got := guidanceState(t, s)["claude"]; got.State != "configured" || got.Mode != memory.ModePassive {
		t.Fatalf("before = %+v", got)
	}

	body := `{"targets":["codex"],"modes":{"codex":"active"}`
	rr := instructionsRequest(t, s, http.MethodPost, "/api/extras/memory/guidance/plan", body+`}`)
	plan = decodeBody[guidancePlan](t, rr)
	if rr.Code != http.StatusOK || len(plan.Changes) != 1 || !strings.Contains(plan.Changes[0].After, "mode=active") {
		t.Fatalf("plan: %d %+v", rr.Code, plan)
	}
	if rr := instructionsRequest(t, s, http.MethodPost, "/api/extras/memory/guidance/apply", body+`,"token":"`+plan.Token+`"}`); rr.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", rr.Code, rr.Body)
	}
	for name, target := range guidanceState(t, s) {
		if target.State != "configured" || target.Mode != memory.ModeActive {
			t.Errorf("%s = %+v", name, target)
		}
	}
}

func TestMemoryGuidance_RejectsDifferentModesForOneFile(t *testing.T) {
	s, _ := newInstructionsServer(t, "claude", "codex")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"personal\n"}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude","codex"],"extras":["personal"]}`)
	rr := instructionsRequest(t, s, http.MethodPost, "/api/extras/memory/guidance/plan", `{"targets":["claude","codex"],"modes":{"claude":"passive","codex":"active"}}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "same mode") {
		t.Errorf("plan: %d %s", rr.Code, rr.Body)
	}
}

func TestMemoryGuidance_FlagsTargetReadingBlocksOfDifferentModes(t *testing.T) {
	s, home := newInstructionsServer(t, "claude", "codex")
	writeHome(t, home, ".claude/CLAUDE.md", "# Me\n")
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"a","content":"a\n"}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"b","content":"b\n"}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude"],"extras":["a","b"]}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["codex"],"extras":["b"]}`)
	plan := planGuidanceFor(t, s, `["claude"]`)
	applyGuidance(t, s, `["claude"]`, plan.Token)
	body := `{"targets":["codex"],"modes":{"codex":"active"}`
	plan = decodeBody[guidancePlan](t, instructionsRequest(t, s, http.MethodPost, "/api/extras/memory/guidance/plan", body+`}`))
	if rr := instructionsRequest(t, s, http.MethodPost, "/api/extras/memory/guidance/apply", body+`,"token":"`+plan.Token+`"}`); rr.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", rr.Code, rr.Body)
	}

	if got := guidanceState(t, s)["claude"]; got.State != "broken" || got.Detail != "mixed_modes" {
		t.Errorf("claude = %+v", got)
	}
}

func TestMemoryGuidance_RefusesModeSwitchForTargetReadingSeveralBlocks(t *testing.T) {
	s, home := newInstructionsServer(t, "claude", "codex")
	writeHome(t, home, ".claude/CLAUDE.md", "# Me\n")
	plan := planGuidanceFor(t, s, `["claude"]`)
	applyGuidance(t, s, `["claude"]`, plan.Token)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions", `{"name":"personal","content":"personal\n"}`)
	instructionsRequest(t, s, http.MethodPost, "/api/instructions/assign", `{"targets":["claude","codex"],"extras":["personal"]}`)
	plan = planGuidanceFor(t, s, `["codex"]`)
	applyGuidance(t, s, `["codex"]`, plan.Token)

	rr := instructionsRequest(t, s, http.MethodPost, "/api/extras/memory/guidance/plan", `{"targets":["claude"],"modes":{"claude":"active"}}`)
	plan = decodeBody[guidancePlan](t, rr)
	if rr.Code != http.StatusOK || len(plan.Changes) != 0 || len(plan.Skipped) != 1 || plan.Skipped[0].Reason != "multiple_blocks" {
		t.Errorf("plan: %d %+v", rr.Code, plan)
	}
}
