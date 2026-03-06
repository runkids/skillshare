package main

import (
	"testing"
)

func TestParseFilterQuery_Empty(t *testing.T) {
	q := parseFilterQuery("")
	if q.TypeTag != "" || q.GroupTag != "" || q.RepoTag != "" || q.FreeText != "" {
		t.Fatalf("expected empty query, got %+v", q)
	}
}

func TestParseFilterQuery_PureText(t *testing.T) {
	q := parseFilterQuery("hello world")
	if q.FreeText != "hello world" {
		t.Fatalf("FreeText = %q, want %q", q.FreeText, "hello world")
	}
	if q.TypeTag != "" || q.GroupTag != "" || q.RepoTag != "" {
		t.Fatalf("expected no tags, got %+v", q)
	}
}

func TestParseFilterQuery_TypeTag(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"t:tracked", "tracked"},
		{"t:remote", "remote"},
		{"t:local", "local"},
		{"type:tracked", "tracked"},
		{"t:github", "remote"},    // alias
		{"t:track", "tracked"},    // short alias
		{"t:_tracked", "tracked"}, // optional _ prefix
		{"t:_track", "tracked"},   // _ prefix + short alias
		{"T:Tracked", "tracked"},
	}
	for _, tt := range tests {
		q := parseFilterQuery(tt.input)
		if q.TypeTag != tt.want {
			t.Errorf("parseFilterQuery(%q).TypeTag = %q, want %q", tt.input, q.TypeTag, tt.want)
		}
	}
}

func TestParseFilterQuery_GroupTag(t *testing.T) {
	q := parseFilterQuery("g:security")
	if q.GroupTag != "security" {
		t.Fatalf("GroupTag = %q, want %q", q.GroupTag, "security")
	}

	q = parseFilterQuery("group:frontend")
	if q.GroupTag != "frontend" {
		t.Fatalf("GroupTag = %q, want %q", q.GroupTag, "frontend")
	}
}

func TestParseFilterQuery_RepoTag(t *testing.T) {
	q := parseFilterQuery("r:team")
	if q.RepoTag != "team" {
		t.Fatalf("RepoTag = %q, want %q", q.RepoTag, "team")
	}

	q = parseFilterQuery("repo:my-skills")
	if q.RepoTag != "my-skills" {
		t.Fatalf("RepoTag = %q, want %q", q.RepoTag, "my-skills")
	}
}

func TestParseFilterQuery_MultipleTags(t *testing.T) {
	q := parseFilterQuery("t:tracked g:security audit")
	if q.TypeTag != "tracked" {
		t.Fatalf("TypeTag = %q, want %q", q.TypeTag, "tracked")
	}
	if q.GroupTag != "security" {
		t.Fatalf("GroupTag = %q, want %q", q.GroupTag, "security")
	}
	if q.FreeText != "audit" {
		t.Fatalf("FreeText = %q, want %q", q.FreeText, "audit")
	}
}

func TestParseFilterQuery_UnknownTagTreatedAsFreeText(t *testing.T) {
	q := parseFilterQuery("x:foo bar")
	if q.FreeText != "x:foo bar" {
		t.Fatalf("FreeText = %q, want %q", q.FreeText, "x:foo bar")
	}
}

func TestParseFilterQuery_EmptyValueIgnored(t *testing.T) {
	q := parseFilterQuery("t: hello")
	// "t:" has no value, treated as free text
	if q.TypeTag != "" {
		t.Fatalf("TypeTag = %q, want empty (empty value ignored)", q.TypeTag)
	}
	if q.FreeText != "t: hello" {
		t.Fatalf("FreeText = %q, want %q", q.FreeText, "t: hello")
	}
}

func TestSkillGroup_TrackedNested(t *testing.T) {
	e := skillEntry{RelPath: "_team-repo/security/audit", RepoName: "team/repo"}
	if got := skillGroup(e); got != "security" {
		t.Fatalf("skillGroup() = %q, want %q", got, "security")
	}
}

func TestSkillGroup_TrackedRoot(t *testing.T) {
	e := skillEntry{RelPath: "_team-repo/my-skill", RepoName: "team/repo"}
	if got := skillGroup(e); got != "" {
		t.Fatalf("skillGroup() = %q, want empty for tracked root-level", got)
	}
}

func TestSkillGroup_LocalNested(t *testing.T) {
	e := skillEntry{RelPath: "frontend/react-tips"}
	if got := skillGroup(e); got != "frontend" {
		t.Fatalf("skillGroup() = %q, want %q", got, "frontend")
	}
}

func TestSkillGroup_LocalRoot(t *testing.T) {
	e := skillEntry{RelPath: "my-skill"}
	if got := skillGroup(e); got != "" {
		t.Fatalf("skillGroup() = %q, want empty for local root-level", got)
	}
}

func TestMatchSkillItem_TypeTracked(t *testing.T) {
	item := skillItem{entry: skillEntry{Name: "audit", RelPath: "_repo/audit", RepoName: "org/repo"}}
	q := filterQuery{TypeTag: "tracked"}
	if !matchSkillItem(item, q) {
		t.Fatal("expected tracked item to match type:tracked")
	}
	q.TypeTag = "local"
	if matchSkillItem(item, q) {
		t.Fatal("expected tracked item NOT to match type:local")
	}
}

func TestMatchSkillItem_TypeRemote(t *testing.T) {
	item := skillItem{entry: skillEntry{Name: "remote-skill", RelPath: "remote-skill", Source: "github.com/foo/bar"}}
	q := filterQuery{TypeTag: "remote"}
	if !matchSkillItem(item, q) {
		t.Fatal("expected remote item to match type:remote")
	}
}

func TestMatchSkillItem_TypeLocal(t *testing.T) {
	item := skillItem{entry: skillEntry{Name: "local-skill", RelPath: "local-skill"}}
	q := filterQuery{TypeTag: "local"}
	if !matchSkillItem(item, q) {
		t.Fatal("expected local item to match type:local")
	}
}

func TestMatchSkillItem_GroupMatch(t *testing.T) {
	item := skillItem{entry: skillEntry{Name: "audit", RelPath: "security/audit"}}
	q := filterQuery{GroupTag: "secur"}
	if !matchSkillItem(item, q) {
		t.Fatal("expected group substring match")
	}
	q.GroupTag = "frontend"
	if matchSkillItem(item, q) {
		t.Fatal("expected group mismatch")
	}
}

func TestMatchSkillItem_RepoMatch(t *testing.T) {
	item := skillItem{entry: skillEntry{Name: "x", RelPath: "_team-repo/x", RepoName: "team/my-skills"}}
	q := filterQuery{RepoTag: "my-skills"}
	if !matchSkillItem(item, q) {
		t.Fatal("expected repo substring match")
	}
	q.RepoTag = "other"
	if matchSkillItem(item, q) {
		t.Fatal("expected repo mismatch")
	}
}

func TestMatchSkillItem_FreeTextMatch(t *testing.T) {
	item := skillItem{entry: skillEntry{Name: "react-tips", RelPath: "frontend/react-tips"}}
	q := filterQuery{FreeText: "react"}
	if !matchSkillItem(item, q) {
		t.Fatal("expected free text match")
	}
	q.FreeText = "golang"
	if matchSkillItem(item, q) {
		t.Fatal("expected free text mismatch")
	}
}

func TestMatchSkillItem_ANDLogic(t *testing.T) {
	item := skillItem{entry: skillEntry{
		Name:     "audit",
		RelPath:  "_team-repo/security/audit",
		RepoName: "team/repo",
	}}

	// All conditions match
	q := filterQuery{TypeTag: "tracked", GroupTag: "security", FreeText: "audit"}
	if !matchSkillItem(item, q) {
		t.Fatal("expected AND match when all conditions are true")
	}

	// One condition fails
	q.TypeTag = "local"
	if matchSkillItem(item, q) {
		t.Fatal("expected AND to fail when type doesn't match")
	}
}

func TestMatchSkillItem_EmptyQueryMatchesAll(t *testing.T) {
	item := skillItem{entry: skillEntry{Name: "anything", RelPath: "anything"}}
	q := filterQuery{}
	if !matchSkillItem(item, q) {
		t.Fatal("empty query should match all items")
	}
}
