package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	versioncheck "skillshare/internal/version"
)

func TestUIRestartHelperArgsPreservesAddressBasePathAndMode(t *testing.T) {
	s := &Server{addr: "0.0.0.0:19420", basePath: "/studio", projectRoot: "/tmp/project"}

	got, err := s.uiRestartHelperArgs(true)
	if err != nil {
		t.Fatalf("uiRestartHelperArgs returned error: %v", err)
	}
	want := []string{"__ui-restart", "--no-open", "--host", "0.0.0.0", "--port", "19420", "--base-path", "/studio", "--clear-cache", "--project"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestRestartRequestAllowedRejectsCrossSite(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/restart", strings.NewReader(`{}`))
	req.RemoteAddr = "203.0.113.10:1234"
	req.Host = "127.0.0.1:19420"
	req.Header.Set("Sec-Fetch-Site", "cross-site")

	if restartRequestAllowed(req, "0.0.0.0:19420") {
		t.Fatal("cross-site restart request should be rejected")
	}
}

func TestMCPRequestAllowed(t *testing.T) {
	tests := []struct {
		name       string
		serverAddr string
		remoteAddr string
		host       string
		fetchSite  string
		want       bool
	}{
		{"rebinding host on loopback bind", "127.0.0.1:19420", "127.0.0.1:5555", "attacker.example:19420", "same-origin", false},
		{"rebinding host on remote bind", "0.0.0.0:19420", "172.17.0.1:5555", "attacker.example:19420", "same-origin", false},
		{"localhost host", "127.0.0.1:19420", "127.0.0.1:5555", "localhost:5173", "", true},
		{"localhost trailing dot", "127.0.0.1:19420", "127.0.0.1:5555", "LOCALHOST.:19420", "", true},
		{"ipv4 loopback host", "127.0.0.1:19420", "127.0.0.1:5555", "127.0.0.1:19420", "", true},
		{"ipv6 loopback host", "127.0.0.1:19420", "[::1]:5555", "[::1]:19420", "", true},
		{"lan ip on remote bind", "0.0.0.0:19420", "192.168.1.30:5555", "192.168.1.20:19420", "same-origin", true},
		{"cross-site fetch", "127.0.0.1:19420", "127.0.0.1:5555", "localhost:19420", "cross-site", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/mcp", nil)
			req.RemoteAddr = tt.remoteAddr
			req.Host = tt.host
			if tt.fetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tt.fetchSite)
			}
			if got := (&Server{addr: tt.serverAddr}).mcpRequestAllowed(req); got != tt.want {
				t.Fatalf("mcpRequestAllowed = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandleUpgradeUsesInjectedRunner(t *testing.T) {
	old := runUIUpgrade
	runUIUpgrade = func(bool) (uiUpgradeResult, error) {
		return uiUpgradeResult{Updated: true, Output: "upgraded"}, nil
	}
	defer func() { runUIUpgrade = old }()

	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/upgrade", nil)

	s.handleUpgrade(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"updated":true`) {
		t.Fatalf("response should include updated=true: %s", rec.Body.String())
	}
}

func TestDefaultRunUIUpgradeSimulatesDevMode(t *testing.T) {
	oldVersion := versioncheck.Version
	versioncheck.Version = "dev"
	defer func() { versioncheck.Version = oldVersion }()

	result, err := defaultRunUIUpgrade(false)
	if err != nil {
		t.Fatalf("defaultRunUIUpgrade returned error: %v", err)
	}
	if !result.Updated || !result.DevMode || result.LatestVersion == "" {
		t.Fatalf("dev upgrade result = %#v", result)
	}
}

func TestHandleUpgradeReturnsRunnerError(t *testing.T) {
	old := runUIUpgrade
	runUIUpgrade = func(bool) (uiUpgradeResult, error) {
		return uiUpgradeResult{}, errors.New("boom")
	}
	defer func() { runUIUpgrade = old }()

	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/upgrade", nil)

	s.handleUpgrade(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("response should include error: %s", rec.Body.String())
	}
}

func TestHandleUpgradeNeedsSudoIsCoded(t *testing.T) {
	old := runUIUpgrade
	runUIUpgrade = func(bool) (uiUpgradeResult, error) {
		return uiUpgradeResult{Output: "run in a terminal: skillshare upgrade"}, errors.New("exit status 1")
	}
	defer func() { runUIUpgrade = old }()

	rec := httptest.NewRecorder()
	(&Server{}).handleUpgrade(rec, httptest.NewRequest(http.MethodPost, "/api/upgrade", nil))

	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"upgrade.needs_sudo"`) {
		t.Errorf("got %d %s, want 403 with upgrade.needs_sudo", rec.Code, rec.Body.String())
	}
}

// stubNixUpgrade fakes a Nix install whose version check reports newerCLI,
// and returns whether the upgrade ran and whether it was skill-only.
func stubNixUpgrade(t *testing.T, method versioncheck.InstallMethod, newerCLI bool) (ran, skillOnly *bool) {
	t.Helper()
	oldDetect, oldCheck, oldRun := detectUIInstallMethod, uiCLIUpdateAvailable, runUIUpgrade
	ran, skillOnly = new(bool), new(bool)
	detectUIInstallMethod = func() versioncheck.InstallMethod { return method }
	uiCLIUpdateAvailable = func(versioncheck.InstallMethod) bool { return newerCLI }
	runUIUpgrade = func(only bool) (uiUpgradeResult, error) {
		*ran, *skillOnly = true, only
		return uiUpgradeResult{Updated: true, Output: "Skill  updated"}, nil
	}
	t.Cleanup(func() { detectUIInstallMethod, uiCLIUpdateAvailable, runUIUpgrade = oldDetect, oldCheck, oldRun })
	return ran, skillOnly
}

func postUpgrade() *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	(&Server{}).handleUpgrade(rec, httptest.NewRequest(http.MethodPost, "/api/upgrade", nil))
	return rec
}

func TestHandleUpgradeNixUpdatesOnlyTheSkill(t *testing.T) {
	ran, skillOnly := stubNixUpgrade(t, versioncheck.InstallNixProfile, true)
	postUpgrade()
	if !*ran || !*skillOnly {
		t.Fatalf("ran = %v skillOnly = %v, want a skill-only upgrade", *ran, *skillOnly)
	}
}

func TestHandleUpgradeNixProfileReportsPendingCLI(t *testing.T) {
	stubNixUpgrade(t, versioncheck.InstallNixProfile, true)
	rec := postUpgrade()
	body := rec.Body.String()
	if rec.Code != http.StatusConflict || !strings.Contains(body, `"error_code":"upgrade.nix"`) || !strings.Contains(body, `"command":"nix profile upgrade skillshare"`) {
		t.Fatalf("status = %d body = %s", rec.Code, body)
	}
}

func TestHandleUpgradeNixConfigReportsPendingCLI(t *testing.T) {
	stubNixUpgrade(t, versioncheck.InstallNix, true)
	rec := postUpgrade()
	body := rec.Body.String()
	if rec.Code != http.StatusConflict || !strings.Contains(body, `"command":""`) || !strings.Contains(body, versioncheck.NixConfigUpgradeHint) {
		t.Fatalf("status = %d body = %s", rec.Code, body)
	}
}

func TestHandleUpgradeNixSucceedsWhenCLICurrent(t *testing.T) {
	stubNixUpgrade(t, versioncheck.InstallNix, false)
	if rec := postUpgrade(); rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpgradeOtherInstallsUpgradeTheCLI(t *testing.T) {
	_, skillOnly := stubNixUpgrade(t, versioncheck.InstallDirect, true)
	if rec := postUpgrade(); rec.Code != http.StatusOK || *skillOnly {
		t.Fatalf("status = %d skillOnly = %v, want a full upgrade", rec.Code, *skillOnly)
	}
}
