package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakePiNpm is a Pi CLI whose install and remove edit the settings file the way Pi does,
// so Skillshare's check after the command finds the package. It records each command.
func fakePiNpm(t *testing.T, commands *[]string) *Service {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(home, ".pi/agent"))
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, env []string, bin string, args ...string) ([]byte, error) {
		if args[0] == "--version" {
			return []byte("0.99.2"), nil
		}
		if slices.Contains(args, "--help") {
			return []byte("usage"), nil
		}
		*commands = append(*commands, bin+" "+strings.Join(args, " "))
		settings := filepath.Join(home, ".pi/agent/settings.json")
		for _, e := range env {
			if dir, ok := strings.CutPrefix(e, "PI_CODING_AGENT_DIR="); ok {
				settings = filepath.Join(dir, "settings.json")
			}
		}
		if slices.Contains(args, "--local") {
			settings = filepath.Join(s.ProjectRoot, ".pi/settings.json")
		}
		var doc struct {
			Packages []json.RawMessage `json:"packages"`
		}
		if raw, err := os.ReadFile(settings); err == nil {
			_ = json.Unmarshal(raw, &doc)
		}
		// An entry is a source string, or an object whose other keys hold its filters.
		source := func(entry json.RawMessage) (string, map[string]json.RawMessage) {
			var text string
			if json.Unmarshal(entry, &text) == nil {
				return text, nil
			}
			var object map[string]json.RawMessage
			_ = json.Unmarshal(entry, &object)
			_ = json.Unmarshal(object["source"], &text)
			return text, object
		}
		switch args[0] {
		case "install":
			// Pi replaces the source of the entry with the same package name and keeps its keys.
			name, _ := npmSpec(args[1])
			value, _ := json.Marshal(args[1])
			replaced := false
			for i, entry := range doc.Packages {
				if old, object := source(entry); !replaced && sameNpmPackage(old, "npm:"+name) {
					if object != nil {
						object["source"] = value
						value, _ = json.Marshal(object)
					}
					doc.Packages[i], replaced = value, true
				}
			}
			if !replaced {
				doc.Packages = append(doc.Packages, value)
			}
		case "remove":
			doc.Packages = slices.DeleteFunc(doc.Packages, func(entry json.RawMessage) bool { old, _ := source(entry); return old == args[1] })
		}
		raw, _ := json.Marshal(doc)
		writePluginFile(t, filepath.Dir(settings), filepath.Base(settings), string(raw))
		return nil, nil
	}
	return s
}

func TestNpmPackageIsInstalledByPi(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	applyPluginRequest(t, s, Request{Action: "add", Source: "npm:@team/pi-tools", Targets: []string{"pi"}})
	if !slices.Equal(commands, []string{"pi install npm:@team/pi-tools"}) {
		t.Fatalf("commands: %q", commands)
	}
}

func TestNpmPackageIsRecordedUnderItsPackageName(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	applyPluginRequest(t, s, Request{Action: "add", Source: "npm:@team/pi-tools@1.2.3", Targets: []string{"pi"}})
	inv, err := s.Packages()
	if err != nil {
		t.Fatal(err)
	}
	if b := inv.Packages["pi-tools"].Bindings["pi"]; b.ID != "npm:@team/pi-tools@1.2.3" || b.Source != "" {
		t.Fatalf("packages: %+v", inv.Packages)
	}
}

func TestNpmPackagePreviewSaysPiRunsItsInstallScripts(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: "npm:demo", Targets: []string{"pi"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Changes[0].MessageKey != "plugins.note.npmInstall" {
		t.Fatalf("changes: %+v", p.Changes)
	}
}

func TestNpmPackageIsBlockedForOtherAgents(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: "npm:demo", Targets: []string{"opencode"}})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Blocked || p.Changes[0].MessageKey != "plugins.error.npmPiOnly" {
		t.Fatalf("plan: %+v", p)
	}
}

// The reason a target can't take npm packages outranks its CLI being missing, and the
// message the CLI prints matches the key the dashboard translates.
func TestNpmPackageBlockedForAMissingAgentKeepsItsReason(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if bin == "opencode" {
			return nil, agentError{cause: ErrCLIMissing, key: "plugins.error.cliMissing", message: "opencode CLI is not installed"}
		}
		return run(ctx, dir, env, bin, args...)
	}
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: "npm:demo", Targets: []string{"opencode"}})
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Changes[0]; c.MessageKey != "plugins.error.npmPiOnly" || !strings.Contains(c.Message, "installed by Pi") {
		t.Fatalf("change: %+v", c)
	}
}

func TestNpmPackageIsBlockedForAnAccountRunningAnotherCLI(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	s.Accounts = map[string]Account{"pi-work": {Agent: "pi", Dir: filepath.Join(t.TempDir(), "pi-work"), CLI: "omo"}}
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: "npm:demo", Targets: []string{"pi-work"}})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Blocked || p.Changes[0].MessageKey != "plugins.error.npmOtherCli" {
		t.Fatalf("plan: %+v", p)
	}
}

func TestNpmPackageInstallsIntoAnAccountsDirectory(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	dir := filepath.Join(t.TempDir(), "pi-work")
	s.Accounts = map[string]Account{"pi-work": {Agent: "pi", Dir: dir}}
	applyPluginRequest(t, s, Request{Action: "add", Source: "npm:demo", Targets: []string{"pi-work"}})
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); err != nil {
		t.Fatalf("not installed in the account: %v", err)
	}
}

func TestNpmPackageInAProjectInstallsLocally(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	s.ProjectRoot = t.TempDir()
	applyPluginRequest(t, s, Request{Action: "add", Source: "npm:demo", Targets: []string{"pi"}})
	if !slices.Equal(commands, []string{"pi install npm:demo --local"}) {
		t.Fatalf("commands: %q", commands)
	}
}

func TestNpmPackageNeedsATarget(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	if _, err := s.Preview(context.Background(), Request{Action: "add", Source: "npm:demo"}); err == nil {
		t.Fatal("an npm package was recorded without a Pi target to install it")
	}
}

func TestNpmPackageAlreadyInPiIsImported(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	writePluginFile(t, os.Getenv("PI_CODING_AGENT_DIR"), "settings.json", `{"packages":["npm:demo"]}`)
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: "npm:demo", Targets: []string{"pi"}})
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Changes[0]; c.Action != "import" {
		t.Fatalf("changes: %+v", p.Changes)
	}
}

// Pi matches packages by name and replaces the entry's source, so another version is an install.
func TestNpmPackageWithAnotherVersionInPiSaysPiReplacesIt(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	writePluginFile(t, os.Getenv("PI_CODING_AGENT_DIR"), "settings.json", `{"packages":["npm:demo@1.0.0"]}`)
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: "npm:demo@2.0.0", Targets: []string{"pi"}})
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Changes[0]; c.Action != "install" || c.MessageKey != "plugins.note.npmReplace" || c.MessageArgs["from"] != "npm:demo@1.0.0" {
		t.Fatalf("changes: %+v", p.Changes)
	}
}

func TestPinnedNpmPackageMovesToTheVersionAddedAgain(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	applyPluginRequest(t, s, Request{Action: "add", Source: "npm:demo@1.0.0", Targets: []string{"pi"}})
	applyPluginRequest(t, s, Request{Action: "add", Source: "npm:demo@2.0.0", Name: "demo", Targets: []string{"pi"}})
	inv, err := s.Packages()
	if err != nil {
		t.Fatal(err)
	}
	if id := inv.Packages["demo"].Bindings["pi"].ID; id != "npm:demo@2.0.0" {
		t.Fatalf("binding: %q", id)
	}
}

func TestPinnedNpmPackageUpdateIsSkipped(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	applyPluginRequest(t, s, Request{Action: "add", Source: "npm:demo@1.2.3", Targets: []string{"pi"}})
	p, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Changes[0]; c.Action != "skip" || c.MessageKey != "plugins.skip.npmPinned" {
		t.Fatalf("changes: %+v", p.Changes)
	}
}

// Pi pins only an exact version; a range still updates.
func TestNpmVersionRangeUpdatesThroughPi(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	applyPluginRequest(t, s, Request{Action: "add", Source: "npm:demo@^1.2.0", Targets: []string{"pi"}})
	p, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Changes[0]; c.Action != "update" {
		t.Fatalf("changes: %+v", p.Changes)
	}
}

func TestDiscoverExplainsNpmPackagesCannotBePreviewed(t *testing.T) {
	_, err := Discover(context.Background(), "npm:demo")
	if err == nil || !strings.Contains(err.Error(), "plugin add npm:") {
		t.Fatalf("error: %v", err)
	}
}

func TestOnlyPiTargetsThatRunPiTakeNpmPackages(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	s.Accounts = map[string]Account{"pi-work": {Agent: "pi", Dir: t.TempDir()}, "pi-omo": {Agent: "pi", Dir: t.TempDir(), CLI: "omo"}}
	npm := []string{}
	for _, d := range s.TargetDefinitions() {
		if d.Npm {
			npm = append(npm, d.Target)
		}
	}
	if !slices.Equal(npm, []string{"pi", "pi-work"}) {
		t.Fatalf("npm targets: %q", npm)
	}
}

// Removing or replacing a Pi package another Skillshare package manages would change that one too.
func TestNpmPackageManagedUnderAnotherNameIsBlocked(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	applyPluginRequest(t, s, Request{Action: "add", Source: "npm:demo", Name: "tools", Targets: []string{"pi"}})
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: "npm:demo@2.0.0", Name: "other", Targets: []string{"pi"}})
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Changes[0]; c.Action != "blocked" || !strings.Contains(c.Message, "tools") {
		t.Fatalf("changes: %+v", p.Changes)
	}
}

// An adopted entry with resource filters keeps them, as an import does.
func TestNpmPackageAdoptedWithFiltersPreservesThem(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	writePluginFile(t, os.Getenv("PI_CODING_AGENT_DIR"), "settings.json", `{"packages":[{"source":"npm:demo","extensions":["-a.ts"]}]}`)
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: "npm:demo", Targets: []string{"pi"}})
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Changes[0]; c.Action != "import" || c.Binding.PiRegistration == "" {
		t.Fatalf("changes: %+v", p.Changes)
	}
}

// Pi keeps a filtered entry's keys when another version replaces its source, so the record of
// the old version does not block the change.
func TestFilteredNpmPackageMovesToAnotherVersion(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	writePluginFile(t, os.Getenv("PI_CODING_AGENT_DIR"), "settings.json", `{"packages":[{"source":"npm:demo@1.0.0","extensions":["-a.ts"]}]}`)
	applyPluginRequest(t, s, Request{Action: "import", From: "pi", Plugin: "npm:demo@1.0.0", Name: "demo"})
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: "npm:demo@2.0.0", Name: "demo", Targets: []string{"pi"}})
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Changes[0]; c.Action != "install" || c.Binding.PiRegistration != "" {
		t.Fatalf("changes: %+v", p.Changes)
	}
}

// Sources Pi itself can't resolve are refused before a binding is recorded.
func TestMalformedNpmSourcesAreInvalid(t *testing.T) {
	for _, source := range []string{"npm:@team", "npm:@team/", "npm:@/tools", "npm:foo/bar", "npm:foo@", "npm:foo..bar", "npm:@team/../x"} {
		if validNpmSource(source) {
			t.Errorf("%s was accepted", source)
		}
	}
	for _, source := range []string{"npm:demo", "npm:foo.bar@latest", "npm:@team/pi-tools@^1.2.0"} {
		if !validNpmSource(source) {
			t.Errorf("%s was refused", source)
		}
	}
}

// After Pi moves a filtered entry to another version, the entry is recorded again, so a later
// reinstall restores its filters.
func TestFilteredNpmPackageIsRecordedAgainAfterAnotherVersion(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	writePluginFile(t, os.Getenv("PI_CODING_AGENT_DIR"), "settings.json", `{"packages":[{"source":"npm:demo@1.0.0","extensions":["-a.ts"]}]}`)
	applyPluginRequest(t, s, Request{Action: "import", From: "pi", Plugin: "npm:demo@1.0.0", Name: "demo"})
	applyPluginRequest(t, s, Request{Action: "add", Source: "npm:demo@2.0.0", Name: "demo", Targets: []string{"pi"}})
	inv, err := s.Packages()
	if err != nil {
		t.Fatal(err)
	}
	b := inv.Packages["demo"].Bindings["pi"]
	record, err := s.readPiRegistration(b.PiRegistration, "pi", "npm:demo@2.0.0")
	if err != nil || !strings.Contains(record.Entry, "-a.ts") {
		t.Fatalf("binding %+v, record %+v: %v", b, record, err)
	}
}

// A dot is part of an npm package name, not an extension, so adding another version finds it.
func TestNpmPackageNameKeepsItsDots(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	applyPluginRequest(t, s, Request{Action: "add", Source: "npm:foo.bar", Targets: []string{"pi"}})
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: "npm:foo.bar@1.2.3", Targets: []string{"pi"}})
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Changes[0]; c.Name != "foo.bar" || c.Action != "install" {
		t.Fatalf("changes: %+v", p.Changes)
	}
}

func TestPiAccountRunningPiByPathTakesNpmPackages(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	s.Accounts = map[string]Account{"pi-work": {Agent: "pi", Dir: t.TempDir(), CLI: "/usr/local/bin/pi"}}
	if !slices.ContainsFunc(s.TargetDefinitions(), func(d TargetDefinition) bool { return d.Target == "pi-work" && d.Npm }) {
		t.Fatal("an account running pi by its path was refused npm packages")
	}
}
