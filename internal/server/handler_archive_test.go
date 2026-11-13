package server

import (
	"archive/zip"
	"bytes"
	"reflect"
	"time"

	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"skillshare/internal/install"
	"strings"
	"testing"
)

func TestArchiveDashboardRoundTrip(t *testing.T) {
	var payload bytes.Buffer
	z := zip.NewWriter(&payload)
	f, _ := z.Create("brief/SKILL.md")
	_, _ = f.Write([]byte("# Brief"))
	_ = z.Close()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(payload.Bytes()) }))
	defer srv.Close()
	old := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport
	defer func() { http.DefaultTransport = old }()
	for _, project := range []bool{false, true} {
		s, _ := newUpdateFollowServer(t, project, false)
		raw := srv.URL + "/pack.zip"
		for _, endpoint := range []string{"discover", "install"} {
			body, _ := json.Marshal(map[string]string{"source": raw})
			rr := httptest.NewRecorder()
			s.mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/"+endpoint, bytes.NewReader(body)))
			if rr.Code != http.StatusOK {
				t.Fatalf("%s: %d %s", endpoint, rr.Code, rr.Body)
			}
			if endpoint == "discover" {
				var response struct{ Skills []struct{ Name, Path string } }
				if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if len(response.Skills) != 1 || response.Skills[0].Name != "brief" || response.Skills[0].Path != "brief" {
					t.Fatalf("discovered member: %s", rr.Body)
				}
			}
		}
		for _, stream := range []bool{false, true} {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/update", strings.NewReader(`{"name":"pack"}`))
			if stream {
				req = httptest.NewRequest(http.MethodGet, "/api/update/stream?names=pack", nil)
			}
			s.mux.ServeHTTP(rr, req)
			results := updateResults(t, rr, stream)
			if len(results) != 1 || results[0].Name != "pack" || results[0].Action != "skipped" {
				t.Fatalf("no-op response = %+v (%s)", results, rr.Body)
			}
		}
		if err := os.WriteFile(filepath.Join(s.skillsSource(), "pack", "SKILL.md"), []byte("# Mine"), 0600); err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		s.mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/resources/pack/local-changes", nil))
		var changes struct {
			Files []struct{ Path, Status, Diff string }
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &changes); err != nil {
			t.Fatal(err)
		}
		if rr.Code != http.StatusOK || len(changes.Files) != 1 || changes.Files[0].Status != "modified" || changes.Files[0].Diff != "" {
			t.Fatalf("ZIP changes endpoint: %d %s", rr.Code, rr.Body)
		}

	}
}

func TestArchiveSourceEdits(t *testing.T) {
	var payload bytes.Buffer
	z := zip.NewWriter(&payload)
	for _, name := range []string{"brief", "other"} {
		f, _ := z.Create(name + "/SKILL.md")
		_, _ = f.Write([]byte("# " + name))
	}
	_ = z.Close()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(payload.Bytes()) }))
	defer srv.Close()
	old := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport
	defer func() { http.DefaultTransport = old }()
	archive := srv.URL + "/pack.zip#skill=brief"
	repo := t.TempDir()
	initGitRepo(t, repo)
	addSkill(t, repo, "brief")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "fixture")
	gitSource := "file://" + repo + "//brief"
	for _, tc := range []struct{ name, from, to string }{
		{"same", archive, archive},
		{"zip-url", archive, srv.URL + "/new.zip#skill=brief"},
		{"zip-member", archive, srv.URL + "/pack.zip#skill=other"},
		{"zip-git", archive, gitSource},
		{"zip-local", archive, repo + "/brief"},
		{"zip-local-project", archive, repo + "/brief"},
		{"git-zip", gitSource, archive},
		{"local-zip", repo + "/brief", archive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, src := newUpdateFollowServer(t, tc.name == "zip-member" || tc.name == "local-zip" || tc.name == "zip-local-project", false)
			addSkill(t, src, "pack")
			dir := filepath.Join(src, "pack")
			parsed, err := install.ParseSource(tc.from)
			if err != nil {
				t.Fatal(err)
			}
			e := s.skillsStore.SetFromSource("pack", parsed)
			e.InstalledAt = time.Unix(100, 0).UTC()
			if parsed.IsGit() {
				e.Version = "abcdef0"
				e.Commit = "abcdef0123456789abcdef0123456789abcdef0123"
				e.TreeHash = "old-tree"
				e.Branch = "main"
			}
			if parsed.Type == install.SourceTypeLocalPath {
				e.Layout = install.LayoutDirectory
			}
			if parsed.Type == install.SourceTypeArchive {
				e.ArchiveSHA256, err = install.ArchiveHash(archive)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := e.ComputeEntryHashes(dir); err != nil {
				t.Fatal(err)
			}
			before := *e
			if err := s.skillsStore.Save(src); err != nil {
				t.Fatal(err)
			}
			if tc.name == "zip-local" {
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Mine"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			beforeContent, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			body, _ := json.Marshal(patchSourceRequest{Source: tc.to})
			rr := httptest.NewRecorder()
			s.mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPatch, "/api/resources/pack/source", bytes.NewReader(body)))
			entry := install.LoadMetadataOrNew(src).Get("pack")
			if strings.HasPrefix(tc.name, "zip-local") {
				afterContent, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
				if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "new folder outside this installation") || !strings.Contains(rr.Body.String(), "same --name, --into if grouped, and --force") {
					t.Fatalf("ZIP-to-local refusal/guidance: %d %s", rr.Code, rr.Body)
				}
				if err != nil || !bytes.Equal(beforeContent, afterContent) || !reflect.DeepEqual(entry, &before) {
					t.Fatal("refused source edit changed installed state")
				}
				return
			}
			if rr.Code != http.StatusOK {
				t.Fatalf("patch: %d %s", rr.Code, rr.Body)
			}
			if tc.name == "same" {
				if !reflect.DeepEqual(entry, &before) {
					t.Fatalf("identical edit changed provenance: %+v", entry)
				}
				return
			}
			target, err := install.ParseSource(tc.to)
			if err != nil {
				t.Fatal(err)
			}
			wantRepo := ""
			if target.IsGit() {
				wantRepo = target.CloneURL
			}
			if entry.Source != tc.to || entry.Type != target.MetaType() || entry.RepoURL != wantRepo || entry.Subdir != target.Subdir || entry.ArchiveSHA256 != "" || entry.Version != "" || entry.TreeHash != "" || entry.Commit != target.Commit || entry.Branch != target.Branch || entry.Layout != "" {
				t.Fatalf("source edit retained incompatible provenance: %+v", entry)
			}
			if !entry.InstalledAt.Equal(before.InstalledAt) || !reflect.DeepEqual(entry.FileHashes, before.FileHashes) {
				t.Fatal("source edit reset installed baseline")
			}
		})
	}
}

func TestArchiveSourceEditRejectsAgent(t *testing.T) {
	s, _ := newTestServer(t)
	src := s.agentsSource()
	addAgent(t, src, "helper.md")
	addAgentMeta(t, src, "helper.md", "https://github.com/team/repo/helper.md")
	s.agentsStore = install.LoadMetadataOrNew(src)
	before, err := os.ReadFile(filepath.Join(src, install.MetadataFileName))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPatch, "/api/resources/helper/source?kind=agent", strings.NewReader(`{"source":"https://host/pack.zip"}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("agent ZIP source: %d %s", rr.Code, rr.Body)
	}
	after, _ := os.ReadFile(filepath.Join(src, install.MetadataFileName))
	if !bytes.Equal(before, after) {
		t.Fatal("agent refusal changed metadata")
	}
}
