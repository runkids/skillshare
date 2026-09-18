//go:build !online

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/codexskills"
	"skillshare/internal/testutil"
)

// A protocol fixture makes the CLI test offline and ensures it cannot touch a
// real Codex installation. It persists enabled state across server processes.
func TestDedupCodexServer(t *testing.T) {
	statePath := os.Getenv("SKILLSHARE_DEDUP_TEST_STATE")
	if statePath == "" {
		return
	}
	decode, encode := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	for {
		var req struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := decode.Decode(&req); err != nil {
			if err == io.EOF {
				os.Exit(0)
			}
			os.Exit(2)
		}
		var result any
		switch req.Method {
		case "initialized":
			continue
		case "initialize":
			result = map[string]string{"codexHome": filepath.Dir(statePath)}
		case "skills/list", "skills/config/write":
			data, err := os.ReadFile(statePath)
			if err != nil {
				os.Exit(3)
			}
			var skills []codexskills.Skill
			if json.Unmarshal(data, &skills) != nil {
				os.Exit(4)
			}
			if req.Method == "skills/list" {
				result = map[string]any{"data": []any{map[string]any{"skills": skills, "errors": []any{}}}}
			} else {
				var edit struct {
					Path    string
					Enabled bool
				}
				if json.Unmarshal(req.Params, &edit) != nil || edit.Enabled {
					os.Exit(5)
				}
				found := false
				for i := range skills {
					if skills[i].Path == edit.Path {
						skills[i].Enabled = false
						found = true
					}
				}
				if !found {
					os.Exit(6)
				}
				data, _ = json.Marshal(skills)
				if os.WriteFile(statePath, data, 0600) != nil {
					os.Exit(7)
				}
				f, err := os.OpenFile(filepath.Join(filepath.Dir(statePath), "config.toml"), os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					os.Exit(8)
				}
				_, err = fmt.Fprintf(f, "\n[[skills.config]]\npath = %q\nenabled = false\n", edit.Path)
				if err != nil || f.Close() != nil {
					os.Exit(9)
				}
				result = map[string]bool{"effectiveEnabled": false}
			}
		default:
			// In particular, thread/start and turn/start are forbidden here.
			os.Exit(10)
		}
		if encode.Encode(map[string]any{"id": req.ID, "result": result}) != nil {
			os.Exit(11)
		}
	}
}

func TestDedupCodex_PreviewApplyAndRepeat(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable shim")
	}
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	var skills []codexskills.Skill
	var skillFiles []string
	for _, dir := range []string{
		filepath.Join(sb.SourcePath, "gem"),
		filepath.Join(sb.SourcePath, "archive/references/gem"),
		filepath.Join(sb.Home, ".codex/skills/gem"),
		filepath.Join(sb.Home, ".codex/skills/archive/references/gem"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(path, []byte("---\nname: gem\ndescription: Find gems.\n---\nFind gems.\n"), 0644); err != nil {
			t.Fatal(err)
		}
		path, err := filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		skills = append(skills, codexskills.Skill{Name: "gem", Path: path, Scope: "user", Enabled: true})
		skillFiles = append(skillFiles, path)
	}
	codexDir := filepath.Join(sb.Home, ".codex")
	configPath := filepath.Join(codexDir, "config.toml")
	original := []byte("# unrelated user setting\nmodel = \"test-model\"\n")
	if err := os.WriteFile(configPath, original, 0600); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(codexDir, "state.json")
	state, _ := json.Marshal(skills)
	if err := os.WriteFile(statePath, state, 0600); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(sb.Root, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	shim := []byte("#!/bin/sh\nexec \"$SKILLSHARE_DEDUP_TEST_BINARY\" -test.run=TestDedupCodexServer\n")
	if err := os.WriteFile(filepath.Join(binDir, "codex"), shim, 0755); err != nil {
		t.Fatal(err)
	}
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"PATH": binDir + string(os.PathListSeparator) + os.Getenv("PATH"), "SKILLSHARE_DEDUP_TEST_BINARY": testBinary, "SKILLSHARE_DEDUP_TEST_STATE": statePath}
	readPlan := func(result *testutil.Result) codexskills.Plan {
		t.Helper()
		result.AssertSuccess(t)
		if result.Stderr != "" {
			t.Fatalf("unexpected stderr: %s", result.Stderr)
		}
		var plan codexskills.Plan
		if err := json.Unmarshal([]byte(result.Stdout), &plan); err != nil {
			t.Fatalf("invalid JSON: %s: %v", result.Stdout, err)
		}
		return plan
	}
	preview := readPlan(sb.RunCLIEnv(env, "dedup", "codex", "--json"))
	if len(preview.Groups) != 1 || preview.Groups[0].Keep != skills[0].Path || len(preview.Groups[0].Disable) != 3 || preview.Backup != "" {
		t.Fatalf("preview: %+v", preview)
	}
	afterPreview, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterPreview, original) {
		t.Fatal("preview wrote config")
	}
	afterState, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterState, state) {
		t.Fatal("preview changed enabled state")
	}
	applied := readPlan(sb.RunCLIEnv(env, "dedup", "codex", "--apply", "--json"))
	if !applied.Verified || len(applied.Applied) != 3 || applied.Backup == "" {
		t.Fatalf("apply: %+v", applied)
	}
	backup, err := os.ReadFile(applied.Backup)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(backup, original) {
		t.Fatal("backup changed unrelated settings")
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(config, original) || strings.Count(string(config), "enabled = false") != 3 {
		t.Fatalf("config writes: %s", config)
	}
	again := readPlan(sb.RunCLIEnv(env, "dedup", "codex", "--apply", "--json"))
	if len(again.Groups) != 0 || len(again.Applied) != 0 || again.Backup != "" || !again.Verified {
		t.Fatalf("repeat not a no-op: %+v", again)
	}
	for _, path := range skillFiles {
		data, err := os.ReadFile(path)
		if err != nil || !bytes.Contains(data, []byte("Find gems.")) {
			t.Fatalf("skill file altered: %s %v", path, err)
		}
	}
}

func TestDedupCodex_ArgumentErrorsAreJSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, args := range [][]string{{"dedup", "other", "--json"}, {"dedup", "codex", "--force", "--json"}} {
		result := sb.RunCLI(args...)
		var output map[string]string
		if result.ExitCode == 0 || json.Unmarshal([]byte(result.Stdout), &output) != nil || output["error"] == "" || result.Stderr != "" {
			t.Fatalf("bad error contract: %+v", result)
		}
	}
}
