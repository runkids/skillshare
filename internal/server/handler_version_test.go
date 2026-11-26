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
	return versionCheckWithSkillIgnore(t, local, remote, "")
}

// versionCheckWithSkillIgnore is versionCheckWithSkill with ignore written to
// the source's .skillignore when it is not empty.
func versionCheckWithSkillIgnore(t *testing.T, local, remote, ignore string) map[string]any {
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
	if ignore != "" {
		if err := os.WriteFile(filepath.Join(source, ".skillignore"), []byte(ignore), 0o644); err != nil {
			t.Fatal(err)
		}
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

func TestHandleVersionCheckReportsDisabledSkill(t *testing.T) {
	body := versionCheckWithSkillIgnore(t, "1.0.0", "1.0.0", "skillshare\n")

	if body["skillVersion"] != "1.0.0" || body["skillDisabled"] != true {
		t.Fatalf("skill fields = version %v, disabled %v; want 1.0.0 reported as disabled", body["skillVersion"], body["skillDisabled"])
	}
}

func TestHandleVersionCheckReportsNixUpgradeCommand(t *testing.T) {
	tests := []struct {
		method versioncheck.InstallMethod
		want   any // nil: skillshare upgrade replaces the CLI itself
	}{
		{method: versioncheck.InstallDirect, want: nil},
		{method: versioncheck.InstallBrew, want: nil},
		{method: versioncheck.InstallNixProfile, want: "nix profile upgrade skillshare"},
		{method: versioncheck.InstallNix, want: ""},
		{method: versioncheck.InstallPip, want: nil},
	}
	oldVersion, oldDetect := versioncheck.Version, detectUIInstallMethod
	defer func() { versioncheck.Version, detectUIInstallMethod = oldVersion, oldDetect }()
	versioncheck.Version = "dev" // skips the network lookup
	for _, tt := range tests {
		detectUIInstallMethod = func() versioncheck.InstallMethod { return tt.method }
		rec := httptest.NewRecorder()
		(&Server{cfg: &config.Config{Source: t.TempDir()}}).handleVersionCheck(rec, httptest.NewRequest(http.MethodGet, "/api/version", nil))

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body["nixUpgradeCommand"] != tt.want {
			t.Errorf("method %v: nixUpgradeCommand = %#v, want %#v", tt.method, body["nixUpgradeCommand"], tt.want)
		}
	}
}

func TestHandleVersionCheckReportsPyPIUpgradeCommand(t *testing.T) {
	tests := []struct {
		method versioncheck.InstallMethod
		want   any // nil: not a PyPI install
	}{
		{method: versioncheck.InstallDirect, want: nil},
		{method: versioncheck.InstallNixProfile, want: nil},
		{method: versioncheck.InstallUV, want: "uv tool upgrade skillshare-cli"},
		{method: versioncheck.InstallPip, want: "pip install --upgrade skillshare-cli"},
		{method: versioncheck.InstallPixiPyPI, want: "pixi upgrade skillshare-cli"},
		{method: versioncheck.InstallPixiGlobal, want: nil},
	}
	oldVersion, oldDetect := versioncheck.Version, detectUIInstallMethod
	defer func() { versioncheck.Version, detectUIInstallMethod = oldVersion, oldDetect }()
	versioncheck.Version = "dev" // skips the network lookup
	for _, tt := range tests {
		detectUIInstallMethod = func() versioncheck.InstallMethod { return tt.method }
		rec := httptest.NewRecorder()
		(&Server{cfg: &config.Config{Source: t.TempDir()}}).handleVersionCheck(rec, httptest.NewRequest(http.MethodGet, "/api/version", nil))

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body["pypiUpgradeCommand"] != tt.want {
			t.Errorf("method %v: pypiUpgradeCommand = %#v, want %#v", tt.method, body["pypiUpgradeCommand"], tt.want)
		}
	}
}

func TestHandleVersionCheckReportsPixiUpgradeCommand(t *testing.T) {
	tests := []struct {
		method versioncheck.InstallMethod
		want   any // nil: not a conda package from pixi
	}{
		{method: versioncheck.InstallDirect, want: nil},
		{method: versioncheck.InstallPixiPyPI, want: nil},
		{method: versioncheck.InstallPixiGlobal, want: "pixi global update skillshare"},
		{method: versioncheck.InstallPixi, want: "pixi upgrade skillshare"},
	}
	oldVersion, oldDetect := versioncheck.Version, detectUIInstallMethod
	defer func() { versioncheck.Version, detectUIInstallMethod = oldVersion, oldDetect }()
	versioncheck.Version = "dev" // skips the network lookup
	for _, tt := range tests {
		detectUIInstallMethod = func() versioncheck.InstallMethod { return tt.method }
		rec := httptest.NewRecorder()
		(&Server{cfg: &config.Config{Source: t.TempDir()}}).handleVersionCheck(rec, httptest.NewRequest(http.MethodGet, "/api/version", nil))

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body["pixiUpgradeCommand"] != tt.want {
			t.Errorf("method %v: pixiUpgradeCommand = %#v, want %#v", tt.method, body["pixiUpgradeCommand"], tt.want)
		}
	}
}
