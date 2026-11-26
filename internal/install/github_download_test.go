package install

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
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

func TestWriteDownloadedFile_ShebangIsExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not meaningful on Windows")
	}
	dir := t.TempDir()
	for name, want := range map[string]struct {
		body string
		mode os.FileMode
	}{
		"run.sh":   {"#!/bin/sh\necho hi\n", 0755},
		"notes.md": {"# notes\n", 0644},
		"tiny":     {"#", 0644},
	} {
		path := filepath.Join(dir, name)
		if err := writeDownloadedFile(path, strings.NewReader(want.body)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want.body {
			t.Fatalf("%s content = %q, want %q", name, data, want.body)
		}
		info, _ := os.Stat(path)
		if got := info.Mode().Perm(); got != want.mode {
			t.Fatalf("%s mode = %o, want %o", name, got, want.mode)
		}
	}

	// A script re-downloaded as plain content loses its bit.
	path := filepath.Join(dir, "run.sh")
	if err := writeDownloadedFile(path, strings.NewReader("plain text\n")); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if got := info.Mode().Perm(); got != 0644 {
		t.Fatalf("re-downloaded run.sh mode = %o, want 644", got)
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
			name:   "ghe data residency",
			source: &Source{CloneURL: "https://acme.ghe.com/acme/repo.git"},
			want:   "https://api.acme.ghe.com",
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

func TestDownloadGitHubDirWithAPIBase_RejectsSymlinkForCloneFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/repos/acme/repo/contents/skills/demo":
			fmt.Fprint(w, `[
{"type":"symlink","name":"ref.md","path":"skills/demo/ref.md","target":"../../shared/ref.md"}
]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	source := &Source{CloneURL: "https://github.acme.com/acme/repo.git"}
	_, err := downloadGitHubDirWithAPIBase("acme", "repo", "skills/demo", t.TempDir(), server.URL+"/api/v3", source, nil)
	if err == nil || !strings.Contains(err.Error(), "requires git clone") {
		t.Fatalf("expected symlink fallback error, got: %v", err)
	}
}
