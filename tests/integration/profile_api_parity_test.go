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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/profileapply"
	"skillshare/internal/server"
	ssync "skillshare/internal/sync"
)

func profileTwinAPI(t *testing.T, f profileFixture, project bool) (int, []byte) {
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
	var s *server.Server
	if project {
		s = server.NewProject(v.Config, v.Project, f.root, addr, "", "")
	} else {
		s = server.New(v.Config, addr, "", "")
	}
	ready, done := make(chan struct{}), make(chan error, 1)
	s.SetOnReady(func() { close(ready) })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	go func() {
		defer close(done)
		done <- s.StartWithContext(ctx)
	}()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error("API shutdown:", err)
		}
	}()
	select {
	case <-ready:
	case err := <-done:
		t.Fatal("API startup:", err)
	case <-ctx.Done():
		t.Fatal("API startup timeout")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	show := f.run("profile", "show", "a", "--json")
	show.AssertSuccess(t)
	var expected struct {
		Profiles []struct{ Selected []string }
	}
	if err := json.Unmarshal([]byte(show.Stdout), &expected); err != nil || len(expected.Profiles) != 1 {
		t.Fatal("CLI show:", show.Stdout, err)
	}
	list, err := client.Get("http://" + addr + "/api/profiles")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Profiles []struct {
			Name     string
			Selected int
		}
	}
	err = json.NewDecoder(list.Body).Decode(&catalog)
	_ = list.Body.Close()
	if err != nil || list.StatusCode != http.StatusOK {
		t.Fatal("API list:", list.StatusCode, err)
	}
	found := false
	for _, row := range catalog.Profiles {
		if row.Name == "a" {
			found = row.Selected == len(expected.Profiles[0].Selected)
		}
	}
	if !found {
		t.Fatal("API desired count disagrees with CLI show", catalog)
	}
	resp, err := client.Post("http://"+addr+"/api/profiles/active", "application/json", strings.NewReader(`{"name":"a"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, body
}

// Compare every path, mode, link and file. Only the manifest's wall-clock
// updated_at is omitted; source mtimes are pinned identically on both twins.
func profileTwinTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += ":" + link
		} else if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if entry.Name() == ssync.ManifestFile {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(data, &fields); err != nil {
					return err
				}
				delete(fields, "updated_at")
				data, err = json.Marshal(fields)
				if err != nil {
					return err
				}
			}
			value += ":" + string(data)
		}
		tree[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestProfiles_CLIAPIParity(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, outcome := range []string{"complete", "refused", "incomplete"} {
			t.Run(fmt.Sprintf("project=%v/%s", project, outcome), func(t *testing.T) {
				var cliState string
				var cliTree map[string]string
				for _, api := range []bool{false, true} {
					f := newProfileFixture(t, project, "copy")
					fixed := time.Unix(1700000000, 0)
					for _, name := range []string{"alpha", "beta"} {
						for _, path := range []string{filepath.Join(f.source, name, "SKILL.md"), filepath.Join(f.source, name)} {
							if err := os.Chtimes(path, fixed, fixed); err != nil {
								t.Fatal(err)
							}
						}
					}
					f.run("profile", "use", "all", "--json").AssertSuccess(t)
					if outcome == "refused" {
						f.sb.WriteFile(filepath.Join(f.target, "beta", "SKILL.md"), "authored departure")
					} else if outcome == "incomplete" {
						bad := filepath.Join(f.sb.Home, "bad")
						if err := os.Symlink(filepath.Join(f.sb.Home, "missing", "dest"), bad); err != nil {
							t.Fatal(err)
						}
						row := fmt.Sprintf("  bad:\n    skills:\n      path: %s\n      mode: copy\n", bad)
						if project {
							row = fmt.Sprintf("  - name: bad\n    skills:\n      path: %s\n      mode: copy\n", bad)
						}
						f.sb.WriteFile(f.config, strings.Replace(f.sb.ReadFile(f.config), "profiles:\n", row+"profiles:\n", 1))
					}
					statePath, _, err := config.ProfilePaths(f.config)
					if err != nil {
						t.Fatal(err)
					}
					beforeState, beforeTree := f.sb.ReadFile(statePath), profileTwinTree(t, f.target)
					if api {
						status, body := profileTwinAPI(t, f, project)
						want := http.StatusOK
						if outcome == "refused" {
							want = http.StatusConflict
						}
						var out struct {
							profileapply.Result
							Code string `json:"error_code"`
						}
						if err := json.Unmarshal(body, &out); err != nil || status != want || (outcome == "refused" && out.Code != "profile_blocked") || (outcome != "refused" && (out.Complete != (outcome == "complete") || out.Selection.Pending != (outcome == "incomplete"))) {
							t.Fatalf("API outcome: %d %s %v", status, body, err)
						}
					} else {
						out := f.run("profile", "use", "a", "--json")
						if outcome == "complete" {
							out.AssertSuccess(t)
						} else {
							out.AssertFailure(t)
						}
					}
					state, tree := f.sb.ReadFile(statePath), profileTwinTree(t, f.target)
					if outcome == "incomplete" {
						link, err := os.Readlink(filepath.Join(f.sb.Home, "bad"))
						if err != nil || link != filepath.Join(f.sb.Home, "missing", "dest") {
							t.Fatal("failed target link topology changed", link, err)
						}
						tree["<bad-target>"] = "link:missing/dest"
					}
					if outcome == "refused" && (state != beforeState || !maps.Equal(tree, beforeTree)) {
						t.Fatal("refused twin changed state/target bytes or topology")
					}
					if !api {
						cliState, cliTree = state, tree
					} else if state != cliState || !maps.Equal(tree, cliTree) {
						t.Fatalf("CLI/API differ: state %q/%q; trees %v/%v", cliState, state, cliTree, tree)
					}
				}
			})
		}
	}
}
