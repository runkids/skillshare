//go:build !online

package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"skillshare/internal/testutil"
	"skillshare/internal/utils"
)

// projectLockPath is where projects.LockConfig locks the sandbox config: the
// state dir, keyed by the resolved config file.
func projectLockPath(t *testing.T, sb *testutil.Sandbox) string {
	t.Helper()
	dest, err := utils.ConfigWritePath(sb.ConfigPath, false)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(sb.Home, ".local", "state", "skillshare", "projects")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(filepath.Clean(dest)))
	return filepath.Join(dir, hex.EncodeToString(sum[:8])+".lock")
}

func TestProject_AddRerunAndSync(t *testing.T) {
	sb, root := projectFixture(t)
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha\n"})
	args := []string{"project", "add", root, "--target", "claude", "--sync", "--json"}
	sb.RunCLI(args...).AssertSuccess(t)
	before, _ := os.ReadFile(sb.ConfigPath)
	if err := os.Remove(filepath.Join(root, ".claude", "skills", "alpha")); err != nil {
		t.Fatal(err)
	}
	previewTree := projectTree(t, sb.Home)
	preview := sb.RunCLI(append(args, "--dry-run")...)
	preview.AssertSuccess(t)
	payload := projectJSON(t, preview)
	if payload["unchanged"] != true || payload["applied"] != false || payload["sync"] == nil || payload["registration"] != "kept" || payload["detach"] == nil {
		t.Fatalf("rerun preview: %s", preview.Stdout)
	}
	if !reflect.DeepEqual(previewTree, projectTree(t, sb.Home)) {
		t.Fatal("rerun preview wrote files")
	}
	result := sb.RunCLI(args...)
	result.AssertSuccess(t)
	got := projectJSON(t, result)
	if got["unchanged"] != true || got["applied"] != false || got["sync"] == nil || got["registration"] != "kept" || got["detach"] == nil {
		t.Fatalf("rerun: %s", result.Stdout)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "alpha", "SKILL.md")); err != nil {
		t.Fatal("rerun skipped sync", err)
	}
	if after, _ := os.ReadFile(sb.ConfigPath); !bytes.Equal(before, after) {
		t.Fatal("rerun rewrote config")
	}
	result = sb.RunCLI("project", "add", root, "--target", "codex", "--json")
	if result.ExitCode == 0 || !strings.Contains(result.Stdout, "project edit") {
		t.Fatalf("different rerun: %+v", result)
	}
}

func TestProject_MutationWaitsAndReloads(t *testing.T) {
	sb, root := projectFixture(t)
	gate := flock.New(projectLockPath(t, sb))
	if err := gate.Lock(); err != nil {
		t.Fatal(err)
	}
	defer gate.Close()
	done := make(chan *testutil.Result, 1)
	go func() { done <- sb.RunCLI("project", "add", root, "--target", "claude", "--json") }()
	select {
	case r := <-done:
		t.Fatalf("mutation bypassed lock: %+v", r)
	case <-time.After(150 * time.Millisecond):
	}
	other := filepath.Join(sb.Home, "other")
	if err := os.Mkdir(other, 0755); err != nil {
		t.Fatal(err)
	}
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nprojects:\n  " + filepath.ToSlash(other) + ":\n    targets: [codex]\n    skills: {}\n")
	if err := gate.Unlock(); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-done:
		r.AssertSuccess(t)
	case <-time.After(5 * time.Second):
		t.Fatal("mutation did not complete")
	}
	r := sb.RunCLI("project", "list", "--json")
	r.AssertSuccess(t)
	if len(projectJSON(t, r)["projects"].([]any)) != 2 {
		t.Fatal("lost concurrent registration", r.Stdout)
	}
	// An edit must patch the state reloaded after acquisition, preserving other fields.
	gate = flock.New(projectLockPath(t, sb))
	if err := gate.Lock(); err != nil {
		t.Fatal(err)
	}
	defer gate.Close()
	go func() { done <- sb.RunCLI("project", "edit", root, "--skills-mode", "copy", "--json") }()
	select {
	case r := <-done:
		t.Fatalf("edit bypassed lock: %+v", r)
	case <-time.After(150 * time.Millisecond):
	}
	raw, _ := os.ReadFile(sb.ConfigPath)
	raw = bytes.Replace(raw, []byte("targets: [claude]"), []byte("targets: [claude, codex]"), 1)
	// The serializer uses block sequences; add agents via an independent config edit.
	raw = bytes.Replace(raw, []byte("skills: {}"), []byte("agents: {}\n    skills: {}"), 1)
	if err := os.WriteFile(sb.ConfigPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	if err := gate.Unlock(); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-done:
		r.AssertSuccess(t)
	case <-time.After(5 * time.Second):
		t.Fatal("edit did not complete")
	}
	r = sb.RunCLI("project", "show", root, "--json")
	r.AssertSuccess(t)
	if projectJSON(t, r)["project"].(map[string]any)["agents"] == nil {
		t.Fatal("patch lost concurrent agents setting", r.Stdout)
	}
}

func TestProject_PartialEditSaveFailure(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("fixture needs Unix directory permissions")
	}
	for _, jsonOutput := range []bool{false, true} {
		t.Run(map[bool]string{false: "text", true: "json"}[jsonOutput], func(t *testing.T) {
			sb, root := projectFixture(t)
			sb.RunCLI("project", "add", root, "--target", "claude", "--json").AssertSuccess(t)
			target := filepath.Join(root, ".claude", "skills")
			if err := os.MkdirAll(target, 0755); err != nil {
				t.Fatal(err)
			}
			manifest := filepath.Join(target, ".skillshare-manifest.json")
			if err := os.WriteFile(manifest, []byte("{}"), 0644); err != nil {
				t.Fatal(err)
			}
			// The lock lives in the state dir, so only candidate temporary-file creation fails.
			configDir := filepath.Dir(sb.ConfigPath)
			if err := os.Chmod(configDir, 0555); err != nil {
				t.Fatal(err)
			}
			defer os.Chmod(configDir, 0755)
			args := []string{"project", "edit", root, "--no-skills"}
			if jsonOutput {
				args = append(args, "--json")
			}
			r := sb.RunCLI(args...)
			if r.ExitCode == 0 {
				t.Fatal("save unexpectedly succeeded")
			}
			if _, err := os.Stat(manifest); !os.IsNotExist(err) {
				t.Fatalf("fixture failed before detach: %v %+v", err, r)
			}
			if jsonOutput {
				got := projectJSON(t, r)
				if got["applied"] != false || got["registration"] != "kept" || got["detach"] == nil {
					t.Fatalf("partial result: %s", r.Stdout)
				}
			} else if !strings.Contains(r.Stdout, "kept") || !strings.Contains(r.Stdout, "Removed") {
				t.Fatalf("partial text lost work: %+v", r)
			}
			entries := projectLogs(t, sb)
			if len(entries) != 2 || entries[1]["status"] != "partial" {
				t.Fatalf("partial edit not logged once: %+v", entries)
			}
		})
	}
}

func TestProject_SyncFailureExitStatus(t *testing.T) {
	for _, tc := range []struct {
		name, part string
		conflict   bool
	}{
		{"invalid config", "config", false},
		{"symlink conflict", "skill", true},
	} {
		for _, action := range []string{"sync", "add", "edit"} {
			t.Run(tc.name+"/"+action, func(t *testing.T) {
				sb, root := projectFixture(t)
				sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha\n"})
				sb.RunCLI("project", "add", root, "--target", "claude,codex", "--skills-mode", "symlink", "--json").AssertSuccess(t)
				target := filepath.Join(root, ".claude", "skills")
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					t.Fatal(err)
				}
				if tc.conflict {
					wrong := filepath.Join(sb.Home, "wrong-source")
					if err := os.MkdirAll(wrong, 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(wrong, target); err != nil {
						t.Skipf("fixture requires directory symlink: %v", err)
					}
				} else if err := os.WriteFile(target, []byte("blocked"), 0644); err != nil {
					t.Fatal(err)
				}
				args := []string{"project", action, root, "--json"}
				if action != "sync" {
					args = append(args, "--sync")
				}
				if action == "add" {
					args = append(args, "--target", "claude,codex", "--skills-mode", "symlink")
				}
				for _, preview := range []bool{true, false} {
					t.Run(map[bool]string{true: "preview", false: "apply"}[preview], func(t *testing.T) {
						before := projectTree(t, sb.Home)
						command := append([]string{}, args...)
						if preview {
							command = append(command, "--dry-run")
						}
						r := sb.RunCLI(command...)
						wantFailure := !preview || !tc.conflict
						if (r.ExitCode != 0) != wantFailure {
							t.Errorf("exit = %d, want failure %v: %s", r.ExitCode, wantFailure, r.Stdout)
						}
						got := projectJSON(t, r)["sync"].(map[string]any)
						failed, results := got["failed"].([]any), got["results"].([]any)
						if len(failed) != 1 || len(results) != 1 {
							t.Fatalf("lost failure or completed target: %s", r.Stdout)
						}
						failure := failed[0].(map[string]any)
						conflict, _ := failure["conflict"].(bool)
						if failure["target"] != "app@claude" || failure["part"] != tc.part || conflict != tc.conflict || results[0].(map[string]any)["target"] != "app@codex" {
							t.Errorf("incorrect failure classification or completed target: %s", r.Stdout)
						}
						if preview && !reflect.DeepEqual(before, projectTree(t, sb.Home)) {
							t.Fatal("preview wrote state")
						}
					})
				}
			})
		}
	}
}

func TestProject_AddExistingMissingDirectory(t *testing.T) {
	sb, root := projectFixture(t)
	args := []string{"project", "add", root, "--target", "claude", "--json"}
	sb.RunCLI(args...).AssertSuccess(t)
	before, _ := os.ReadFile(sb.ConfigPath)
	if err := os.Rename(root, root+"-moved"); err != nil {
		t.Fatal(err)
	}
	r := sb.RunCLI(args...)
	r.AssertFailure(t)
	if !strings.Contains(r.Stdout, "folder not found") {
		t.Fatal("add accepted a missing directory:", r.Stdout)
	}
	if after, _ := os.ReadFile(sb.ConfigPath); !bytes.Equal(before, after) {
		t.Fatal("failed re-add changed registration")
	}
}

func TestProject_ReAddValidationErrors(t *testing.T) {
	sb, root := projectFixture(t)
	sb.RunCLI("project", "add", root, "--target", "claude", "--json").AssertSuccess(t)
	r := sb.RunCLI("project", "add", root, "--target", "claude", "--skills-mode", "bogus", "--json")
	r.AssertFailure(t)
	if !strings.Contains(r.Stdout, "invalid sync mode") {
		t.Fatal("re-add hides validation error:", r.Stdout)
	}
}

type projectPromptCapture struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	ready chan struct{}
	once  sync.Once
}

func (c *projectPromptCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n, err := c.buf.Write(p)
	if strings.Contains(c.buf.String(), "[y/N]") {
		c.once.Do(func() { close(c.ready) })
	}
	return n, err
}
func (c *projectPromptCapture) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

func TestProject_RemovePromptDoesNotHoldLock(t *testing.T) {
	for _, change := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "changed registration"}[change], func(t *testing.T) {
			sb, root := projectFixture(t)
			sb.RunCLI("project", "add", root, "--target", "claude", "--json").AssertSuccess(t)
			cmd := exec.Command(sb.BinaryPath, "project", "remove", root)
			cmd.Env = append(os.Environ(), "HOME="+sb.Home, "SKILLSHARE_CONFIG="+sb.ConfigPath)
			output := &projectPromptCapture{ready: make(chan struct{})}
			cmd.Stdout = output
			cmd.Stderr = output
			input, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill() })
			select {
			case <-output.ready:
			case <-time.After(10 * time.Second):
				t.Fatal("remove never reached confirmation:", output.String())
			}
			gate := flock.New(projectLockPath(t, sb))
			defer gate.Close()
			ok, err := gate.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				_, _ = input.Write([]byte("n\n"))
				_ = cmd.Wait()
				t.Fatal("unanswered remove holds the mutation lock")
			}
			if change {
				sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nprojects:\n  " + filepath.ToSlash(root) + ":\n    name: changed\n    targets: [claude]\n    skills: {}\n")
			}
			if err := gate.Unlock(); err != nil {
				t.Fatal(err)
			}
			answer := "n\n"
			if change {
				answer = "y\n"
			}
			if _, err := input.Write([]byte(answer)); err != nil {
				t.Fatal(err)
			}
			err = cmd.Wait()
			if change && (err == nil || !strings.Contains(output.String(), "changed while confirming")) {
				t.Fatal("removed changed registration:", output.String(), err)
			}
			if !change && err != nil {
				t.Fatal("cancel failed:", output.String(), err)
			}
			r := sb.RunCLI("project", "show", root, "--json")
			r.AssertSuccess(t)
			if change && projectJSON(t, r)["project"].(map[string]any)["name"] != "changed" {
				t.Fatal("confirmation overwrote new registration")
			}
		})
	}
}
