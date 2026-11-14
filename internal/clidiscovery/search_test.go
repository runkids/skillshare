package clidiscovery

import (
	"reflect"
	"testing"
)

func TestSearch(t *testing.T) {
	entries := []Result{
		{Command: "skillshare uninstall", Summary: "Remove from the source", Usage: "skillshare uninstall --help"},
		{Command: "skillshare sync", Summary: "Link skills into Targets", Usage: "skillshare sync --help"},
		{Command: "skillshare link", Summary: "Link a folder of skills", Usage: "skillshare link --help"},
	}
	for _, tt := range []struct {
		query string
		limit int
		want  []string
	}{
		{"sync", 5, []string{"skillshare sync"}},
		{"DELETE", 5, []string{"skillshare uninstall"}},
		{"link skills", 1, []string{"skillshare link"}},
		{"skills", 5, []string{"skillshare link", "skillshare sync"}},
		{"skillshare", 5, []string{}},
		{"the to", 5, []string{}},
		{"---", 5, []string{}},
		{"absent", 5, []string{}},
		{"link", 0, []string{}},
	} {
		t.Run(tt.query, func(t *testing.T) {
			got := Search(entries, tt.query, tt.limit)
			if got == nil {
				t.Fatal("results must be a non-nil slice")
			}
			names := make([]string, 0, len(got))
			for _, result := range got {
				names = append(names, result.Command)
				if result.Usage == "" {
					t.Fatal("missing usage")
				}
			}
			if !reflect.DeepEqual(names, tt.want) {
				t.Fatalf("got %v, want %v", names, tt.want)
			}
		})
	}
	if got, want := Search(entries, "skills skills", 20), Search(entries, "skills", 20); !reflect.DeepEqual(got, want) {
		t.Fatal("repeated query words changed ranking")
	}
}
