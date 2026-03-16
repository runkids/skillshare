package sync

import "testing"

func TestClassifySkillForTarget(t *testing.T) {
	tests := []struct {
		name       string
		flatName   string
		targets    []string
		targetName string
		include    []string
		exclude    []string
		wantStatus string
		wantReason string
	}{
		{name: "synced_no_filters", flatName: "my-skill", targets: nil, targetName: "claude", wantStatus: "synced"},
		{name: "excluded_by_pattern", flatName: "legacy-tool", targets: nil, targetName: "claude", exclude: []string{"legacy*"}, wantStatus: "excluded", wantReason: "legacy*"},
		{name: "not_included", flatName: "backend-api", targets: nil, targetName: "claude", include: []string{"frontend*"}, wantStatus: "not_included"},
		{name: "included_by_pattern", flatName: "frontend-design", targets: nil, targetName: "claude", include: []string{"frontend*"}, wantStatus: "synced"},
		{name: "skill_target_mismatch", flatName: "my-skill", targets: []string{"cursor"}, targetName: "claude", wantStatus: "skill_target_mismatch", wantReason: "cursor"},
		{name: "skill_target_match", flatName: "my-skill", targets: []string{"claude", "cursor"}, targetName: "claude", wantStatus: "synced"},
		{name: "exclude_wins_over_include", flatName: "frontend-legacy", targets: nil, targetName: "claude", include: []string{"frontend*"}, exclude: []string{"*legacy*"}, wantStatus: "excluded", wantReason: "*legacy*"},
		{name: "include_miss_beats_exclude_match", flatName: "legacy-tool", targets: nil, targetName: "claude", include: []string{"frontend*"}, exclude: []string{"legacy*"}, wantStatus: "not_included"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, reason := ClassifySkillForTarget(tt.flatName, tt.targets, tt.targetName, tt.include, tt.exclude)
			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}
			if tt.wantReason != "" && reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}
