package install

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGiteaDownloadDir_WritesNestedFiles(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/o/r/contents/skills":
			fmt.Fprintf(w, `[{"type":"file","name":"SKILL.md","path":"skills/SKILL.md","download_url":"%s/raw/SKILL.md"},{"type":"dir","name":"ref","path":"skills/ref"}]`, srv.URL)
		case "/api/v1/repos/o/r/contents/skills/ref":
			fmt.Fprintf(w, `[{"type":"file","name":"a.md","path":"skills/ref/a.md","download_url":"%s/raw/a.md"}]`, srv.URL)
		case "/raw/SKILL.md", "/raw/a.md":
			fmt.Fprint(w, "body of "+filepath.Base(r.URL.Path))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dest := t.TempDir()
	if err := giteaDownloadDirRecursive(srv.Client(), nil, srv.URL+"/api/v1", "o", "r", "skills", dest, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "ref", "a.md"))
	if err != nil || string(got) != "body of a.md" {
		t.Fatalf("nested file = %q, %v", got, err)
	}
}

func TestGiteaDownloadDir_RefusesDownloadURLOnAnotherHost(t *testing.T) {
	t.Setenv("GITEA_TOKEN", "secret")
	var leaked string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("Authorization")
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `[{"type":"file","name":"SKILL.md","path":"SKILL.md","download_url":"%s/steal"}]`, other.URL)
	}))
	defer srv.Close()

	err := giteaDownloadDirRecursive(srv.Client(), nil, srv.URL+"/api/v1", "o", "r", "", t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "not on the API host") {
		t.Fatalf("err = %v", err)
	}
	if leaked != "" {
		t.Fatalf("token reached another host: %q", leaked)
	}
}

func TestGiteaDownloadDir_RefusesTraversalName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"type":"file","name":"..","path":"..","download_url":"http://x/y"}]`)
	}))
	defer srv.Close()

	if err := giteaDownloadDirRecursive(srv.Client(), nil, srv.URL, "o", "r", "", t.TempDir(), nil); err == nil {
		t.Fatal("expected an error for a \"..\" name")
	}
}

func TestCNBDownloadDir_WritesBlobFromTree(t *testing.T) {
	content := base64.StdEncoding.EncodeToString([]byte("hello"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/org/repo/-/git/contents/skills":
			fmt.Fprint(w, `{"type":"tree","name":"skills","path":"skills","entries":[{"type":"blob","name":"SKILL.md","path":"skills/SKILL.md"}]}`)
		case "/org/repo/-/git/contents/skills/SKILL.md":
			fmt.Fprintf(w, `{"type":"blob","name":"SKILL.md","path":"skills/SKILL.md","content":"%s"}`, content)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dest := t.TempDir()
	if err := cnbDownloadDirRecursive(srv.Client(), nil, srv.URL, "org/repo", "skills", dest, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	if err != nil || string(got) != "hello" {
		t.Fatalf("file = %q, %v", got, err)
	}
}

func TestForgeRequestsScopeProviderCredentials(t *testing.T) {
	tests := []struct {
		name, url, variable, want string
		hosts                     []string
		request                   func(string, []string) (*http.Request, error)
	}{
		{"Gitea public", "https://gitea.com/api/v1/repos/o/r", "GITEA_TOKEN", "token audit-dummy", nil, giteaNewRequest},
		{"Gitea lookalike", "https://gitea.attacker.invalid/api/v1/repos/o/r", "GITEA_TOKEN", "", nil, giteaNewRequest},
		{"Gitea explicit", "https://forge.company.invalid/api/v1/repos/o/r", "GITEA_TOKEN", "token audit-dummy", []string{"forge.company.invalid"}, giteaNewRequest},
		{"Gitea plaintext", "http://forge.company.invalid/api/v1/repos/o/r", "GITEA_TOKEN", "", []string{"forge.company.invalid"}, giteaNewRequest},
		{"CNB public", "https://cnb.cool/o/r/-/git/contents", "CNB_TOKEN", "Bearer audit-dummy", nil, cnbNewRequest},
		{"CNB lookalike", "https://cnb.cool.attacker.invalid/o/r/-/git/contents", "CNB_TOKEN", "", nil, cnbNewRequest},
		{"CNB explicit", "https://forge.company.invalid/o/r/-/git/contents", "CNB_TOKEN", "Bearer audit-dummy", []string{"forge.company.invalid"}, cnbNewRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetAuthTestEnv(t)
			t.Setenv(tt.variable, "audit-dummy")
			req, err := tt.request(tt.url, tt.hosts)
			if err != nil {
				t.Fatal(err)
			}
			if got := req.Header.Get("Authorization"); got != tt.want {
				t.Fatalf("Authorization = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestForgeFallbackPreservesCredentialOrigin(t *testing.T) {
	for _, sourceURL := range []string{
		"http://forge.company.invalid/o/r.git",
		"https://forge.company.invalid:8443/o/r.git",
	} {
		t.Run(sourceURL, func(t *testing.T) {
			resetAuthTestEnv(t)
			t.Setenv("GITEA_TOKEN", "audit-dummy")
			t.Setenv("CNB_TOKEN", "audit-dummy")
			source := &Source{CloneURL: sourceURL}
			for _, constructor := range []struct {
				base    string
				request func(string, []string) (*http.Request, error)
			}{
				{giteaAPIBase(source), giteaNewRequest},
				{forgeAPIOrigin(source), cnbNewRequest},
			} {
				req, err := constructor.request(constructor.base, []string{"forge.company.invalid"})
				if err != nil {
					t.Fatal(err)
				}
				if got := req.Header.Get("Authorization"); got != "" {
					t.Fatalf("fallback changed credential origin: %s received authorization", req.URL)
				}
			}
		})
	}
}
