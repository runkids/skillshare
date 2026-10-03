package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	versioncheck "skillshare/internal/version"
)

func TestHandleVersionCheckDevModeExposesSimulatedUpdate(t *testing.T) {
	oldVersion := versioncheck.Version
	versioncheck.Version = "dev"
	defer func() { versioncheck.Version = oldVersion }()

	s := &Server{cfg: &config.Config{Source: t.TempDir()}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)

	s.handleVersionCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["cliDevMode"] != true || body["cliUpdateAvailable"] != true || body["cliLatest"] == "" {
		t.Fatalf("dev version response = %#v", body)
	}
}

// versionCheckWithSkill serves /api/version for a source holding the
// built-in skill at local, with remote already in the skill version cache.
func versionCheckWithSkill(t *testing.T, local, remote string) map[string]any {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	versioncheck.SaveRemoteSkillVersion(remote)

	source := t.TempDir()
	skillDir := filepath.Join(source, "skillshare")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: skillshare\nmetadata:\n  version: " + local + "\n---\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Server{cfg: &config.Config{Source: source}}
	rec := httptest.NewRecorder()
	s.handleVersionCheck(rec, httptest.NewRequest(http.MethodGet, "/api/version", nil))

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body
}

func TestHandleVersionCheckUsesCachedSkillVersion(t *testing.T) {
	body := versionCheckWithSkill(t, "1.0.0", "9.9.9")

	if body["skillLatest"] != "9.9.9" || body["skillUpdateAvailable"] != true {
		t.Fatalf("skill fields = latest %v, update %v; want the cached 9.9.9 offered", body["skillLatest"], body["skillUpdateAvailable"])
	}
}

func TestHandleVersionCheckNewerLocalSkillIsNotAnUpdate(t *testing.T) {
	body := versionCheckWithSkill(t, "10.0.0", "9.9.9")

	if body["skillUpdateAvailable"] != false {
		t.Fatalf("skillUpdateAvailable = %v, want false when the local skill is newer", body["skillUpdateAvailable"])
	}
}
