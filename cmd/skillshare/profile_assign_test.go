package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	ssync "skillshare/internal/sync"
)

func TestResolveSkillFlatNames(t *testing.T) {
	skills := []ssync.DiscoveredSkill{
		{FlatName: "alpha", RelPath: "alpha"},
		{FlatName: "team__review", RelPath: "team/review", Disabled: true},
		{FlatName: "other__review", RelPath: "other/review"},
		{FlatName: "team__solo", RelPath: "team/solo"},
	}
	for _, tc := range []struct {
		name       string
		args, want []string
		err        string
	}{
		{"flat and path", []string{"alpha", "team__review", "other/review"}, []string{"alpha", "team__review", "other__review"}, ""},
		{"native source path", []string{filepath.FromSlash("team/review")}, []string{"team__review"}, ""},
		{"unique basename", []string{"solo"}, []string{"team__solo"}, ""},
		{"duplicates", []string{"alpha", "alpha"}, []string{"alpha", "alpha"}, ""},
		{"ambiguous", []string{"review"}, nil, "other__review, team__review"},
		{"missing batch", []string{"alpha", "missing"}, nil, "missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveSkillFlatNames(skills, tc.args)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) || got != nil {
					t.Fatalf("got %v, %v", got, err)
				}
			} else if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}
