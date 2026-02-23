package install

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ghclient "skillshare/internal/github"
)

func TestDownloadGitHubDirWithAPIBase_Recursive(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/repos/acme/repo/contents/skills/bundle":
			fmt.Fprint(w, `[
{"type":"file","name":"SKILL.md","path":"skills/bundle/SKILL.md","download_url":"`+server.URL+`/raw/skill"},
{"type":"dir","name":"child","path":"skills/bundle/child"}
]`)
		case "/api/v3/repos/acme/repo/contents/skills/bundle/child":
			fmt.Fprint(w, `[
{"type":"file","name":"README.md","path":"skills/bundle/child/README.md","download_url":"`+server.URL+`/raw/readme"}
]`)
		case "/api/v3/repos/acme/repo/commits":
			fmt.Fprint(w, `[{"sha":"1234567890abcdef"}]`)
		case "/raw/skill":
			fmt.Fprint(w, "# skill")
		case "/raw/readme":
			fmt.Fprint(w, "child")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dest := t.TempDir()
	source := &Source{CloneURL: "https://github.acme.com/acme/repo.git"}
	commit, err := downloadGitHubDirWithAPIBase("acme", "repo", "skills/bundle", dest, server.URL+"/api/v3", source, nil)
	if err != nil {
		t.Fatalf("downloadGitHubDirWithAPIBase() error = %v", err)
	}
	if commit != "1234567" {
		t.Fatalf("commit = %q, want %q", commit, "1234567")
	}

	skillPath := filepath.Join(dest, "SKILL.md")
	readmePath := filepath.Join(dest, "child", "README.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("missing %s: %v", skillPath, err)
	}
	if _, err := os.Stat(readmePath); err != nil {
		t.Fatalf("missing %s: %v", readmePath, err)
	}
}

func TestDownloadGitHubDirWithAPIBase_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	dest := t.TempDir()
	source := &Source{CloneURL: "https://github.acme.com/acme/repo.git"}
	_, err := downloadGitHubDirWithAPIBase("acme", "repo", "skills/missing", dest, server.URL+"/api/v3", source, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "returned 404") {
		t.Fatalf("error = %v, want status 404", err)
	}
}

func TestDownloadGitHubDirWithAPIBase_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	dest := t.TempDir()
	source := &Source{CloneURL: "https://github.acme.com/acme/repo.git"}
	_, err := downloadGitHubDirWithAPIBase("acme", "repo", "skills/bundle", dest, server.URL+"/api/v3", source, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var rl *ghclient.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("error = %T %v, want RateLimitError", err, err)
	}
}

func TestGitHubAPIBase(t *testing.T) {
	tests := []struct {
		name    string
		source  *Source
		want    string
		wantErr bool
	}{
		{
			name:   "github.com",
			source: &Source{CloneURL: "https://github.com/acme/repo.git"},
			want:   "https://api.github.com",
		},
		{
			name:   "ghe",
			source: &Source{CloneURL: "https://github.acme.com/acme/repo.git"},
			want:   "https://github.acme.com/api/v3",
		},
		{
			name:    "non-github host",
			source:  &Source{CloneURL: "https://gitlab.com/acme/repo.git"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gitHubAPIBase(tt.source)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("gitHubAPIBase() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("gitHubAPIBase() = %q, want %q", got, tt.want)
			}
		})
	}
}
