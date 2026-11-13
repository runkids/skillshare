package config

import (
	"testing"
)

func TestValidateExtraRemote_RequiresInstallableGitSource(t *testing.T) {
	for _, remote := range []string{
		"nonsense", "owner", "https://github.com/owner", "https://example.com",
		"https://github.com:443/owner", "https://GitHub.com:443/owner",
		"http://github.com:80/owner", "github.com:443/owner",
	} {
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
	for _, remote := range []string{
		"https://github.com:443/owner/repo", "https://GitHub.com:443/owner/repo",
		"http://github.com:80/owner/repo", "https://example.com:8443/repo",
	} {
		t.Run("valid/"+remote, func(t *testing.T) {
			if err := ValidateExtraRemote(ExtraConfig{Name: "rules", Remote: remote}); err != nil {
				t.Errorf("supported port-bearing origin refused: %q: %v", remote, err)
			}
		})
	}
}
