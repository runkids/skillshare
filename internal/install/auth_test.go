package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// wantAuthEnv is the env authEnv returns for host, appending at config index base.
func wantAuthEnv(base int, host, user, token string) []string {
	key := "credential.https://" + host + ".helper"
	return []string{
		fmt.Sprintf("GIT_CONFIG_COUNT=%d", base+2),
		fmt.Sprintf("GIT_CONFIG_KEY_%d=%s", base, key),
		fmt.Sprintf("GIT_CONFIG_VALUE_%d=", base),
		fmt.Sprintf("GIT_CONFIG_KEY_%d=%s", base+1, key),
		fmt.Sprintf("GIT_CONFIG_VALUE_%d=%s", base+1, gitCredentialHelper),
		"SKILLSHARE_GIT_AUTH_USERNAME=" + user,
		"SKILLSHARE_GIT_AUTH_PASSWORD=" + token,
	}
}

var authTestEnvKeys = []string{
	"GITHUB_TOKEN",
	"GH_TOKEN",
	"GITLAB_TOKEN",
	"BITBUCKET_TOKEN",
	"BITBUCKET_USERNAME",
	"AZURE_DEVOPS_TOKEN",
	"CNB_TOKEN",
	"GITEA_TOKEN",
	"SKILLSHARE_GIT_TOKEN",
	"GIT_CONFIG_COUNT",
}

func resetAuthTestEnv(t *testing.T) {
	t.Helper()
	for _, k := range authTestEnvKeys {
		t.Setenv(k, "")
	}
}

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
		{"github enterprise data residency", "https://acme.ghe.com/org/repo.git", PlatformGitHub},
		{"github enterprise data residency ssh", "acme@acme.ghe.com:org/repo.git", PlatformGitHub},
		{"gitlab.com", "https://gitlab.com/org/repo.git", PlatformGitLab},
		{"self-hosted gitlab", "https://gitlab.internal.co/org/repo.git", PlatformGitLab},
		{"bitbucket.org", "https://bitbucket.org/team/repo.git", PlatformBitbucket},
		{"ssh github", "git@github.com:org/repo.git", PlatformGitHub},
		{"azure devops", "https://dev.azure.com/org/proj/_git/repo", PlatformAzureDevOps},
		{"azure ssh", "git@ssh.dev.azure.com:v3/org/proj/repo", PlatformAzureDevOps},
		{"visualstudio.com", "https://myorg.visualstudio.com/proj/_git/repo", PlatformAzureDevOps},
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
			name:     "github GH_TOKEN fallback",
			url:      "https://github.com/org/repo.git",
			envVars:  map[string]string{"GH_TOKEN": "gho_fromcli"},
			wantTok:  "gho_fromcli",
			wantUser: "x-access-token",
		},
		{
			name:     "GITHUB_TOKEN takes priority over GH_TOKEN",
			url:      "https://github.com/org/repo.git",
			envVars:  map[string]string{"GITHUB_TOKEN": "ghp_primary", "GH_TOKEN": "gho_secondary"},
			wantTok:  "ghp_primary",
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
			name:     "cnb token",
			url:      "https://cnb.cool/org/repo.git",
			envVars:  map[string]string{"CNB_TOKEN": "cnb_pat"},
			wantTok:  "cnb_pat",
			wantUser: "cnb",
		},
		{
			name:     "gitea token",
			url:      "https://gitea.example.com/org/repo.git",
			envVars:  map[string]string{"GITEA_TOKEN": "gitea_pat"},
			wantTok:  "gitea_pat",
			wantUser: "x-access-token",
		},
		{
			name:     "azure devops token",
			url:      "https://dev.azure.com/org/proj/_git/repo",
			envVars:  map[string]string{"AZURE_DEVOPS_TOKEN": "ado_pat_test"},
			wantTok:  "ado_pat_test",
			wantUser: "x-access-token",
		},
		{
			name:     "azure devops generic fallback",
			url:      "https://dev.azure.com/org/proj/_git/repo",
			envVars:  map[string]string{"SKILLSHARE_GIT_TOKEN": "generic"},
			wantTok:  "generic",
			wantUser: "x-access-token",
		},
		{
			name:     "azure devops specific beats generic",
			url:      "https://dev.azure.com/org/proj/_git/repo",
			envVars:  map[string]string{"AZURE_DEVOPS_TOKEN": "ado_specific", "SKILLSHARE_GIT_TOKEN": "generic"},
			wantTok:  "ado_specific",
			wantUser: "x-access-token",
		},
		{
			name:    "azure devops ssh returns empty",
			url:     "git@ssh.dev.azure.com:v3/org/proj/repo",
			envVars: map[string]string{"AZURE_DEVOPS_TOKEN": "ado_pat"},
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
			resetAuthTestEnv(t)
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

func TestSourceAuthEnv_ConfiguredCNBGiteaHosts(t *testing.T) {
	tests := []struct {
		name    string
		source  *Source
		envVars map[string]string
		want    []string
	}{
		{
			name: "configured CNB host uses CNB_TOKEN",
			source: &Source{
				CloneURL: "https://git.corp.example/org/repo.git",
				CNBHosts: []string{"git.corp.example"},
			},
			envVars: map[string]string{"CNB_TOKEN": "cnb_private"},
			want:    wantAuthEnv(0, "git.corp.example", "cnb", "cnb_private"),
		},
		{
			name: "configured Gitea host uses GITEA_TOKEN",
			source: &Source{
				CloneURL:   "https://git.example.com/org/repo.git",
				GiteaHosts: []string{"git.example.com"},
			},
			envVars: map[string]string{"GITEA_TOKEN": "gitea_private"},
			want:    wantAuthEnv(0, "git.example.com", "x-access-token", "gitea_private"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetAuthTestEnv(t)
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}
			if got := tt.source.authEnv(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("authEnv() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestAPISourceDetection_UsesConfiguredHosts(t *testing.T) {
	cnb := &Source{CloneURL: "https://git.corp.example/org/repo.git", CNBHosts: []string{"git.corp.example"}}
	if !isCNBAPISource(cnb) {
		t.Fatalf("configured CNB host was not treated as CNB API source")
	}
	gitea := &Source{CloneURL: "https://git.example.com/org/repo.git", GiteaHosts: []string{"git.example.com"}}
	if !isGiteaAPISource(gitea) {
		t.Fatalf("configured Gitea host was not treated as Gitea API source")
	}
}

func TestAuthEnv(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		envVars map[string]string
		wantNil bool
		want    []string
	}{
		{
			name:    "github with token",
			url:     "https://github.com/org/repo.git",
			envVars: map[string]string{"GITHUB_TOKEN": "ghp_abc"},
			want:    wantAuthEnv(0, "github.com", "x-access-token", "ghp_abc"),
		},
		{
			name:    "gitlab with token",
			url:     "https://gitlab.com/org/repo.git",
			envVars: map[string]string{"GITLAB_TOKEN": "glpat-xyz"},
			want:    wantAuthEnv(0, "gitlab.com", "oauth2", "glpat-xyz"),
		},
		{
			name:    "bitbucket with token",
			url:     "https://bitbucket.org/team/repo.git",
			envVars: map[string]string{"BITBUCKET_TOKEN": "bb_tok"},
			want:    wantAuthEnv(0, "bitbucket.org", "x-token-auth", "bb_tok"),
		},
		{
			name:    "bitbucket token auth key ignores url username by default",
			url:     "https://willie0903@bitbucket.org/team/repo.git",
			envVars: map[string]string{"BITBUCKET_TOKEN": "app_pwd"},
			want:    wantAuthEnv(0, "bitbucket.org", "x-token-auth", "app_pwd"),
		},
		{
			name:    "bitbucket app password with BITBUCKET_USERNAME",
			url:     "https://bitbucket.org/team/repo.git",
			envVars: map[string]string{"BITBUCKET_TOKEN": "app_pwd", "BITBUCKET_USERNAME": "willie0903"},
			want:    wantAuthEnv(0, "bitbucket.org", "willie0903", "app_pwd"),
		},
		{
			name:    "token with equals sign",
			url:     "https://bitbucket.org/team/repo.git",
			envVars: map[string]string{"BITBUCKET_TOKEN": "ATATT3x_test=FBFF"},
			want:    wantAuthEnv(0, "bitbucket.org", "x-token-auth", "ATATT3x_test=FBFF"),
		},
		{
			name:    "azure devops with token",
			url:     "https://dev.azure.com/org/proj/_git/repo",
			envVars: map[string]string{"AZURE_DEVOPS_TOKEN": "ado_pat"},
			want:    wantAuthEnv(0, "dev.azure.com", "x-access-token", "ado_pat"),
		},
		{
			name:    "token with at sign",
			url:     "https://github.com/org/repo.git",
			envVars: map[string]string{"GITHUB_TOKEN": "ghp_abc@def"},
			want:    wantAuthEnv(0, "github.com", "x-access-token", "ghp_abc@def"),
		},
		{
			name:    "token with percent and hash",
			url:     "https://gitlab.com/org/repo.git",
			envVars: map[string]string{"GITLAB_TOKEN": "tok%20val#frag"},
			want:    wantAuthEnv(0, "gitlab.com", "oauth2", "tok%20val#frag"),
		},
		{
			name:    "token with slash",
			url:     "https://bitbucket.org/team/repo.git",
			envVars: map[string]string{"BITBUCKET_TOKEN": "tok/with/slashes"},
			want:    wantAuthEnv(0, "bitbucket.org", "x-token-auth", "tok/with/slashes"),
		},
		{
			name:    "url with existing username preserves it",
			url:     "https://demouser@bitbucket.org/team/repo.git",
			envVars: map[string]string{"SKILLSHARE_GIT_TOKEN": "tok123"},
			want:    wantAuthEnv(0, "bitbucket.org", "demouser", "tok123"),
		},
		{
			name:    "appends to existing GIT_CONFIG_COUNT",
			url:     "https://github.com/org/repo.git",
			envVars: map[string]string{"GITHUB_TOKEN": "ghp_abc", "GIT_CONFIG_COUNT": "2"},
			want:    wantAuthEnv(2, "github.com", "x-access-token", "ghp_abc"),
		},
		{
			name:    "invalid GIT_CONFIG_COUNT treated as zero",
			url:     "https://github.com/org/repo.git",
			envVars: map[string]string{"GITHUB_TOKEN": "ghp_abc", "GIT_CONFIG_COUNT": "abc"},
			want:    wantAuthEnv(0, "github.com", "x-access-token", "ghp_abc"),
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
			resetAuthTestEnv(t)
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
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("authEnv(%q) =\n%q\nwant\n%q", tt.url, got, tt.want)
			}
		})
	}
}

// Real git must take the token from skillshare's helper and never ask or
// erase the user's own helper (their keychain) for that host (#565).
func TestAuthEnv_GitAsksOnlySkillshareHelper(t *testing.T) {
	resetAuthTestEnv(t)
	t.Setenv("GITHUB_TOKEN", "ghp_from_env")
	dir := t.TempDir()
	global := filepath.Join(dir, "gitconfig")
	userLog := filepath.ToSlash(filepath.Join(dir, "user-helper.log"))
	userHelper := `!f() { echo "$1" >> '` + userLog + `'; echo username=realuser; echo password=stored; }; f`
	if out, err := exec.Command("git", "config", "--file", global, "credential.helper", userHelper).CombinedOutput(); err != nil {
		t.Fatalf("git config: %v\n%s", err, out)
	}

	cmd := exec.Command("git", "credential", "fill")
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+global, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	cmd.Env = append(cmd.Env, authEnv("https://github.com/org/repo.git")...)
	cmd.Stdin = strings.NewReader("protocol=https\nhost=github.com\n\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git credential fill: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "username=x-access-token\npassword=ghp_from_env\n") {
		t.Errorf("credential fill = %q, want skillshare's token", out)
	}

	// A rejected token must not erase the user's stored credentials either.
	reject := exec.Command("git", "credential", "reject")
	reject.Env = cmd.Env
	reject.Stdin = strings.NewReader("protocol=https\nhost=github.com\nusername=x-access-token\npassword=ghp_from_env\n\n")
	if out, err := reject.CombinedOutput(); err != nil {
		t.Fatalf("git credential reject: %v\n%s", err, out)
	}
	if _, err := os.Stat(userLog); err == nil {
		t.Errorf("user's credential helper was called")
	}
}

func TestIsSSLError(t *testing.T) {
	tests := []struct {
		output string
		want   bool
	}{
		{"fatal: unable to access 'https://gitlab.internal.com/': SSL certificate problem: self signed certificate", true},
		{"SSL certificate problem: unable to get local issuer certificate", true},
		{"fatal: unable to access 'https://git.company.com/': server certificate verification failed. CAfile: /etc/ssl/certs/ca-certificates.crt", true},
		{"ssl certificate problem: self signed certificate in certificate chain", true},
		{"fatal: Authentication failed for 'https://github.com/'", false},
		{"fatal: repository not found", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsSSLError(tt.output)
		if got != tt.want {
			t.Errorf("IsSSLError(%q) = %v, want %v", tt.output, got, tt.want)
		}
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
			name:    "redacts azure devops token",
			text:    "fatal: auth failed for https://x-access-token:ado_secret@dev.azure.com/",
			envVars: map[string]string{"AZURE_DEVOPS_TOKEN": "ado_secret"},
			want:    "fatal: auth failed for https://x-access-token:[REDACTED]@dev.azure.com/",
		},
		{
			name:    "redacts GH_TOKEN",
			text:    "fatal: auth failed for https://x-access-token:gho_cli_tok@github.com/",
			envVars: map[string]string{"GH_TOKEN": "gho_cli_tok"},
			want:    "fatal: auth failed for https://x-access-token:[REDACTED]@github.com/",
		},
		{
			name: "no token no change",
			text: "fatal: repository not found",
			want: "fatal: repository not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetAuthTestEnv(t)
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
