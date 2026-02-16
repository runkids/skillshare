package main

import (
	"testing"
)

func TestParseFilterFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantOpts filterUpdateOpts
		wantRest []string
		wantErr  bool
	}{
		{
			name:     "no flags",
			args:     []string{"--mode", "merge"},
			wantOpts: filterUpdateOpts{},
			wantRest: []string{"--mode", "merge"},
		},
		{
			name: "all four flags",
			args: []string{
				"--add-include", "team-*",
				"--add-exclude", "_legacy*",
				"--remove-include", "old-*",
				"--remove-exclude", "test-*",
			},
			wantOpts: filterUpdateOpts{
				AddInclude:    []string{"team-*"},
				AddExclude:    []string{"_legacy*"},
				RemoveInclude: []string{"old-*"},
				RemoveExclude: []string{"test-*"},
			},
		},
		{
			name: "multiple values for same flag",
			args: []string{
				"--add-include", "a-*",
				"--add-include", "b-*",
			},
			wantOpts: filterUpdateOpts{
				AddInclude: []string{"a-*", "b-*"},
			},
		},
		{
			name: "mixed with other flags",
			args: []string{
				"--mode", "merge",
				"--add-include", "team-*",
			},
			wantOpts: filterUpdateOpts{
				AddInclude: []string{"team-*"},
			},
			wantRest: []string{"--mode", "merge"},
		},
		{
			name:    "missing value for --add-include",
			args:    []string{"--add-include"},
			wantErr: true,
		},
		{
			name:    "missing value for --add-exclude",
			args:    []string{"--add-exclude"},
			wantErr: true,
		},
		{
			name:    "missing value for --remove-include",
			args:    []string{"--remove-include"},
			wantErr: true,
		},
		{
			name:    "missing value for --remove-exclude",
			args:    []string{"--remove-exclude"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, rest, err := parseFilterFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertStringSlice(t, "AddInclude", opts.AddInclude, tt.wantOpts.AddInclude)
			assertStringSlice(t, "AddExclude", opts.AddExclude, tt.wantOpts.AddExclude)
			assertStringSlice(t, "RemoveInclude", opts.RemoveInclude, tt.wantOpts.RemoveInclude)
			assertStringSlice(t, "RemoveExclude", opts.RemoveExclude, tt.wantOpts.RemoveExclude)
			assertStringSlice(t, "rest", rest, tt.wantRest)
		})
	}
}

func TestApplyFilterUpdates(t *testing.T) {
	tests := []struct {
		name        string
		include     []string
		exclude     []string
		opts        filterUpdateOpts
		wantInclude []string
		wantExclude []string
		wantChanges int
		wantErr     bool
	}{
		{
			name: "add include",
			opts: filterUpdateOpts{
				AddInclude: []string{"team-*"},
			},
			wantInclude: []string{"team-*"},
			wantChanges: 1,
		},
		{
			name: "add exclude",
			opts: filterUpdateOpts{
				AddExclude: []string{"_legacy*"},
			},
			wantExclude: []string{"_legacy*"},
			wantChanges: 1,
		},
		{
			name:    "remove include",
			include: []string{"team-*", "org-*"},
			opts: filterUpdateOpts{
				RemoveInclude: []string{"team-*"},
			},
			wantInclude: []string{"org-*"},
			wantChanges: 1,
		},
		{
			name:    "remove exclude",
			exclude: []string{"_legacy*", "test-*"},
			opts: filterUpdateOpts{
				RemoveExclude: []string{"_legacy*"},
			},
			wantExclude: []string{"test-*"},
			wantChanges: 1,
		},
		{
			name:    "deduplicate add",
			include: []string{"team-*"},
			opts: filterUpdateOpts{
				AddInclude: []string{"team-*"},
			},
			wantInclude: []string{"team-*"},
			wantChanges: 0,
		},
		{
			name: "remove nonexistent is no-op",
			opts: filterUpdateOpts{
				RemoveInclude: []string{"nope"},
			},
			wantChanges: 0,
		},
		{
			name: "invalid include pattern",
			opts: filterUpdateOpts{
				AddInclude: []string{"[invalid"},
			},
			wantErr: true,
		},
		{
			name: "invalid exclude pattern",
			opts: filterUpdateOpts{
				AddExclude: []string{"[invalid"},
			},
			wantErr: true,
		},
		{
			name:    "multiple operations",
			include: []string{"old-*"},
			exclude: []string{"old-exc-*"},
			opts: filterUpdateOpts{
				AddInclude:    []string{"new-*"},
				RemoveInclude: []string{"old-*"},
				AddExclude:    []string{"new-exc-*"},
				RemoveExclude: []string{"old-exc-*"},
			},
			wantInclude: []string{"new-*"},
			wantExclude: []string{"new-exc-*"},
			wantChanges: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			include := append([]string(nil), tt.include...)
			exclude := append([]string(nil), tt.exclude...)

			changes, err := applyFilterUpdates(&include, &exclude, tt.opts)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(changes) != tt.wantChanges {
				t.Errorf("changes count = %d, want %d (changes: %v)", len(changes), tt.wantChanges, changes)
			}
			assertStringSlice(t, "include", include, tt.wantInclude)
			assertStringSlice(t, "exclude", exclude, tt.wantExclude)
		})
	}
}

func TestFilterUpdateOpts_HasUpdates(t *testing.T) {
	if (filterUpdateOpts{}).hasUpdates() {
		t.Error("empty opts should not have updates")
	}
	if !(filterUpdateOpts{AddInclude: []string{"x"}}).hasUpdates() {
		t.Error("opts with AddInclude should have updates")
	}
}

func assertStringSlice(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if len(got) != len(want) {
		t.Errorf("%s: got %v, want %v", label, got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %q, want %q", label, i, got[i], want[i])
		}
	}
}
