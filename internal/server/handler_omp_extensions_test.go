package server

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/plugin"
)

func newOMPExtensionsServer(t *testing.T) (*Server, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, key := range []string{"PI_CONFIG_DIR", "PI_CODING_AGENT_DIR", "OMP_PROFILE", "PI_PROFILE", "PI_CONFIG_FILES", "XDG_DATA_HOME"} {
		t.Setenv(key, "")
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(home, "skillshare/config.yaml"))
	writeFiles(t, filepath.Join(home, ".omp/agent"), map[string]string{"extensions/global.ts": "throw new Error('must not execute');", "config.yml": "password: do-not-send\n"})
	account := filepath.Join(home, "account/agent")
	writeFiles(t, account, map[string]string{"extensions/account.ts": "x"})
	cfg := &config.Config{Source: filepath.Join(home, "skills"), Targets: map[string]config.TargetConfig{
		"omp":      {Path: filepath.Join(home, ".omp/agent/skills")},
		"omp-work": {Agent: "omp", ConfigDir: account},
		"pi":       {Path: filepath.Join(home, ".pi/agent/skills")},
		"custom":   {Path: filepath.Join(home, "custom")},
	}}
	return New(cfg, "127.0.0.1:0", "", ""), home
}

func TestOMPExtensionsAPIReadOnlyScopes(t *testing.T) {
	s, home := newOMPExtensionsServer(t)
	for _, target := range []string{"omp", "omp-work"} {
		w := piRequest(t, s, http.MethodGet, "/api/targets/"+target+"/omp-extensions", "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", target, w.Code, w.Body.String())
		}
		v := decodePi[plugin.OMPExtensionsView](t, w)
		if !v.ReadOnly || v.Target != target || len(v.Rows) != 1 || strings.Contains(w.Body.String(), "do-not-send") || strings.Contains(w.Body.String(), "throw new Error") {
			t.Fatalf("response: %+v", v)
		}
		if target == "omp-work" && (v.Scope != "account" || v.Root != filepath.Join(home, "account/agent")) {
			t.Fatalf("account: %+v", v)
		}
	}
	for _, url := range []string{"/api/targets/omp/omp-extensions/preview", "/api/targets/omp/omp-extensions/apply"} {
		if w := piRequest(t, s, http.MethodPost, url, `{}`); w.Code == 200 && strings.Contains(w.Header().Get("Content-Type"), "application/json") {
			t.Fatal("write route available")
		}
	}
	before, err := os.ReadFile(filepath.Join(home, ".omp/agent/config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	piRequest(t, s, http.MethodGet, "/api/targets/omp/omp-extensions", "")
	after, _ := os.ReadFile(filepath.Join(home, ".omp/agent/config.yml"))
	if string(before) != string(after) {
		t.Fatal("settings were changed")
	}
}

func TestOMPExtensionsAPIWrongTargetAndLocalGuard(t *testing.T) {
	s, _ := newOMPExtensionsServer(t)
	for _, target := range []string{"pi", "custom", "unknown"} {
		w := piRequest(t, s, http.MethodGet, "/api/targets/"+target+"/omp-extensions", "")
		if w.Code != 404 || decodePi[map[string]any](t, w)["error_code"] != "omp_extensions_not_omp" {
			t.Fatalf("wrong target: %d %s", w.Code, w.Body.String())
		}
	}
	for _, tc := range []struct{ host, remote, origin string }{
		{"attacker.example", "127.0.0.1:1234", ""},
		{"127.0.0.1", "203.0.113.10:1234", ""},
		{"127.0.0.1", "127.0.0.1:1234", "https://attacker.example"},
	} {
		r := httptest.NewRequest(http.MethodGet, "http://"+tc.host+"/api/targets/omp/omp-extensions", nil)
		r.RemoteAddr = tc.remote
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("guard %+v: %d %s", tc, w.Code, w.Body.String())
		}
	}
}

func TestOMPExtensionsAPILocalProjectScope(t *testing.T) {
	s, home := newOMPExtensionsServer(t)
	root := filepath.Join(home, "project")
	writeFiles(t, root, map[string]string{".omp/extensions/local.ts": "x", ".omp/config.yml": "extensions: []\n"})
	pcfg := &config.ProjectConfig{}
	s = NewProject(s.cfg, pcfg, root, "127.0.0.1:0", "", "")
	w := piRequest(t, s, http.MethodGet, "/api/targets/omp/omp-extensions", "")
	if w.Code != http.StatusOK {
		t.Fatalf("local project: %d %s", w.Code, w.Body.String())
	}
	v := decodePi[plugin.OMPExtensionsView](t, w)
	if v.Scope != "project" || v.Root != filepath.Join(root, ".omp") || v.SettingsPath != filepath.Join(root, ".omp/config.yml") || len(v.Rows) != 2 {
		t.Fatalf("local project: %+v", v)
	}
	if w := piRequest(t, s, http.MethodGet, "/api/targets/omp-work/omp-extensions", ""); w.Code != http.StatusNotFound {
		t.Fatalf("project account accepted: %d %s", w.Code, w.Body.String())
	}
}

func TestOMPExtensionsAPIGlobalProjectTarget(t *testing.T) {
	s, home := newOMPExtensionsServer(t)
	project := filepath.Join(home, "repo")
	writeFiles(t, project, map[string]string{".omp/extensions/local.ts": "x"})
	// Derived project metadata comes from loading the config, not the target name.
	writeFiles(t, filepath.Dir(os.Getenv("SKILLSHARE_CONFIG")), map[string]string{"config.yaml": "source: " + filepath.Join(home, "skills") + "\ntargets:\n  omp:\n    path: " + filepath.Join(home, ".omp/agent/skills") + "\nprojects:\n  " + project + ":\n    targets: [omp]\n    skills: {}\n"})
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	s = New(cfg, "127.0.0.1:0", "", "")
	var name string
	for key, tc := range s.cfg.Targets {
		if tc.ProjectRoot() == project {
			name = key
			break
		}
	}
	if name == "" {
		t.Fatal("project target missing")
	}
	local := filepath.Join(project, ".omp/extensions/local.ts")
	key := fmt.Sprintf("%x", sha256.Sum256([]byte("file\x00"+local)))
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte("x")))
	writeFiles(t, config.StateDir(), map[string]string{"hooks/state.json": `{"version":1,"records":{"` + key + `":{"owner":"` + s.configPath() + `","target":"omp","root":"` + project + `","path":"` + local + `","hash":"` + contentHash + `"}}}`})
	w := piRequest(t, s, http.MethodGet, "/api/targets/"+name+"/omp-extensions", "")
	if w.Code != 200 {
		t.Fatalf("project: %d %s", w.Code, w.Body.String())
	}
	v := decodePi[plugin.OMPExtensionsView](t, w)
	if v.Scope != "project" || v.Root != filepath.Join(project, ".omp") || len(v.Rows) != 2 {
		t.Fatalf("project: %+v", v)
	}
	for _, row := range v.Rows {
		if row.Path == local && (row.Owner != "hooks" || row.HooksTarget != name) {
			t.Fatalf("project hooks jump: %+v", row)
		}
	}
}
