package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
)

func TestSkillItem_ImplementsListItem(t *testing.T) {
	entry := skillEntry{
		Name:    "react-helper",
		RelPath: "frontend/react-helper",
		Source:  "github.com/user/skills",
		Type:    "git",
	}
	item := skillItem{entry: entry}

	// Must implement list.Item (FilterValue)
	var _ list.Item = item

	if got := item.FilterValue(); got != "react-helper frontend/react-helper github.com/user/skills" {
		t.Errorf("FilterValue() = %q", got)
	}
}

func TestSkillItem_Title_TopLevel(t *testing.T) {
	item := skillItem{entry: skillEntry{Name: "my-skill"}}
	got := item.Title()
	if !strings.Contains(got, "my-skill") || !strings.Contains(got, "local") {
		t.Errorf("Title() = %q, want my-skill + local badge", got)
	}
}

func TestSkillItem_Title_Nested(t *testing.T) {
	item := skillItem{entry: skillEntry{Name: "react-helper", RelPath: "frontend/react-helper"}}
	got := item.Title()
	if !strings.Contains(got, "react-helper") || !strings.Contains(got, "local") {
		t.Errorf("Title() = %q, want react-helper + local badge", got)
	}
}

func TestSkillItem_Title_SameNameAsRelPath(t *testing.T) {
	item := skillItem{entry: skillEntry{Name: "my-skill", RelPath: "my-skill"}}
	got := item.Title()
	if !strings.Contains(got, "my-skill") || !strings.Contains(got, "local") {
		t.Errorf("Title() = %q, want my-skill + local badge", got)
	}
}

func TestCompactSkillPath_TrackedDeep(t *testing.T) {
	e := skillEntry{Name: "skill-name", RelPath: "_repo/security/skill-name", RepoName: "org/repo"}
	if got := compactSkillPath(e); got != "security/skill-name" {
		t.Errorf("compactSkillPath() = %q, want %q", got, "security/skill-name")
	}
}

func TestCompactSkillPath_TrackedRoot(t *testing.T) {
	e := skillEntry{Name: "skillshare", RelPath: "_repo/skillshare", RepoName: "org/repo"}
	if got := compactSkillPath(e); got != "skillshare" {
		t.Errorf("compactSkillPath() = %q, want %q", got, "skillshare")
	}
}

func TestCompactSkillPath_LocalNested(t *testing.T) {
	e := skillEntry{Name: "skill-name", RelPath: "group/skill-name"}
	if got := compactSkillPath(e); got != "group/skill-name" {
		t.Errorf("compactSkillPath() = %q, want %q", got, "group/skill-name")
	}
}

func TestBuildGroupedItems(t *testing.T) {
	skills := []skillItem{
		{entry: skillEntry{Name: "a", RelPath: "_repo/security/a", RepoName: "_repo"}},
		{entry: skillEntry{Name: "b", RelPath: "_repo/security/b", RepoName: "_repo"}},
		{entry: skillEntry{Name: "local-skill", RelPath: "local-skill"}},
	}
	items := buildGroupedItems(skills)
	// Expect: groupItem("repo") + skill + skill + groupItem("local") + skill = 5 items
	if len(items) != 5 {
		t.Fatalf("buildGroupedItems: got %d items, want 5", len(items))
	}
	g1, ok := items[0].(groupItem)
	if !ok {
		t.Fatal("items[0] should be groupItem")
	}
	if g1.label != "repo" || g1.count != 2 {
		t.Errorf("group 1: label=%q count=%d, want label=repo count=2", g1.label, g1.count)
	}
	g2, ok := items[3].(groupItem)
	if !ok {
		t.Fatal("items[3] should be groupItem")
	}
	if g2.label != "standalone" || g2.count != 1 {
		t.Errorf("group 2: label=%q count=%d, want label=standalone count=1", g2.label, g2.count)
	}
}

func TestBuildGroupedItems_SingleGroup(t *testing.T) {
	skills := []skillItem{
		{entry: skillEntry{Name: "a", RelPath: "a"}},
		{entry: skillEntry{Name: "b", RelPath: "b"}},
	}
	items := buildGroupedItems(skills)
	// All standalone — no separators expected.
	if len(items) != 2 {
		t.Fatalf("buildGroupedItems single group: got %d items, want 2", len(items))
	}
	if _, isGroup := items[0].(groupItem); isGroup {
		t.Error("items[0] should NOT be groupItem when single group")
	}
}

func TestSkillItem_Description_Tracked(t *testing.T) {
	item := skillItem{entry: skillEntry{RepoName: "team-repo"}}
	if got := item.Description(); got != "" {
		t.Errorf("Description() = %q", got)
	}
}

func TestSkillItem_Description_Remote(t *testing.T) {
	item := skillItem{entry: skillEntry{Source: "github.com/user/repo"}}
	if got := item.Description(); got != "" {
		t.Errorf("Description() = %q", got)
	}
}

func TestSkillItem_Description_Local(t *testing.T) {
	item := skillItem{entry: skillEntry{}}
	// Local skills return "" — the [local] badge is shown in Title() instead
	if got := item.Description(); got != "" {
		t.Errorf("Description() = %q, want %q", got, "")
	}
}
