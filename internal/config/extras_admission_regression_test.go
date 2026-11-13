package config

import (
	"testing"
)

func TestValidateExtraRemote_RequiresInstallableGitSource(t *testing.T) {
	for _, remote := range []string{"nonsense", "owner", "https://github.com/owner", "https://example.com"} {
		t.Run(remote, func(t *testing.T) {
			entry := ExtraConfig{Name: "rules", Remote: remote}
			if err := ValidateExtraRemote(entry); err == nil {
				t.Errorf("unparseable origin admitted: %q", remote)
			}
			for name, after := range map[string][]ExtraConfig{
				"delete":  nil,
				"correct": {{Name: "rules", Remote: "example-org/ai-hub"}},
			} {
				if err := ValidateRemoteExtraEdits([]ExtraConfig{entry}, after); err != nil {
					t.Errorf("invalid baseline cannot %s: %v", name, err)
				}
			}
		})
	}
}
