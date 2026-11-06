package config

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// Equivalent lexical root spellings must select one local winner, even when
// the shared declaration is excluded. This is the documented overlay contract.
func TestLocalProjectsRootPrecedenceCases(t *testing.T) {
	base := t.TempDir()
	for _, n := range []int{-1, 0, 42} {
		for _, choice := range []bool{false, true} {
			root := filepath.Join(base, fmt.Sprintf("project-%d", n))
			alias := root + string(filepath.Separator) + "."
			shared := map[string]ManagedProject{root: {Name: "shared"}}
			local := ManagedProject{Name: "local", Targets: []string{"claude"}}
			state := localProjects{Version: 1, Projects: map[string]ManagedProject{alias: local}}
			if choice {
				state.Excluded = []string{root}
			}
			got, origins, err := mergeLocalProjects(shared, state)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[alias].Name != "local" || origins[alias] != "local" {
				t.Fatalf("wrong root winner: %v %v", got, origins)
			}
		}
	}
}

func TestLocalProjectsUnrelatedSharedEntriesSurviveCases(t *testing.T) {
	base := t.TempDir()
	for _, n := range []int{-1, 0, 42} {
		for _, choice := range []bool{false, true} {
			root := filepath.Join(base, fmt.Sprintf("project-%d", n))
			untouched := filepath.Join(base, "untouched")
			shared := map[string]ManagedProject{root: {Name: "shared"}, untouched: {Name: "keep"}}
			state := localProjects{Version: 1, Projects: map[string]ManagedProject{}}
			if choice {
				state.Excluded = []string{root}
			} else {
				state.Projects[root] = ManagedProject{Name: "override"}
			}
			got, origins, err := mergeLocalProjects(shared, state)
			if err != nil {
				t.Fatal(err)
			}
			if got[untouched].Name != "keep" || origins[untouched] != "shared" {
				t.Fatalf("unrelated entry changed: %v %v", got, origins)
			}
		}
	}
}

// Disk encoding preserves every resource value in the winning local entry.
func TestLocalProjectsRoundtripCases(t *testing.T) {
	base := t.TempDir()
	for _, n := range []int{-1, 0, 42} {
		for _, choice := range []bool{false, true} {
			root := filepath.Join(base, fmt.Sprintf("p-%d", n))
			other := filepath.Join(base, "shared")
			resource := &ResourceTargetConfig{Mode: "copy", Include: []string{fmt.Sprintf("skill-%d", n)}}
			local := ManagedProject{Name: fmt.Sprintf("local-%d", n), Targets: []string{"claude"}, Skills: resource}
			if choice {
				local.Agents = resource
			}
			shared := map[string]ManagedProject{root: {Name: "original", Targets: []string{"codex"}}, other: {Name: "keep"}}
			state := localProjects{Version: 1, Projects: map[string]ManagedProject{root: local}, Excluded: []string{other}}
			raw, err := yaml.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeLocalProjects(raw)
			if err != nil {
				t.Fatal(err)
			}
			got, _, err := mergeLocalProjects(shared, decoded)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, map[string]ManagedProject{root: local}) {
				t.Fatalf("roundtrip changed entry: %#v", got)
			}
		}
	}
}

// Equivalent roots within a layer are invalid; across layers they replace.
func TestLocalProjectsDuplicateAliasesRejectedCases(t *testing.T) {
	base := t.TempDir()
	for _, n := range []int{-1, 0, 42} {
		for _, choice := range []bool{false, true} {
			root := filepath.Join(base, fmt.Sprintf("p-%d", n))
			alias := root + string(filepath.Separator) + "."
			entries := map[string]ManagedProject{root: {Name: "first"}, alias: {Name: "second"}}
			shared := entries
			state := localProjects{Version: 1}
			if choice {
				shared = nil
				state.Projects = entries
			}
			if _, _, err := mergeLocalProjects(shared, state); err == nil {
				t.Fatal("duplicate roots accepted")
			}
		}
	}
}

// Shared output is independent of local additions, full replacements and masks.
func TestLocalProjectsSharedProjectionCases(t *testing.T) {
	base := t.TempDir()
	for _, n := range []int{-1, 0, 42} {
		for _, choice := range []bool{false, true} {
			root := filepath.Join(base, fmt.Sprintf("p-%d", n))
			extra := filepath.Join(base, "local-only")
			shared := map[string]ManagedProject{root: {Name: "shared", Targets: []string{"claude"}, Agents: &ResourceTargetConfig{Mode: "copy"}}}
			state := localProjects{Version: 1, Projects: map[string]ManagedProject{extra: {Name: "extra"}}}
			if choice {
				state.Excluded = []string{root}
			} else {
				state.Projects[root] = ManagedProject{Name: "override", Skills: &ResourceTargetConfig{}}
			}
			effective, origins, err := mergeLocalProjects(shared, state)
			if err != nil {
				t.Fatal(err)
			}
			cfg := &Config{Projects: effective, projectOverlay: &projectOverlay{shared: shared, state: state, origins: origins}}
			if !reflect.DeepEqual(cfg.sharedProjectDeclarations(), shared) {
				t.Fatal("overlay leaked into shared declarations")
			}
		}
	}
}

// Prepared, persisted and reloaded views agree across remove and re-add.
func TestLocalProjectsMutationSequenceCases(t *testing.T) {
	_, base, err := loadWithProjects(t, "  $ROOT/app: {targets: [claude], skills: {}}\n", "app")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "app")
	path := ConfigPath()
	for _, n := range []int{-1, 0, 42} {
		for _, choice := range []bool{false, true} {
			for _, choice2 := range []bool{false, true} {
				p := ManagedProject{Name: fmt.Sprintf("p-%d", n), Targets: []string{"claude"}}
				if choice {
					p.Skills = &ResourceTargetConfig{Mode: "copy", Include: []string{fmt.Sprintf("skill-%d", n)}}
				}
				if choice2 {
					p.Agents = &ResourceTargetConfig{Mode: "merge", Exclude: []string{fmt.Sprintf("agent-%d", n)}}
				}
				for _, entries := range []map[string]ManagedProject{{root: p}, {}, {root: p}} {
					cfg, err := LoadReadOnlyPath(path)
					if err != nil {
						t.Fatal(err)
					}
					c, err := PrepareLocalProjects(path, cfg, entries, "")
					if err != nil {
						t.Fatal(err)
					}
					if err := c.Write(); err != nil {
						t.Fatal(err)
					}
					loaded, err := LoadReadOnlyPath(path)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(loaded.Projects, c.Expanded.Projects) || !reflect.DeepEqual(loaded.Targets, c.Expanded.Targets) {
						t.Fatal("persisted view disagrees with candidate")
					}
					if len(entries) == 0 {
						if len(loaded.Projects) != 0 {
							t.Fatal("shared root reappeared")
						}
					} else if !reflect.DeepEqual(loaded.Projects[root], p) {
						t.Fatalf("roundtrip lost values: %+v", loaded.Projects[root])
					}
				}
			}
		}
	}
}
