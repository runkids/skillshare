package sync

import (
	"reflect"
	"testing"
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
