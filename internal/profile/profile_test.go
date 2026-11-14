package profile

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func section(t *testing.T, text string) *yaml.Node {
	t.Helper()
	var wrapper struct {
		Profiles yaml.Node `yaml:"profiles"`
	}
	if err := yaml.Unmarshal([]byte(text), &wrapper); err != nil {
		t.Fatal(err)
	}
	return &wrapper.Profiles
}

func TestMutateFutureDefinitionRawRepair(t *testing.T) {
	for _, active := range []string{"", "future"} {
		node := section(t, "profiles:\n  future: # preserve comment\n    new_rule: [alpha, beta]\n  other: {}\n")
		before, err := yaml.Marshal(node)
		if err != nil {
			t.Fatal(err)
		}
		renamed, change, err := Mutate(node, active, Mutation{Op: "rename", Name: "future", NewName: "renamed"})
		if err != nil || !change.Changed || (active != "" && change.Active != "renamed") {
			t.Errorf("raw rename: %+v %v", change, err)
		} else if data, err := yaml.Marshal(renamed); err != nil || !strings.Contains(string(data), "preserve comment") || !strings.Contains(string(data), "new_rule: [alpha, beta]") || Parse(renamed).Definitions["renamed"].Usable {
			t.Errorf("raw rename interpreted/lost future content: %s %v", data, err)
		}
		_, _, err = Mutate(node, active, Mutation{Op: "delete", Name: "future"})
		if (err != nil) != (active == "future") {
			t.Errorf("delete active=%q: %v", active, err)
		}
		if _, _, err := Mutate(node, active, Mutation{Op: "edit", Name: "future", AddInclude: []string{"alpha"}}); err == nil {
			t.Error("semantic edit interpreted unknown definition")
		}
		if _, _, err := Mutate(node, active, Mutation{Op: "rename", Name: "future", NewName: "other"}); err == nil {
			t.Error("raw rename replaced another definition")
		}
		after, err := yaml.Marshal(node)
		if err != nil || string(after) != string(before) {
			t.Fatal("raw operation mutated input node", err)
		}
	}
}

func TestParseSelectionAndValidation(t *testing.T) {
	n := section(t, `profiles:
  all: {}
  empty: {skills: {}}
  nullskills: {skills: null}
  blank: {skills: {include: [" ", ""]}}
  inert: {skills: {include: [], exclude: [old]}}
  selected: {skills: {include: ["team__*"], exclude: ["*__old"]}}
  invalid: {skills: {exclude: [old]}}
  invalidglob: {skills: {include: ["["]}}
  future: {future_key: true}
`)
	c := Parse(n)
	for _, tc := range []struct {
		name, skill string
		want        bool
	}{
		{"all", "anything", true}, {"empty", "anything", false}, {"nullskills", "anything", false},
		{"blank", "anything", false}, {"inert", "old", false},
		{"selected", "team__new", true}, {"selected", "team__old", false},
	} {
		d := c.Definitions[tc.name]
		if !d.Usable || d.Selects(tc.skill) != tc.want {
			t.Errorf("%s selects %s: %+v, want %v", tc.name, tc.skill, d, tc.want)
		}
	}
	for _, name := range []string{"invalid", "invalidglob", "future"} {
		if d := c.Definitions[name]; d.Usable || d.Reason == "" {
			t.Errorf("%s should be unusable: %+v", name, d)
		}
	}
	if !strings.Contains(c.Definitions["invalid"].Reason, `include: ["*"]`) {
		t.Fatal(c.Definitions["invalid"].Reason)
	}
	for _, name := range []string{"none", "Bad", strings.Repeat("x", 65), ""} {
		if ValidateName(name) == nil {
			t.Errorf("name %q accepted", name)
		}
	}
	if ValidateName("team_1-dev") != nil {
		t.Fatal("valid profile name rejected")
	}
}

func TestValidateSectionPreservesUnknownKeys(t *testing.T) {
	for _, tc := range []struct {
		name, definition, reason string
		valid                    bool
	}{
		{"profile key", "{future_key: true}", "future_key", true},
		{"skills key", "{skills: {future_key: true, include: [alpha]}}", "future_key", true},
		{"profile key and pattern", `{future_key: true, skills: {include: ["["]}}`, "invalid include pattern", false},
		{"skills key and pattern", `{skills: {future_key: true, include: ["["]}}`, "invalid include pattern", false},
		{"profile key and exclude", "{future_key: true, skills: {exclude: [old]}}", "exclude requires an include", false},
		{"skills key and exclude", "{skills: {future_key: true, exclude: [old]}}", "exclude requires an include", false},
		{"known field before unknown", `{skills: {include: ["["]}, future_key: true}`, "invalid include pattern", false},
		{"unknown and wrong type", "{future_key: true, skills: {include: alpha}}", "must be a list of strings", false},
		{"unknown and duplicate", "{future_key: true, skills: {}, skills: {}}", "duplicate profile key", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node := section(t, "profiles: {work: "+tc.definition+"}")
			d := Parse(node).Definitions["work"]
			if d.Usable || !strings.Contains(d.Reason, tc.reason) {
				t.Fatalf("definition should remain unusable: %+v", d)
			}
			if selected, err := Resolve(Parse(node), State{Active: "work"}, "config.yaml", nil).Selects("alpha"); selected || err == nil {
				t.Fatal("unusable definition activated")
			}
			err := ValidateSection(node)
			if tc.valid && err != nil || !tc.valid && (err == nil || !strings.Contains(err.Error(), tc.reason)) {
				t.Fatalf("validation = %v, valid=%v, reason=%s", err, tc.valid, tc.reason)
			}
		})
	}
}

func TestMutatePreservesUnrelatedYAML(t *testing.T) {
	original := section(t, "profiles:\n  future: # keep this\n    unknown: value\n  work:\n    description: before\n    skills: {include: [old], exclude: [blocked]}\n")
	desc := "after"
	next, change, err := Mutate(original, "work", Mutation{Op: "edit", Name: "work", Description: &desc, AddInclude: []string{"new"}})
	if err != nil || !change.Changed || !change.NeedsSync {
		t.Fatalf("edit: %+v, %v", change, err)
	}
	data, err := yaml.Marshal(next)
	if err != nil || !strings.Contains(string(data), "keep this") || !strings.Contains(string(data), "unknown: value") || !strings.Contains(string(data), "blocked") {
		t.Fatalf("unrelated content lost: %s (%v)", data, err)
	}
	if Parse(original).Definitions["work"].Description != "before" {
		t.Fatal("mutation changed original node")
	}
	renamed, change, err := Mutate(next, "work", Mutation{Op: "rename", Name: "work", NewName: "focus"})
	if err != nil || change.Active != "focus" || !Parse(renamed).Definitions["focus"].Selects("new") {
		t.Fatalf("rename: %+v, %v", change, err)
	}
	if _, _, err := Mutate(renamed, "focus", Mutation{Op: "delete", Name: "focus"}); err == nil {
		t.Fatal("active deletion accepted")
	}
	if _, _, err := Mutate(renamed, "focus", Mutation{Op: "create", Name: "focus"}); err == nil {
		t.Fatal("duplicate creation accepted")
	}
	created, _, err := Mutate(renamed, "focus", Mutation{Op: "create", Name: "empty"})
	if err != nil || Parse(created).Definitions["empty"].Selects("anything") {
		t.Fatalf("empty creation: %v", err)
	}
}

func TestPreview(t *testing.T) {
	d := Parse(section(t, `profiles:
  work: {skills: {include: ["team__*", missing], exclude: ["*__old"]}}
`)).Definitions["work"]
	p, err := Preview(d, []string{"team__new", "team__old", "other"})
	if err != nil || len(p.Selected) != 1 || p.Selected[0] != "team__new" || len(p.Unmatched) != 1 || p.Unmatched[0] != "missing" {
		t.Fatalf("preview = %+v, %v", p, err)
	}
	empty := Parse(section(t, `profiles: {work: {skills: {include: [], exclude: [old]}}}`)).Definitions["work"]
	p, err = Preview(empty, []string{"old"})
	if err != nil || len(p.InertExcludes) != 1 || p.InertExcludes[0] != "old" {
		t.Fatalf("inert excludes = %+v, %v", p, err)
	}
}
