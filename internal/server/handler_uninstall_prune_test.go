package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

// pruneCase seeds a source and a metadata store, uninstalls one name through
// a dashboard route, and names the store keys that must be gone or kept.
type pruneCase struct {
	repos   []string // tracked repos to create (committed, one member skill "a")
	entries map[string]*install.MetadataEntry
	remove  string
	gone    []string
	kept    []string
}

// pruneSource marks an entry reconcile keeps while its directory exists.
const pruneSource = "github.com/acme/skills"

// repoPrune checks that a route hands the repo to the prune and saves both the
// in-memory and on-disk store. Which keys a removal matches is
// install.MetadataStore.RemoveByNames's rule and is tested there.
var repoPrune = pruneCase{
	repos: []string{"_team", "_team-other"},
	entries: map[string]*install.MetadataEntry{
		"_team":         {Tracked: true},
		"_team/a":       {Group: "_team", Tracked: true},
		"_team-other/a": {Group: "_team-other", Source: pruneSource},
	},
	remove: "_team",
	gone:   []string{"_team", "_team/a"},
	kept:   []string{"_team-other/a"},
}

func seedPruneCase(t *testing.T, tc pruneCase) (*Server, string) {
	t.Helper()
	s, src := newTestServer(t)
	for _, repo := range tc.repos {
		addSkill(t, src, repo+"/a")
		initGitRepo(t, filepath.Join(src, repo)) // commits the member skill
	}
	s.skillsStore = install.NewMetadataStore()
	for key, entry := range tc.entries {
		e := *entry
		s.skillsStore.Set(key, &e)
	}
	if err := s.skillsStore.Save(src); err != nil {
		t.Fatal(err)
	}
	return s, src
}

func assertPruned(t *testing.T, s *Server, src string, tc pruneCase) {
	t.Helper()
	saved, err := install.LoadMetadata(src)
	if err != nil {
		t.Fatalf("load saved metadata: %v", err)
	}
	for _, key := range tc.gone {
		if s.skillsStore.Has(key) || saved.Has(key) {
			t.Errorf("entry %q should be pruned (memory=%v, disk=%v)", key, s.skillsStore.Has(key), saved.Has(key))
		}
	}
	for _, key := range tc.kept {
		if !s.skillsStore.Has(key) || !saved.Has(key) {
			t.Errorf("entry %q should be kept (memory=%v, disk=%v)", key, s.skillsStore.Has(key), saved.Has(key))
		}
	}
}

func postBatchUninstall(t *testing.T, s *Server, force bool, names ...string) (int, []batchUninstallItemResult) {
	t.Helper()
	b, _ := json.Marshal(batchUninstallRequest{Names: names, Force: force})
	rr := httptest.NewRecorder()
	s.handleBatchUninstall(rr, httptest.NewRequest(http.MethodPost, "/api/uninstall/batch", bytes.NewReader(b)))
	var resp struct {
		Results []batchUninstallItemResult `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode batch response: %v: %s", err, rr.Body.String())
	}
	return rr.Code, resp.Results
}

func deleteRepo(t *testing.T, s *Server, name, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/api/repos/"+name+query, nil)
	req.SetPathValue("name", name)
	rr := httptest.NewRecorder()
	s.handleUninstallRepo(rr, req)
	return rr
}

func deleteSkill(t *testing.T, s *Server, name string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/api/resources/"+name, nil)
	req.SetPathValue("name", name)
	rr := httptest.NewRecorder()
	s.handleUninstallSkill(rr, req)
	return rr
}

func TestHandleBatchUninstall_PrunesMetadata(t *testing.T) {
	s, src := seedPruneCase(t, repoPrune)
	if code, results := postBatchUninstall(t, s, false, repoPrune.remove); code != http.StatusOK || len(results) != 1 || !results[0].Success {
		t.Fatalf("uninstall failed: %d %+v", code, results)
	}
	assertPruned(t, s, src, repoPrune)
}

func TestHandleUninstallRepo_PrunesMetadata(t *testing.T) {
	s, src := seedPruneCase(t, repoPrune)
	if rr := deleteRepo(t, s, repoPrune.remove, ""); rr.Code != http.StatusOK {
		t.Fatalf("uninstall failed: %d %s", rr.Code, rr.Body.String())
	}
	assertPruned(t, s, src, repoPrune)
}

// A plain skill and a grouped skill may share a basename; removing one must
// leave the other's install metadata alone.
func TestHandleUninstallSkill_PrunesMetadata(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "batch"}[batch], func(t *testing.T) {
			s, src := newTestServer(t)
			addSkill(t, src, "foo")
			addSkill(t, src, "frontend/foo")
			s.skillsStore = install.NewMetadataStore()
			s.skillsStore.Set("foo", &install.MetadataEntry{Source: pruneSource})
			s.skillsStore.Set("frontend/foo", &install.MetadataEntry{Group: "frontend", Source: pruneSource})
			s.skillsStore.SetTargetOverride("foo", []string{"claude"})
			if err := s.skillsStore.Save(src); err != nil {
				t.Fatal(err)
			}

			if batch {
				if code, results := postBatchUninstall(t, s, false, "foo"); code != http.StatusOK || !results[0].Success {
					t.Fatalf("uninstall failed: %d %+v", code, results)
				}
			} else if rr := deleteSkill(t, s, "foo"); rr.Code != http.StatusOK {
				t.Fatalf("uninstall failed: %d %s", rr.Code, rr.Body.String())
			}

			assertPruned(t, s, src, pruneCase{gone: []string{"foo"}, kept: []string{"frontend/foo"}})
			if overrides := install.LoadTargetOverrides(src); len(overrides) != 0 {
				t.Errorf("target override of the removed skill should be gone, got %v", overrides)
			}
		})
	}
}

func TestHandleUninstallRepo_DirtyRepoNeedsForce(t *testing.T) {
	s, src := newTestServer(t)
	repoDir := filepath.Join(src, "_team")
	os.MkdirAll(repoDir, 0755)
	initGitRepo(t, repoDir)
	addSkill(t, src, "_team/a") // uncommitted

	rr := deleteRepo(t, s, "_team", "")
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a dirty repo, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"error_code":"repo_dirty"`) {
		t.Fatalf("the dashboard tells a dirty repo by its code, got %s", rr.Body.String())
	}
	if _, err := os.Stat(repoDir); err != nil {
		t.Fatalf("dirty repo must stay in place: %v", err)
	}

	if rr := deleteRepo(t, s, "_team", "?force=true"); rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with force, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(repoDir); !os.IsNotExist(err) {
		t.Fatalf("forced uninstall should remove the repo, stat err=%v", err)
	}
}

func TestHandleUninstallRepo_GitStatusErrorNeedsForce(t *testing.T) {
	s, src := newTestServer(t)
	repoDir := addTrackedRepoWithBrokenIndex(t, src, "_team")

	rr := deleteRepo(t, s, "_team", "")
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), `"error_code":"repo_status_failed"`) {
		t.Fatalf("expected 409 repo_status_failed when git status fails, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(repoDir); err != nil {
		t.Fatalf("repo must stay in place: %v", err)
	}
}

// A followed source link whose folder is itself a skill is refused by every
// route, also when the checkout is a git repo and named as a tracked repo.
func TestHandleUninstallRepo_FollowedLinkRootSkillRefused(t *testing.T) {
	s, src := newTestServer(t)
	s.cfg.FollowSourceLinks = true
	if err := s.saveConfig(); err != nil {
		t.Fatal(err)
	}
	checkout := t.TempDir()
	if err := os.WriteFile(filepath.Join(checkout, "SKILL.md"), []byte("---\nname: dev\n---\n# dev"), 0644); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, checkout)
	link := filepath.Join(src, "_dev")
	if err := os.Symlink(checkout, link); err != nil {
		t.Fatal(err)
	}

	rr := deleteRepo(t, s, "_dev", "?force=true")

	if !strings.Contains(rr.Body.String(), `"error_code":"conflict"`) {
		t.Fatalf("force cannot override a linked skill root, so it gets no force code: %s", rr.Body.String())
	}
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "use unlink to remove the link") {
		t.Fatalf("expected 409 pointing at unlink, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("the link must stay: %v", err)
	}
}

// In project mode the single-skill route must also drop the config entry, or
// the next `install -p` brings the skill back.
func TestHandleUninstallSkill_ProjectDropsConfigEntry(t *testing.T) {
	s, projectRoot := newProjectTargetServer(t, nil)
	addSkill(t, filepath.Join(projectRoot, ".skillshare", "skills"), "demo")
	s.projectCfg.Skills = []config.SkillEntry{{Name: "demo", Source: pruneSource}}

	if rr := deleteSkill(t, s, "demo"); rr.Code != http.StatusOK {
		t.Fatalf("uninstall failed: %d %s", rr.Code, rr.Body.String())
	}

	loaded, err := config.LoadProject(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Skills) != 0 {
		t.Errorf("config skills = %+v, want none", loaded.Skills)
	}
}

func TestHandleUninstallRepo_DeclarationResults(t *testing.T) {
	for _, scenario := range []string{"absent", "occupied", "shorthand", "save failure", "project", "unknown"} {
		t.Run(scenario, func(t *testing.T) {
			var s *Server
			var src string
			if scenario == "project" {
				s, _ = newProjectTargetServer(t, nil)
				src = s.skillsSource()
			} else {
				s, src = newTestServer(t)
			}
			s.skillsStore.Set("org/_team", &install.MetadataEntry{Source: "https://example.invalid/unreachable.git", Tracked: true, Group: "org"})
			if err := s.skillsStore.Save(src); err != nil {
				t.Fatal(err)
			}
			if scenario == "occupied" {
				addSkill(t, src, "org/_team/kept")
			}
			if scenario == "save failure" {
				if err := os.Remove(install.MetadataPath(src)); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(install.MetadataPath(src), 0755); err != nil {
					t.Fatal(err)
				}
			}
			name := "org/_team"
			if scenario == "shorthand" {
				name = "team"
			}
			if scenario == "unknown" {
				name = "missing"
			}
			rr := deleteRepo(t, s, name, "")
			switch scenario {
			case "save failure":
				if rr.Code != http.StatusInternalServerError || strings.Contains(rr.Body.String(), `"success":true`) {
					t.Fatalf("false success: %d %s", rr.Code, rr.Body.String())
				}
			case "project", "unknown":
				if rr.Code != http.StatusBadRequest || !s.skillsStore.Has("org/_team") {
					t.Fatalf("project declaration removed: %d %s", rr.Code, rr.Body.String())
				}
			default:
				var body struct {
					Success      bool
					Name         string
					MovedToTrash *bool
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if rr.Code != http.StatusOK || !body.Success || body.Name != "org/_team" || body.MovedToTrash == nil || *body.MovedToTrash {
					t.Fatalf("wrong metadata-only result: %d %s", rr.Code, rr.Body.String())
				}
				loaded, err := install.LoadMetadata(src)
				if err != nil || loaded.Has("org/_team") || s.skillsStore.Has("org/_team") {
					t.Fatalf("removal not persisted: %v", err)
				}
				if scenario == "occupied" {
					if _, err := os.Stat(filepath.Join(src, "org/_team/kept/SKILL.md")); err != nil {
						t.Fatal("occupant touched")
					}
				}
			}
		})
	}
}

func TestHandleUninstallRepo_ExactDeclarationBeforeCheckoutShorthand(t *testing.T) {
	for _, occupied := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "occupied"}[occupied], func(t *testing.T) {
			s, src := newTestServer(t)
			addSkill(t, src, "org/_team/a")
			repoDir := filepath.Join(src, "org", "_team")
			initGitRepo(t, repoDir)
			if occupied {
				addSkill(t, src, "_team/kept")
			}
			ignore := []byte("# managed repositories\n_team/\norg/_team/\n")
			if err := os.WriteFile(filepath.Join(src, ".gitignore"), ignore, 0644); err != nil {
				t.Fatal(err)
			}
			// Model a pull/CLI write after the dashboard loaded its cached store.
			store := install.NewMetadataStore()
			store.Set("_team", &install.MetadataEntry{Source: "https://example.invalid/root.git", Tracked: true})
			store.Set("org/_team", &install.MetadataEntry{Source: "https://example.invalid/grouped.git", Tracked: true, Group: "org", Branch: "main", FileHashes: map[string]string{"a/SKILL.md": "kept-hash"}})
			store.AuditAccepted = map[string][]string{
				"_team":              {"remove-root"},
				"_team/a":            {"remove-member"},
				"org/_team":          {"keep-root"},
				"org/_team/a":        {"keep-member"},
				"org/_team/unlisted": {"keep-path-only"},
			}
			if err := store.Save(src); err != nil {
				t.Fatal(err)
			}
			keptFiles := map[string][]byte{}
			for _, rel := range []string{"README.md", "a/SKILL.md", ".git/HEAD", ".git/config", ".git/index"} {
				file := filepath.Join(repoDir, filepath.FromSlash(rel))
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				keptFiles[file] = data
			}
			if occupied {
				file := filepath.Join(src, "_team", "kept", "SKILL.md")
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				keptFiles[file] = data
			}

			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/repos/_team", nil))
			var body struct {
				Success      bool
				Name         string
				MovedToTrash *bool
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if rr.Code != http.StatusOK || !body.Success || body.Name != "_team" || body.MovedToTrash == nil || *body.MovedToTrash {
				t.Fatalf("exact declaration selected another checkout: %d %s", rr.Code, rr.Body.String())
			}
			loaded, err := install.LoadMetadata(src)
			if err != nil {
				t.Fatal(err)
			}
			store.Remove("_team")
			delete(store.AuditAccepted, "_team/a")
			if !reflect.DeepEqual(loaded, store) || !reflect.DeepEqual(s.skillsStore, store) {
				t.Fatalf("wrong declaration or audit removal: disk=%+v cache=%+v want=%+v", loaded, s.skillsStore, store)
			}
			for file, want := range keptFiles {
				if got, err := os.ReadFile(file); err != nil || !bytes.Equal(got, want) {
					t.Errorf("unselected checkout or occupant changed at %s: %v", file, err)
				}
			}
			if got, err := os.ReadFile(filepath.Join(src, ".gitignore")); err != nil || !bytes.Equal(got, ignore) {
				t.Errorf("ignore bytes changed: %q, %v", got, err)
			}
			if !occupied {
				if _, err := os.Lstat(filepath.Join(src, "_team")); !os.IsNotExist(err) {
					t.Errorf("absent declaration path changed: %v", err)
				}
			}
		})
	}
}

func TestHandleUninstallRepo_CheckoutBeforeDeclarationShorthand(t *testing.T) {
	for _, tc := range []struct{ name, input, repo string }{
		{"exact checkout", "_team", "_team"},
		{"underscore shorthand", "team", "_team"},
		{"basename shorthand", "team", "org/_team"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prune := pruneCase{
				repos: []string{tc.repo},
				entries: map[string]*install.MetadataEntry{
					tc.repo:   {Source: pruneSource, Tracked: true},
					"a/_team": {Source: pruneSource, Tracked: true, Group: "a"},
					"b/_team": {Source: pruneSource, Tracked: true, Group: "b"},
				},
				gone: []string{tc.repo},
				kept: []string{"a/_team", "b/_team"},
			}
			s, src := seedPruneCase(t, prune)
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/repos/"+tc.input, nil))
			var body struct {
				Success      bool
				Name         string
				MovedToTrash *bool
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if rr.Code != http.StatusOK || !body.Success || body.Name != tc.repo || body.MovedToTrash == nil || !*body.MovedToTrash {
				t.Fatalf("declaration shorthand displaced checkout: %d %s", rr.Code, rr.Body.String())
			}
			assertPruned(t, s, src, prune)
			if _, err := os.Stat(filepath.Join(src, filepath.FromSlash(tc.repo))); !os.IsNotExist(err) {
				t.Errorf("checkout was not moved to trash: %v", err)
			}
		})
	}
}

func TestHandleUninstallRepo_ReloadsDeclarations(t *testing.T) {
	for _, addedAfterStart := range []bool{false, true} {
		t.Run(map[bool]string{false: "cached declaration", true: "new declaration"}[addedAfterStart], func(t *testing.T) {
			s, src := newTestServer(t)
			if !addedAfterStart {
				s.skillsStore.Set("_team", &install.MetadataEntry{Source: "https://example.invalid/team.git", Tracked: true})
				if err := s.skillsStore.Save(src); err != nil {
					t.Fatal(err)
				}
			}
			latest, err := install.LoadMetadata(src)
			if err != nil {
				t.Fatal(err)
			}
			latest.Set("_team", &install.MetadataEntry{Source: "https://example.invalid/team.git", Tracked: true})
			latest.Set("_cli", &install.MetadataEntry{Source: "https://example.invalid/cli.git", Tracked: true})
			latest.AuditAccepted = map[string][]string{"_team/member": {"remove"}, "_cli/member": {"cli-accepted"}}
			if err := latest.Save(src); err != nil {
				t.Fatal(err)
			}
			rr := deleteRepo(t, s, "_team", "")
			if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"movedToTrash":false`) {
				t.Fatalf("declaration not removed: %d %s", rr.Code, rr.Body.String())
			}
			// Verify disk before a later save, then verify the cache cannot lose CLI state.
			loaded, err := install.LoadMetadata(src)
			if err != nil {
				t.Fatal(err)
			}
			latest.ForgetTrackedRepo("_team")
			if !reflect.DeepEqual(loaded, latest) || !reflect.DeepEqual(s.skillsStore, latest) {
				t.Fatalf("removal lost newer records or acceptances: disk=%+v cache=%+v", loaded, s.skillsStore)
			}
		})
	}
}

func TestHandleUninstallRepo_MetadataReloadFailure(t *testing.T) {
	s, src := newTestServer(t)
	s.skillsStore.Set("_team", &install.MetadataEntry{Source: "https://example.invalid/team.git", Tracked: true})
	invalid := []byte("invalid metadata from interrupted external write")
	if err := os.WriteFile(install.MetadataPath(src), invalid, 0644); err != nil {
		t.Fatal(err)
	}
	rr := deleteRepo(t, s, "_team", "")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("reload failure must fail removal: %d %s", rr.Code, rr.Body.String())
	}
	if got, err := os.ReadFile(install.MetadataPath(src)); err != nil || !bytes.Equal(got, invalid) || !s.skillsStore.Has("_team") {
		t.Fatalf("reload failure saved stale metadata: %q, %v", got, err)
	}
}
