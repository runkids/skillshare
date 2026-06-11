package search

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
)

const sampleSkillMD = `---
name: foo
description: A foo skill
license: MIT
tags: [alpha, beta]
---
# Foo

Body content.
`

// contentsResponse encodes body as the GitHub Contents API would.
func contentsJSON(t *testing.T, body string) string {
	t.Helper()
	payload := map[string]any{
		"encoding": "base64",
		"content":  base64.StdEncoding.EncodeToString([]byte(body)),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}

// contentsServer routes by decoded path suffix. A ServeMux can't be used here
// because the Contents API URL percent-escapes the file path ("/" -> "%2F"),
// which the mux pattern matcher won't match. r.URL.Path is already decoded.
func contentsServer(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for suffix, body := range routes {
			if strings.HasSuffix(r.URL.Path, suffix) {
				_, _ = w.Write([]byte(body))
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func TestFetchViaContentsAPI_Success(t *testing.T) {
	srv := contentsServer(t, map[string]string{
		"/contents/skills/foo/SKILL.md": contentsJSON(t, sampleSkillMD),
		"/repos/o/r":                    `{"stargazers_count": 42}`,
	})
	defer srv.Close()

	preview, err := fetchViaContentsAPI(srv.Client(), srv.URL, "", "o", "r", "skills/foo", "")
	if err != nil {
		t.Fatalf("fetchViaContentsAPI: %v", err)
	}
	if preview.Name != "foo" {
		t.Errorf("Name = %q, want foo", preview.Name)
	}
	if preview.Description != "A foo skill" {
		t.Errorf("Description = %q", preview.Description)
	}
	if preview.License != "MIT" {
		t.Errorf("License = %q", preview.License)
	}
	if len(preview.Tags) != 2 || preview.Tags[0] != "alpha" || preview.Tags[1] != "beta" {
		t.Errorf("Tags = %v", preview.Tags)
	}
	if preview.Stars != 42 {
		t.Errorf("Stars = %d, want 42", preview.Stars)
	}
	if preview.Source != "o/r/skills/foo" {
		t.Errorf("Source = %q", preview.Source)
	}
	if !strings.Contains(preview.Content, "Body content.") {
		t.Errorf("Content missing body: %q", preview.Content)
	}
}

func TestFetchViaContentsAPI_TreeFallback(t *testing.T) {
	// Direct shorthand path 404s (no route); the git tree resolves the real
	// path under source/, then the resolved content path serves SKILL.md.
	srv := contentsServer(t, map[string]string{
		"/git/trees/HEAD": `{"tree":[{"path":"source/skills/critique/SKILL.md","type":"blob"}]}`,
		"/contents/source/skills/critique/SKILL.md": contentsJSON(t, sampleSkillMD),
		"/repos/o/r": `{"stargazers_count": 0}`,
	})
	defer srv.Close()

	preview, err := fetchViaContentsAPI(srv.Client(), srv.URL, "", "o", "r", "critique", "")
	if err != nil {
		t.Fatalf("fetchViaContentsAPI: %v", err)
	}
	if preview.Name != "foo" {
		t.Errorf("Name = %q", preview.Name)
	}
	if preview.Source != "o/r/source/skills/critique" {
		t.Errorf("Source = %q", preview.Source)
	}
}

func TestFetchViaContentsAPI_NotFound(t *testing.T) {
	srv := contentsServer(t, map[string]string{
		"/git/trees/HEAD": `{"tree":[]}`,
	})
	defer srv.Close()

	_, err := fetchViaContentsAPI(srv.Client(), srv.URL, "", "o", "r", "missing", "")
	if !errors.Is(err, ErrSkillNotFound) {
		t.Fatalf("err = %v, want ErrSkillNotFound", err)
	}
}

func TestFetchViaClone_DirectSubdir(t *testing.T) {
	repo := initGitRepoWithFile(t, "skills/foo/SKILL.md", sampleSkillMD)

	src := &install.Source{CloneURL: repo, Subdir: "skills/foo"}
	preview, err := fetchViaClone(src, "git@example.com:o/r.git//skills/foo", "")
	if err != nil {
		t.Fatalf("fetchViaClone: %v", err)
	}
	if preview.Name != "foo" {
		t.Errorf("Name = %q", preview.Name)
	}
	if preview.Description != "A foo skill" {
		t.Errorf("Description = %q", preview.Description)
	}
	if preview.Source != "git@example.com:o/r.git//skills/foo" {
		t.Errorf("Source = %q", preview.Source)
	}
}

func TestFetchViaClone_ShorthandFallback(t *testing.T) {
	// Hub shorthand: Subdir is the skill name, real file lives under source/.
	repo := initGitRepoWithFile(t, "source/skills/critique/SKILL.md", sampleSkillMD)

	src := &install.Source{CloneURL: repo, Subdir: "critique"}
	preview, err := fetchViaClone(src, "git@example.com:o/r.git//critique", "")
	if err != nil {
		t.Fatalf("fetchViaClone: %v", err)
	}
	if preview.Name != "foo" {
		t.Errorf("Name = %q", preview.Name)
	}
}

func TestFetchViaClone_NotFound(t *testing.T) {
	repo := initGitRepoWithFile(t, "README.md", "no skill here")

	src := &install.Source{CloneURL: repo, Subdir: "ghost"}
	_, err := fetchViaClone(src, "git@example.com:o/r.git//ghost", "")
	if !errors.Is(err, ErrSkillNotFound) {
		t.Fatalf("err = %v, want ErrSkillNotFound", err)
	}
}

func TestFetchPreviewFromSource_SSHWithoutTokenUsesClone(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("SKILLSHARE_GIT_TOKEN", "")

	repo := initGitRepoWithFile(t, "skills/foo/SKILL.md", sampleSkillMD)
	src := &install.Source{
		Type:     install.SourceTypeGitSSH,
		Raw:      repo + "//skills/foo",
		CloneURL: repo,
		Subdir:   "skills/foo",
	}

	preview, err := fetchPreviewFromSource(http.DefaultClient, src, "git@example.com:o/r.git//skills/foo", "")
	if err != nil {
		t.Fatalf("fetchPreviewFromSource: %v", err)
	}
	if preview.Name != "foo" {
		t.Errorf("Name = %q, want foo", preview.Name)
	}
	if !strings.Contains(preview.Content, "Body content.") {
		t.Errorf("Content missing body: %q", preview.Content)
	}
}

func TestFetchPreviewFromSource_SSHContentsAPIFailureFallsBackToClone(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test")

	repo := initGitRepoWithFile(t, "skills/foo/SKILL.md", sampleSkillMD)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "url."+repo+".insteadOf")
	t.Setenv("GIT_CONFIG_VALUE_0", "git@github.com:o/r.git")

	apiRequests := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiRequests++
		if got := r.Header.Get("Authorization"); got != "token ghp_test" {
			t.Errorf("Authorization = %q, want token ghp_test", got)
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer api.Close()

	apiURL, err := url.Parse(api.URL)
	if err != nil {
		t.Fatalf("parse test api URL: %v", err)
	}
	client := &http.Client{Transport: rewriteHostTransport{target: apiURL}}
	src := &install.Source{
		Type:     install.SourceTypeGitSSH,
		Raw:      "git@github.com:o/r.git//skills/foo",
		CloneURL: "git@github.com:o/r.git",
		Subdir:   "skills/foo",
	}

	preview, err := fetchPreviewFromSource(client, src, "git@github.com:o/r.git//skills/foo", "")
	if err != nil {
		t.Fatalf("fetchPreviewFromSource: %v", err)
	}
	if preview.Name != "foo" {
		t.Errorf("Name = %q, want foo", preview.Name)
	}
	if preview.Owner != "o" {
		t.Errorf("Owner = %q, want o", preview.Owner)
	}
	if preview.Repo != "r" {
		t.Errorf("Repo = %q, want r", preview.Repo)
	}
	if apiRequests == 0 {
		t.Fatal("expected SSH preview with token to try Contents API before clone fallback")
	}
}

func TestPreviewToken_ResolvesEnvForSSHGitHubSource(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("SKILLSHARE_GIT_TOKEN", "")

	got := previewToken("git@github.com:o/r.git", "https://api.github.com")
	if got != "ghp_test" {
		t.Fatalf("previewToken() = %q, want ghp_test", got)
	}
}

type rewriteHostTransport struct {
	target *url.URL
}

func (t rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(clone)
}

func TestFindSkillFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "SKILL.md", "root")
	writeFile(t, root, "skills/foo/SKILL.md", "foo")
	writeFile(t, root, "source/skills/bar/SKILL.md", "bar-source")
	writeFile(t, root, "other/bar/SKILL.md", "bar-other")

	tests := []struct {
		name   string
		subdir string
		want   string
	}{
		{"root", "", "SKILL.md"},
		{"explicit root dot", ".", "SKILL.md"},
		{"direct subdir", "skills/foo", filepath.Join("skills", "foo", "SKILL.md")},
		{"shorthand prefers source", "bar", filepath.Join("source", "skills", "bar", "SKILL.md")},
		{"missing", "nope", ""},
		{"traversal rejected", "../escape", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findSkillFile(root, tt.subdir); got != tt.want {
				t.Errorf("findSkillFile(%q) = %q, want %q", tt.subdir, got, tt.want)
			}
		})
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
