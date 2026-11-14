//go:build !online

package integration

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/testutil"
)

// Hosted test children inherit only fixture config, never native-agent accounts.
func hostedCLI(t *testing.T, sb *testutil.Sandbox, dir string, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, sb.BinaryPath, append([]string{"mcp", "serve"}, args...)...)
	cmd.Dir = dir
	for _, entry := range os.Environ() {
		k, _, _ := strings.Cut(entry, "=")
		if slices.Contains([]string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR", "SKILLSHARE_MCP_TOKEN", "WRITING_TOKEN", "CODING_TOKEN"}, k) {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "WRITING_TOKEN=fixture-writing", "CODING_TOKEN=fixture-coding")
	return cmd
}

const hostedDefinitions = "profiles:\n  writing:\n    skills:\n      include: [alpha, shared]\n  coding:\n    skills:\n      include: [beta, shared]\n  unbound:\n    skills:\n      include: [beta]\n"

func hostedCLIFixture(t *testing.T, project bool) (*testutil.Sandbox, string, string, []string) {
	t.Helper()
	sb := testutil.NewSandbox(t)
	t.Cleanup(sb.Cleanup)
	dir, path, source, mode := sb.Root, sb.ConfigPath, sb.SourcePath, "-g"
	targets := "targets:\n  claude:\n    path: " + filepath.Join(sb.Home, "uncreated-target") + "\n    exclude: [shared]\n"
	if project {
		dir = sb.SetupProjectDir()
		path = config.ProjectConfigPath(dir)
		source = filepath.Join(dir, ".skillshare", "skills")
		mode = "-p"
		sb.WriteFile(sb.ConfigPath, "source: "+sb.SourcePath+"\ntargets:\n  claude:\n    path: "+filepath.Join(sb.Home, "uncreated-target")+"\n    exclude: [shared]\n")
		targets = "targets:\n  - name: claude\n    path: .claude/uncreated-skills\n    skills:\n      exclude: [shared]\n"
	}
	text := "source: " + source + "\n" + targets + hostedDefinitions + "mcp:\n  serve:\n    edit:\n      skills: [shared]\n"
	sb.WriteFile(path, text)
	for _, n := range []string{"alpha", "beta", "shared"} {
		sb.WriteFile(filepath.Join(source, n, "SKILL.md"), "---\nname: "+n+"\ndescription: fixture\n---\n# "+n+"\n")
	}
	state, _, err := config.ProfilePaths(path)
	if err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(state, "[broken active state")
	// A runtime constructor would migrate this legacy metadata; hosted loads must not.
	sb.WriteFile(filepath.Join(source, "registry.yaml"), "skills: {}\n")
	if err := os.Remove(filepath.Join(sb.Home, ".local", "state", "skillshare", "logs")); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(filepath.Dir(sb.ConfigPath), "logs")
	sb.WriteFile(filepath.Join(legacy, "fixture.log"), "preserve legacy state")
	return sb, dir, path, []string{mode, "--profile-token", "writing=WRITING_TOKEN", "--profile-token", "coding=CODING_TOKEN"}
}

func TestMCPServe_HostedProfiles(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprintf("project=%t", project), func(t *testing.T) {
			sb, dir, path, bindings := hostedCLIFixture(t, project)
			before := testutil.SnapshotTree(t, false, sb.Root)
			for _, targeted := range []bool{false, true} {
				args := append(slices.Clone(bindings), "--check")
				if targeted {
					args = append(args, "--target", "claude")
				}
				cmd := hostedCLI(t, sb, dir, args...)
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				out, err := cmd.Output()
				if err != nil {
					t.Fatalf("check: %v %s", err, out)
				}
				for _, profile := range []string{"writing", "coding", "unbound"} {
					if !bytes.Contains(out, []byte(profile)) {
						t.Errorf("stdout check omitted %s: %s; stderr: %s", profile, out, &stderr)
					}
				}
				wantTwos, wantOnes := 2, 1
				if targeted {
					wantTwos, wantOnes = 0, 3
				}
				if bytes.Count(out, []byte("2 skills served")) != wantTwos || bytes.Count(out, []byte("1 skill served")) != wantOnes {
					t.Fatalf("wrong per-profile counts: %s", out)
				}
				if bytes.Contains(out, []byte("fixture-writing")) {
					t.Fatal("secret in check")
				}
			}
			cmd := hostedCLI(t, sb, dir, append(slices.Clone(bindings), "--http", "127.0.0.1:0")...)
			base, _ := startHostedCLI(t, cmd)
			client := &http.Client{Timeout: 5 * time.Second}
			var wg sync.WaitGroup
			for _, tc := range []struct{ token, own, foreign string }{{"fixture-writing", "alpha", "beta"}, {"fixture-coding", "beta", "alpha"}} {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for _, tail := range []string{"index.json", tc.own + "/SKILL.md", tc.foreign + "/SKILL.md"} {
						req, _ := http.NewRequest("GET", base+"/.well-known/agent-skills/"+tail, nil)
						req.Header.Set("Authorization", "Bearer "+tc.token)
						resp, err := client.Do(req)
						if err != nil {
							t.Error(err)
							return
						}
						data, _ := io.ReadAll(resp.Body)
						resp.Body.Close()
						want := 200
						if tail == tc.foreign+"/SKILL.md" {
							want = 404
						}
						if resp.StatusCode != want {
							t.Errorf("%s HTTP %d: %s", tail, resp.StatusCode, data)
						}
						if tail == "index.json" {
							var idx struct {
								Skills []struct {
									Name string `json:"name"`
								} `json:"skills"`
							}
							if err := json.Unmarshal(data, &idx); err != nil {
								t.Error(err)
								return
							}
							var names []string
							for _, s := range idx.Skills {
								names = append(names, s.Name)
							}
							if !slices.Equal(names, []string{tc.own, "shared"}) {
								t.Errorf("wrong account index: %s", data)
							}
						}
					}
				}()
			}
			wg.Wait()
			for _, token := range []string{"", "unknown"} {
				req, _ := http.NewRequest("GET", base+"/.well-known/agent-skills/index.json", nil)
				if token != "" {
					req.Header.Set("Authorization", "Bearer "+token)
				}
				resp, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
				if resp.StatusCode != 401 {
					t.Fatalf("unauthorized HTTP %d", resp.StatusCode)
				}
			}
			after := testutil.SnapshotTree(t, false, sb.Root)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("hosted startup/check/requests changed config, active state, metadata or targets")
			}
			// A preview may append its audit record, but must not migrate supported
			// legacy flat-target config or modify source/state before apply.
			configBefore, globalBefore := sb.ReadFile(path), sb.ReadFile(sb.ConfigPath)
			statePath, _, err := config.ProfilePaths(path)
			if err != nil {
				t.Fatal(err)
			}
			stateBefore := sb.ReadFile(statePath)
			source := sb.SourcePath
			if project {
				source = filepath.Join(dir, ".skillshare", "skills")
			}
			if err := os.Remove(filepath.Join(source, "registry.yaml")); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(source, "shared", "SKILL.md")
			old := sb.ReadFile(file)
			body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"preview_skill_edit","arguments":{"uri":"skill://shared/SKILL.md","base_revision":"sha256:%x","content":%q},%s}}`, sha256.Sum256([]byte(old)), old+"\nFixture preview.\n", serveMeta)
			req, _ := http.NewRequest("POST", base+"/", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer fixture-writing")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set("Mcp-Protocol-Version", "2026-07-28")
			req.Header.Set("Mcp-Method", "tools/call")
			req.Header.Set("Mcp-Name", "preview_skill_edit")
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 200 || !bytes.Contains(data, []byte("Preview ")) || bytes.Contains(data, []byte(`"isError":true`)) {
				t.Fatalf("preview HTTP %d: %s", resp.StatusCode, data)
			}
			if sb.ReadFile(path) != configBefore || sb.ReadFile(sb.ConfigPath) != globalBefore || sb.ReadFile(file) != old || sb.ReadFile(statePath) != stateBefore {
				t.Fatal("preview changed config, source or active state")
			}
			var audit oplog.Entry
			if err := json.Unmarshal([]byte(sb.ReadFile(filepath.Join(oplog.LogDir(path), oplog.AuditFile))), &audit); err != nil || audit.Command != "audit" || audit.Args["mode"] != "mcp serve edit" || audit.Args["name"] != "shared" {
				t.Fatalf("missing preview audit: %v %+v", err, audit)
			}

		})
	}
}

func TestMCPServe_HostedValidation(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprintf("project=%t", project), func(t *testing.T) {
			sb, dir, path, bindings := hostedCLIFixture(t, project)
			base := sb.ReadFile(path)
			for _, tc := range []struct {
				name                        string
				args                        []string
				extraEnv, configExtra, want string
			}{
				{name: "stdio", args: bindings, want: "--http or --check"},
				{name: "legacy-token", args: append(slices.Clone(bindings), "--check"), extraEnv: "SKILLSHARE_MCP_TOKEN=legacy-fixture", want: "SKILLSHARE_MCP_TOKEN"},
				{name: "duplicate-profile", args: []string{bindings[0], "--check", "--profile-token", "writing=WRITING_TOKEN", "--profile-token", "writing=CODING_TOKEN"}, want: "duplicate"},
				{name: "duplicate-credential", args: []string{bindings[0], "--check", "--profile-token", "writing=WRITING_TOKEN", "--profile-token", "coding=WRITING_TOKEN"}, want: "reuses"},
				{name: "unknown", args: []string{bindings[0], "--check", "--profile-token", "missing=WRITING_TOKEN"}, want: "missing"},
				{name: "unusable-unbound", args: append(slices.Clone(bindings), "--check"), configExtra: "  future:\n    unknown: true\n", want: "future"},
				{name: "network-tls", args: append(slices.Clone(bindings), "--http", "0.0.0.0:0"), want: "--tls-cert"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					text := base
					if tc.configExtra != "" {
						text = strings.Replace(text, "mcp:\n", tc.configExtra+"mcp:\n", 1)
					}
					sb.WriteFile(path, text)
					before := testutil.SnapshotTree(t, false, sb.Root)
					cmd := hostedCLI(t, sb, dir, tc.args...)
					if tc.extraEnv != "" {
						cmd.Env = append(cmd.Env, tc.extraEnv)
					}
					out, err := cmd.CombinedOutput()
					if err == nil || !bytes.Contains(out, []byte(tc.want)) {
						t.Fatalf("want %s rejection: %v %s", tc.want, err, out)
					}
					for _, secret := range []string{"fixture-writing", "fixture-coding", "legacy-fixture"} {
						if bytes.Contains(out, []byte(secret)) {
							t.Fatal("secret in diagnostic")
						}
					}
					if !reflect.DeepEqual(before, testutil.SnapshotTree(t, false, sb.Root)) {
						t.Fatal("rejected startup changed persisted tree")
					}
				})
			}
		})
	}
}

func startHostedCLI(t *testing.T, cmd *exec.Cmd) (string, string) {
	t.Helper()
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	scanner := bufio.NewScanner(stderr)
	var diagnostics strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(&diagnostics, line)
		if base, ok := strings.CutPrefix(line, "Serving skills over MCP at "); ok {
			return strings.TrimSuffix(base, "/"), diagnostics.String()
		}
	}
	t.Fatalf("server did not start: %v %s", scanner.Err(), &diagnostics)
	return "", ""
}

func TestMCPServe_HostedNetwork(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(fmt.Sprintf("tls=%t", secure), func(t *testing.T) {
			sb, dir, _, bindings := hostedCLIFixture(t, false)
			args := append(slices.Clone(bindings), "--http", "0.0.0.0:0")
			scheme := "http"
			if secure {
				cert, key := writeSelfSignedCert(t)
				args = append(args, "--tls-cert", cert, "--tls-key", key)
				scheme = "https"
			} else {
				args = append(args, "--http-allow-insecure")
			}
			cmd := hostedCLI(t, sb, dir, args...)
			base, diagnostics := startHostedCLI(t, cmd)
			warnings := strings.Count(diagnostics, "warning: --http-allow-insecure")
			// Listener address may be [::]; dial the same listener over loopback.
			i := strings.LastIndex(base, ":")
			base = scheme + "://127.0.0.1" + base[i:]
			transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} // fixture self-signed certificate
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
			for _, token := range []string{"", "fixture-writing", "fixture-coding"} {
				req, _ := http.NewRequest("GET", base+"/.well-known/agent-skills/index.json", nil)
				if token != "" {
					req.Header.Set("Authorization", "Bearer "+token)
				}
				resp, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
				want := 200
				if token == "" {
					want = 401
				}
				if resp.StatusCode != want {
					t.Fatalf("HTTP %d want %d", resp.StatusCode, want)
				}
			}
			wantWarnings := 1
			if secure {
				wantWarnings = 0
			}
			if warnings != wantWarnings {
				t.Fatalf("warnings %d want %d", warnings, wantWarnings)
			}
		})
	}
}
