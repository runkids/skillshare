package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

func projectRequest(t *testing.T, s *Server, method, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(method, url, strings.NewReader(body)))
	return rr
}

func addProject(t *testing.T, s *Server, root string) {
	t.Helper()
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	body := `{"create":true,"root":"` + root + `","targets":["claude","codex"],"skills":{"mode":"copy","include":["team-*"],"exclude":[]}}`
	if rr := projectRequest(t, s, http.MethodPut, "/api/projects", body); rr.Code != http.StatusOK {
		t.Fatalf("add project: %d %s", rr.Code, rr.Body)
	}
}

func TestListProjects_DetectsVisibleProjectConfig(t *testing.T) {
	s, src := newTestServer(t)
	root := filepath.Join(filepath.Dir(src), "app")
	addProject(t, s, root)
	if err := os.MkdirAll(filepath.Join(root, "skillshare"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skillshare", "config.yaml"), []byte("targets:\n  - claude\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var listed struct {
		Projects []projectItem `json:"projects"`
	}
	json.Unmarshal(projectRequest(t, s, http.MethodGet, "/api/projects", "").Body.Bytes(), &listed)
	if len(listed.Projects) != 1 || !listed.Projects[0].HasOwnConfig {
		t.Fatalf("projects %+v, want hasOwnConfig for the visible project dir", listed.Projects)
	}
}

func TestSaveProject_AddsTargetsThatOnlyTheProjectScopeLists(t *testing.T) {
	s, src := newTestServer(t)
	root := filepath.Join(filepath.Dir(src), "app")
	addProject(t, s, root)

	var listed struct {
		Targets []targetItem `json:"targets"`
	}
	json.Unmarshal(projectRequest(t, s, http.MethodGet, "/api/targets?scope=projects", "").Body.Bytes(), &listed)
	if len(listed.Targets) != 2 || listed.Targets[0].Project != root {
		t.Fatalf("project targets %+v", listed.Targets)
	}
	json.Unmarshal(projectRequest(t, s, http.MethodGet, "/api/targets", "").Body.Bytes(), &listed)
	if len(listed.Targets) != 0 {
		t.Fatalf("own targets %+v", listed.Targets)
	}
}

func TestSaveProject_Rejected(t *testing.T) {
	s, src := newTestServer(t)
	root := filepath.Join(filepath.Dir(src), "app")
	addProject(t, s, root)
	for name, body := range map[string]string{
		"already declared": `{"create":true,"root":"` + root + `","targets":["claude"]}`,
		"folder not found": `{"create":true,"root":"` + root + `-gone","targets":["claude"]}`,
		"unknown target":   `{"root":"` + root + `","targets":["nope"]}`,
		"invalid mode":     `{"root":"` + root + `","targets":["claude"],"skills":{"mode":"mirror"}}`,
	} {
		if rr := projectRequest(t, s, http.MethodPut, "/api/projects", body); rr.Code == http.StatusOK {
			t.Errorf("%s: saved", name)
		}
	}
}

func TestProjectTargets_CannotBeEditedAsTargets(t *testing.T) {
	s, src := newTestServer(t)
	addProject(t, s, filepath.Join(filepath.Dir(src), "app"))

	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		if rr := projectRequest(t, s, method, "/api/targets/app@claude", `{"mode":"merge"}`); rr.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", method, rr.Code, rr.Body)
		}
	}
}

func TestRemoveProject_DropsItsTargets(t *testing.T) {
	s, src := newTestServer(t)
	root := filepath.Join(filepath.Dir(src), "app")
	addProject(t, s, root)

	if rr := projectRequest(t, s, http.MethodDelete, "/api/projects?root="+root, ""); rr.Code != http.StatusOK {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	if len(s.cfg.Projects) != 0 || len(s.cfg.Targets) != 0 {
		t.Fatalf("projects %v, targets %v", s.cfg.Projects, s.cfg.Targets)
	}
}

func TestConvertProject_MovesTargetsUnderProjectsWithTheSameFolders(t *testing.T) {
	root := filepath.Join(t.TempDir(), "legacy")
	skills := filepath.Join(root, ".claude", "skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"old-claude": skills})

	if rr := projectRequest(t, s, http.MethodPost, "/api/projects/convert", `{"root":"`+root+`"}`); rr.Code != http.StatusOK {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	if _, kept := s.cfg.Targets["old-claude"]; kept {
		t.Error("old-claude is still a target")
	}
	converted := s.cfg.Targets[config.ProjectTargetName("legacy", "claude")]
	if got := converted.SkillsConfig().Path; got != skills {
		t.Errorf("skills path %q, want %q", got, skills)
	}
}

func TestProjectReplacementParity(t *testing.T) {
	for _, tc := range []struct {
		name, replacement string
		skills, agents    bool
	}{
		{"clear name and skills", `"agents":{"mode":"symlink","include":["team-*"],"exclude":["private"]}`, false, true},
		{"clear agents", `"skills":{}`, true, false},
		{"MCP hooks only", `"skills":null,"agents":null`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, src := newTestServer(t)
			root := filepath.Join(filepath.Dir(src), "app")
			addProject(t, s, root)
			rr := projectRequest(t, s, http.MethodPut, "/api/projects", `{"root":"`+root+`","name":"custom","targets":["claude"],"skills":{},"agents":{}}`)
			if rr.Code != 200 {
				t.Fatalf("seed: %d %s", rr.Code, rr.Body)
			}
			rr = projectRequest(t, s, http.MethodPut, "/api/projects", `{"root":"`+root+`","targets":["claude"],`+tc.replacement+`}`)
			if rr.Code != 200 || strings.TrimSpace(rr.Body.String()) != `{"root":"`+root+`","success":true}` {
				t.Fatalf("replacement: %d %s", rr.Code, rr.Body)
			}
			p := s.cfg.Projects[root]
			if p.Name != "" || (p.Skills != nil) != tc.skills || (p.Agents != nil) != tc.agents {
				t.Fatalf("replacement not full value: %+v", p)
			}
			if tc.agents && (len(p.Agents.Include) != 1 || p.Agents.Include[0] != "team-*" || len(p.Agents.Exclude) != 1) {
				t.Fatalf("agent filters lost: %+v", p.Agents)
			}
			listed := projectRequest(t, s, http.MethodGet, "/api/projects", "")
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(listed.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if len(envelope) != 3 || envelope["projects"] == nil || envelope["convertible"] == nil || envelope["tools"] == nil {
				t.Fatalf("list envelope: %s", listed.Body)
			}
		})
	}
}

func TestSaveProject_EffectiveCandidateRejectedWithoutMutation(t *testing.T) {
	s, src := newTestServer(t)
	root := filepath.Join(filepath.Dir(src), "app")
	other := filepath.Join(filepath.Dir(src), "other")
	for _, dir := range []string{root, other} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	raw := []byte("source: " + src + "\ntargets: {}\nprojects:\n  " + root + ": &shared\n    targets: [claude]\n    skills: {}\n  " + other + ": *shared\n")
	if err := os.WriteFile(config.ConfigPath(), raw, 0644); err != nil {
		t.Fatal(err)
	}
	rr := projectRequest(t, s, http.MethodPut, "/api/projects", `{"root":"`+root+`","name":"shared","targets":["claude"],"skills":{}}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "both named") {
		t.Fatalf("effective validation: %d %s", rr.Code, rr.Body)
	}
	if got, _ := os.ReadFile(config.ConfigPath()); !bytes.Equal(got, raw) {
		t.Fatal("rejected candidate changed disk")
	}
	if s.cfg.Projects[root].Name != "" || s.cfg.Projects[other].Name != "" {
		t.Fatal("rejected candidate changed in-memory declarations")
	}
}

func TestRemoveProject_DetachFailureKeepsRegistration(t *testing.T) {
	s, src := newTestServer(t)
	root := filepath.Join(filepath.Dir(src), "app")
	addProject(t, s, root)
	original := detachSkillsTarget
	t.Cleanup(func() { detachSkillsTarget = original })
	calls := 0
	detachSkillsTarget = func(path string, opts ssync.DetachOptions) (ssync.DetachReport, error) {
		calls++
		if opts.Source != src || opts.DryRun {
			t.Fatalf("detach inputs: %+v", opts)
		}
		if calls == 2 {
			return ssync.DetachReport{}, errors.New("later detach denied")
		}
		return ssync.DetachReport{Removed: []string{path}}, nil
	}
	rr := projectRequest(t, s, http.MethodDelete, "/api/projects?root="+root, "")
	if rr.Code != 500 || !strings.Contains(rr.Body.String(), "later detach denied") {
		t.Fatalf("detach response: %d %s", rr.Code, rr.Body)
	}
	if _, ok := s.cfg.Projects[root]; !ok {
		t.Fatal("detach failure changed memory")
	}
	disk, err := config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := disk.Projects[root]; !ok {
		t.Fatal("detach failure changed disk")
	}
}

func TestProjectsHTTP_RootOnlyLookupAndStatusParity(t *testing.T) {
	s, src := newTestServer(t)
	root := filepath.Join(filepath.Dir(src), "app")
	addProject(t, s, root)
	for _, tc := range []struct {
		method, url, body string
		status            int
	}{
		{http.MethodPut, "/api/projects", `{"create":true,"root":"` + root + `","targets":["claude"]}`, 409},
		{http.MethodPut, "/api/projects", `{"root":"app","targets":["claude"],"skills":{}}`, 404},
		{http.MethodDelete, "/api/projects?root=app", "", 404},
		{http.MethodPut, "/api/projects", `{"create":true,"root":"` + root + `-gone","targets":["claude"]}`, 400},
		{http.MethodPut, "/api/projects", `{"root":"` + root + `","targets":["claude"],"agents":{"include":["["]}}`, 400},
	} {
		rr := projectRequest(t, s, tc.method, tc.url, tc.body)
		if rr.Code != tc.status {
			t.Fatalf("%s %s: %d %s, want %d", tc.method, tc.url, rr.Code, rr.Body, tc.status)
		}
	}
}

func TestProjectsHTTP_AmbiguousRootsRejected(t *testing.T) {
	s, src := newTestServer(t)
	root := filepath.Join(filepath.Dir(src), "app")
	addProject(t, s, root)
	alias := root + string(filepath.Separator) + "."
	p := s.cfg.Projects[root]
	p.Name = "secondary"
	s.cfg.Projects[alias] = p
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, url, body string }{
		{http.MethodPut, "/api/projects", `{"root":"` + root + `","targets":["claude"],"skills":{}}`},
		{http.MethodDelete, "/api/projects?root=" + root, ""},
		{http.MethodPost, "/api/sync", `{"project":"` + root + `","dryRun":true}`},
	} {
		rr := projectRequest(t, s, tc.method, tc.url, tc.body)
		if rr.Code != 400 || !strings.Contains(rr.Body.String(), "ambiguous project") {
			t.Fatalf("%s %s: %d %s", tc.method, tc.url, rr.Code, rr.Body)
		}
	}
}
