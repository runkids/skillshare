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

func TestSkillItem_Title_Tracked(t *testing.T) {
	item := skillItem{entry: skillEntry{Name: "my-skill", RelPath: "_repo/my-skill", RepoName: "_repo"}}
	got := item.Title()
	if !strings.Contains(got, "my-skill") {
		t.Errorf("Title() = %q, want my-skill in title", got)
	}
	// Tracked skills should not have "local" badge (group header provides context)
	if strings.Contains(got, "local") {
		t.Errorf("Title() = %q, tracked skill should not have local badge", got)
	}
}

func TestCompactSkillPath_TrackedDeep(t *testing.T) {
	e := skillEntry{Name: "skill-name", RelPath: "_repo/security/skill-name", RepoName: "org/repo"}
	if got := compactSkillPath(e, false); got != "security/skill-name" {
		t.Errorf("compactSkillPath() = %q, want %q", got, "security/skill-name")
	}
}

func TestCompactSkillPath_TrackedRoot(t *testing.T) {
	e := skillEntry{Name: "skillshare", RelPath: "_repo/skillshare", RepoName: "org/repo"}
	if got := compactSkillPath(e, false); got != "skillshare" {
		t.Errorf("compactSkillPath() = %q, want %q", got, "skillshare")
	}
}

func TestCompactSkillPath_LocalNested(t *testing.T) {
	e := skillEntry{Name: "skill-name", RelPath: "group/skill-name"}
	if got := compactSkillPath(e, true); got != "skill-name" {
		t.Errorf("compactSkillPath() = %q, want %q (the group heading shows the folder)", got, "skill-name")
	}
}

func TestCompactSkillPath_LocalNestedWithoutHeadingKeepsTheFolder(t *testing.T) {
	e := skillEntry{Name: "skill-name", RelPath: "group/skill-name"}
	if got := compactSkillPath(e, false); got != "group/skill-name" {
		t.Errorf("compactSkillPath() = %q, want %q (no heading names the folder)", got, "group/skill-name")
	}
}

func TestBuildGroupedItems_MarksRowsUnderAHeading(t *testing.T) {
	items := buildGroupedItems([]skillItem{
		{entry: skillEntry{Name: "a", RelPath: "frontend/a"}},
		{entry: skillEntry{Name: "b", RelPath: "backend/b"}},
	})
	for _, it := range items {
		if s, ok := it.(skillItem); ok && !s.grouped {
			t.Errorf("%s is under a heading but not marked grouped", s.entry.RelPath)
		}
	}
	single := buildGroupedItems([]skillItem{{entry: skillEntry{Name: "a", RelPath: "frontend/a"}}})
	if s := single[0].(skillItem); s.grouped {
		t.Error("a single group has no heading, so its rows are not grouped")
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

// Local nested skills (e.g. "mma/agent-browser") must form their own
// visual group even though they have no RepoName, so they don't get
// lumped together with flat root-level locals under "standalone".
func TestBuildGroupedItems_LocalNestedGroup(t *testing.T) {
	// Pre-sorted: mma group items contiguous, then flat standalone.
	// (sortSkillEntries places standalone last.)
	skills := []skillItem{
		{entry: skillEntry{Name: "agent-browser", RelPath: "mma/agent-browser"}},
		{entry: skillEntry{Name: "docker-expert", RelPath: "mma/docker-expert"}},
		{entry: skillEntry{Name: "flat-a", RelPath: "flat-a"}},
	}
	items := buildGroupedItems(skills)
	// Expect: group("mma") + 2 skills + group("standalone") + 1 skill = 5
	if len(items) != 5 {
		t.Fatalf("buildGroupedItems local nested: got %d items, want 5", len(items))
	}
	g1, ok := items[0].(groupItem)
	if !ok || g1.label != "mma" || g1.count != 2 {
		t.Errorf("group 1: %+v, want label=mma count=2", g1)
	}
	g2, ok := items[3].(groupItem)
	if !ok || g2.label != "standalone" || g2.count != 1 {
		t.Errorf("group 2: %+v, want label=standalone count=1", g2)
	}
}

// Agents deep inside a tracked repo (e.g. _repo/agents/core/x.md,
// _repo/agents/specialized/python/y.md) must all belong to ONE group
// keyed by the tracked repo root — not split into core/specialized buckets.
func TestBuildGroupedItems_TrackedAgentsDeepPath(t *testing.T) {
	skills := []skillItem{
		{entry: skillEntry{
			Kind: "agent", Name: "code-reviewer",
			RelPath: "_vijay-agents/agents/core/code-reviewer.md", RepoName: "_vijay-agents",
		}},
		{entry: skillEntry{
			Kind: "agent", Name: "python-expert",
			RelPath: "_vijay-agents/agents/specialized/python/python-expert.md", RepoName: "_vijay-agents",
		}},
		{entry: skillEntry{
			Kind: "agent", Name: "local-agent",
			RelPath: "local-agent.md",
		}},
	}
	items := buildGroupedItems(skills)
	// Expect: group("vijay-agents") + 2 agents + group("standalone") + 1 agent = 5
	if len(items) != 5 {
		t.Fatalf("buildGroupedItems tracked agents: got %d items, want 5", len(items))
	}
	g1, ok := items[0].(groupItem)
	if !ok || g1.label != "vijay-agents" || g1.count != 2 {
		t.Errorf("group 1: %+v, want label=vijay-agents count=2", g1)
	}
	g2, ok := items[3].(groupItem)
	if !ok || g2.label != "standalone" || g2.count != 1 {
		t.Errorf("group 2: %+v, want label=standalone count=1", g2)
	}
}

func TestSkillTopGroup(t *testing.T) {
	cases := []struct {
		name string
		e    skillEntry
		want string
	}{
		{"tracked repo", skillEntry{RepoName: "_repo", RelPath: "_repo/security/audit"}, "_repo"},
		{"local nested", skillEntry{RelPath: "mma/docker-expert"}, "mma"},
		{"local deep", skillEntry{RelPath: "frontend/react/hooks"}, "frontend"},
		{"flat local", skillEntry{RelPath: "my-skill"}, ""},
		{"empty", skillEntry{}, ""},
	}
	for _, tc := range cases {
		if got := skillTopGroup(tc.e); got != tc.want {
			t.Errorf("%s: skillTopGroup=%q, want %q", tc.name, got, tc.want)
		}
	}
}

// The default sort must group entries by top-level bucket so
// buildGroupedItems produces contiguous groups. Standalone entries
// (empty top group) must sort last so they don't split named groups.
func TestSortSkillEntries_TopGroupOrder(t *testing.T) {
	skills := []skillEntry{
		{Name: "react-best-practices", RelPath: "react-best-practices"},
		{Name: "mma-slack", RelPath: "mma/slack"},
		{Name: "tracked-a", RelPath: "_team-repo/a", RepoName: "_team-repo"},
		{Name: "agent-browser", RelPath: "agent-browser"},
		{Name: "mma-dogfood", RelPath: "mma/dogfood"},
	}
	sortSkillEntries(skills, "")

	got := make([]string, len(skills))
	for i, s := range skills {
		got[i] = s.RelPath
	}
	want := []string{
		"_team-repo/a",         // tracked repo first (underscore sorts first)
		"mma/dogfood",          // then local nested "mma" group
		"mma/slack",            //
		"agent-browser",        // standalone flat items last, alphabetical
		"react-best-practices", //
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sort order[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
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
