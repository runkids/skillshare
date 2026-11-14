package profileapply

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/profile"
)

func TestProfileDefinitions_Transaction(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newFixture(t, project, "copy")
			before := read(t, f.path)
			_, err := Mutate(context.Background(), f.scope, profile.Mutation{Op: "create", Name: "first"}, profile.Mutation{Op: "create", Name: "none"})
			if err == nil || read(t, f.path) != before {
				t.Fatal("failed batch was partly saved")
			}
			change, err := Mutate(context.Background(), f.scope, profile.Mutation{Op: "create", Name: "new"})
			if err != nil || !change.Changed {
				t.Fatal(change, err)
			}
			v, err := Load(f.scope)
			if err != nil || v.Catalog.Definitions["new"].Selects("alpha") {
				t.Fatal("new profile not empty", err)
			}
			statePath, _, _ := config.ProfilePaths(f.path)
			if err := profile.WriteState(statePath, profile.State{Version: 1, Active: "a", Pending: true}); err != nil {
				t.Fatal(err)
			}
			change, err = Mutate(context.Background(), f.scope, profile.Mutation{Op: "rename", Name: "a", NewName: "renamed"})
			if err != nil || change.Active != "renamed" {
				t.Fatal(change, err)
			}
			state, _ := profile.ReadState(statePath)
			if state.Active != "renamed" || !state.Pending {
				t.Fatal("rename lost local pending state")
			}
			before = read(t, f.path)
			if _, err := Mutate(context.Background(), f.scope, profile.Mutation{Op: "delete", Name: "renamed"}); err == nil {
				t.Fatal("deleted active profile")
			}
			if read(t, f.path) != before {
				t.Fatal("delete refusal changed config")
			}
			if strings.Contains(read(t, f.path), "pending:") || strings.Contains(read(t, f.path), "active:") {
				t.Fatal("local state leaked into config")
			}
		})
	}
}

func TestProfileDefinitions_MembershipResults(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newFixture(t, project, "copy")
			change, err := Mutate(context.Background(), f.scope,
				profile.Mutation{Op: "add-skills", Name: "a", Skills: []string{"beta"}},
				profile.Mutation{Op: "remove-skills", Name: "a", Skills: []string{"alpha"}})
			if err != nil || !change.Changed || len(change.Skills) != 2 ||
				change.Skills[0] != (profile.SkillResult{Name: "beta", Result: "added"}) ||
				change.Skills[1] != (profile.SkillResult{Name: "alpha", Result: "removed"}) {
				t.Fatal(change, err)
			}
			before := read(t, f.path)
			change, err = Mutate(context.Background(), f.scope,
				profile.Mutation{Op: "add-skills", Name: "a", Skills: []string{"beta"}})
			if err != nil || change.Changed || len(change.Skills) != 1 || change.Skills[0].Result != "already-in" || read(t, f.path) != before {
				t.Fatal("no-op lost results or wrote config", change, err)
			}
		})
	}
}

func TestProfileDefinitions_FutureRawRepair(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, active := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/%v", project, active), func(t *testing.T) {
				f := newFixture(t, project, "copy")
				write(t, f.path, read(t, f.path)+"  future: # keep future\n    new_rule: [alpha, beta]\n")
				statePath, _, err := config.ProfilePaths(f.path)
				if err != nil {
					t.Fatal(err)
				}
				name := "a"
				if active {
					name = "future"
				}
				if err := profile.WriteState(statePath, profile.State{Version: 1, Active: name, Pending: true}); err != nil {
					t.Fatal(err)
				}
				change, err := Mutate(context.Background(), f.scope, profile.Mutation{Op: "rename", Name: "future", NewName: "renamed"})
				if err != nil || !change.Changed {
					t.Fatalf("raw rename: %+v %v", change, err)
				}
				v, err := Load(f.scope)
				if err != nil || v.Catalog.Definitions["renamed"].Usable || !strings.Contains(read(t, f.path), "keep future") || !strings.Contains(read(t, f.path), "new_rule: [alpha, beta]") {
					t.Fatal("future content changed", err)
				}
				state, err := profile.ReadState(statePath)
				if active {
					name = "renamed"
				}
				if err != nil || state.Active != name || !state.Pending {
					t.Fatalf("rename changed pending/active incorrectly: %+v %v", state, err)
				}
				before := read(t, f.path)
				_, err = Mutate(context.Background(), f.scope, profile.Mutation{Op: "delete", Name: "renamed"})
				if active {
					if err == nil || read(t, f.path) != before || v.ActiveProfileError() == nil {
						t.Fatal("active unusable definition lost protection")
					}
				} else if err != nil || strings.Contains(read(t, f.path), "new_rule:") {
					t.Fatal("inactive raw delete failed", err)
				}
			})
		}
	}
}
