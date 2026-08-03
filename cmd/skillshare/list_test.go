package main

import "testing"

func TestParseListArgs_Status(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want listStatusFilter
	}{
		{"space form", []string{"--status", "enabled"}, statusFilterEnabled},
		{"equals form", []string{"--status=disabled"}, statusFilterDisabled},
		{"case insensitive", []string{"--status", "DISABLED"}, statusFilterDisabled},
		{"all", []string{"--status", "all"}, statusFilterAll},
		{"all equals form", []string{"--status=ALL"}, statusFilterAll},
		{"absent", []string{}, statusFilterAll},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseListArgs(tc.args)
			if err != nil {
				t.Fatalf("parseListArgs(%v) error = %v", tc.args, err)
			}
			if opts.Status != tc.want {
				t.Errorf("Status = %q, want %q", opts.Status, tc.want)
			}
		})
	}
}

func TestParseListArgs_StatusErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--status"},          // missing value
		{"--status", "bogus"}, // invalid value
		{"--status="},         // empty value
		{"--status=off"},      // invalid value, equals form
	} {
		if _, err := parseListArgs(args); err == nil {
			t.Errorf("parseListArgs(%v) = nil error, want error", args)
		}
	}
}

func TestParseListArgs_StatusCombinesWithOtherFilters(t *testing.T) {
	opts, err := parseListArgs([]string{"react", "--type", "local", "--status", "disabled", "--sort", "newest"})
	if err != nil {
		t.Fatalf("parseListArgs error = %v", err)
	}
	if opts.Pattern != "react" || opts.TypeFilter != "local" || opts.Status != statusFilterDisabled || opts.SortBy != "newest" {
		t.Errorf("got %+v, want pattern=react type=local status=disabled sort=newest", opts)
	}
}

func TestFilterSkillEntries_Status(t *testing.T) {
	entries := []skillEntry{
		{Name: "on-a", RelPath: "on-a"},
		{Name: "off-a", RelPath: "off-a", Disabled: true},
		{Name: "on-b", RelPath: "on-b"},
	}

	tests := []struct {
		status listStatusFilter
		want   []string
	}{
		{statusFilterAll, []string{"on-a", "off-a", "on-b"}},
		{statusFilterEnabled, []string{"on-a", "on-b"}},
		{statusFilterDisabled, []string{"off-a"}},
	}
	for _, tc := range tests {
		got := filterSkillEntries(entries, "", "", tc.status)
		if len(got) != len(tc.want) {
			t.Fatalf("status %q: got %d entries, want %d", tc.status, len(got), len(tc.want))
		}
		for i, name := range tc.want {
			if got[i].Name != name {
				t.Errorf("status %q: entry %d = %q, want %q", tc.status, i, got[i].Name, name)
			}
		}
	}
}

func TestFilterSkillEntries_StatusAndPatternAreAND(t *testing.T) {
	entries := []skillEntry{
		{Name: "react-on", RelPath: "react-on"},
		{Name: "react-off", RelPath: "react-off", Disabled: true},
		{Name: "vue-off", RelPath: "vue-off", Disabled: true},
	}

	got := filterSkillEntries(entries, "react", "", statusFilterDisabled)
	if len(got) != 1 || got[0].Name != "react-off" {
		t.Fatalf("got %+v, want only react-off", got)
	}
}

func TestNoMatchMessage_StatusOnly(t *testing.T) {
	got := noMatchMessage("skills", listOptions{Status: statusFilterDisabled})
	want := `No skills matching status "disabled"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNoMatchMessage_StatusWithOtherFilters(t *testing.T) {
	got := noMatchMessage("agents", listOptions{Pattern: "react", Status: statusFilterEnabled})
	want := `No agents matching "react" (status: enabled)`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestListStatusFilterString(t *testing.T) {
	tests := []struct {
		status listStatusFilter
		want   string
	}{
		{statusFilterAll, "all"},
		{statusFilterEnabled, "enabled"},
		{statusFilterDisabled, "disabled"},
	}
	for _, tc := range tests {
		if got := tc.status.String(); got != tc.want {
			t.Errorf("%v.String() = %q, want %q", tc.status, got, tc.want)
		}
	}
}

// The `s` key cycles All → Enabled → Disabled → All from whatever --status
// seeded, so an initial filter never traps the user.
func TestStatusFilterCyclesBackToAll(t *testing.T) {
	s := statusFilterDisabled
	if next := (s + 1) % 3; next != statusFilterAll {
		t.Errorf("cycling from Disabled = %v, want All", next)
	}
}
