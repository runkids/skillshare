package mcp

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const legacyConfig = `# my MCP servers
mcp:
  targets: [pi, opencode]
  directTools: search
  servers:
    ext:
      command: a
      piExtension: pi-mcp-adapter
    prune:
      command: b
      piOptionsPrune: true
    adapter:
      command: c
      piOptions:
        idleTimeout: 5
    lists:
      command: d
      piOptions:
        includeTools: [get_*]
  projects:
    $TMP/shop:
      servers:
        ext:
          disabled: true
          targets: [pi, opencode]
`

var retiredFields = []string{"piExtension", "piOptionsPrune", "idleTimeout", "includeTools", "directTools", "[pi, opencode]"}

// legacyService is a service over legacyConfig that records the files it backs up.
func legacyService(t *testing.T) (*Service, *[]string) {
	t.Helper()
	s, tmp := projectsService(t, legacyConfig)
	if err := os.MkdirAll(filepath.Join(tmp, "shop"), 0700); err != nil {
		t.Fatal(err)
	}
	backups := &[]string{}
	s.BackupSource = func(path string) (string, error) {
		*backups = append(*backups, path)
		return path + ".bak", nil
	}
	return s, backups
}

func assertMigrated(t *testing.T, path string) {
	t.Helper()
	data, _ := os.ReadFile(path)
	for _, gone := range retiredFields {
		if strings.Contains(string(data), gone) {
			t.Fatalf("saved config keeps %s:\n%s", gone, data)
		}
	}
	if !strings.Contains(string(data), "# my MCP servers") {
		t.Fatalf("saving dropped the comment:\n%s", data)
	}
	source, err := LoadSource(path)
	if err != nil || len(source.Notices) != 0 || source.NeedsMigration() {
		t.Fatalf("after sync: %q %v", source.Notices, err)
	}
}

// A sync saves the config without the settings 0.23.0 retired, backed up first, so the
// notices naming them go away.
func TestSyncSavesTheConfigWithoutRetiredSettings(t *testing.T) {
	s, backups := legacyService(t)
	plan, err := s.Preview()
	if err != nil || plan.Blocked {
		t.Fatalf("%+v %v", plan, err)
	}
	// Five retired kinds, and the tool policy lists converted from includeTools.
	if len(plan.Notices) != 6 || !plan.Migrates {
		t.Fatalf("notices: %q", plan.Notices)
	}
	result, err := s.Apply(plan.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if want := []MigratedFile{{s.ConfigPath, s.ConfigPath + ".bak"}}; !reflect.DeepEqual(result.Migrated, want) || len(result.Plan.Notices) != 1 || len(*backups) != 1 {
		t.Fatalf("migrated %v, notices %q, backups %v", result.Migrated, result.Plan.Notices, *backups)
	}
	assertMigrated(t, s.ConfigPath)
	if data, err := os.ReadFile(filepath.Join(s.Home, ".pi", "agent", "mcp.json")); err != nil || !strings.Contains(string(data), `"ext"`) {
		t.Fatalf("Pi file: %s %v", data, err)
	}
	if result, err = s.Apply(""); err != nil || result.Migrated != nil || len(*backups) != 1 {
		t.Fatalf("a second sync saved again: %+v %v %v", result, err, *backups)
	}
}

func TestPreviewLeavesTheLegacyConfigAlone(t *testing.T) {
	s, backups := legacyService(t)
	before, _ := os.ReadFile(s.ConfigPath)
	if _, err := s.Preview(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(s.ConfigPath)
	if string(before) != string(after) || len(*backups) != 0 {
		t.Fatalf("preview wrote the config: %s", after)
	}
}

func TestProjectSyncSavesTheConfigWithoutRetiredSettings(t *testing.T) {
	s, backups := legacyService(t)
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.ApplyProject(plan.Revision, filepath.Join(filepath.Dir(s.ConfigPath), "shop"))
	if err != nil || result.Migrated == nil || len(*backups) != 1 {
		t.Fatalf("%+v %v %v", result, err, *backups)
	}
	assertMigrated(t, s.ConfigPath)
}

// The dashboard's sync sends no change with sync set.
func TestDashboardSyncSavesTheConfigWithoutRetiredSettings(t *testing.T) {
	s, _ := legacyService(t)
	plan, err := s.PreviewMutation(Mutation{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.Mutate(Mutation{}, plan.Revision, true)
	if err != nil || result.Migrated == nil {
		t.Fatalf("%+v %v", result, err)
	}
	assertMigrated(t, s.ConfigPath)
}

func TestSyncSavesAnExternalSourceWithoutRetiredSettings(t *testing.T) {
	s, _ := projectsService(t, "sources:\n  mcp: ./mcp.yaml\nmcp:\n  targets: [pi]\n")
	external := filepath.Join(filepath.Dir(s.ConfigPath), "mcp.yaml")
	if err := os.WriteFile(external, []byte("# mine\nservers:\n  ext:\n    command: a\n    piExtension: pi-mcp-adapter\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var backups []string
	s.BackupSource = func(path string) (string, error) { backups = append(backups, path); return "", nil }
	config, _ := os.ReadFile(s.ConfigPath)
	if result, err := s.Apply(""); err != nil || result.Migrated == nil {
		t.Fatalf("%+v %v", result, err)
	}
	data, _ := os.ReadFile(external)
	if strings.Contains(string(data), "piExtension") || !strings.Contains(string(data), "# mine") || len(backups) != 1 || backups[0] != external {
		t.Fatalf("external source %v:\n%s", backups, data)
	}
	if after, _ := os.ReadFile(s.ConfigPath); string(after) != string(config) {
		t.Fatalf("config.yaml changed:\n%s", after)
	}
}

// Agent files are written by then, so a failure says so and leaves the config as it was.
func TestSyncReportsAConfigItCouldNotSave(t *testing.T) {
	s, _ := legacyService(t)
	s.BackupSource = func(string) (string, error) { return "", errors.New("disk full") }
	before, _ := os.ReadFile(s.ConfigPath)
	result, err := s.Apply("")
	if err == nil || !strings.Contains(err.Error(), "MCP files applied, but saving") || !strings.Contains(err.Error(), "disk full") || result == nil || result.Migrated != nil {
		t.Fatalf("%+v %v", result, err)
	}
	if after, _ := os.ReadFile(s.ConfigPath); string(after) != string(before) {
		t.Fatalf("config changed:\n%s", after)
	}
}
