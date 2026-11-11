package github

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNewRequestScopesGitHubCredentials(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "audit-dummy-token")
	for _, tt := range []struct {
		url  string
		want string
	}{
		{"https://api.github.com/repos/o/r", "token audit-dummy-token"},
		{"https://raw.githubusercontent.com/o/r/main/SKILL.md", "token audit-dummy-token"},
		{"https://github.attacker.invalid/api/v3/repos/o/r", ""},
		{"https://api.github.com.attacker.invalid/repos/o/r", ""},
		{"https://api.github.com:8443/repos/o/r", ""},
		{"http://api.github.com/repos/o/r", ""},
	} {
		t.Run(tt.url, func(t *testing.T) {
			req, err := NewRequest(tt.url)
			if err != nil {
				t.Fatal(err)
			}
			if got := req.Header.Get("Authorization"); got != tt.want {
				t.Fatalf("Authorization = %q, want %q", got, tt.want)
			}
		})
	}
}

type credentialRedirectTransport func(*http.Request) (*http.Response, error)

func (f credentialRedirectTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestClientDoesNotForwardCredentialsAcrossOrigins(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "audit-dummy-token")
	for _, tt := range []struct {
		redirect string
		want     string
	}{
		{"https://api.github.com/finish", "token audit-dummy-token"},
		{"https://sub.api.github.com/finish", ""},
		{"https://api.github.com:8443/finish", ""},
		{"http://api.github.com/finish", ""},
		{"https://unrelated.invalid/finish", ""},
	} {
		t.Run(tt.redirect, func(t *testing.T) {
			finalAuth := "not reached"
			finalPrivate := "not reached"
			client := NewClient()
			client.Transport = credentialRedirectTransport(func(r *http.Request) (*http.Response, error) {
				header := make(http.Header)
				status := http.StatusOK
				if r.URL.Path == "/start" {
					header.Set("Location", tt.redirect)
					status = http.StatusFound
				} else {
					finalAuth = r.Header.Get("Authorization")
					finalPrivate = r.Header.Get("PRIVATE-TOKEN")
				}
				return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader("fixture")), Request: r}, nil
			})
			req, err := NewRequest("https://api.github.com/start")
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("PRIVATE-TOKEN", "audit-dummy-token")
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if finalAuth != tt.want {
				t.Fatalf("redirect Authorization = %q, want %q", finalAuth, tt.want)
			}
			if want := strings.TrimPrefix(tt.want, "token "); finalPrivate != want {
				t.Fatalf("redirect PRIVATE-TOKEN = %q, want %q", finalPrivate, want)
			}
		})
	}
}
