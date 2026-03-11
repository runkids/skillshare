package config

import (
	"testing"
)

func TestValidateExtraName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		// valid names
		{name: "rules", wantErr: false},
		{name: "my-commands", wantErr: false},
		{name: "team_prompts", wantErr: false},
		{name: "a1", wantErr: false},
		// reserved names
		{name: "skills", wantErr: true},
		{name: "extras", wantErr: true},
		// empty
		{name: "", wantErr: true},
		// path traversal / invalid characters
		{name: "../escape", wantErr: true},
		{name: "has space", wantErr: true},
		{name: "has/slash", wantErr: true},
		{name: "-starts-dash", wantErr: true},
		{name: "_starts-under", wantErr: true},
		{name: ".hidden", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateExtraName(tc.name)
			if tc.wantErr && err == nil {
				t.Errorf("ValidateExtraName(%q) = nil, want error", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ValidateExtraName(%q) = %v, want nil", tc.name, err)
			}
		})
	}
}

func TestValidateExtraName_NoDuplicate(t *testing.T) {
	existing := []ExtraConfig{{Name: "rules"}}

	if err := ValidateExtraNameUnique("rules", existing); err == nil {
		t.Error("ValidateExtraNameUnique(\"rules\", existing) = nil, want error for duplicate")
	}

	if err := ValidateExtraNameUnique("commands", existing); err != nil {
		t.Errorf("ValidateExtraNameUnique(\"commands\", existing) = %v, want nil", err)
	}
}
