package install

import "testing"

func TestParseSource_GitHubRequiresRepository(t *testing.T) {
	for _, raw := range []string{
		"https://github.com/owner", "http://github.com/owner", "https://GitHub.com/owner",
		"https://github.com:443/owner", "https://GitHub.com:443/owner",
		"http://github.com:80/owner", "github.com:443/owner",
	} {
		if source, err := ParseSource(raw); err == nil {
			t.Errorf("incomplete GitHub identity accepted: %+v", source)
		}
	}
	for _, raw := range []string{
		"https://example.com/repo", "https://github.com/owner/repo", "https://GitHub.com/owner/repo", "owner/repo",
		"https://github.com:443/owner/repo", "https://GitHub.com:443/owner/repo",
		"http://github.com:80/owner/repo", "https://example.com:8443/repo",
	} {
		if source, err := ParseSource(raw); err != nil || !source.IsGit() {
			t.Errorf("supported identity refused: %q %+v %v", raw, source, err)
		}
	}
}
