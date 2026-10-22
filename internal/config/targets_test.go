package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGroupedProjectTargets_UniversalGrouped(t *testing.T) {
	grouped := GroupedProjectTargets()

	// Find the universal group entry
	var universalGroup *GroupedProjectTarget
	for i, g := range grouped {
		if g.Name == "universal" {
			universalGroup = &grouped[i]
			break
		}
	}

	if universalGroup == nil {
		t.Fatal("expected 'universal' group in GroupedProjectTargets result")
	}

	if filepath.ToSlash(universalGroup.Path) != ".agents/skills" {
		t.Errorf("universal group path = %q, want %q", universalGroup.Path, ".agents/skills")
	}

	if len(universalGroup.Members) == 0 {
		t.Fatal("universal group should have members")
	}

	// Verify known members are present
	memberSet := make(map[string]bool)
	for _, m := range universalGroup.Members {
		memberSet[m] = true
	}

	expectedMembers := []string{"amp", "codex", "cursor", "dexto", "kimi", "replit"}
	for _, name := range expectedMembers {
		if !memberSet[name] {
			t.Errorf("expected %q in universal group members, got %v", name, universalGroup.Members)
		}
	}

	// Canonical name should NOT be in members
	if memberSet["universal"] {
		t.Error("canonical name 'universal' should not appear in members list")
	}
}

func TestGroupedProjectTargets_SinglePathNotGrouped(t *testing.T) {
	grouped := GroupedProjectTargets()

	// copilot has a unique path (.github/skills), should not have members
	for _, g := range grouped {
		if g.Name == "copilot" {
			if len(g.Members) != 0 {
				t.Errorf("copilot should have no members, got %v", g.Members)
			}
			return
		}
	}

	t.Error("copilot not found in GroupedProjectTargets result")
}

func TestGroupedProjectTargets_NoDuplicatePaths(t *testing.T) {
	grouped := GroupedProjectTargets()

	seen := make(map[string]bool)
	for _, g := range grouped {
		if seen[g.Path] {
			t.Errorf("duplicate path %q in GroupedProjectTargets", g.Path)
		}
		seen[g.Path] = true
	}
}

func TestGroupedProjectTargets_MembersAreSorted(t *testing.T) {
	grouped := GroupedProjectTargets()

	for _, g := range grouped {
		if len(g.Members) < 2 {
			continue
		}
		for i := 1; i < len(g.Members); i++ {
			if g.Members[i] < g.Members[i-1] {
				t.Errorf("members of %q not sorted: %v", g.Name, g.Members)
				break
			}
		}
	}
}

func TestLookupProjectTarget_Alias(t *testing.T) {
	// Canonical name should resolve
	tc, ok := LookupProjectTarget("claude")
	if !ok {
		t.Fatal("LookupProjectTarget should find canonical name 'claude'")
	}
	if tc.Path == "" {
		t.Error("expected non-empty path for claude")
	}

	// Alias should also resolve to the same target
	tcAlias, ok := LookupProjectTarget("claude-code")
	if !ok {
		t.Fatal("LookupProjectTarget should find alias 'claude-code'")
	}
	if tcAlias.Path != tc.Path {
		t.Errorf("alias path %q != canonical path %q", tcAlias.Path, tc.Path)
	}

	// Unknown name should not resolve
	_, ok = LookupProjectTarget("nonexistent-tool")
	if ok {
		t.Error("LookupProjectTarget should not find unknown name")
	}
}

// --- Agent target tests ---

func TestDefaultAgentTargets_V1TargetsHaveAgentPaths(t *testing.T) {
	agents := DefaultAgentTargets()

	v1Targets := []string{"claude", "cursor", "opencode", "augment"}
	for _, name := range v1Targets {
		tc, ok := agents[name]
		if !ok {
			t.Errorf("expected %q in DefaultAgentTargets", name)
			continue
		}
		if tc.Path == "" {
			t.Errorf("expected non-empty agent path for %q", name)
		}
	}
}

func TestDefaultAgentTargets_NonV1Excluded(t *testing.T) {
	agents := DefaultAgentTargets()

	// codex requires Markdown->TOML conversion (handled via extension), windsurf
	// has no native agent format. These should NOT have agent paths.
	// copilot uses .agent.md natively, so it DOES have an agent path (see #184).
	for _, name := range []string{"codex", "windsurf"} {
		if _, ok := agents[name]; ok {
			t.Errorf("%q should not be in DefaultAgentTargets (not v1 agent target)", name)
		}
	}
}

func TestProjectAgentTargets_V1TargetsHaveAgentPaths(t *testing.T) {
	agents := ProjectAgentTargets()

	v1Targets := []string{"claude", "cursor", "opencode", "augment"}
	for _, name := range v1Targets {
		tc, ok := agents[name]
		if !ok {
			t.Errorf("expected %q in ProjectAgentTargets", name)
			continue
		}
		if tc.Path == "" {
			t.Errorf("expected non-empty agent project path for %q", name)
		}
	}
}

func TestProjectAgentTargets_NonV1Excluded(t *testing.T) {
	agents := ProjectAgentTargets()

	// copilot uses .agent.md natively and now has a project agent path (see #184).
	for _, name := range []string{"codex", "windsurf"} {
		if _, ok := agents[name]; ok {
			t.Errorf("%q should not be in ProjectAgentTargets", name)
		}
	}
}

func TestLookupAgentTarget_Alias(t *testing.T) {
	global, ok := LookupGlobalAgentTarget("factory")
	if !ok {
		t.Fatal("LookupGlobalAgentTarget should find alias 'factory'")
	}
	if global.Path == "" {
		t.Fatal("expected non-empty global agent path for factory")
	}

	project, ok := LookupProjectAgentTarget("factory")
	if !ok {
		t.Fatal("LookupProjectAgentTarget should find alias 'factory'")
	}
	if filepath.ToSlash(project.Path) != ".factory/droids" {
		t.Fatalf("factory project agent path = %q, want %q", project.Path, ".factory/droids")
	}
}

func TestTargetSpec_ParsesDetect(t *testing.T) {
	var file targetsFile
	if err := yaml.Unmarshal([]byte("targets:\n  - name: codex\n    detect: \"~/.codex\"\n    skills:\n      global: \"~/.agents/skills\"\n"), &file); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := file.Targets[0].Detect; got != "~/.codex" {
		t.Errorf("Detect = %q, want %q", got, "~/.codex")
	}
}

func TestDetectDir_UnsetTargetsReturnEmpty(t *testing.T) {
	if got := DetectDir("claude"); got != "" {
		t.Errorf("DetectDir(claude) = %q, want empty (no detect in targets.yaml)", got)
	}
	if got := DetectDir("nonexistent-tool"); got != "" {
		t.Errorf("DetectDir(unknown) = %q, want empty", got)
	}
}

func TestRuntimeScanPaths_UnionOfAlsoScansAndPrimary(t *testing.T) {
	got := RuntimeScanPaths("codex", false)
	for _, want := range []string{normalizeTargetPath("~/.agents/skills"), normalizeTargetPath("~/.codex/skills")} {
		if !slices.Contains(got, want) {
			t.Errorf("RuntimeScanPaths(codex, global) = %v, missing %s", got, want)
		}
	}

	project := RuntimeScanPaths("codex", true)
	for i := range project {
		project[i] = filepath.ToSlash(project[i])
	}
	for _, want := range []string{".codex/skills", ".agents/skills"} {
		if !slices.Contains(project, want) {
			t.Errorf("RuntimeScanPaths(codex, project) = %v, missing %s", project, want)
		}
	}
}

func TestRuntimeScanPaths_SkipsPathTurnedOffByEnv(t *testing.T) {
	t.Setenv("OPENCODE_DISABLE_CLAUDE_CODE_SKILLS", "1")
	got := RuntimeScanPaths("opencode", false)
	if slices.Contains(got, normalizeTargetPath("~/.claude/skills")) {
		t.Errorf("RuntimeScanPaths(opencode) = %v, want ~/.claude/skills skipped", got)
	}
	if !slices.Contains(got, normalizeTargetPath("~/.agents/skills")) {
		t.Errorf("RuntimeScanPaths(opencode) = %v, want ~/.agents/skills kept", got)
	}
}

func TestRuntimeScanPaths_ExternalSkillsOffSkipsBothProjectPaths(t *testing.T) {
	t.Setenv("OPENCODE_DISABLE_EXTERNAL_SKILLS", "true")
	got := RuntimeScanPaths("opencode", true)
	if len(got) != 1 || filepath.ToSlash(got[0]) != ".opencode/skills" {
		t.Errorf("RuntimeScanPaths(opencode, project) = %v, want only .opencode/skills", got)
	}
}

func TestRuntimeScanPaths_KeepsPathWhenEnvIsFalse(t *testing.T) {
	t.Setenv("OPENCODE_DISABLE_CLAUDE_CODE_SKILLS", "0")
	if got := RuntimeScanPaths("opencode", false); !slices.Contains(got, normalizeTargetPath("~/.claude/skills")) {
		t.Errorf("RuntimeScanPaths(opencode) = %v, want ~/.claude/skills kept", got)
	}
}

func TestRuntimeScanPaths_Deduplicates(t *testing.T) {
	// warp's primary path is ~/.agents/skills and it has no also_scans.
	got := RuntimeScanPaths("warp", false)
	seen := make(map[string]bool, len(got))
	for _, p := range got {
		if seen[p] {
			t.Errorf("RuntimeScanPaths(warp) has duplicate %q: %v", p, got)
		}
		seen[p] = true
	}
}

// Codex documents ~/.agents/skills as its user-level skills path; ~/.codex/skills
// is still read but deprecated. Writing to both would show every skill twice.
func TestDefaultTargets_CodexSharesUniversalPath(t *testing.T) {
	targets := DefaultTargets()
	if got, want := targets["codex"].Path, targets["universal"].Path; got != want {
		t.Errorf("codex default global path = %q, want universal's %q", got, want)
	}
}

func TestAlsoScans_CodexKeepsDeprecatedPath(t *testing.T) {
	if got := alsoScansSpec(t, "codex").Global; !slices.Contains(got, "~/.codex/skills") {
		t.Errorf("codex also_scans.global = %v, missing the deprecated ~/.codex/skills", got)
	}
}

func TestDetectDir_Codex(t *testing.T) {
	// codex's skills path belongs to universal, so only its install dir
	// identifies the tool.
	if got, want := DetectDir("codex"), normalizeTargetPath("~/.codex"); got != want {
		t.Errorf("DetectDir(codex) = %q, want %q", got, want)
	}
}

func TestMatchesTargetName_CodexMatchesUniversal(t *testing.T) {
	// A skill with frontmatter `targets: [codex]` must still sync for users who
	// only have the universal target — both resolve to the same directory.
	if !MatchesTargetName("codex", "universal") {
		t.Error("MatchesTargetName(codex, universal) = false, want true")
	}
}

// goose and openhands both document ~/.agents/skills and .agents/skills as the
// recommended skills locations, keeping their own directories for compatibility.
func TestDefaultTargets_GooseAndOpenHandsShareUniversalPath(t *testing.T) {
	targets := DefaultTargets()
	projects := ProjectTargets()
	for _, name := range []string{"goose", "openhands"} {
		if got, want := targets[name].Path, targets["universal"].Path; got != want {
			t.Errorf("%s default global path = %q, want universal's %q", name, got, want)
		}
		if got, want := filepath.ToSlash(projects[name].Path), ".agents/skills"; got != want {
			t.Errorf("%s default project path = %q, want %q", name, got, want)
		}
	}
}

func TestAlsoScans_GooseAndOpenHandsKeepLegacyPaths(t *testing.T) {
	for name, want := range map[string][2]string{
		"goose":     {"~/.config/goose/skills", ".goose/skills"},
		"openhands": {"~/.openhands/skills", ".openhands/skills"},
	} {
		spec := alsoScansSpec(t, name)
		if !slices.Contains(spec.Global, want[0]) {
			t.Errorf("%s also_scans.global = %v, missing the legacy %s", name, spec.Global, want[0])
		}
		if !slices.Contains(spec.Project, want[1]) {
			t.Errorf("%s also_scans.project = %v, missing the legacy %s", name, spec.Project, want[1])
		}
	}
}

func TestDetectDir_GooseAndOpenHands(t *testing.T) {
	// Their skills paths belong to universal, so only the install dir identifies
	// the tool.
	for name, want := range map[string]string{"goose": "~/.config/goose", "openhands": "~/.openhands"} {
		if got := DetectDir(name); got != normalizeTargetPath(want) {
			t.Errorf("DetectDir(%s) = %q, want %q", name, got, normalizeTargetPath(want))
		}
	}
}

// Zencoder documents ~/.agents/skills as primary and ~/.zencoder/skills as
// deprecated but still read, so it follows the shared directory like codex.
func TestDefaultTargets_ZencoderSharesUniversalPath(t *testing.T) {
	targets := DefaultTargets()
	if got, want := targets["zencoder"].Path, targets["universal"].Path; got != want {
		t.Errorf("zencoder default global path = %q, want universal's %q", got, want)
	}
	if got := alsoScansSpec(t, "zencoder"); !slices.Contains(got.Global, "~/.zencoder/skills") || !slices.Contains(got.Project, ".zencoder/skills") {
		t.Errorf("zencoder also_scans = %+v, missing the deprecated .zencoder/skills paths", got)
	}
	if got, want := DetectDir("zencoder"), normalizeTargetPath("~/.zencoder"); got != want {
		t.Errorf("DetectDir(zencoder) = %q, want %q", got, want)
	}
}

// Replit documents only the project directory. A global config that already
// pins replit to an absolute path is a custom-path target and keeps working.
func TestReplitIsProjectOnly(t *testing.T) {
	if _, ok := DefaultTargets()["replit"]; ok {
		t.Error("replit has no documented global skills path, want no default")
	}
	if got := filepath.ToSlash(ProjectTargets()["replit"].Path); got != ".agents/skills" {
		t.Errorf("replit project path = %q, want .agents/skills", got)
	}
}

func TestProjectTargetDotDirs_IncludesAlsoScans(t *testing.T) {
	dirs := ProjectTargetDotDirs()

	// .clinerules only appears in cline's also_scans.project.
	if !dirs[".clinerules"] {
		t.Error("expected .clinerules (cline also_scans.project) in ProjectTargetDotDirs")
	}
	// .goose and .openhands moved out of the primary paths into also_scans.
	for _, dir := range []string{".goose", ".openhands"} {
		if !dirs[dir] {
			t.Errorf("expected %s (also_scans.project) in ProjectTargetDotDirs", dir)
		}
	}
}

func TestProjectTargetDotDirs_IncludesAgentPaths(t *testing.T) {
	dirs := ProjectTargetDotDirs()

	// .claude should be included (from both skill and agent project paths)
	if !dirs[".claude"] {
		t.Error("expected .claude in ProjectTargetDotDirs")
	}

	// .cursor should be included
	if !dirs[".cursor"] {
		t.Error("expected .cursor in ProjectTargetDotDirs")
	}

	// .skillshare always included
	if !dirs[".skillshare"] {
		t.Error("expected .skillshare in ProjectTargetDotDirs")
	}
}

func TestDefaultTargets_ClaudePath(t *testing.T) {
	targets := DefaultTargets()
	tc, ok := targets["claude"]
	if !ok {
		t.Fatal("expected claude in DefaultTargets")
	}
	// Path should contain "skills" (not "agents")
	if tc.Path == "" {
		t.Error("expected non-empty global path for claude")
	}
}

func TestProjectTargets_ClaudePath(t *testing.T) {
	targets := ProjectTargets()
	tc, ok := targets["claude"]
	if !ok {
		t.Fatal("expected claude in ProjectTargets")
	}
	if filepath.ToSlash(tc.Path) != ".claude/skills" {
		t.Errorf("claude project path = %q, want %q", tc.Path, ".claude/skills")
	}
}

func alsoScansSpec(t *testing.T, name string) targetAlsoScans {
	t.Helper()
	specs, err := loadTargetSpecs()
	if err != nil {
		t.Fatalf("loadTargetSpecs: %v", err)
	}
	for _, spec := range specs {
		if spec.Name == name {
			return spec.AlsoScans
		}
	}
	t.Fatalf("target %q not found", name)
	return targetAlsoScans{}
}

// A target's own primary path is covered by shared_target_paths; listing it in
// also_scans would double-report every overlap.
func TestAlsoScans_ExcludesOwnPrimaryPath(t *testing.T) {
	specs, err := loadTargetSpecs()
	if err != nil {
		t.Fatalf("loadTargetSpecs: %v", err)
	}
	for _, spec := range specs {
		if slices.Contains(spec.AlsoScans.Global, spec.Skills.Global) {
			t.Errorf("%s: also_scans.global repeats its primary path %s", spec.Name, spec.Skills.Global)
		}
		if slices.Contains(spec.AlsoScans.Project, spec.Skills.Project) {
			t.Errorf("%s: also_scans.project repeats its primary path %s", spec.Name, spec.Skills.Project)
		}
	}
}

// Kimi reads ~/.agents/skills only when ~/.config/agents/skills is absent, and
// skillshare always creates the latter, so it is never actually scanned.
func TestAlsoScans_KimiSkipsAgentsFallback(t *testing.T) {
	if got := alsoScansSpec(t, "kimi").Global; slices.Contains(got, "~/.agents/skills") {
		t.Errorf("kimi also_scans.global = %v, must not list the mutually exclusive ~/.agents/skills fallback", got)
	}
}

func TestAlsoScans_OpencodeReadsClaudeAndAgents(t *testing.T) {
	got := alsoScansSpec(t, "opencode")
	for _, want := range []string{"~/.claude/skills", "~/.agents/skills"} {
		if !slices.Contains(got.Global, want) {
			t.Errorf("opencode also_scans.global = %v, missing %s", got.Global, want)
		}
	}
	for _, want := range []string{".claude/skills", ".agents/skills"} {
		if !slices.Contains(got.Project, want) {
			t.Errorf("opencode also_scans.project = %v, missing %s", got.Project, want)
		}
	}
}

// The Antigravity CLI (agy) reads its own global directory and never the
// Antigravity app's ~/.gemini/config/skills, so it is a target of its own
// rather than an alias.
func TestDefaultTargets_AntigravityCLIHasOwnPath(t *testing.T) {
	tc, ok := DefaultTargets()["antigravity-cli"]
	if !ok {
		t.Fatal("expected antigravity-cli in DefaultTargets")
	}
	if want := normalizeTargetPath("~/.gemini/antigravity-cli/skills"); tc.Path != want {
		t.Errorf("antigravity-cli global path = %q, want %q", tc.Path, want)
	}
}

func TestLookupProjectTarget_AntigravityCLIIsNotAntigravityAlias(t *testing.T) {
	tc, ok := LookupProjectTarget("antigravity-cli")
	if !ok {
		t.Fatal("LookupProjectTarget should find antigravity-cli")
	}
	if filepath.ToSlash(tc.Path) != ".agents/skills" {
		t.Errorf("antigravity-cli project path = %q, want %q", tc.Path, ".agents/skills")
	}
	if _, ok := ProjectTargets()["antigravity-cli"]; !ok {
		t.Error("antigravity-cli should be a canonical project target, not an alias")
	}
}

func TestAlsoScans_AntigravityHasNone(t *testing.T) {
	for _, name := range []string{"antigravity", "antigravity-cli"} {
		if got := AlsoScansGlobal(name); len(got) != 0 {
			t.Errorf("AlsoScansGlobal(%s) = %v, want none", name, got)
		}
	}
}

func TestTargetSpec_ParsesInstructions(t *testing.T) {
	var file targetsFile
	data := "targets:\n  - name: x\n    skills:\n      global: \"~/.x/skills\"\n    instructions:\n      global: \"~/.x/X.md\"\n      project: \"X.md\"\n      import: true\n"
	if err := yaml.Unmarshal([]byte(data), &file); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := file.Targets[0].Instructions
	if got.Global != "~/.x/X.md" || got.Project != "X.md" || !got.Import {
		t.Errorf("Instructions = %+v", got)
	}
}

func TestLookupInstructions_KnownTargets(t *testing.T) {
	home, _ := os.UserHomeDir()
	tests := []struct {
		name, global, project string
		imp                   bool
	}{
		{"claude", ".claude/CLAUDE.md", "CLAUDE.md", true},
		{"codex", ".codex/AGENTS.md", "AGENTS.md", false},
		{"gemini", ".gemini/GEMINI.md", "GEMINI.md", false},
		{"antigravity", ".gemini/GEMINI.md", "AGENTS.md", false},
		{"opencode", ".config/opencode/AGENTS.md", "AGENTS.md", false},
		{"amp", ".config/amp/AGENTS.md", "AGENTS.md", false},
		{"windsurf", ".codeium/windsurf/memories/global_rules.md", "AGENTS.md", false},
		{"goose", ".config/goose/.goosehints", "AGENTS.md", false},
		{"kiro", ".kiro/steering/AGENTS.md", "AGENTS.md", false},
		{"roo", ".roo/rules/AGENTS.md", "AGENTS.md", false},
		{"kilocode", ".config/kilo/AGENTS.md", "AGENTS.md", false},
	}
	for _, tt := range tests {
		g, ok := LookupInstructions(tt.name, false)
		if !ok || g.Path != filepath.Join(home, filepath.FromSlash(tt.global)) || g.Import != tt.imp {
			t.Errorf("LookupInstructions(%s, global) = %+v, %v", tt.name, g, ok)
		}
		p, ok := LookupInstructions(tt.name, true)
		if !ok || p.Path != tt.project || p.Import != tt.imp {
			t.Errorf("LookupInstructions(%s, project) = %+v, %v", tt.name, p, ok)
		}
	}
}

func TestLookupInstructions_AliasAndMissing(t *testing.T) {
	if got, ok := LookupInstructions("claude-code", true); !ok || got.Path != "CLAUDE.md" {
		t.Errorf("alias claude-code = %+v, %v", got, ok)
	}
	if _, ok := LookupInstructions("cursor", false); ok {
		t.Error("cursor has no global instructions file")
	}
	if got, ok := LookupInstructions("cursor", true); !ok || got.Path != "AGENTS.md" {
		t.Errorf("cursor project = %+v, %v", got, ok)
	}
	if _, ok := LookupInstructions("adal", true); ok {
		t.Error("adal has no instructions metadata")
	}
}

func TestLookupInstructions_Extras(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got, _ := LookupInstructions("antigravity", false); got.SameAs != "gemini" {
		t.Errorf("antigravity SameAs = %q, want gemini", got.SameAs)
	}
	if got, _ := LookupInstructions("windsurf", false); got.MaxChars != 6000 {
		t.Errorf("windsurf MaxChars = %d, want 6000", got.MaxChars)
	}
	g, _ := LookupInstructions("claude", false)
	if g.Rules != filepath.Join(home, ".claude", "rules") || g.Fallback != "" {
		t.Errorf("claude global = %+v", g)
	}
	if p, _ := LookupInstructions("claude", true); p.Fallback != "AGENTS.md" || p.Rules != filepath.Join(".claude", "rules") {
		t.Errorf("claude project = %+v", p)
	}
}

func TestTargetInstructions_AccountMovesIntoConfigDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "work")
	got, ok := TargetInstructions("claude-work", TargetConfig{Agent: "claude", ConfigDir: dir}, false)
	if !ok || got.Path != filepath.Join(dir, "CLAUDE.md") || got.Rules != filepath.Join(dir, "rules") {
		t.Errorf("account instructions = %+v, %v", got, ok)
	}
}

func TestTargetInstructions_OpenCodeAccountMovesIntoConfigDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "work")
	got, ok := TargetInstructions("opencode-work", TargetConfig{Agent: "opencode", ConfigDir: dir}, false)
	if !ok || got.Path != filepath.Join(dir, "AGENTS.md") {
		t.Errorf("opencode account instructions = %+v, %v", got, ok)
	}
}

func TestTargetInstructions_CustomGlobalExpandsTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	tc := TargetConfig{Instructions: &TargetInstructionsConfig{Path: "~/.myagent/AGENTS.md", Import: true}}
	got, ok := TargetInstructions("myagent", tc, false)
	if !ok || got.Path != filepath.Join(home, ".myagent", "AGENTS.md") || !got.Import {
		t.Errorf("custom global = %+v, %v", got, ok)
	}
}

func TestTargetInstructions_CustomProjectStaysRelative(t *testing.T) {
	tc := TargetConfig{Instructions: &TargetInstructionsConfig{Path: ".myagent/RULES.md"}}
	got, ok := TargetInstructions("myagent", tc, true)
	if !ok || got.Path != filepath.Join(".myagent", "RULES.md") {
		t.Errorf("custom project = %+v, %v", got, ok)
	}
}

func TestTargetInstructions_CustomOverridesBuiltin(t *testing.T) {
	tc := TargetConfig{Instructions: &TargetInstructionsConfig{Path: "/elsewhere/NOTES.md"}}
	got, ok := TargetInstructions("claude", tc, false)
	if !ok || got != (InstructionsTarget{Path: filepath.Clean("/elsewhere/NOTES.md")}) {
		t.Errorf("override = %+v, %v", got, ok)
	}
}

func TestTargetScanPaths_CodexAccount(t *testing.T) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(t.TempDir(), "codex-work")
	target := TargetConfig{
		Agent:     "codex",
		ConfigDir: dir,
		Skills:    &ResourceTargetConfig{Path: filepath.Join(dir, "skills")},
	}
	paths := TargetScanPaths("codex-work", target, false)
	wantUniversal := filepath.Join(home, ".agents", "skills")
	wantAccount := filepath.Join(dir, "skills")
	dontWantDefault := filepath.Join(home, ".codex", "skills")

	if !slices.Contains(paths, wantUniversal) {
		t.Errorf("TargetScanPaths(codex-work) = %v, missing universal %q", paths, wantUniversal)
	}
	if !slices.Contains(paths, wantAccount) {
		t.Errorf("TargetScanPaths(codex-work) = %v, missing account skills %q", paths, wantAccount)
	}
	if slices.Contains(paths, dontWantDefault) {
		t.Errorf("TargetScanPaths(codex-work) = %v, unexpectedly contains default account %q", paths, dontWantDefault)
	}
}
