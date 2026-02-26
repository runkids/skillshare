package main

import (
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
	if got := item.Title(); got != "my-skill  [local]" {
		t.Errorf("Title() = %q, want %q", got, "my-skill  [local]")
	}
}

func TestSkillItem_Title_Nested(t *testing.T) {
	item := skillItem{entry: skillEntry{Name: "react-helper", RelPath: "frontend/react-helper"}}
	if got := item.Title(); got != "frontend/react-helper  [local]" {
		t.Errorf("Title() = %q, want %q", got, "frontend/react-helper  [local]")
	}
}

func TestSkillItem_Title_SameNameAsRelPath(t *testing.T) {
	item := skillItem{entry: skillEntry{Name: "my-skill", RelPath: "my-skill"}}
	if got := item.Title(); got != "my-skill  [local]" {
		t.Errorf("Title() = %q, want %q", got, "my-skill  [local]")
	}
}

func TestSkillItem_Description_Tracked(t *testing.T) {
	item := skillItem{entry: skillEntry{RepoName: "team-repo"}}
	if got := item.Description(); got != "tracked: team-repo" {
		t.Errorf("Description() = %q", got)
	}
}

func TestSkillItem_Description_Remote(t *testing.T) {
	item := skillItem{entry: skillEntry{Source: "github.com/user/repo"}}
	if got := item.Description(); got != "github.com/user/repo" {
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
