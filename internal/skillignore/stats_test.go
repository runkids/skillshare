package skillignore

import "testing"

func TestIgnoreStats_Active(t *testing.T) {
	tests := []struct {
		name string
		s    IgnoreStats
		want bool
	}{
		{"root file", IgnoreStats{RootFile: "/path/.skillignore"}, true},
		{"repo file", IgnoreStats{RepoFiles: []string{"_repo/.skillignore"}}, true},
		{"both", IgnoreStats{RootFile: "/path", RepoFiles: []string{"_r"}}, true},
		{"neither", IgnoreStats{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Active(); got != tt.want {
				t.Errorf("Active() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIgnoreStats_Counts(t *testing.T) {
	s := IgnoreStats{
		Patterns:      []string{"test-*", "vendor/", "!important"},
		IgnoredSkills: []string{"test-a", "test-b"},
	}
	if s.PatternCount() != 3 {
		t.Errorf("PatternCount() = %d, want 3", s.PatternCount())
	}
	if s.IgnoredCount() != 2 {
		t.Errorf("IgnoredCount() = %d, want 2", s.IgnoredCount())
	}
}

func TestIgnoreStats_ZeroCounts(t *testing.T) {
	s := IgnoreStats{}
	if s.PatternCount() != 0 || s.IgnoredCount() != 0 {
		t.Error("empty stats should have zero counts")
	}
}
