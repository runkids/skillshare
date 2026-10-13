package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/sync"
	"skillshare/internal/utils"
)

func writeSourceFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeSyncSkill(t *testing.T, source, relPath, name string) {
	t.Helper()
	writeSourceFile(t, filepath.Join(source, relPath, "SKILL.md"), "---\nname: "+name+"\n---\n# "+name+"\n")
}

func skillTarget(path, mode, naming string) config.TargetConfig {
	return config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: path, Mode: mode, TargetNaming: naming}}
}

// syncedTargetsModel is the list model runListTUI builds once the source is loaded.
func syncedTargetsModel(t *testing.T, source string, targets map[string]config.TargetConfig, defaultMode string) listTUIModel {
	t.Helper()
	discovered, err := sync.DiscoverSourceSkillsAll(source)
	if err != nil {
		t.Fatal(err)
	}
	m := newListTUIModel(nil, nil, 0, "global", source, "", targets, kindAll)
	m.discovered = discovered
	m.defaultMode = defaultMode
	return m
}

func skillEntryAt(relPath string) skillEntry {
	return skillEntry{Name: utils.PathToFlatName(relPath), Kind: "skill", RelPath: relPath}
}

func assertSyncedTo(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("synced targets = %v, want %v", got, want)
	}
}

func TestFindSyncedTargets_MergeLinksRespectFilters(t *testing.T) {
	source, dst := t.TempDir(), t.TempDir()
	writeSyncSkill(t, source, "demo", "demo")
	writeSyncSkill(t, source, "other", "other")
	tc := skillTarget(dst, "merge", "")
	tc.Skills.Include = []string{"demo"}
	m := syncedTargetsModel(t, source, map[string]config.TargetConfig{"claude": tc}, "")
	if _, err := sync.SyncTargetMergeWithSkills("claude", tc, m.discovered, source, false, false, ""); err != nil {
		t.Fatal(err)
	}

	assertSyncedTo(t, m.findSyncedTargets(skillEntryAt("demo")), "claude")
	assertSyncedTo(t, m.findSyncedTargets(skillEntryAt("other")))
}

func TestFindSyncedTargets_DisablingSkillDropsItsTargets(t *testing.T) {
	source, dst := t.TempDir(), t.TempDir()
	writeSyncSkill(t, source, "demo", "demo")
	tc := skillTarget(dst, "merge", "")
	targets := map[string]config.TargetConfig{"claude": tc}
	discovered := syncedTargetsModel(t, source, targets, "").discovered
	if _, err := sync.SyncTargetMergeWithSkills("claude", tc, discovered, source, false, false, ""); err != nil {
		t.Fatal(err)
	}
	m := newListTUIModel(nil, []skillItem{{entry: skillEntryAt("demo")}}, 1, "global", source, "", targets, kindAll)
	m.discovered = discovered
	assertSyncedTo(t, m.findSyncedTargets(skillEntryAt("demo")), "claude")

	m = pressKey(t, m, "t")

	// A reopened list rediscovers the skill as disabled and shows no target; the open one must agree.
	assertSyncedTo(t, m.findSyncedTargets(skillEntryAt("demo")))
}

func TestFindSyncedTargets_CopyMode(t *testing.T) {
	source, dst := t.TempDir(), t.TempDir()
	writeSyncSkill(t, source, "demo", "demo")
	tc := skillTarget(dst, "copy", "")
	m := syncedTargetsModel(t, source, map[string]config.TargetConfig{"codex": tc}, "")
	if _, err := sync.SyncTargetCopyWithSkills("codex", tc, m.discovered, source, false, false, nil); err != nil {
		t.Fatal(err)
	}

	assertSyncedTo(t, m.findSyncedTargets(skillEntryAt("demo")), "codex")
}

func TestFindSyncedTargets_TargetInheritsDefaultMode(t *testing.T) {
	source, dst := t.TempDir(), t.TempDir()
	writeSyncSkill(t, source, "demo", "demo")
	m := syncedTargetsModel(t, source, map[string]config.TargetConfig{"codex": skillTarget(dst, "", "")}, "copy")
	if _, err := sync.SyncTargetCopyWithSkills("codex", skillTarget(dst, "copy", ""), m.discovered, source, false, false, nil); err != nil {
		t.Fatal(err)
	}

	assertSyncedTo(t, m.findSyncedTargets(skillEntryAt("demo")), "codex")
}

func TestFindSyncedTargets_SymlinkModeLinksWholeDirectory(t *testing.T) {
	source := t.TempDir()
	dst := filepath.Join(t.TempDir(), "skills")
	writeSyncSkill(t, source, "demo", "demo")
	tc := skillTarget(dst, "symlink", "")
	if err := sync.SyncTarget("claude", tc, source, false, ""); err != nil {
		t.Fatal(err)
	}
	m := syncedTargetsModel(t, source, map[string]config.TargetConfig{"claude": tc}, "")

	assertSyncedTo(t, m.findSyncedTargets(skillEntryAt("demo")), "claude")
}

func TestFindSyncedTargets_StandardNamingUsesSkillName(t *testing.T) {
	source, dst := t.TempDir(), t.TempDir()
	writeSyncSkill(t, source, "team/demo", "demo")
	tc := skillTarget(dst, "merge", "standard")
	m := syncedTargetsModel(t, source, map[string]config.TargetConfig{"claude": tc}, "")
	if _, err := sync.SyncTargetMergeWithSkills("claude", tc, m.discovered, source, false, false, ""); err != nil {
		t.Fatal(err)
	}

	assertSyncedTo(t, m.findSyncedTargets(skillEntryAt("team/demo")), "claude")
}

func TestFindSyncedTargets_PrefixedNamingCopy(t *testing.T) {
	source, dst := t.TempDir(), t.TempDir()
	writeSyncSkill(t, source, "_team/demo", "demo")
	tc := skillTarget(dst, "copy", "prefixed")
	m := syncedTargetsModel(t, source, map[string]config.TargetConfig{"claude": tc}, "")
	if _, err := sync.SyncTargetCopyWithSkills("claude", tc, m.discovered, source, false, false, nil); err != nil {
		t.Fatal(err)
	}

	assertSyncedTo(t, m.findSyncedTargets(skillEntryAt("_team/demo")), "claude")
}

func TestFindSyncedTargets_SkillsOffTargetIsNotListed(t *testing.T) {
	source, dst := t.TempDir(), t.TempDir()
	writeSyncSkill(t, source, "demo", "demo")
	tc := skillTarget(dst, "merge", "")
	m := syncedTargetsModel(t, source, map[string]config.TargetConfig{"claude": tc}, "")
	if _, err := sync.SyncTargetMergeWithSkills("claude", tc, m.discovered, source, false, false, ""); err != nil {
		t.Fatal(err)
	}
	tc.Skills.SetEnabled(false)
	m.targets["claude"] = tc

	assertSyncedTo(t, m.findSyncedTargets(skillEntryAt("demo")))
}

func TestFindSyncedTargets_Agents(t *testing.T) {
	entry := skillEntry{Name: "helper", Kind: "agent", RelPath: "team/helper.md"}
	for _, mode := range []string{"merge", "copy", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			agentsSrc := t.TempDir()
			dst := filepath.Join(t.TempDir(), "agents")
			writeSourceFile(t, filepath.Join(agentsSrc, "team", "helper.md"), "---\nname: helper\n---\n# helper\n")
			agents, err := resource.AgentKind{}.Discover(agentsSrc)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := sync.SyncAgents(agents, agentsSrc, dst, mode, false, false); err != nil {
				t.Fatal(err)
			}
			tc := config.TargetConfig{Agents: &config.ResourceTargetConfig{Path: dst, Mode: mode}}
			m := newListTUIModel(nil, nil, 0, "global", t.TempDir(), agentsSrc, map[string]config.TargetConfig{"claude": tc}, kindAll)

			assertSyncedTo(t, m.findSyncedTargets(entry), "claude")
		})
	}
}

func TestFindSyncedTargets_AgentsUseBuiltInPathWhenUnset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	agentsSrc := t.TempDir()
	writeSourceFile(t, filepath.Join(agentsSrc, "helper.md"), "---\nname: helper\n---\n# helper\n")
	agents, err := resource.AgentKind{}.Discover(agentsSrc)
	if err != nil {
		t.Fatal(err)
	}
	dst := config.ExpandPath(config.DefaultAgentTargets()["claude"].Path)
	if _, err := sync.SyncAgents(agents, agentsSrc, dst, "merge", false, false); err != nil {
		t.Fatal(err)
	}
	m := newListTUIModel(nil, nil, 0, "global", t.TempDir(), agentsSrc, map[string]config.TargetConfig{"claude": {}}, kindAll)

	assertSyncedTo(t, m.findSyncedTargets(skillEntry{Name: "helper", Kind: "agent", RelPath: "helper.md"}), "claude")
}

func TestFindSyncedTargets_ProjectAgentsUseOnlyTheirResolvedPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	agentsSrc := t.TempDir()
	writeSourceFile(t, filepath.Join(agentsSrc, "helper.md"), "---\nname: helper\n---\n# helper\n")
	agents, err := resource.AgentKind{}.Discover(agentsSrc)
	if err != nil {
		t.Fatal(err)
	}
	// The global built-in agents directory holds the agent; a project target without agents must not read it.
	global := config.ExpandPath(config.DefaultAgentTargets()["claude"].Path)
	projectDir := filepath.Join(t.TempDir(), ".codex", "agents")
	for _, dir := range []string{global, projectDir} {
		if _, err := sync.SyncAgents(agents, agentsSrc, dir, "merge", false, false); err != nil {
			t.Fatal(err)
		}
	}
	targets := map[string]config.TargetConfig{
		"claude": {},
		"codex":  {Agents: &config.ResourceTargetConfig{Path: projectDir}},
	}
	m := newListTUIModel(nil, nil, 0, "project", t.TempDir(), agentsSrc, targets, kindAll)

	assertSyncedTo(t, m.findSyncedTargets(skillEntry{Name: "helper", Kind: "agent", RelPath: "helper.md"}), "codex")
}

func TestFindSyncedTargets_AgentExtensionOutput(t *testing.T) {
	agentsSrc := t.TempDir()
	dst := filepath.Join(t.TempDir(), "agents")
	writeSourceFile(t, filepath.Join(agentsSrc, "helper.md"), "---\nname: helper\n---\n# helper\n")
	agents, err := resource.AgentKind{}.Discover(agentsSrc)
	if err != nil {
		t.Fatal(err)
	}
	spec := &sync.ExtensionSpec{Run: []string{"cat"}, Dir: agentsSrc, Name: "id", OutputExt: "toml"}
	if _, err := sync.SyncAgentsTransform(agents, agentsSrc, dst, "copy", spec, false, false); err != nil {
		t.Fatal(err)
	}
	tc := config.TargetConfig{Agents: &config.ResourceTargetConfig{Path: dst, Extension: "id"}}
	m := newListTUIModel(nil, nil, 0, "global", t.TempDir(), agentsSrc, map[string]config.TargetConfig{"claude": tc}, kindAll)

	assertSyncedTo(t, m.findSyncedTargets(skillEntry{Name: "helper", Kind: "agent", RelPath: "helper.md"}), "claude")
}

func TestFindSyncedTargets_EntryKeepsNameFromEarlierNaming(t *testing.T) {
	source, dst := t.TempDir(), t.TempDir()
	writeSyncSkill(t, source, "team/demo", "demo")
	flat := skillTarget(dst, "merge", "")
	m := syncedTargetsModel(t, source, map[string]config.TargetConfig{"claude": skillTarget(dst, "merge", "standard")}, "")
	if _, err := sync.SyncTargetMergeWithSkills("claude", flat, m.discovered, source, false, false, ""); err != nil {
		t.Fatal(err)
	}

	assertSyncedTo(t, m.findSyncedTargets(skillEntryAt("team/demo")), "claude")
}
