//go:build !online

package integration

import (
	"os"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func projectOrigin(payload map[string]any) string {
	project, _ := payload["project"].(map[string]any)
	origin, _ := project["origin"].(string)
	return origin
}

func localRegistry(t *testing.T, configPath string) string {
	t.Helper()
	path, err := config.LocalProjectsPath(configPath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(b)
}

func TestProjectStorage_AddSavesInConfigByDefault(t *testing.T) {
	sb, root := projectFixture(t)
	r := sb.RunCLI("project", "add", root, "--target", "claude", "--json")
	r.AssertSuccess(t)
	if origin := projectOrigin(projectJSON(t, r)); origin != "shared" {
		t.Fatalf("origin %q, want shared", origin)
	}
	if !strings.Contains(sb.ReadFile(sb.ConfigPath), root+":") {
		t.Fatalf("config.yaml lacks the project:\n%s", sb.ReadFile(sb.ConfigPath))
	}
	if local := localRegistry(t, sb.ConfigPath); strings.Contains(local, root) {
		t.Fatalf("default add wrote the local registry:\n%s", local)
	}
}

func TestProjectStorage_AddMachineLocal(t *testing.T) {
	sb, root := projectFixture(t)
	before := sb.ReadFile(sb.ConfigPath)
	r := sb.RunCLI("project", "add", root, "--target", "claude", "--machine-local", "--json")
	r.AssertSuccess(t)
	if origin := projectOrigin(projectJSON(t, r)); origin != "local" {
		t.Fatalf("origin %q, want local", origin)
	}
	if sb.ReadFile(sb.ConfigPath) != before {
		t.Fatal("--machine-local changed config.yaml")
	}
	r = sb.RunCLI("project", "add", root, "--target", "claude", "--shared", "--json")
	if r.ExitCode == 0 || !strings.Contains(r.Stdout, "project share") {
		t.Fatalf("re-add with another layer: %s", r.Stdout)
	}
}

func TestProjectStorage_SettingPicksDefaultLayer(t *testing.T) {
	sb, root := projectFixture(t)
	sb.WriteConfig("source: " + sb.SourcePath + "\nprojects_storage: machine-local\ntargets: {}\n")
	r := sb.RunCLI("project", "add", root, "--target", "claude", "--json")
	r.AssertSuccess(t)
	if origin := projectOrigin(projectJSON(t, r)); origin != "local" {
		t.Fatalf("origin %q, want local", origin)
	}
	other := root + "-other"
	if err := os.MkdirAll(other, 0755); err != nil {
		t.Fatal(err)
	}
	r = sb.RunCLI("project", "add", other, "--target", "claude", "--shared", "--json")
	r.AssertSuccess(t)
	if origin := projectOrigin(projectJSON(t, r)); origin != "shared" {
		t.Fatalf("--shared origin %q, want shared", origin)
	}
}

func TestProjectStorage_InvalidSetting(t *testing.T) {
	sb, _ := projectFixture(t)
	sb.WriteConfig("source: " + sb.SourcePath + "\nprojects_storage: elsewhere\ntargets: {}\n")
	r := sb.RunCLI("project", "list")
	r.AssertFailure(t)
	r.AssertAnyOutputContains(t, "invalid projects_storage")
}

func TestProjectStorage_ShareMovesIntoConfig(t *testing.T) {
	sb, root := projectFixture(t)
	sb.RunCLI("project", "add", root, "--target", "claude", "--machine-local", "--json").AssertSuccess(t)
	r := sb.RunCLI("project", "share", root, "--json")
	r.AssertSuccess(t)
	if origin := projectOrigin(projectJSON(t, r)); origin != "shared" {
		t.Fatalf("origin %q, want shared", origin)
	}
	if !strings.Contains(sb.ReadFile(sb.ConfigPath), root+":") {
		t.Fatal("share did not write config.yaml")
	}
	if local := localRegistry(t, sb.ConfigPath); strings.Contains(local, root) {
		t.Fatalf("share kept the local copy:\n%s", local)
	}
	r = sb.RunCLI("project", "share", root, "--json")
	r.AssertSuccess(t)
	if projectJSON(t, r)["unchanged"] != true {
		t.Fatalf("repeat share: %s", r.Stdout)
	}
}

func TestProjectStorage_ShareConfirmsDifferentSharedSettings(t *testing.T) {
	sb, root := projectFixture(t)
	sb.RunCLI("project", "add", root, "--target", "claude", "--json").AssertSuccess(t)
	sb.RunCLI("project", "localize", root, "--json").AssertSuccess(t)
	sb.RunCLI("project", "edit", root, "--name", "mine", "--json").AssertSuccess(t)
	shared := sb.ReadFile(sb.ConfigPath)
	r := sb.RunCLI("project", "share", "mine", "--json")
	if r.ExitCode == 0 || !strings.Contains(r.Stdout, "--yes") || sb.ReadFile(sb.ConfigPath) != shared {
		t.Fatalf("share replaced different settings without --yes: %s", r.Stdout)
	}
	sb.RunCLI("project", "share", "mine", "--yes", "--json").AssertSuccess(t)
	if !strings.Contains(sb.ReadFile(sb.ConfigPath), "name: mine") {
		t.Fatal("share --yes did not replace config.yaml settings")
	}
}

// A dry run previews the replacement it would ask about, without asking.
func TestProjectStorage_ShareDryRunPreviewsReplacement(t *testing.T) {
	sb, root := projectFixture(t)
	sb.RunCLI("project", "add", root, "--target", "claude", "--json").AssertSuccess(t)
	sb.RunCLI("project", "localize", root, "--json").AssertSuccess(t)
	sb.RunCLI("project", "edit", root, "--name", "mine", "--json").AssertSuccess(t)
	shared := sb.ReadFile(sb.ConfigPath)
	sb.RunCLI("project", "share", "mine", "--dry-run", "--json").AssertSuccess(t)
	if sb.ReadFile(sb.ConfigPath) != shared {
		t.Fatal("share --dry-run wrote config.yaml")
	}
}

func TestProjectStorage_RemoveLocalCopyKeepsConfigVersion(t *testing.T) {
	sb, root := projectFixture(t)
	sb.RunCLI("project", "add", root, "--target", "claude", "--json").AssertSuccess(t)
	sb.RunCLI("project", "localize", root, "--json").AssertSuccess(t)
	shared := sb.ReadFile(sb.ConfigPath)
	r := sb.RunCLI("project", "remove", root, "--yes", "--json")
	r.AssertSuccess(t)
	if remaining, _ := projectJSON(t, r)["remaining"].(map[string]any); remaining["origin"] != "shared" {
		t.Fatalf("remaining: %s", r.Stdout)
	}
	if sb.ReadFile(sb.ConfigPath) != shared {
		t.Fatal("removing the local copy changed config.yaml")
	}
	r = sb.RunCLI("project", "show", root, "--json")
	r.AssertSuccess(t)
	if origin := projectOrigin(projectJSON(t, r)); origin != "shared" {
		t.Fatalf("origin %q, want shared", origin)
	}
}

func TestProjectStorage_RemoveSharedDropsBoth(t *testing.T) {
	sb, root := projectFixture(t)
	sb.RunCLI("project", "add", root, "--target", "claude", "--json").AssertSuccess(t)
	sb.RunCLI("project", "localize", root, "--json").AssertSuccess(t)
	sb.RunCLI("project", "remove", root, "--shared", "--yes", "--json").AssertSuccess(t)
	if strings.Contains(sb.ReadFile(sb.ConfigPath), root+":") {
		t.Fatal("remove --shared kept config.yaml's declaration")
	}
	r := sb.RunCLI("project", "list", "--json")
	r.AssertSuccess(t)
	if len(projectJSON(t, r)["projects"].([]any)) != 0 {
		t.Fatal(r.Stdout)
	}
}

// The log tells a removal of both copies from a removal of this machine's copy.
func TestProjectStorage_RemoveSharedIsLogged(t *testing.T) {
	sb, root := projectFixture(t)
	sb.RunCLI("project", "add", root, "--target", "claude", "--json").AssertSuccess(t)
	sb.RunCLI("project", "localize", root, "--json").AssertSuccess(t)
	sb.RunCLI("project", "remove", root, "--shared", "--yes", "--json").AssertSuccess(t)
	logs := projectLogs(t, sb)
	if args, _ := logs[len(logs)-1]["args"].(map[string]any); args["shared"] != true {
		t.Fatalf("remove --shared log args: %v", logs[len(logs)-1])
	}
}

func TestProjectStorage_FlagValidation(t *testing.T) {
	sb, root := projectFixture(t)
	for _, args := range [][]string{
		{"project", "edit", root, "--machine-local"},
		{"project", "localize", root, "--shared"},
		{"project", "add", root, "--target", "claude", "--machine-local", "--shared"},
	} {
		sb.RunCLI(args...).AssertFailure(t)
	}
}
