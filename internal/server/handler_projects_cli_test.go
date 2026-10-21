package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
	"skillshare/internal/testutil"
	"skillshare/internal/utils"
)

// This crosses the real CLI and dashboard adapters; use an explicitly rebuilt binary.
func TestProjectCLI_ConvertedDisabledSkills(t *testing.T) {
	if os.Getenv("SKILLSHARE_TEST_BINARY") == "" {
		t.Skip("requires SKILLSHARE_TEST_BINARY")
	}
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha\n"})
	root := filepath.Join(sb.Home, "app")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".claude", "skills")
	sb.RunCLI("target", "add", "app-claude", path, "--no-skills").AssertSuccess(t)
	cfg, err := config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, "127.0.0.1:0", "", "")
	body, _ := json.Marshal(map[string]string{"root": root})
	rr := projectRequest(t, s, http.MethodPost, "/api/projects/convert", string(body))
	if rr.Code != http.StatusOK {
		t.Fatalf("convert: %d %s", rr.Code, rr.Body)
	}
	before, err := os.ReadFile(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"list", "show"} {
		args := []string{"project", action}
		if action == "show" {
			args = append(args, "app")
		}
		r := sb.RunCLI(args...)
		r.AssertSuccess(t)
		r.AssertRowContains(t, "Skills", "off")
	}
	r := sb.RunCLI("project", "edit", "app", "--skills", "--dry-run", "--sync", "--json")
	r.AssertSuccess(t)
	var preview struct {
		Project struct {
			Skills map[string]any `json:"skills"`
		} `json:"project"`
		Applied  bool     `json:"applied"`
		Affected []string `json:"affected"`
		Sync     struct {
			Results []any `json:"results"`
		} `json:"sync"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Project.Skills) != 3 {
		t.Errorf("skills JSON contract changed: %v", preview.Project.Skills)
	}
	if preview.Applied || len(preview.Affected) != 1 || len(preview.Sync.Results) != 1 {
		t.Errorf("enable preview: %s", r.Stdout)
	}
	after, err := os.ReadFile(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("preview persisted config")
	}
	if _, err := os.Lstat(filepath.Join(path, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("preview wrote skill: %v", err)
	}
	r = sb.RunCLI("project", "edit", "app", "--skills", "--sync", "--json")
	r.AssertSuccess(t)
	cfg, err = config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Projects[root].Skills.IsEnabled() {
		t.Error("explicit --skills kept converted resource disabled")
	}
	if _, err := os.Lstat(filepath.Join(path, "alpha", "SKILL.md")); err != nil {
		t.Errorf("enabled sync did not send alpha: %v", err)
	}
	sb.RunCLI("project", "show", "app").AssertRowContains(t, "Skills", "on")
}

func TestProjectCLI_EditDetachesDashboardDisabledTarget(t *testing.T) {
	if os.Getenv("SKILLSHARE_TEST_BINARY") == "" {
		t.Skip("requires SKILLSHARE_TEST_BINARY")
	}
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	const content = "---\nname: alpha\n---\n# Alpha\n"
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": content})
	root := filepath.Join(sb.Home, "app")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	sb.RunCLI("project", "add", root, "--target", "claude,codex", "--skills-mode", "merge", "--sync", "--json").AssertSuccess(t)
	cfg, err := config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, "127.0.0.1:0", "", "")
	body, err := json.Marshal(map[string]any{"root": root, "targets": []string{"claude", "codex"}, "skills": nil})
	if err != nil {
		t.Fatal(err)
	}
	rr := projectRequest(t, s, http.MethodPut, "/api/projects", string(body))
	if rr.Code != http.StatusOK {
		t.Fatalf("dashboard disable: %d %s", rr.Code, rr.Body)
	}
	departing := filepath.Join(root, ".claude", "skills")
	survivor := filepath.Join(root, ".agents", "skills")
	for _, path := range []string{departing, survivor} {
		if _, err := os.Stat(filepath.Join(path, ssync.ManifestFile)); err != nil {
			t.Fatalf("dashboard removed manifest: %v", err)
		}
		if !utils.IsSymlinkOrJunction(filepath.Join(path, "alpha")) {
			t.Fatalf("dashboard detached skill in %s", path)
		}
	}
	before, err := os.ReadFile(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	// Re-enable skills for the surviving Codex target while dropping Claude.
	args := []string{"project", "edit", root, "--target", "codex", "--skills", "--json"}
	preview := sb.RunCLI(append(args, "--dry-run")...)
	preview.AssertSuccess(t)
	if after, err := os.ReadFile(sb.ConfigPath); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("preview changed config: %v", err)
	}
	for _, path := range []string{departing, survivor} {
		if _, err := os.Stat(filepath.Join(path, ssync.ManifestFile)); err != nil {
			t.Fatalf("preview removed manifest: %v", err)
		}
		if !utils.IsSymlinkOrJunction(filepath.Join(path, "alpha")) {
			t.Fatalf("preview detached skill in %s", path)
		}
	}
	result := sb.RunCLI(args...)
	result.AssertSuccess(t)
	var payload struct{ Detach ssync.DetachReport }
	if err := json.Unmarshal([]byte(result.Stdout), &payload); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(departing, ssync.ManifestFile)
	if _, err := os.Stat(manifest); !os.IsNotExist(err) {
		t.Errorf("departing target retained manifest: %v", err)
	}
	if utils.IsSymlinkOrJunction(filepath.Join(departing, "alpha")) {
		t.Error("departing skill still follows source")
	}
	if got, err := os.ReadFile(filepath.Join(departing, "alpha", "SKILL.md")); err != nil || string(got) != content {
		t.Errorf("departing skill was not copied back: %q %v", got, err)
	}
	if !slices.Contains(payload.Detach.Removed, manifest) || !slices.Contains(payload.Detach.CopiedBack, filepath.Join(departing, "alpha")) {
		t.Errorf("completed detach missing from result: %s", result.Stdout)
	}
	if _, err := os.Stat(filepath.Join(survivor, ssync.ManifestFile)); err != nil {
		t.Fatalf("surviving target lost manifest: %v", err)
	}
	if !utils.IsSymlinkOrJunction(filepath.Join(survivor, "alpha")) {
		t.Fatal("surviving target was detached")
	}
}
