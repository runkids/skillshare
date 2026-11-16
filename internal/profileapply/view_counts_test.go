package profileapply

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTargetCounts_ExcludesLocalInventory(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"merge", "copy"} {
			t.Run(fmt.Sprintf("project=%v/%s", project, mode), func(t *testing.T) {
				f := newFixture(t, project, mode)
				write(t, filepath.Join(f.target, "local", "SKILL.md"), "---\nname: local\n---\n# authored locally\n")
				// Shared-folder peers route distinct source skills to the same
				// physical target. Both counts include that union, not local inventory.
				peer := "  second:\n    skills:\n      path: " + f.target + "\n      mode: " + mode + "\n"
				if project {
					peer = "  - name: second\n    skills:\n      path: " + f.target + "\n      mode: " + mode + "\n"
				}
				write(t, f.path, strings.Replace(read(t, f.path), "targets:\n", "targets:\n"+peer, 1))
				write(t, filepath.Join(f.source, "alpha", "SKILL.md"), "---\nname: alpha\nmetadata:\n  targets: [custom]\n---\n# alpha\n")
				write(t, filepath.Join(f.source, "beta", "SKILL.md"), "---\nname: beta\nmetadata:\n  targets: [second]\n---\n# beta\n")
				v, err := Load(f.scope)
				if err != nil {
					t.Fatal(err)
				}
				discovery, err := v.Discover()
				if err != nil {
					t.Fatal(err)
				}
				before := tree(t, f.target)
				for _, tc := range []struct {
					name string
					want int
				}{{"empty", 0}, {"a", 1}, {"", 2}} {
					counts, err := v.Candidate(tc.name).TargetCounts(discovery.Skills)
					want := map[string]int{"custom": tc.want, "second": tc.want}
					if err != nil || !reflect.DeepEqual(counts, want) {
						t.Fatalf("profile %q: counts=%v, err=%v; want %v", tc.name, counts, err, want)
					}
				}
				if !reflect.DeepEqual(before, tree(t, f.target)) {
					t.Fatal("counting changed local inventory")
				}
			})
		}
	}
}
