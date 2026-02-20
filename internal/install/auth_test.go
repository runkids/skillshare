package install

import (
	"testing"
)

func TestExtractHost(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"https github", "https://github.com/org/repo.git", "github.com"},
		{"https gitlab", "https://gitlab.com/org/repo.git", "gitlab.com"},
		{"https bitbucket", "https://bitbucket.org/team/repo.git", "bitbucket.org"},
		{"https with port", "https://git.example.com:8443/org/repo.git", "git.example.com"},
		{"ssh github", "git@github.com:org/repo.git", "github.com"},
		{"ssh gitlab", "git@gitlab.com:org/repo.git", "gitlab.com"},
		{"ssh bitbucket", "git@bitbucket.org:team/repo.git", "bitbucket.org"},
		{"file url", "file:///path/to/repo", ""},
		{"empty", "", ""},
		{"bare path", "/some/local/repo", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHost(tt.url)
			if got != tt.want {
				t.Errorf("extractHost(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want Platform
	}{
		{"github.com", "https://github.com/org/repo.git", PlatformGitHub},
		{"github enterprise", "https://github.mycompany.com/org/repo.git", PlatformGitHub},
		{"gitlab.com", "https://gitlab.com/org/repo.git", PlatformGitLab},
		{"self-hosted gitlab", "https://gitlab.internal.co/org/repo.git", PlatformGitLab},
		{"bitbucket.org", "https://bitbucket.org/team/repo.git", PlatformBitbucket},
		{"ssh github", "git@github.com:org/repo.git", PlatformGitHub},
		{"unknown host", "https://git.example.com/repo.git", PlatformUnknown},
		{"empty", "", PlatformUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectPlatform(tt.url)
			if got != tt.want {
				t.Errorf("detectPlatform(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestResolveToken(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		envVars  map[string]string
		wantTok  string
		wantUser string
	}{
		{
			name:     "github token",
			url:      "https://github.com/org/repo.git",
			envVars:  map[string]string{"GITHUB_TOKEN": "ghp_test123"},
			wantTok:  "ghp_test123",
			wantUser: "x-access-token",
		},
		{
			name:     "gitlab token",
			url:      "https://gitlab.com/org/repo.git",
			envVars:  map[string]string{"GITLAB_TOKEN": "glpat-test"},
			wantTok:  "glpat-test",
			wantUser: "oauth2",
		},
		{
			name:     "bitbucket token",
			url:      "https://bitbucket.org/team/repo.git",
			envVars:  map[string]string{"BITBUCKET_TOKEN": "bb_test"},
			wantTok:  "bb_test",
			wantUser: "x-token-auth",
		},
		{
			name:     "bitbucket token ignores url username by default",
			url:      "https://willie0903@bitbucket.org/team/repo.git",
			envVars:  map[string]string{"BITBUCKET_TOKEN": "app_pwd"},
			wantTok:  "app_pwd",
			wantUser: "x-token-auth",
		},
		{
			name:     "bitbucket app password with BITBUCKET_USERNAME",
			url:      "https://bitbucket.org/team/repo.git",
			envVars:  map[string]string{"BITBUCKET_TOKEN": "app_pwd", "BITBUCKET_USERNAME": "willie0903"},
			wantTok:  "app_pwd",
			wantUser: "willie0903",
		},
		{
			name:     "generic fallback",
			url:      "https://git.example.com/org/repo.git",
			envVars:  map[string]string{"SKILLSHARE_GIT_TOKEN": "custom_tok"},
			wantTok:  "custom_tok",
			wantUser: "x-access-token",
		},
		{
			name:     "generic fallback preserves url username",
			url:      "https://demouser@bitbucket.org/team/repo.git",
			envVars:  map[string]string{"SKILLSHARE_GIT_TOKEN": "app_pwd"},
			wantTok:  "app_pwd",
			wantUser: "demouser",
		},
		{
			name:     "generic fallback uses bitbucket username",
			url:      "https://bitbucket.org/team/repo.git",
			envVars:  map[string]string{"SKILLSHARE_GIT_TOKEN": "tok"},
			wantTok:  "tok",
			wantUser: "x-token-auth",
		},
		{
			name:     "generic fallback uses gitlab username",
			url:      "https://gitlab.com/org/repo.git",
			envVars:  map[string]string{"SKILLSHARE_GIT_TOKEN": "tok"},
			wantTok:  "tok",
			wantUser: "oauth2",
		},
		{
			name:     "platform specific beats generic",
			url:      "https://github.com/org/repo.git",
			envVars:  map[string]string{"GITHUB_TOKEN": "ghp_specific", "SKILLSHARE_GIT_TOKEN": "generic"},
			wantTok:  "ghp_specific",
			wantUser: "x-access-token",
		},
		{
			name:     "generic fallback for github when no GITHUB_TOKEN",
			url:      "https://github.com/org/repo.git",
			envVars:  map[string]string{"SKILLSHARE_GIT_TOKEN": "generic"},
			wantTok:  "generic",
			wantUser: "x-access-token",
		},
		{
			name:    "ssh returns empty",
			url:     "git@github.com:org/repo.git",
			envVars: map[string]string{"GITHUB_TOKEN": "ghp_test"},
		},
		{
			name: "no env var returns empty",
			url:  "https://github.com/org/repo.git",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}
			tok, user := resolveToken(tt.url)
			if tok != tt.wantTok {
				t.Errorf("resolveToken(%q) token = %q, want %q", tt.url, tok, tt.wantTok)
			}
			if user != tt.wantUser {
				t.Errorf("resolveToken(%q) username = %q, want %q", tt.url, user, tt.wantUser)
			}
		})
	}
}

func TestAuthEnv(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		envVars   map[string]string
		wantNil   bool
		wantCount string // expected GIT_CONFIG_COUNT value
		wantIdx   string // expected index in KEY/VALUE names
		wantKey   string // expected GIT_CONFIG_KEY_<idx> value
		wantVal   string // expected GIT_CONFIG_VALUE_<idx> value
	}{
		{
			name:      "github with token",
			url:       "https://github.com/org/repo.git",
			envVars:   map[string]string{"GITHUB_TOKEN": "ghp_abc"},
			wantCount: "1", wantIdx: "0",
			wantKey: "url.https://x-access-token:ghp_abc@github.com/.insteadOf",
			wantVal: "https://github.com/",
		},
		{
			name:      "gitlab with token",
			url:       "https://gitlab.com/org/repo.git",
			envVars:   map[string]string{"GITLAB_TOKEN": "glpat-xyz"},
			wantCount: "1", wantIdx: "0",
			wantKey: "url.https://oauth2:glpat-xyz@gitlab.com/.insteadOf",
			wantVal: "https://gitlab.com/",
		},
		{
			name:      "bitbucket with token",
			url:       "https://bitbucket.org/team/repo.git",
			envVars:   map[string]string{"BITBUCKET_TOKEN": "bb_tok"},
			wantCount: "1", wantIdx: "0",
			wantKey: "url.https://x-token-auth:bb_tok@bitbucket.org/.insteadOf",
			wantVal: "https://bitbucket.org/",
		},
		{
			name:      "bitbucket token auth key ignores url username by default",
			url:       "https://willie0903@bitbucket.org/team/repo.git",
			envVars:   map[string]string{"BITBUCKET_TOKEN": "app_pwd"},
			wantCount: "1", wantIdx: "0",
			wantKey: "url.https://x-token-auth:app_pwd@bitbucket.org/.insteadOf",
			wantVal: "https://willie0903@bitbucket.org/",
		},
		{
			name:      "bitbucket app password with BITBUCKET_USERNAME",
			url:       "https://bitbucket.org/team/repo.git",
			envVars:   map[string]string{"BITBUCKET_TOKEN": "app_pwd", "BITBUCKET_USERNAME": "willie0903"},
			wantCount: "1", wantIdx: "0",
			wantKey: "url.https://willie0903:app_pwd@bitbucket.org/.insteadOf",
			wantVal: "https://bitbucket.org/",
		},
		{
			name:      "token with equals sign",
			url:       "https://bitbucket.org/team/repo.git",
			envVars:   map[string]string{"BITBUCKET_TOKEN": "ATATT3x_test=FBFF"},
			wantCount: "1", wantIdx: "0",
			wantKey: "url.https://x-token-auth:ATATT3x_test=FBFF@bitbucket.org/.insteadOf",
			wantVal: "https://bitbucket.org/",
		},
		{
			name:      "token with at sign",
			url:       "https://github.com/org/repo.git",
			envVars:   map[string]string{"GITHUB_TOKEN": "ghp_abc@def"},
			wantCount: "1", wantIdx: "0",
			wantKey: "url.https://x-access-token:ghp_abc@def@github.com/.insteadOf",
			wantVal: "https://github.com/",
		},
		{
			name:      "token with percent and hash",
			url:       "https://gitlab.com/org/repo.git",
			envVars:   map[string]string{"GITLAB_TOKEN": "tok%20val#frag"},
			wantCount: "1", wantIdx: "0",
			wantKey: "url.https://oauth2:tok%20val#frag@gitlab.com/.insteadOf",
			wantVal: "https://gitlab.com/",
		},
		{
			name:      "token with slash",
			url:       "https://bitbucket.org/team/repo.git",
			envVars:   map[string]string{"BITBUCKET_TOKEN": "tok/with/slashes"},
			wantCount: "1", wantIdx: "0",
			wantKey: "url.https://x-token-auth:tok/with/slashes@bitbucket.org/.insteadOf",
			wantVal: "https://bitbucket.org/",
		},
		{
			name:      "url with existing username preserves it",
			url:       "https://demouser@bitbucket.org/team/repo.git",
			envVars:   map[string]string{"SKILLSHARE_GIT_TOKEN": "tok123"},
			wantCount: "1", wantIdx: "0",
			wantKey: "url.https://demouser:tok123@bitbucket.org/.insteadOf",
			wantVal: "https://demouser@bitbucket.org/",
		},
		{
			name:      "appends to existing GIT_CONFIG_COUNT",
			url:       "https://github.com/org/repo.git",
			envVars:   map[string]string{"GITHUB_TOKEN": "ghp_abc", "GIT_CONFIG_COUNT": "2"},
			wantCount: "3", wantIdx: "2",
			wantKey: "url.https://x-access-token:ghp_abc@github.com/.insteadOf",
			wantVal: "https://github.com/",
		},
		{
			name:      "invalid GIT_CONFIG_COUNT treated as zero",
			url:       "https://github.com/org/repo.git",
			envVars:   map[string]string{"GITHUB_TOKEN": "ghp_abc", "GIT_CONFIG_COUNT": "abc"},
			wantCount: "1", wantIdx: "0",
			wantKey: "url.https://x-access-token:ghp_abc@github.com/.insteadOf",
			wantVal: "https://github.com/",
		},
		{
			name:    "ssh returns nil",
			url:     "git@github.com:org/repo.git",
			envVars: map[string]string{"GITHUB_TOKEN": "ghp_abc"},
			wantNil: true,
		},
		{
			name:    "file url returns nil",
			url:     "file:///path/to/repo",
			envVars: map[string]string{"SKILLSHARE_GIT_TOKEN": "tok"},
			wantNil: true,
		},
		{
			name:    "no token returns nil",
			url:     "https://github.com/org/repo.git",
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}
			got := authEnv(tt.url)
			if tt.wantNil {
				if got != nil {
					t.Errorf("authEnv(%q) = %v, want nil", tt.url, got)
				}
				return
			}
			wantCountEnv := "GIT_CONFIG_COUNT=" + tt.wantCount
			if len(got) != 3 || got[0] != wantCountEnv {
				t.Fatalf("authEnv(%q) = %v, want [%q, ...]", tt.url, got, wantCountEnv)
			}
			wantKeyEnv := "GIT_CONFIG_KEY_" + tt.wantIdx + "=" + tt.wantKey
			wantValEnv := "GIT_CONFIG_VALUE_" + tt.wantIdx + "=" + tt.wantVal
			if got[1] != wantKeyEnv {
				t.Errorf("authEnv(%q)[1] = %q, want %q", tt.url, got[1], wantKeyEnv)
			}
			if got[2] != wantValEnv {
				t.Errorf("authEnv(%q)[2] = %q, want %q", tt.url, got[2], wantValEnv)
			}
		})
	}
}

func TestSanitizeTokens(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		envVars map[string]string
		want    string
	}{
		{
			name:    "redacts github token",
			text:    "fatal: auth failed for https://x-access-token:ghp_secret@github.com/",
			envVars: map[string]string{"GITHUB_TOKEN": "ghp_secret"},
			want:    "fatal: auth failed for https://x-access-token:[REDACTED]@github.com/",
		},
		{
			name:    "redacts multiple tokens",
			text:    "tok1=ghp_aaa tok2=glpat-bbb",
			envVars: map[string]string{"GITHUB_TOKEN": "ghp_aaa", "GITLAB_TOKEN": "glpat-bbb"},
			want:    "tok1=[REDACTED] tok2=[REDACTED]",
		},
		{
			name:    "redacts bitbucket username",
			text:    "fatal: auth failed for https://willie0903:app_pwd@bitbucket.org/",
			envVars: map[string]string{"BITBUCKET_TOKEN": "app_pwd", "BITBUCKET_USERNAME": "willie0903"},
			want:    "fatal: auth failed for https://[REDACTED]:[REDACTED]@bitbucket.org/",
		},
		{
			name: "no token no change",
			text: "fatal: repository not found",
			want: "fatal: repository not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}
			got := sanitizeTokens(tt.text)
			if got != tt.want {
				t.Errorf("sanitizeTokens() = %q, want %q", got, tt.want)
			}
		})
	}
}
