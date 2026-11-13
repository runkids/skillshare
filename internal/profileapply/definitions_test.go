package profileapply

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
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

func TestProfileDefinitions_RenameStateWriteFailure(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newFixture(t, project, "copy")
			statePath, _, err := config.ProfilePaths(f.path)
			if err != nil {
				t.Fatal(err)
			}
			if err := profile.WriteState(statePath, profile.State{Version: 1, Active: "a", Pending: true}); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(f.target, "authored.txt"), "keep target bytes")
			configBefore, stateBefore := read(t, f.path), read(t, statePath)
			sourceBefore, targetBefore := tree(t, f.source), tree(t, f.target)
			failure := errors.New("state save sentinel")
			calls := 0
			change, err := mutate(context.Background(), f.scope, func(path string, state profile.State) error {
				calls++
				if path != statePath || state != (profile.State{Version: 1, Active: "renamed", Pending: true}) {
					t.Fatalf("state write = %s, %+v", path, state)
				}
				v, err := Load(f.scope)
				if err != nil || !v.Catalog.Definitions["renamed"].Selects("alpha") || v.Catalog.Definitions["a"].Usable {
					t.Fatalf("config was not saved before state: %+v, %v", v, err)
				}
				return failure
			}, profile.Mutation{Op: "rename", Name: "a", NewName: "renamed"})
			if !errors.Is(err, failure) || err.Error() != "definitions saved, but local active rename failed; repair with skillshare profile use renamed: state save sentinel" || calls != 1 {
				t.Fatalf("failure = %v, writer calls=%d", err, calls)
			}
			if !change.Changed || change.Active != "renamed" || change.NeedsSync {
				t.Fatalf("saved change = %+v", change)
			}
			if read(t, f.path) == configBefore || read(t, statePath) != stateBefore {
				t.Fatal("config rename lost or failed state write changed old state bytes")
			}
			if !reflect.DeepEqual(tree(t, f.source), sourceBefore) || !reflect.DeepEqual(tree(t, f.target), targetBefore) {
				t.Fatal("definition failure changed source or target tree")
			}
			v, err := Load(f.scope)
			if err != nil || !v.Catalog.Definitions["renamed"].Selects("alpha") {
				t.Fatalf("saved definition unavailable: %+v, %v", v, err)
			}
			var unavailable *profile.UnavailableError
			if _, exists := v.Catalog.Definitions["a"]; exists || !errors.As(v.ActiveProfileError(), &unavailable) || unavailable.Name != "a" || unavailable.ConfigPath != f.path {
				t.Fatalf("old active name not repairable: %+v, %v", v.State, v.ActiveProfileError())
			}
		})
	}
}
