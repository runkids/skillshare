package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/projectdir"
)

type moveResponse struct {
	Results []batchMoveItemResult `json:"results"`
	Summary batchUninstallSummary `json:"summary"`
	Warns   []string              `json:"warnings"`
	DryRun  bool                  `json:"dryRun"`
}

func postMove(t *testing.T, s *Server, body any) (*httptest.ResponseRecorder, moveResponse) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/resources/batch/move", bytes.NewReader(raw)))
	var resp moveResponse
	if rr.Code == http.StatusOK {
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v: %s", err, rr.Body.String())
		}
	}
	return rr, resp
}

func TestHandleBatchMove_MovesSkillWithItsRecord(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "demo")
	s.skillsStore.Set("demo", &install.MetadataEntry{Source: "github.com/user/repo/demo"})
	s.skillsStore.AuditAccepted = map[string][]string{"demo": {"accepted-key"}}
	if err := s.skillsStore.Save(src); err != nil {
		t.Fatal(err)
	}

	rr, resp := postMove(t, s, map[string]any{"names": []string{"demo"}, "dest": "grp"})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
	want := batchMoveItemResult{Name: "demo", Success: true, From: "demo", To: "grp/demo", FlatName: "grp__demo", Record: true}
	if len(resp.Results) != 1 || resp.Results[0] != want {
		t.Fatalf("results = %+v, want %+v", resp.Results, want)
	}
	if resp.Summary.Succeeded != 1 || resp.Summary.Failed != 0 {
		t.Errorf("summary = %+v", resp.Summary)
	}
	if _, err := os.Stat(filepath.Join(src, "grp", "demo", "SKILL.md")); err != nil {
		t.Errorf("skill not at the new path: %v", err)
	}
	saved := install.LoadMetadataOrNew(src)
	if got := saved.Get("grp/demo"); got == nil || got.Group != "grp" || saved.Has("demo") {
		t.Errorf("saved keys = %v, want the record re-keyed with its group", saved.List())
	}
	if len(saved.AuditAccepted["grp/demo"]) != 1 {
		t.Errorf("accepted findings = %v, want them carried", saved.AuditAccepted)
	}
	entries, _ := oplog.Read(s.configPath(), oplog.OpsFile, 10)
	if len(entries) != 1 || entries[0].Command != "move" || entries[0].Status != "ok" {
		t.Errorf("oplog = %+v, want one ok move entry", entries)
	}
}

func TestHandleBatchMove_DryRunChangesNothing(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "demo")

	_, resp := postMove(t, s, map[string]any{"names": []string{"demo"}, "dest": "grp", "dryRun": true})

	if !resp.DryRun || len(resp.Results) != 1 || !resp.Results[0].Success || resp.Results[0].To != "grp/demo" {
		t.Fatalf("response = %+v, want the plan reported as a success", resp)
	}
	if _, err := os.Stat(filepath.Join(src, "demo", "SKILL.md")); err != nil {
		t.Errorf("dry run moved the skill: %v", err)
	}
	if entries, _ := oplog.Read(s.configPath(), oplog.OpsFile, 10); len(entries) != 0 {
		t.Errorf("dry run was logged: %+v", entries)
	}
}

func TestHandleBatchMove_PartialFailureIsStill200(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "demo")
	addSkill(t, src, "taken")
	addSkill(t, src, "grp/taken")

	rr, resp := postMove(t, s, map[string]any{"names": []string{"demo", "ghost", "taken"}, "dest": "grp"})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	codes := []string{resp.Results[0].ErrorCode, resp.Results[1].ErrorCode, resp.Results[2].ErrorCode}
	if want := []string{"", "skill_not_found", "dest_exists"}; strings.Join(codes, ",") != strings.Join(want, ",") {
		t.Errorf("error codes = %v, want %v", codes, want)
	}
	if resp.Summary.Succeeded != 1 || resp.Summary.Failed != 2 {
		t.Errorf("summary = %+v", resp.Summary)
	}
	entries, _ := oplog.Read(s.configPath(), oplog.OpsFile, 10)
	if len(entries) != 1 || entries[0].Status != "partial" {
		t.Errorf("oplog = %+v, want one partial entry", entries)
	}
}

func TestHandleBatchMove_FolderMovesWithEverythingBelowIt(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "frontend/a")
	addSkill(t, src, "frontend/deep/b")

	_, resp := postMove(t, s, map[string]any{"names": []string{"frontend"}, "dest": "archive"})

	if len(resp.Results) != 1 || !resp.Results[0].Success || resp.Results[0].Skills != 2 || resp.Results[0].To != "archive/frontend" {
		t.Fatalf("results = %+v, want one folder result with 2 skills", resp.Results)
	}
	if _, err := os.Stat(filepath.Join(src, "archive", "frontend", "deep", "b", "SKILL.md")); err != nil {
		t.Errorf("nested skill did not move: %v", err)
	}
}

func TestHandleBatchMove_NameCollisionNeedsForce(t *testing.T) {
	s, src := newTestServerWithTargets(t, map[string]string{"claude": filepath.Join(t.TempDir(), "claude-skills")})
	addSkill(t, src, "grp__demo")
	addSkill(t, src, "demo")

	_, resp := postMove(t, s, map[string]any{"names": []string{"demo"}, "dest": "grp"})
	if resp.Results[0].ErrorCode != "name_collision" {
		t.Fatalf("results = %+v, want name_collision", resp.Results)
	}

	_, resp = postMove(t, s, map[string]any{"names": []string{"demo"}, "dest": "grp", "force": true})
	if !resp.Results[0].Success || len(resp.Warns) == 0 {
		t.Errorf("forced response = %+v, want success with a warning", resp)
	}
}

func TestHandleBatchMove_RejectsMalformedRequests(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "demo")

	cases := []struct {
		name string
		body any
		code string
	}{
		{"no names", map[string]any{"names": []string{}, "dest": "grp"}, "invalid_body"},
		{"agent kind", map[string]any{"names": []string{"demo"}, "dest": "grp", "kind": "agent"}, "unsupported_kind"},
		{"empty dest", map[string]any{"names": []string{"demo"}, "dest": ""}, "invalid_dest"},
		{"dest outside the source", map[string]any{"names": []string{"demo"}, "dest": "../out"}, "invalid_dest"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr, _ := postMove(t, s, c.body)
			var got struct {
				ErrorCode string `json:"error_code"`
			}
			_ = json.Unmarshal(rr.Body.Bytes(), &got)
			if rr.Code != http.StatusBadRequest || got.ErrorCode != c.code {
				t.Errorf("status = %d, code = %q; want 400 %s (%s)", rr.Code, got.ErrorCode, c.code, rr.Body.String())
			}
		})
	}

	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/resources/batch/move", strings.NewReader("{")))
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "invalid_body") {
		t.Errorf("malformed JSON: status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(src, "demo", "SKILL.md")); err != nil {
		t.Errorf("a rejected request moved something: %v", err)
	}
}

func TestHandleBatchMove_ProjectModeCarriesPinAndGroup(t *testing.T) {
	s, _ := newTestServer(t)
	root := t.TempDir()
	skills := filepath.Join(root, ".skillshare", "skills")
	s.projectRoot = root
	s.projectCfg = &config.ProjectConfig{
		Targets: []config.ProjectTargetEntry{{Name: "claude"}},
		Skills:  []config.SkillEntry{{Name: "demo", Source: "github.com/user/repo/demo"}},
	}
	s.cfg.Source = skills
	if err := s.projectCfg.Save(root); err != nil {
		t.Fatal(err)
	}
	addSkill(t, skills, "demo")
	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("demo", &install.MetadataEntry{Source: "github.com/user/repo/demo"})
	if err := s.skillsStore.Save(skills); err != nil {
		t.Fatal(err)
	}
	const pinned = "0123456789abcdef0123456789abcdef01234567"
	dir := projectdir.Resolve(root)
	if err := (&install.Lock{Skills: map[string]install.LockEntry{"demo": {Source: "github.com/user/repo/demo", Commit: pinned}}}).Save(dir); err != nil {
		t.Fatal(err)
	}

	rr, resp := postMove(t, s, map[string]any{"names": []string{"demo"}, "dest": "grp"})

	if len(resp.Results) != 1 || !resp.Results[0].Success {
		t.Fatalf("status = %d, results = %+v, body = %s", rr.Code, resp.Results, rr.Body.String())
	}
	if got, err := install.LoadLock(dir); err != nil || got.Skills["grp/demo"].Commit != pinned || len(got.Skills) != 1 {
		t.Errorf("lock = %+v (err %v), want the pin carried to grp/demo", got, err)
	}
	if len(s.projectCfg.Skills) != 1 || s.projectCfg.Skills[0].Group != "grp" {
		t.Errorf("config skills = %+v, want one entry in group grp", s.projectCfg.Skills)
	}
}

// A name that is already in the destination is a no-op, not a failed move.
func TestHandleBatchMove_AlreadyInPlaceIsNotAFailure(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "grp/demo")

	_, resp := postMove(t, s, map[string]any{"names": []string{"grp/demo"}, "dest": "grp"})

	if len(resp.Results) != 1 || !resp.Results[0].Success || resp.Results[0].ErrorCode != "same_folder" || resp.Summary.Failed != 0 {
		t.Fatalf("response = %+v, want a success carrying same_folder", resp)
	}
	if entries, _ := oplog.Read(s.configPath(), oplog.OpsFile, 10); len(entries) != 1 || entries[0].Status != "ok" {
		t.Errorf("oplog = %+v, want one ok entry", entries)
	}
}

// A failed reconcile after the rename is reported in the response and the log,
// not swallowed: the project's recorded state is out of step until fixed.
func TestHandleBatchMove_ReportsReconcileFailure(t *testing.T) {
	s, _ := newTestServer(t)
	root := t.TempDir()
	skills := filepath.Join(root, ".skillshare", "skills")
	s.projectRoot = root
	s.projectCfg = &config.ProjectConfig{
		Targets: []config.ProjectTargetEntry{{Name: "claude"}},
		Skills:  []config.SkillEntry{{Name: "demo", Source: "github.com/user/repo/demo"}},
	}
	s.cfg.Source = skills
	addSkill(t, skills, "demo")
	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("demo", &install.MetadataEntry{Source: "github.com/user/repo/demo"})
	// A directory where config.yaml belongs: the group change cannot be saved.
	if err := os.MkdirAll(filepath.Join(root, ".skillshare", "config.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}

	raw, _ := json.Marshal(map[string]any{"names": []string{"demo"}, "dest": "grp"})
	rr := httptest.NewRecorder()
	s.handleBatchMove(rr, httptest.NewRequest(http.MethodPost, "/api/resources/batch/move", bytes.NewReader(raw)))

	var resp moveResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v: %s", err, rr.Body.String())
	}
	if len(resp.Results) != 1 || !resp.Results[0].Success {
		t.Fatalf("results = %+v, want the rename reported", resp.Results)
	}
	if len(resp.Warns) != 1 || !strings.Contains(resp.Warns[0], "follow-up step failed") {
		t.Errorf("warnings = %v, want the reconcile failure", resp.Warns)
	}
	entries, _ := oplog.Read(s.configPath(), oplog.OpsFile, 10)
	if len(entries) != 1 || entries[0].Status == "ok" {
		t.Errorf("oplog = %+v, want a non-ok entry", entries)
	}
}
