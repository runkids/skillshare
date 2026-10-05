package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/plugin"
)

func newOMPWritableServer(t *testing.T) (*Server, string) {
	t.Helper()
	s, home := newOMPExtensionsServer(t)
	pkg := filepath.Join(home, "npm/omp")
	writeFiles(t, pkg, map[string]string{"src/cli.ts": "throw new Error('must not execute');", "package.json": `{"name":"@oh-my-pi/pi-coding-agent","version":"18.6.1","bin":{"omp":"src/cli.ts"}}`})
	cli := filepath.Join(pkg, "src/cli.ts")
	if err := os.Chmod(cli, 0755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cli, filepath.Join(bin, "omp")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return s, home
}

func ompAPIChange(t *testing.T, s *Server, target string) (plugin.OMPExtensionsView, []plugin.OMPExtensionChange) {
	t.Helper()
	w := piRequest(t, s, http.MethodGet, "/api/targets/"+target+"/omp-extensions", "")
	if w.Code != 200 {
		t.Fatalf("GET: %d %s", w.Code, w.Body.String())
	}
	view := decodePi[plugin.OMPExtensionsView](t, w)
	for _, r := range view.Rows {
		if r.Selectable {
			return view, []plugin.OMPExtensionChange{{Key: r.Key, Enabled: false}}
		}
	}
	t.Fatalf("no selectable rows: %+v", view)
	return view, nil
}

func ompAPIBody(changes []plugin.OMPExtensionChange, revision string) string {
	raw, _ := json.Marshal(map[string]any{"changes": changes, "revision": revision})
	return string(raw)
}

func TestOMPExtensionsAPIPreviewApplyAndStale(t *testing.T) {
	s, home := newOMPWritableServer(t)
	view, changes := ompAPIChange(t, s, "omp")
	url := "/api/targets/omp/omp-extensions"
	before, _ := os.ReadFile(view.SettingsPath)
	w := piRequest(t, s, http.MethodPost, url+"/preview", ompAPIBody(changes, view.Revision))
	if w.Code != 200 {
		t.Fatalf("preview: %d %s", w.Code, w.Body.String())
	}
	plan := decodePi[plugin.OMPExtensionsPlan](t, w)
	if len(plan.Rows) != 1 || !plan.Rows[0].Before || plan.Rows[0].After {
		t.Fatalf("plan: %+v", plan)
	}
	after, _ := os.ReadFile(view.SettingsPath)
	if string(after) != string(before) {
		t.Fatal("preview wrote")
	}
	w = piRequest(t, s, http.MethodPost, url+"/apply", ompAPIBody(changes, plan.Revision))
	if w.Code != 200 {
		t.Fatalf("apply: %d %s", w.Code, w.Body.String())
	}
	applied := decodePi[plugin.OMPExtensionsPlan](t, w)
	if applied.BackupID == "" {
		t.Fatal("no backup")
	}
	backupPath := filepath.Join(config.StateDir(), "omp-extensions/backups", applied.BackupID+".json")
	backup, err := os.ReadFile(backupPath)
	if err != nil || strings.Contains(string(backup), "password") || strings.Contains(string(backup), "do-not-send") {
		t.Fatalf("unsafe backup: %v %s", err, backup)
	}
	after, _ = os.ReadFile(view.SettingsPath)
	if !strings.Contains(string(after), "password: do-not-send\n") || !strings.Contains(string(after), "extension-module:global") {
		t.Fatalf("settings: %s", after)
	}
	if strings.Contains(w.Body.String(), "do-not-send") || strings.Contains(w.Body.String(), "throw new Error") {
		t.Fatal("API exposed config/code")
	}
	if w = piRequest(t, s, http.MethodPost, url+"/apply", ompAPIBody(changes, plan.Revision)); w.Code != 409 {
		t.Fatalf("stale: %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".omp/agent/agent.db")); !os.IsNotExist(err) {
		t.Fatal("native storage/migration touched")
	}
}

func TestOMPExtensionsAPIWritesAccountAndProjectBoundaries(t *testing.T) {
	for _, scope := range []string{"account", "project", "global-project"} {
		t.Run(scope, func(t *testing.T) {
			s, home := newOMPWritableServer(t)
			target := "omp-work"
			file := filepath.Join(home, "account/agent/config.yml")
			switch scope {
			case "project":
				root := filepath.Join(home, "repo")
				writeFiles(t, root, map[string]string{".omp/config.yml": "# project\nunknown: true\n", ".omp/extensions/local.ts": "x"})
				s = NewProject(s.cfg, &config.ProjectConfig{}, root, "127.0.0.1:0", "", "")
				target, file = "omp", filepath.Join(root, ".omp/config.yml")
			case "global-project":
				root := filepath.Join(home, "registered")
				writeFiles(t, root, map[string]string{".omp/extensions/local.ts": "x"})
				writeFiles(t, filepath.Dir(os.Getenv("SKILLSHARE_CONFIG")), map[string]string{"config.yaml": "source: " + filepath.Join(home, "skills") + "\ntargets:\n  omp:\n    path: " + filepath.Join(home, ".omp/agent/skills") + "\nprojects:\n  " + root + ":\n    targets: [omp]\n    skills: {}\n"})
				cfg, err := config.Load()
				if err != nil {
					t.Fatal(err)
				}
				s = New(cfg, "127.0.0.1:0", "", "")
				for key, tc := range cfg.Targets {
					if tc.ProjectRoot() == root {
						target = key
					}
				}
				file = filepath.Join(root, ".omp/config.yml")
			}
			defaultFile := filepath.Join(home, ".omp/agent/config.yml")
			before, _ := os.ReadFile(defaultFile)
			view, changes := ompAPIChange(t, s, target)
			if view.SettingsPath != file {
				t.Fatalf("scope path: %+v", view)
			}
			url := "/api/targets/" + target + "/omp-extensions"
			w := piRequest(t, s, http.MethodPost, url+"/preview", ompAPIBody(changes, view.Revision))
			if w.Code != 200 {
				t.Fatalf("preview: %d %s", w.Code, w.Body.String())
			}
			plan := decodePi[plugin.OMPExtensionsPlan](t, w)
			w = piRequest(t, s, http.MethodPost, url+"/apply", ompAPIBody(changes, plan.Revision))
			if w.Code != 200 {
				t.Fatalf("apply: %d %s", w.Code, w.Body.String())
			}
			after, _ := os.ReadFile(defaultFile)
			if string(after) != string(before) {
				t.Fatal("scope wrote default settings")
			}
			raw, err := os.ReadFile(file)
			if err != nil || !strings.Contains(string(raw), "disabledExtensions:") {
				t.Fatalf("scope write: %v %s", err, raw)
			}
		})
	}
}

func TestOMPExtensionsAPIRefusesInvalidRequestsAndRemoteWriters(t *testing.T) {
	s, _ := newOMPWritableServer(t)
	view, changes := ompAPIChange(t, s, "omp")
	url := "/api/targets/omp/omp-extensions"
	for _, body := range []string{`{}`, `{"revision":"` + view.Revision + `","changes":[{"key":"` + changes[0].Key + `"}]}`, `{"revision":"` + view.Revision + `","changes":[{"key":"` + changes[0].Key + `","enabled":null}]}`, `{"revision":"` + view.Revision + `","changes":[{"key":"unknown","enabled":false}]}`, `{"revision":"` + view.Revision + `","changes":[],"extra":true}`, `{"revision":"` + view.Revision + `","changes":[]}`} {
		if w := piRequest(t, s, http.MethodPost, url+"/preview", body); w.Code != 400 {
			t.Fatalf("invalid %s: %d %s", body, w.Code, w.Body.String())
		}
	}
	for _, route := range []string{"preview", "apply"} {
		for _, tc := range []struct{ host, remote, origin string }{{"evil.example", "127.0.0.1:1234", ""}, {"127.0.0.1", "203.0.113.10:1234", ""}, {"127.0.0.1", "127.0.0.1:1234", "https://evil.example"}} {
			r := httptest.NewRequest(http.MethodPost, "http://"+tc.host+url+"/"+route, strings.NewReader(ompAPIBody(changes, view.Revision)))
			r.RemoteAddr = tc.remote
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			s.mux.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("local guard %s: %d %s", route, w.Code, w.Body.String())
			}
		}
	}
	if w := piRequest(t, s, http.MethodPost, "/api/targets/pi/omp-extensions/preview", ompAPIBody(changes, view.Revision)); w.Code != 404 {
		t.Fatalf("wrong target: %d", w.Code)
	}
	// A preview token cannot be replayed to a different account.
	w := piRequest(t, s, http.MethodPost, url+"/preview", ompAPIBody(changes, view.Revision))
	plan := decodePi[plugin.OMPExtensionsPlan](t, w)
	if w := piRequest(t, s, http.MethodPost, "/api/targets/omp-work/omp-extensions/apply", ompAPIBody(changes, plan.Revision)); w.Code != 409 {
		t.Fatalf("cross-account revision: %d %s", w.Code, w.Body.String())
	}
}
