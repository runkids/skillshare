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
