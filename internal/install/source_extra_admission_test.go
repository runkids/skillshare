package install

import "testing"

func TestParseSource_GitHubRequiresRepository(t *testing.T) {
	for _, raw := range []string{"https://github.com/owner", "http://github.com/owner", "https://GitHub.com/owner"} {
		if source, err := ParseSource(raw); err == nil {
			t.Errorf("incomplete GitHub identity accepted: %+v", source)
		}
	}
	for _, raw := range []string{"https://example.com/repo", "https://github.com/owner/repo", "https://GitHub.com/owner/repo", "owner/repo"} {
		if source, err := ParseSource(raw); err != nil || !source.IsGit() {
			t.Errorf("supported identity refused: %q %+v %v", raw, source, err)
		}
	}
}
