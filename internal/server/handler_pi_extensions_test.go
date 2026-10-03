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

// newPiExtensionsServer serves a config with Pi, Claude, a Pi account and a project
// that syncs to Pi. A stub "pi" on PATH answers --version; nothing else runs.
func newPiExtensionsServer(t *testing.T, version string) (*Server, string) {
	t.Helper()
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg-config"))
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(home, ".pi", "agent"))
	bin := filepath.Join(tmp, "bin")
	writeFiles(t, bin, map[string]string{"pi": "#!/bin/sh\necho " + version + "\n"})
	if err := os.Chmod(filepath.Join(bin, "pi"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	pkg := filepath.Join(home, "pkgs", "tools")
	writeFiles(t, pkg, map[string]string{"package.json": `{"name":"tools"}`, "extensions/a.ts": "x", "extensions/b.ts": "x"})
	settings := `{"packages": [{"source": "` + pkg + `", "extensions": ["-extensions/b.ts"], "x-keep": {"token": "do-not-send"}}]}`
	writeFiles(t, filepath.Join(home, ".pi", "agent"), map[string]string{"settings.json": settings})
	writeFiles(t, filepath.Join(home, ".pi-work", "agent"), map[string]string{"settings.json": `{"packages": ["` + pkg + `"]}`})
	project := filepath.Join(tmp, "acme")
	writeFiles(t, filepath.Join(project, ".pi"), map[string]string{"settings.json": `{"packages": [{"source": "` + pkg + `", "autoload": false, "extensions": ["+extensions/b.ts"]}]}`})

	source := filepath.Join(tmp, "skills")
	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	writeFiles(t, filepath.Dir(cfgPath), map[string]string{"config.yaml": "source: " + source + "\nmode: merge\ntargets:\n" +
		"  pi:\n    path: " + filepath.Join(tmp, "pi-skills") + "\n" +
		"  claude:\n    path: " + filepath.Join(tmp, "claude-skills") + "\n" +
		"  pi-work:\n    agent: pi\n    config_dir: " + filepath.Join(home, ".pi-work", "agent") + "\n" +
		"projects:\n  " + project + ":\n    targets: [pi]\n    skills: {}\n"})
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return New(cfg, "127.0.0.1:0", "", ""), pkg
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func piRequest(t *testing.T, s *Server, method, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	r := httptest.NewRequest(method, "http://127.0.0.1"+url, reader)
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, r)
	return w
}

// piView is the part of the response the dashboard reads (ui/src/api/piExtensions.ts).
type piView struct {
	Target   string `json:"target"`
	Scope    string `json:"scope"`
	Editable bool   `json:"editable"`
	ReadOnly string `json:"readOnly"`
	Revision string `json:"revision"`
	Packages []struct {
		Index     int      `json:"index"`
		Source    string   `json:"source"`
		Shape     string   `json:"shape"`
		Rules     []string `json:"rules"`
		OtherKeys []string `json:"otherKeys"`
		Rows      []struct {
			Path      string `json:"path"`
			Selection string `json:"selection"`
			Editable  bool   `json:"editable"`
		} `json:"rows"`
	} `json:"packages"`
}

func decodePi[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("%d %s: %v", w.Code, w.Body.String(), err)
	}
	return v
}

func TestPiExtensionsAPIPreviewThenApply(t *testing.T) {
	s, pkg := newPiExtensionsServer(t, "0.99.2")
	w := piRequest(t, s, http.MethodGet, "/api/targets/pi/pi-extensions", "")
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "do-not-send") {
		t.Fatal("an unrelated settings value reached the dashboard")
	}
	view := decodePi[piView](t, w)
	if !view.Editable || view.Scope != "global" || len(view.Packages) != 1 || view.Packages[0].OtherKeys[0] != "x-keep" || view.Packages[0].Rows[1].Selection != "skipped" {
		t.Fatalf("view: %+v", view)
	}
	changes := `{"changes":[{"index":0,"source":"` + pkg + `","path":"extensions/b.ts","action":"select"}]`
	w = piRequest(t, s, http.MethodPost, "/api/targets/pi/pi-extensions/preview", changes+`}`)
	if w.Code != 200 || strings.Contains(w.Body.String(), "do-not-send") {
		t.Fatalf("preview %d: %s", w.Code, w.Body.String())
	}
	plan := decodePi[struct {
		Revision string `json:"revision"`
		Rows     []struct {
			Before string `json:"before"`
			After  string `json:"after"`
		} `json:"rows"`
		Entries []struct {
			After    []string `json:"after"`
			KeptKeys []string `json:"keptKeys"`
		} `json:"entries"`
	}](t, w)
	if plan.Revision == "" || plan.Rows[0].Before != "skipped" || plan.Rows[0].After != "loads" || plan.Entries[0].After[0] != "+extensions/b.ts" || plan.Entries[0].KeptKeys[0] != "x-keep" {
		t.Fatalf("plan: %+v", plan)
	}
	// A preview of pi is not a preview of the account, even for the same package.
	account := filepath.Join(filepath.Dir(os.Getenv("PI_CODING_AGENT_DIR")), "..", ".pi-work", "agent", "settings.json")
	accountBefore, _ := os.ReadFile(account)
	w = piRequest(t, s, http.MethodPost, "/api/targets/pi-work/pi-extensions/apply", changes+`,"revision":"`+plan.Revision+`"}`)
	if accountAfter, _ := os.ReadFile(account); w.Code != http.StatusConflict || string(accountAfter) != string(accountBefore) {
		t.Fatalf("account apply with pi's preview %d: %s", w.Code, w.Body.String())
	}
	w = piRequest(t, s, http.MethodPost, "/api/targets/pi/pi-extensions/apply", changes+`,"revision":"`+plan.Revision+`"}`)
	if w.Code != 200 {
		t.Fatalf("apply %d: %s", w.Code, w.Body.String())
	}
	data, _ := os.ReadFile(filepath.Join(os.Getenv("PI_CODING_AGENT_DIR"), "settings.json"))
	if !strings.Contains(string(data), `"extensions": ["+extensions/b.ts"], "x-keep": {"token": "do-not-send"}`) {
		t.Fatalf("settings: %s", data)
	}
	// The same revision again is stale now.
	w = piRequest(t, s, http.MethodPost, "/api/targets/pi/pi-extensions/apply", changes+`,"revision":"`+plan.Revision+`"}`)
	if w.Code != http.StatusConflict || decodePi[map[string]any](t, w)["error_code"] != "pi_extensions_stale" {
		t.Fatalf("stale apply %d: %s", w.Code, w.Body.String())
	}
}

func TestPiExtensionsAPIRefusesBadRequests(t *testing.T) {
	s, pkg := newPiExtensionsServer(t, "0.99.2")
	change := `{"index":0,"source":"` + pkg + `","path":"extensions/b.ts","action":"select"}`
	for name, c := range map[string]struct {
		url, body, code string
		status          int
	}{
		"not a Pi target": {"/api/targets/claude/pi-extensions/preview", `{"changes":[` + change + `]}`, "pi_extensions_not_pi", 404},
		"unknown target":  {"/api/targets/nope/pi-extensions/preview", `{"changes":[` + change + `]}`, "pi_extensions_not_pi", 404},
		"no preview":      {"/api/targets/pi/pi-extensions/apply", `{"changes":[` + change + `]}`, "pi_extensions_preview_first", 400},
		"bad action":      {"/api/targets/pi/pi-extensions/preview", `{"changes":[` + strings.Replace(change, "select", "enable", 1) + `]}`, "pi_extensions_invalid", 400},
	} {
		w := piRequest(t, s, http.MethodPost, c.url, c.body)
		if w.Code != c.status || decodePi[map[string]any](t, w)["error_code"] != c.code {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	if w := piRequest(t, s, http.MethodPost, "/api/targets/pi/pi-extensions/preview", `{"changes":[],"extra":1}`); w.Code != 400 {
		t.Errorf("unknown field: %d", w.Code)
	}
	if w := piRequest(t, s, http.MethodGet, "/api/targets/claude/pi-extensions", ""); w.Code != 404 {
		t.Errorf("claude view: %d", w.Code)
	}
	r := httptest.NewRequest(http.MethodGet, "http://attacker.example/api/targets/pi/pi-extensions", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Errorf("rebind accepted: %d", w.Code)
	}
}

func TestPiExtensionsAPIAccountAndUnsupportedVersion(t *testing.T) {
	s, _ := newPiExtensionsServer(t, "0.99.1")
	for _, target := range []string{"pi", "pi-work"} {
		w := piRequest(t, s, http.MethodGet, "/api/targets/"+target+"/pi-extensions", "")
		view := decodePi[piView](t, w)
		if w.Code != 200 || view.Editable || view.ReadOnly != "unsupportedVersion" || view.Packages[0].Rows[0].Editable {
			t.Fatalf("%s: %d %+v", target, w.Code, view)
		}
	}
	if view := decodePi[piView](t, piRequest(t, s, http.MethodGet, "/api/targets/pi-work/pi-extensions", "")); view.Scope != "account" {
		t.Fatalf("account scope: %+v", view)
	}
}

// A project change is saved to the project's .pi/settings.json only: the global
// settings stay byte for byte, the apply is logged under the project target, and
// the project's revision is refused by the global target.
func TestPiExtensionsAPIProjectSavesOnlyProjectSettings(t *testing.T) {
	s, pkg := newPiExtensionsServer(t, "0.99.2")
	w := piRequest(t, s, http.MethodGet, "/api/targets/acme@pi/pi-extensions", "")
	view := decodePi[piView](t, w)
	if w.Code != 200 || view.Scope != "project" || !view.Editable || view.ReadOnly != "" || view.Target != "acme@pi" || view.Packages[0].Shape != "delta" {
		t.Fatalf("project view %d: %s", w.Code, w.Body.String())
	}
	var project string
	for _, tc := range s.cfg.Targets {
		if tc.ProjectRoot() != "" {
			project = filepath.Join(tc.ProjectRoot(), ".pi", "settings.json")
		}
	}
	global := filepath.Join(os.Getenv("PI_CODING_AGENT_DIR"), "settings.json")
	globalBefore, _ := os.ReadFile(global)
	changes := `{"changes":[{"scope":"project","index":0,"source":"` + pkg + `","path":"extensions/a.ts","action":"exclude"}]`
	preview := piRequest(t, s, http.MethodPost, "/api/targets/acme@pi/pi-extensions/preview", changes+`}`)
	plan := decodePi[plugin.PiExtensionsPlan](t, preview)
	if preview.Code != 200 || plan.SettingsPath != project {
		t.Fatalf("project preview %d: %s", preview.Code, preview.Body.String())
	}
	if w := piRequest(t, s, http.MethodPost, "/api/targets/pi/pi-extensions/apply", changes+`,"revision":"`+plan.Revision+`"}`); w.Code != 409 {
		t.Fatalf("global apply of a project preview %d: %s", w.Code, w.Body.String())
	}
	if w := piRequest(t, s, http.MethodPost, "/api/targets/acme@pi/pi-extensions/apply", changes+`,"revision":"`+plan.Revision+`"}`); w.Code != 200 {
		t.Fatalf("project apply %d: %s", w.Code, w.Body.String())
	}
	want := `{"packages": [{"source": "` + pkg + `", "autoload": false, "extensions": ["+extensions/b.ts","-extensions/a.ts"]}]}`
	if got, _ := os.ReadFile(project); string(got) != want {
		t.Fatalf("project settings:\n got %s\nwant %s", got, want)
	}
	if globalAfter, _ := os.ReadFile(global); string(globalAfter) != string(globalBefore) {
		t.Fatal("a project request wrote the global settings")
	}
	if e := latestOpsEntry(t, s); e.Command != "pi-extensions" || e.Status != "ok" || e.Args["target"] != "acme@pi" {
		t.Fatalf("ops log: %+v", e)
	}
}

// Package sources may carry credentials in a URL's userinfo or query. GET and preview
// show them redacted and leave the settings file byte for byte as it was.
func TestPiExtensionsAPINeverSendsSourceCredentials(t *testing.T) {
	s, pkg := newPiExtensionsServer(t, "0.99.2")
	settings := filepath.Join(filepath.Dir(filepath.Dir(pkg)), ".pi", "agent", "settings.json")
	raw := `{"packages": [{"source": "` + pkg + `", "extensions": ["-extensions/b.ts"]},
		"git:https://dummy-user:dummy-pass@example.com/acme/a?access_token=dummy-token",
		"https://example.com/acme/b?access_token=dummy-token",
		{"source": "git:example.com/acme/c?token=dummy-token", "extensions": ["-x.ts"]}]}`
	writeFiles(t, filepath.Dir(settings), map[string]string{"settings.json": raw})
	get := piRequest(t, s, http.MethodGet, "/api/targets/pi/pi-extensions", "")
	preview := piRequest(t, s, http.MethodPost, "/api/targets/pi/pi-extensions/preview",
		`{"changes":[{"index":0,"source":"`+pkg+`","path":"extensions/b.ts","action":"select"}]}`)
	if get.Code != 200 || preview.Code != 200 {
		t.Fatalf("get %d %s\npreview %d %s", get.Code, get.Body.String(), preview.Code, preview.Body.String())
	}
	for name, body := range map[string]string{"get": get.Body.String(), "preview": preview.Body.String()} {
		if strings.Contains(body, "dummy-") {
			t.Errorf("%s sent a credential: %s", name, body)
		}
	}
	if got, err := os.ReadFile(settings); err != nil || string(got) != raw {
		t.Fatalf("settings changed: %v\n%s", err, got)
	}
}

// A string entry whose package declares only its extension in package.json, but
// has a skill and a prompt in the convention folders, can't take a first rule:
// the object form it needs would load them. GET says so, and a forced preview or
// apply is refused with the file left byte for byte as it was.
func TestPiExtensionsAPIRefusesAConversionThatLoadsOtherResources(t *testing.T) {
	s, pkg := newPiExtensionsServer(t, "0.99.2")
	partial := filepath.Join(filepath.Dir(pkg), "partial")
	writeFiles(t, partial, map[string]string{
		"package.json":           `{"name":"fixture","pi":{"extensions":["extensions/a.ts"]}}`,
		"extensions/a.ts":        "x",
		"skills/review/SKILL.md": "x",
		"prompts/commit.md":      "x",
	})
	settings := filepath.Join(filepath.Dir(filepath.Dir(pkg)), ".pi", "agent", "settings.json")
	raw := `{"packages": ["` + partial + `"]}`
	writeFiles(t, filepath.Dir(settings), map[string]string{"settings.json": raw})
	get := piRequest(t, s, http.MethodGet, "/api/targets/pi/pi-extensions", "")
	var view struct {
		Packages []struct {
			ReadOnly string `json:"readOnly"`
			Rows     []struct {
				Editable bool `json:"editable"`
			} `json:"rows"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &view); err != nil || view.Packages[0].ReadOnly != "otherResources" || view.Packages[0].Rows[0].Editable {
		t.Fatalf("view %d: %s", get.Code, get.Body.String())
	}
	changes := `"changes":[{"index":0,"source":"` + partial + `","path":"extensions/a.ts","action":"exclude"}]`
	if w := piRequest(t, s, http.MethodPost, "/api/targets/pi/pi-extensions/preview", `{`+changes+`}`); w.Code == 200 {
		t.Fatalf("preview accepted: %s", w.Body.String())
	}
	if w := piRequest(t, s, http.MethodPost, "/api/targets/pi/pi-extensions/apply", `{`+changes+`,"revision":"forced"}`); w.Code == 200 {
		t.Fatalf("apply accepted: %s", w.Body.String())
	}
	if got, err := os.ReadFile(settings); err != nil || string(got) != raw {
		t.Fatalf("settings changed: %v\n%s", err, got)
	}
}

// A rule Go would decode differently from Pi (an unpaired surrogate escape) makes
// its entry unsupported: GET sends no rules or rows, and a forced preview or apply
// writes nothing.
func TestPiExtensionsAPIRefusesAnEntryGoCannotReadLosslessly(t *testing.T) {
	s, pkg := newPiExtensionsServer(t, "0.99.2")
	settings := filepath.Join(filepath.Dir(filepath.Dir(pkg)), ".pi", "agent", "settings.json")
	raw := `{"packages": [{"source": "` + pkg + `", "extensions": ["*", "!extensions/\ud800.ts", "-extensions/a.ts"]}]}`
	writeFiles(t, filepath.Dir(settings), map[string]string{"settings.json": raw})
	get := piRequest(t, s, http.MethodGet, "/api/targets/pi/pi-extensions", "")
	var view struct {
		Packages []struct {
			Problem string            `json:"problem"`
			Rules   []string          `json:"rules"`
			Rows    []json.RawMessage `json:"rows"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &view); err != nil || view.Packages[0].Problem != "unsupportedEntry" || len(view.Packages[0].Rules) != 0 || len(view.Packages[0].Rows) != 0 {
		t.Fatalf("view %d: %s", get.Code, get.Body.String())
	}
	changes := `"changes":[{"index":0,"source":"` + pkg + `","path":"extensions/a.ts","action":"select"}]`
	if w := piRequest(t, s, http.MethodPost, "/api/targets/pi/pi-extensions/preview", `{`+changes+`}`); w.Code == 200 {
		t.Fatalf("preview accepted: %s", w.Body.String())
	}
	if w := piRequest(t, s, http.MethodPost, "/api/targets/pi/pi-extensions/apply", `{`+changes+`,"revision":"forced"}`); w.Code == 200 {
		t.Fatalf("apply accepted: %s", w.Body.String())
	}
	if got, err := os.ReadFile(settings); err != nil || string(got) != raw {
		t.Fatalf("settings changed: %v\n%s", err, got)
	}
}
