//go:build !online

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/profileapply"
	"skillshare/internal/server"
)

func profileDefinitionAPI(t *testing.T, f profileFixture, project bool) func(string, string, string) (int, []byte) {
	t.Helper()
	scope := profileapply.Scope{ConfigPath: f.config}
	if project {
		scope.ProjectRoot = f.root
	}
	v, err := profileapply.Load(scope)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	s := server.New(v.Config, addr, "", "")
	if project {
		s = server.NewProject(v.Config, v.Project, f.root, addr, "", "")
	}
	ready, done := make(chan struct{}), make(chan error, 1)
	s.SetOnReady(func() { close(ready) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	go func() { done <- s.StartWithContext(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error("API shutdown:", err)
		}
	})
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("API startup timeout")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	return func(method, path, body string) (int, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, "http://"+addr+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, data
	}
}

func TestProfiles_DefinitionCLIAPIParity(t *testing.T) {
	steps := []struct {
		cli                []string
		method, path, body string
		refused            bool
	}{
		{[]string{"create", "draft"}, http.MethodPost, "/api/profiles", `{"name":"draft"}`, false},
		{[]string{"edit", "draft", "--description", "Focus", "--add-include", "alpha"}, http.MethodPut, "/api/profiles/draft", `{"description":"Focus","skills":{"include":["alpha"]}}`, false},
		{[]string{"add", "draft", "beta"}, http.MethodPost, "/api/profiles/draft/skills", `{"add":["beta"]}`, false},
		{[]string{"remove", "draft", "alpha"}, http.MethodPost, "/api/profiles/draft/skills", `{"remove":["alpha"]}`, false},
		{[]string{"rename", "draft", "saved"}, http.MethodPut, "/api/profiles/draft", `{"name":"saved"}`, false},
		{[]string{"edit", "a", "--remove-include", "alpha", "--add-include", "beta", "--add-include", "missing-*"}, http.MethodPut, "/api/profiles/a", `{"skills":{"include":["beta","missing-*"]}}`, false},
		{[]string{"rename", "a", "renamed"}, http.MethodPut, "/api/profiles/a", `{"name":"renamed"}`, false},
		{[]string{"delete", "renamed"}, http.MethodDelete, "/api/profiles/renamed", "", true},
		{[]string{"create", "saved"}, http.MethodPost, "/api/profiles", `{"name":"saved"}`, true},
		{[]string{"delete", "saved"}, http.MethodDelete, "/api/profiles/saved", "", false},
	}
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			var configs, states []string
			var selected, unmatched []string
			for _, api := range []bool{false, true} {
				f := newProfileFixture(t, project, "copy")
				f.run("profile", "use", "a", "--no-sync", "--json").AssertSuccess(t)
				f.sb.WriteFile(f.target+"/authored.txt", "definitions never sync")
				sourceBefore, targetBefore := profileTwinTree(t, f.source), profileTwinTree(t, f.target)
				statePath, _, err := config.ProfilePaths(f.config)
				if err != nil {
					t.Fatal(err)
				}
				// Only these fixture path identities differ. Config structure,
				// comments, selection order and every other byte must agree.
				paths := strings.NewReplacer(f.source, "<source>", f.target, "<target>", f.root, "<root>", f.sb.Home, "<home>")
				var request func(string, string, string) (int, []byte)
				if api {
					request = profileDefinitionAPI(t, f, project)
				}
				for i, step := range steps {
					before, beforeState := f.sb.ReadFile(f.config), f.sb.ReadFile(statePath)
					if api {
						status, body := request(step.method, step.path, step.body)
						var out struct {
							NeedsSync bool   `json:"needsSync"`
							Error     string `json:"error"`
							Code      string `json:"error_code"`
						}
						if err := json.Unmarshal(body, &out); err != nil || (!step.refused && (status != http.StatusOK || out.NeedsSync != (i == 5))) || (step.refused && ((status != http.StatusBadRequest && status != http.StatusConflict) || out.Error == "" || out.Code == "")) {
							t.Fatalf("API step %v: %d %s %v", step.cli, status, body, err)
						}
					} else {
						out := f.run(append([]string{"profile"}, step.cli...)...)
						if step.refused {
							out.AssertFailure(t)
						} else {
							out.AssertSuccess(t)
						}
					}
					raw, state := f.sb.ReadFile(f.config), f.sb.ReadFile(statePath)
					if step.refused && (raw != before || state != beforeState) {
						t.Fatal("refusal changed config or local state", step.cli)
					}
					if !maps.Equal(sourceBefore, profileTwinTree(t, f.source)) || !maps.Equal(targetBefore, profileTwinTree(t, f.target)) {
						t.Fatal("definition transport changed source/target bytes or topology", step.cli)
					}
					if !api {
						configs, states = append(configs, paths.Replace(raw)), append(states, state)
					} else if paths.Replace(raw) != configs[i] || state != states[i] {
						t.Fatalf("CLI/API differ after %v: config %q/%q; state %q/%q", step.cli, configs[i], paths.Replace(raw), states[i], state)
					}
				}
				if !api {
					out := f.run("profile", "show", "renamed", "--json")
					out.AssertSuccess(t)
					var rows struct{ Profiles []struct{ Selected, Unmatched []string } }
					if err := json.Unmarshal([]byte(out.Stdout), &rows); err != nil || len(rows.Profiles) != 1 {
						t.Fatal("CLI preview:", out.Stdout, err)
					}
					selected, unmatched = rows.Profiles[0].Selected, rows.Profiles[0].Unmatched
				} else {
					before, state := f.sb.ReadFile(f.config), f.sb.ReadFile(statePath)
					status, body := request(http.MethodPost, "/api/profiles/preview", `{"skills":{"include":["beta","missing-*"]}}`)
					var out struct{ Selected, Unmatched []string }
					if err := json.Unmarshal(body, &out); err != nil || status != http.StatusOK || !slices.Equal(out.Selected, selected) || !slices.Equal(out.Unmatched, unmatched) || f.sb.ReadFile(f.config) != before || f.sb.ReadFile(statePath) != state {
						t.Fatalf("CLI/API read-only preview differs: %d %s %v", status, body, err)
					}
				}
			}
		})
	}
}
