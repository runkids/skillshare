package config

import "testing"

func TestSkillEntry_FullName(t *testing.T) {
	tests := []struct {
		name     string
		entry    SkillEntry
		expected string
	}{
		{
			name:     "simple name without group",
			entry:    SkillEntry{Name: "pdf"},
			expected: "pdf",
		},
		{
			name:     "with group",
			entry:    SkillEntry{Name: "pdf", Group: "frontend"},
			expected: "frontend/pdf",
		},
		{
			name:     "with multi-level group",
			entry:    SkillEntry{Name: "pdf", Group: "frontend/vue"},
			expected: "frontend/vue/pdf",
		},
		{
			name:     "legacy slash name without group",
			entry:    SkillEntry{Name: "frontend/pdf"},
			expected: "frontend/pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.FullName()
			if got != tt.expected {
				t.Errorf("FullName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSkillEntry_EffectiveParts(t *testing.T) {
	tests := []struct {
		name          string
		entry         SkillEntry
		expectedGroup string
		expectedName  string
	}{
		{
			name:          "simple name",
			entry:         SkillEntry{Name: "pdf"},
			expectedGroup: "",
			expectedName:  "pdf",
		},
		{
			name:          "with group",
			entry:         SkillEntry{Name: "pdf", Group: "frontend"},
			expectedGroup: "frontend",
			expectedName:  "pdf",
		},
		{
			name:          "with multi-level group",
			entry:         SkillEntry{Name: "pdf", Group: "frontend/vue"},
			expectedGroup: "frontend/vue",
			expectedName:  "pdf",
		},
		{
			name:          "legacy slash name (backward compat)",
			entry:         SkillEntry{Name: "frontend/pdf"},
			expectedGroup: "frontend",
			expectedName:  "pdf",
		},
		{
			name:          "legacy multi-level slash name",
			entry:         SkillEntry{Name: "frontend/vue/pdf"},
			expectedGroup: "frontend/vue",
			expectedName:  "pdf",
		},
		{
			name:          "tracked repo no group",
			entry:         SkillEntry{Name: "_team-skills", Tracked: true},
			expectedGroup: "",
			expectedName:  "_team-skills",
		},
		{
			name:          "tracked repo with group",
			entry:         SkillEntry{Name: "_team-skills", Group: "devops", Tracked: true},
			expectedGroup: "devops",
			expectedName:  "_team-skills",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, name := tt.entry.EffectiveParts()
			if group != tt.expectedGroup {
				t.Errorf("EffectiveParts() group = %q, want %q", group, tt.expectedGroup)
			}
			if name != tt.expectedName {
				t.Errorf("EffectiveParts() name = %q, want %q", name, tt.expectedName)
			}
		})
	}
}

func TestSkillEntry_RoundTrip(t *testing.T) {
	// FullName should be consistent with EffectiveParts
	entries := []SkillEntry{
		{Name: "pdf"},
		{Name: "pdf", Group: "frontend"},
		{Name: "pdf", Group: "frontend/vue"},
		{Name: "frontend/pdf"}, // legacy
	}

	for _, entry := range entries {
		fullName := entry.FullName()
		group, bareName := entry.EffectiveParts()

		// Reconstruct from parts
		var reconstructed string
		if group != "" {
			reconstructed = group + "/" + bareName
		} else {
			reconstructed = bareName
		}

		if reconstructed != fullName {
			t.Errorf("round-trip failed for %+v: FullName()=%q, reconstructed=%q", entry, fullName, reconstructed)
		}
	}
}
