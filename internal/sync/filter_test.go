package sync

import (
	"reflect"
	"testing"

	"skillshare/internal/resource"
)

func TestFilterSkills_IncludeOnly(t *testing.T) {
	skills := testSkills("codex-plan", "claude-help", "gemini-notes")
	filtered, err := FilterSkills(skills, []string{"codex-*", "claude-help"}, nil)
	if err != nil {
		t.Fatalf("FilterSkills returned error: %v", err)
	}

	assertFlatNames(t, filtered, []string{"codex-plan", "claude-help"})
}

func TestFilterSkills_ExcludeOnly(t *testing.T) {
	skills := testSkills("codex-plan", "claude-help", "gemini-notes")
	filtered, err := FilterSkills(skills, nil, []string{"codex-*", "gemini-*"})
	if err != nil {
		t.Fatalf("FilterSkills returned error: %v", err)
	}

	assertFlatNames(t, filtered, []string{"claude-help"})
}

func TestFilterSkills_IncludeThenExclude(t *testing.T) {
	skills := testSkills("codex-plan", "codex-test", "claude-help")
	filtered, err := FilterSkills(skills, []string{"codex-*"}, []string{"*-test"})
	if err != nil {
		t.Fatalf("FilterSkills returned error: %v", err)
	}

	assertFlatNames(t, filtered, []string{"codex-plan"})
}

func TestFilterSkills_GlobPatterns(t *testing.T) {
	skills := testSkills("test", "best", "zest", "toast")
	filtered, err := FilterSkills(skills, []string{"?est"}, []string{"z*"})
	if err != nil {
		t.Fatalf("FilterSkills returned error: %v", err)
	}

	assertFlatNames(t, filtered, []string{"test", "best"})
}

func TestFilterSkills_EmptyPatternsReturnAll(t *testing.T) {
	skills := testSkills("one", "two", "three")
	filtered, err := FilterSkills(skills, nil, nil)
	if err != nil {
		t.Fatalf("FilterSkills returned error: %v", err)
	}

	assertFlatNames(t, filtered, []string{"one", "two", "three"})
}

func TestFilterSkills_InvalidPattern(t *testing.T) {
	skills := testSkills("one")

	if _, err := FilterSkills(skills, []string{"["}, nil); err == nil {
		t.Fatal("expected invalid include pattern error")
	}
	if _, err := FilterSkills(skills, nil, []string{"["}); err == nil {
		t.Fatal("expected invalid exclude pattern error")
	}
}

func TestShouldSyncFlatName(t *testing.T) {
	keep, err := ShouldSyncFlatName("codex-plan", []string{"codex-*"}, []string{"*-test"})
	if err != nil {
		t.Fatalf("ShouldSyncFlatName returned error: %v", err)
	}
	if !keep {
		t.Fatal("expected codex-plan to be managed")
	}

	keep, err = ShouldSyncFlatName("codex-test", []string{"codex-*"}, []string{"*-test"})
	if err != nil {
		t.Fatalf("ShouldSyncFlatName returned error: %v", err)
	}
	if keep {
		t.Fatal("expected codex-test to be filtered out")
	}
}

// --- FilterAgents tests ---

func TestFilterAgents_IncludeOnly(t *testing.T) {
	// FlatNames include .md; patterns should match without extension
	agents := testAgents("code-reviewer.md", "tutor.md", "debugger.md")
	filtered, err := FilterAgents(agents, []string{"code-*", "tutor"}, nil)
	if err != nil {
		t.Fatalf("FilterAgents returned error: %v", err)
	}
	assertAgentFlatNames(t, filtered, []string{"code-reviewer.md", "tutor.md"})
}

func TestFilterAgents_ExcludeOnly(t *testing.T) {
	agents := testAgents("code-reviewer.md", "tutor.md", "debugger.md")
	filtered, err := FilterAgents(agents, nil, []string{"tutor", "debug*"})
	if err != nil {
		t.Fatalf("FilterAgents returned error: %v", err)
	}
	assertAgentFlatNames(t, filtered, []string{"code-reviewer.md"})
}

func TestFilterAgents_IncludeThenExclude(t *testing.T) {
	agents := testAgents("team-reviewer.md", "team-debugger.md", "personal-tutor.md")
	filtered, err := FilterAgents(agents, []string{"team-*"}, []string{"*-debugger"})
	if err != nil {
		t.Fatalf("FilterAgents returned error: %v", err)
	}
	assertAgentFlatNames(t, filtered, []string{"team-reviewer.md"})
}

func TestFilterAgents_EmptyPatternsReturnAll(t *testing.T) {
	agents := testAgents("a.md", "b.md", "c.md")
	filtered, err := FilterAgents(agents, nil, nil)
	if err != nil {
		t.Fatalf("FilterAgents returned error: %v", err)
	}
	assertAgentFlatNames(t, filtered, []string{"a.md", "b.md", "c.md"})
}

func TestFilterAgents_NestedFlatNames(t *testing.T) {
	agents := testAgents("team__reviewer.md", "team__debugger.md", "personal__tutor.md")
	filtered, err := FilterAgents(agents, []string{"team__*"}, nil)
	if err != nil {
		t.Fatalf("FilterAgents returned error: %v", err)
	}
	assertAgentFlatNames(t, filtered, []string{"team__reviewer.md", "team__debugger.md"})
}

func TestFilterAgents_InvalidPattern(t *testing.T) {
	agents := testAgents("one.md")
	if _, err := FilterAgents(agents, []string{"["}, nil); err == nil {
		t.Fatal("expected invalid include pattern error")
	}
	if _, err := FilterAgents(agents, nil, []string{"["}); err == nil {
		t.Fatal("expected invalid exclude pattern error")
	}
}

func testAgents(names ...string) []resource.DiscoveredResource {
	agents := make([]resource.DiscoveredResource, 0, len(names))
	for _, name := range names {
		agents = append(agents, resource.DiscoveredResource{FlatName: name, Kind: "agent"})
	}
	return agents
}

func assertAgentFlatNames(t *testing.T, agents []resource.DiscoveredResource, want []string) {
	t.Helper()
	got := make([]string, 0, len(agents))
	for _, a := range agents {
		got = append(got, a.FlatName)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("agent flat names = %v, want %v", got, want)
	}
}

func testSkills(names ...string) []DiscoveredSkill {
	skills := make([]DiscoveredSkill, 0, len(names))
	for _, name := range names {
		skills = append(skills, DiscoveredSkill{FlatName: name})
	}
	return skills
}

func assertFlatNames(t *testing.T, skills []DiscoveredSkill, want []string) {
	t.Helper()

	got := make([]string, 0, len(skills))
	for _, skill := range skills {
		got = append(got, skill.FlatName)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("flat names = %v, want %v", got, want)
	}
}

func TestFindUnmatchedIncludes_BareNameOfNestedSkill(t *testing.T) {
	skills := testSkills("frontend__dev", "_repos__demo__lark-sheets", "alpha")

	unmatched := FindUnmatchedIncludes([]string{"dev", "alpha", "lark-sheets"}, skills)

	if len(unmatched) != 2 {
		t.Fatalf("FindUnmatchedIncludes() = %+v, want dev and lark-sheets", unmatched)
	}
	if unmatched[0].Pattern != "dev" ||
		!reflect.DeepEqual(unmatched[0].Suggestions, []string{"frontend__dev"}) {
		t.Fatalf("first unmatched = %+v, want dev suggesting frontend__dev", unmatched[0])
	}
	if unmatched[1].Pattern != "lark-sheets" ||
		!reflect.DeepEqual(unmatched[1].Suggestions, []string{"_repos__demo__lark-sheets"}) {
		t.Fatalf("second unmatched = %+v, want lark-sheets suggesting _repos__demo__lark-sheets", unmatched[1])
	}
}

func TestFindUnmatchedIncludes_SilentWhenPatternsSelect(t *testing.T) {
	skills := testSkills("frontend__dev", "alpha")

	if got := FindUnmatchedIncludes([]string{"frontend__dev", "alpha", "front*", ""}, skills); len(got) != 0 {
		t.Fatalf("FindUnmatchedIncludes() = %+v, want none", got)
	}
	if got := FindUnmatchedIncludes(nil, skills); len(got) != 0 {
		t.Fatalf("FindUnmatchedIncludes(nil) = %+v, want none", got)
	}
}

func TestFindUnmatchedIncludes_NoSuggestionForGlobOrUnknownName(t *testing.T) {
	skills := testSkills("frontend__dev", "alpha")

	unmatched := FindUnmatchedIncludes([]string{"missing-*", "zzz"}, skills)
	if len(unmatched) != 2 {
		t.Fatalf("FindUnmatchedIncludes() = %+v, want both patterns", unmatched)
	}
	for _, entry := range unmatched {
		if len(entry.Suggestions) != 0 {
			t.Fatalf("pattern %q suggestions = %v, want none", entry.Pattern, entry.Suggestions)
		}
	}
}

func TestFindUnmatchedIncludes_NoSuggestionForTypoOfTopLevelSkill(t *testing.T) {
	skills := testSkills("alpha", "frontend__dev")

	unmatched := FindUnmatchedIncludes([]string{"lpha", "ev"}, skills)
	if len(unmatched) != 2 {
		t.Fatalf("FindUnmatchedIncludes() = %+v, want both patterns", unmatched)
	}
	for _, entry := range unmatched {
		if len(entry.Suggestions) != 0 {
			t.Fatalf("pattern %q suggestions = %v, want none", entry.Pattern, entry.Suggestions)
		}
	}
}

func TestFindUnmatchedIncludes_CapsSuggestions(t *testing.T) {
	skills := testSkills("d__dev", "a__dev", "c__dev", "b__dev")

	unmatched := FindUnmatchedIncludes([]string{"dev"}, skills)
	if len(unmatched) != 1 {
		t.Fatalf("FindUnmatchedIncludes() = %+v, want one entry", unmatched)
	}
	if want := []string{"a__dev", "b__dev", "c__dev"}; !reflect.DeepEqual(unmatched[0].Suggestions, want) {
		t.Fatalf("suggestions = %v, want %v", unmatched[0].Suggestions, want)
	}
}

func TestFindUnmatchedIncludes_SilentOnInvalidPattern(t *testing.T) {
	skills := testSkills("frontend__dev")

	// FilterSkills rejects the pattern and reports it first; repeating it here
	// would read as two separate complaints about one typo.
	if got := FindUnmatchedIncludes([]string{"[unclosed"}, skills); len(got) != 0 {
		t.Fatalf("FindUnmatchedIncludes() = %+v, want none", got)
	}
}
