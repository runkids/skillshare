package install

import "testing"

func TestShouldInstallAll(t *testing.T) {
	tests := []struct {
		name string
		opts InstallOptions
		want bool
	}{
		{"default", InstallOptions{}, false},
		{"all", InstallOptions{All: true}, true},
		{"yes", InstallOptions{Yes: true}, true},
		{"both", InstallOptions{All: true, Yes: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.ShouldInstallAll(); got != tt.want {
				t.Errorf("ShouldInstallAll() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasSkillFilter(t *testing.T) {
	tests := []struct {
		name string
		opts InstallOptions
		want bool
	}{
		{"empty", InstallOptions{}, false},
		{"with filter", InstallOptions{Skills: []string{"a"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.HasSkillFilter(); got != tt.want {
				t.Errorf("HasSkillFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}
