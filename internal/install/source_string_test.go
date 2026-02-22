package install

import "testing"

func TestSourceType_String(t *testing.T) {
	tests := []struct {
		st   SourceType
		want string
	}{
		{SourceTypeUnknown, "unknown"},
		{SourceTypeLocalPath, "local"},
		{SourceTypeGitHub, "github"},
		{SourceTypeGitHTTPS, "git-https"},
		{SourceTypeGitSSH, "git-ssh"},
		{SourceType(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.st.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAuthEnvForURL_NoToken(t *testing.T) {
	// With no environment tokens set, AuthEnvForURL should return empty
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	env := AuthEnvForURL("https://github.com/user/repo.git")
	// Should not panic; may return empty or contain config entries
	_ = env
}

func TestResolveTokenForURL_NoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	token, username := ResolveTokenForURL("https://github.com/user/repo.git")
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
	if username != "" {
		t.Errorf("expected empty username, got %q", username)
	}
}

func TestDetectPlatformForURL(t *testing.T) {
	tests := []struct {
		url  string
		want Platform
	}{
		{"https://github.com/user/repo.git", PlatformGitHub},
		{"https://gitlab.com/user/repo.git", PlatformGitLab},
		{"https://example.com/repo.git", PlatformUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := DetectPlatformForURL(tt.url); got != tt.want {
				t.Errorf("DetectPlatformForURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
