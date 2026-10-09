package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateConfig_SourceNotExist(t *testing.T) {
	cfg := &Config{
		Source:  "/nonexistent/source/path",
		Targets: map[string]TargetConfig{},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
	if !strings.Contains(err.Error(), "source path does not exist") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_DefaultSource_OK(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	defaultSource := filepath.Join(tmpDir, "skillshare", "skills")
	if err := os.MkdirAll(defaultSource, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		Source:  "",
		Targets: map[string]TargetConfig{},
	}
	warnings, err := ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error for default source: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestValidateConfig_SourceIsFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(tmpFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		Source:  tmpFile,
		Targets: map[string]TargetConfig{},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for file source")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_InvalidGlobalMode(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Source:  tmpDir,
		Mode:    "invalid",
		Targets: map[string]TargetConfig{},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid global mode")
	}
	if !strings.Contains(err.Error(), "invalid global sync mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_InvalidGlobalTargetNaming(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Source:       tmpDir,
		TargetNaming: "weird",
		Targets:      map[string]TargetConfig{},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid global target naming")
	}
	if !strings.Contains(err.Error(), "invalid global target naming") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_InvalidGitRoot(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Source:  tmpDir,
		GitRoot: "agnets", // typo for "agents"
		Targets: map[string]TargetConfig{},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid git_root")
	}
	if !strings.Contains(err.Error(), "invalid git_root") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_InvalidTargetMode(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)
	cfg := &Config{
		Source: tmpDir,
		Targets: map[string]TargetConfig{
			"test": {Path: targetDir, Mode: "badmode"},
		},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid target mode")
	}
	if !strings.Contains(err.Error(), "invalid sync mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_InvalidTargetNaming(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)
	cfg := &Config{
		Source: tmpDir,
		Targets: map[string]TargetConfig{
			"test": {Skills: &ResourceTargetConfig{Path: targetDir, TargetNaming: "odd"}},
		},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid target naming")
	}
	if !strings.Contains(err.Error(), "invalid target naming") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateConfig_TargetNotExist_Accepted(t *testing.T) {
	tmpDir := t.TempDir()
	// Missing target path is accepted — sync will auto-create and notify
	cfg := &Config{
		Source: tmpDir,
		Targets: map[string]TargetConfig{
			"test": {Path: filepath.Join(tmpDir, "nonexistent"), Mode: "merge"},
		},
	}
	_, err := ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("expected no error for missing target (sync auto-creates), got: %v", err)
	}
}

func TestValidateConfig_TargetDeepPathNotExist_Accepted(t *testing.T) {
	tmpDir := t.TempDir()
	// Even deeply missing paths are accepted (e.g., universal target ~/.agents/skills)
	cfg := &Config{
		Source: tmpDir,
		Targets: map[string]TargetConfig{
			"test": {Path: filepath.Join(tmpDir, "no", "parent", "target"), Mode: "copy"},
		},
	}
	_, err := ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("expected no error for deeply missing target (sync auto-creates), got: %v", err)
	}
}

func TestValidateConfig_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)
	cfg := &Config{
		Source: tmpDir,
		Mode:   "merge",
		Targets: map[string]TargetConfig{
			"test": {Path: targetDir, Mode: "merge"},
		},
	}
	warnings, err := ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestValidateConfig_CustomTarget_MissingPath(t *testing.T) {
	cfg := &Config{
		Source: t.TempDir(),
		Targets: map[string]TargetConfig{
			"my-custom-ide": {Skills: &ResourceTargetConfig{Mode: "merge"}},
		},
	}
	_, err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for custom target without path")
	}
	if !strings.Contains(err.Error(), "missing path") {
		t.Fatalf("expected 'missing path' error, got: %v", err)
	}
}

func TestValidateConfig_BuiltinTarget_NoPath_OK(t *testing.T) {
	cfg := &Config{
		Source: t.TempDir(),
		Targets: map[string]TargetConfig{
			"claude": {Skills: &ResourceTargetConfig{Mode: "merge"}},
		},
	}
	_, err := ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("built-in target should accept empty path: %v", err)
	}
}

func TestValidateProjectConfig_CustomTarget_MissingPath(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".skillshare", "skills"), 0755)
	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{
			{Name: "my-custom-ide", Skills: &ResourceTargetConfig{Mode: "merge"}},
		},
	}
	_, err := ValidateProjectConfig(cfg, root)
	if err == nil {
		t.Fatal("expected error for custom project target without path")
	}
	if !strings.Contains(err.Error(), "missing path") {
		t.Fatalf("expected 'missing path' error, got: %v", err)
	}
}

func TestValidateProjectConfig_SourceAliasesBuiltinTarget(t *testing.T) {
	root := t.TempDir()
	// Configure source to alias the built-in claude target path
	os.MkdirAll(filepath.Join(root, ".claude", "skills"), 0755)
	cfg := &ProjectConfig{
		Sources: ProjectSources{Skills: ".claude/skills"},
		Targets: []ProjectTargetEntry{
			{Name: "claude", Skills: &ResourceTargetConfig{Mode: "merge"}},
		},
	}
	_, err := ValidateProjectConfig(cfg, root)
	if err == nil {
		t.Fatal("expected overlap error when skills source aliases claude target path")
	}
	if !strings.Contains(err.Error(), "overlaps skills source") {
		t.Fatalf("expected 'overlaps skills source' error, got: %v", err)
	}
}

func TestValidateProjectConfig_SourceNestsTargetPath(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "shared", "skills", "nested"), 0755)
	cfg := &ProjectConfig{
		Sources: ProjectSources{Skills: "shared"},
		Targets: []ProjectTargetEntry{
			{Name: "custom", Skills: &ResourceTargetConfig{Path: "shared/skills/nested", Mode: "merge"}},
		},
	}
	_, err := ValidateProjectConfig(cfg, root)
	if err == nil {
		t.Fatal("expected overlap error when target nests inside source")
	}
	if !strings.Contains(err.Error(), "overlaps") {
		t.Fatalf("expected overlap error, got: %v", err)
	}
}

func TestValidateProjectConfig_BuiltinTarget_NoPath_OK(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".skillshare", "skills"), 0755)
	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{
			{Name: "claude", Skills: &ResourceTargetConfig{Mode: "merge"}},
		},
	}
	_, err := ValidateProjectConfig(cfg, root)
	if err != nil {
		t.Fatalf("built-in target should accept empty path: %v", err)
	}
}

func TestValidateConfig_EmptyMode_OK(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)
	cfg := &Config{
		Source: tmpDir,
		Mode:   "", // empty = default merge
		Targets: map[string]TargetConfig{
			"test": {Path: targetDir, Mode: ""}, // empty = inherit global
		},
	}
	warnings, err := ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestIsValidSyncMode(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{"", true},
		{"merge", true},
		{"symlink", true},
		{"copy", true},
		{"invalid", false},
		{"MERGE", false}, // case sensitive
	}
	for _, tt := range tests {
		if got := IsValidSyncMode(tt.mode); got != tt.want {
			t.Errorf("IsValidSyncMode(%q) = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestValidateProjectConfig_InvalidMode(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".skillshare", "skills"), 0755)
	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{
			{Name: "claude", Mode: "badmode"},
		},
	}
	_, err := ValidateProjectConfig(cfg, tmpDir)
	if err == nil {
		t.Fatal("expected error for invalid project target mode")
	}
	if !strings.Contains(err.Error(), "invalid sync mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateProjectConfig_InvalidTargetNaming(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".skillshare", "skills"), 0755)
	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{
			{Name: "claude", Skills: &ResourceTargetConfig{TargetNaming: "bad"}},
		},
	}
	_, err := ValidateProjectConfig(cfg, tmpDir)
	if err == nil {
		t.Fatal("expected error for invalid project target naming")
	}
	if !strings.Contains(err.Error(), "invalid target naming") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEffectiveTargetNaming(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "default", input: "", expect: "flat"},
		{name: "flat", input: "flat", expect: "flat"},
		{name: "standard", input: "standard", expect: "standard"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveTargetNaming(tt.input); got != tt.expect {
				t.Fatalf("EffectiveTargetNaming(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestValidateProjectConfig_MissingSource_Warning(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't create .skillshare/skills/
	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{
			{Name: "claude"},
		},
	}
	warnings, err := ValidateProjectConfig(cfg, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "source directory does not exist yet") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected source warning, got: %v", warnings)
	}
}

func TestValidateConfig_RejectsImportWithoutFile(t *testing.T) {
	cfg := &Config{
		Source:  t.TempDir(),
		Targets: map[string]TargetConfig{},
		Extras:  []ExtraConfig{{Name: "rules", Targets: []ExtraTargetConfig{{Path: "/t", Mode: "import"}}}},
	}
	_, err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "import mode requires file") {
		t.Fatalf("err = %v, want import mode requires file", err)
	}
}

func TestValidateConfig_SkillsTargetRejectsImport(t *testing.T) {
	cfg := &Config{
		Source:  t.TempDir(),
		Targets: map[string]TargetConfig{"claude": {Mode: "import"}},
	}
	_, err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "invalid sync mode") {
		t.Fatalf("err = %v, want invalid sync mode", err)
	}
}

func TestValidateTargetInstructions(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		project bool
		wantErr string
	}{
		{"global tilde", "~/.myagent/AGENTS.md", false, ""},
		{"global absolute", absPath("/opt/agent/AGENTS.md"), false, ""},
		{"global relative", ".myagent/AGENTS.md", false, "absolute or start with ~/"},
		{"empty", "  ", false, "is empty"},
		{"directory", "~/.myagent/", false, "not a directory"},
		{"project relative", ".myagent/AGENTS.md", true, ""},
		{"project absolute", absPath("/opt/agent/AGENTS.md"), true, "relative to the project root"},
		{"project tilde", "~/AGENTS.md", true, "relative to the project root"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTargetInstructions(&TargetInstructionsConfig{Path: tc.path}, tc.project)
			if tc.wantErr == "" && err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateConfig_RejectsRelativeInstructionsPath(t *testing.T) {
	cfg := &Config{
		Source: t.TempDir(),
		Targets: map[string]TargetConfig{"myagent": {
			Skills:       &ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "skills")},
			Instructions: &TargetInstructionsConfig{Path: "AGENTS.md"},
		}},
	}
	_, err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), `target "myagent": instructions.path`) {
		t.Fatalf("err = %v, want instructions.path error", err)
	}
}

func TestValidateConfigForSync_TargetProblemFailsOnlyThatTarget(t *testing.T) {
	file := filepath.Join(t.TempDir(), "skills-file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		Source: t.TempDir(),
		Targets: map[string]TargetConfig{
			"broken": {Skills: &ResourceTargetConfig{Path: file}},
			"good":   {Skills: &ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "skills")}},
		},
	}
	_, invalid, err := ValidateConfigForSync(cfg)
	if err != nil || len(invalid) != 1 || invalid["broken"] == nil || !strings.Contains(invalid["broken"].Error(), "path is not a directory") {
		t.Fatalf("err = %v, invalid = %v; want only broken invalid", err, invalid)
	}
}

func TestValidateConfigForSync_GlobalProblemStillFails(t *testing.T) {
	cfg := &Config{Source: t.TempDir(), Mode: "bogus", Targets: map[string]TargetConfig{}}
	if _, _, err := ValidateConfigForSync(cfg); err == nil || !strings.Contains(err.Error(), "invalid global sync mode") {
		t.Fatalf("err = %v, want invalid global sync mode", err)
	}
}

func TestValidateConfig_TargetPathIsFileStillRejected(t *testing.T) {
	file := filepath.Join(t.TempDir(), "skills-file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Source: t.TempDir(), Targets: map[string]TargetConfig{"broken": {Skills: &ResourceTargetConfig{Path: file}}}}
	if _, err := ValidateConfig(cfg); err == nil || !strings.Contains(err.Error(), `target "broken": path is not a directory`) {
		t.Fatalf("err = %v, want target path error", err)
	}
}

func TestValidateProjectConfigForSync_TargetProblemFailsOnlyThatTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".skillshare", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills-file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &ProjectConfig{Targets: []ProjectTargetEntry{
		{Name: "broken", Skills: &ResourceTargetConfig{Path: "skills-file"}},
		{Name: "claude"},
	}}
	_, invalid, err := ValidateProjectConfigForSync(cfg, root)
	if err != nil || len(invalid) != 1 || invalid["broken"] == nil || !strings.Contains(invalid["broken"].Error(), "path is not a directory") {
		t.Fatalf("err = %v, invalid = %v; want only broken invalid", err, invalid)
	}
}

func TestValidateConfigForSync_PrefixedNamingRequiresCopyMode(t *testing.T) {
	dir := func() string { return filepath.Join(t.TempDir(), "skills") }
	cfg := &Config{
		Source:       t.TempDir(),
		Mode:         "copy",
		TargetNaming: "prefixed",
		Targets: map[string]TargetConfig{
			"inherits-copy": {Skills: &ResourceTargetConfig{Path: dir()}},
			"merge":         {Skills: &ResourceTargetConfig{Path: dir(), Mode: "merge"}},
			"own-naming":    {Skills: &ResourceTargetConfig{Path: dir(), Mode: "symlink", TargetNaming: "flat"}},
		},
	}
	_, invalid, err := ValidateConfigForSync(cfg)
	if err != nil || len(invalid) != 1 || invalid["merge"] == nil || !strings.Contains(invalid["merge"].Error(), `target naming "prefixed" requires copy mode`) || !strings.Contains(invalid["merge"].Error(), "set mode: copy on the target") {
		t.Fatalf("err = %v, invalid = %v; want only merge invalid", err, invalid)
	}
}

func TestValidateProjectConfigForSync_PrefixedNamingRequiresCopyMode(t *testing.T) {
	root := t.TempDir()
	cfg := &ProjectConfig{Targets: []ProjectTargetEntry{
		{Name: "claude", Skills: &ResourceTargetConfig{TargetNaming: "prefixed"}},
		{Name: "cursor", Skills: &ResourceTargetConfig{TargetNaming: "prefixed", Mode: "copy"}},
	}}
	_, invalid, err := ValidateProjectConfigForSync(cfg, root)
	if err != nil || len(invalid) != 1 || invalid["claude"] == nil || !strings.Contains(invalid["claude"].Error(), `requires copy mode, but the target syncs in "merge" mode`) {
		t.Fatalf("err = %v, invalid = %v; want only claude invalid", err, invalid)
	}
}

func TestValidateProjectConfigForSync_InheritedPrefixedNamingRequiresCopyMode(t *testing.T) {
	// Not loaded through LoadProject, like the dashboard's raw-config save.
	cfg := &ProjectConfig{TargetNaming: "prefixed", Targets: []ProjectTargetEntry{{Name: "claude"}}}
	_, invalid, err := ValidateProjectConfigForSync(cfg, t.TempDir())
	if err != nil || invalid["claude"] == nil || !strings.Contains(invalid["claude"].Error(), `target naming "prefixed" requires copy mode`) {
		t.Fatalf("err = %v, invalid = %v; want claude invalid", err, invalid)
	}
}

func TestValidateConfig_ProjectPrefixedNamingRequiresCopyMode(t *testing.T) {
	// A raw config, as the dashboard saves it: projects are not expanded into targets.
	cfg := &Config{Source: t.TempDir(), Mode: "merge", Projects: map[string]ManagedProject{
		"~/work/app": {Skills: &ResourceTargetConfig{TargetNaming: "prefixed"}},
	}}
	_, err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), `projects: ~/work/app: target naming "prefixed" requires copy mode`) {
		t.Fatalf("err = %v, want the project's prefixed naming rejected", err)
	}
	if !strings.Contains(err.Error(), "set mode: copy in the project's skills settings") {
		t.Fatalf("err = %v, want the project settings hint", err)
	}
}

func TestValidateConfigForSync_SkillsOffSkipsNamingModeCheck(t *testing.T) {
	// Agents still sync for a target whose skills are off, so its skills settings must not block it.
	skills := &ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "skills"), TargetNaming: "prefixed"}
	skills.SetEnabled(false)
	cfg := &Config{Source: t.TempDir(), Mode: "merge", Targets: map[string]TargetConfig{"claude": {Skills: skills}}}
	if _, invalid, err := ValidateConfigForSync(cfg); err != nil || invalid["claude"] != nil {
		t.Fatalf("global: err = %v, invalid = %v; want claude valid", err, invalid)
	}

	projSkills := &ResourceTargetConfig{TargetNaming: "prefixed"}
	projSkills.SetEnabled(false)
	proj := &ProjectConfig{Targets: []ProjectTargetEntry{{Name: "claude", Skills: projSkills}}}
	if _, invalid, err := ValidateProjectConfigForSync(proj, t.TempDir()); err != nil || invalid["claude"] != nil {
		t.Fatalf("project: err = %v, invalid = %v; want claude valid", err, invalid)
	}

	off := &ResourceTargetConfig{TargetNaming: "prefixed"}
	off.SetEnabled(false)
	if err := cfg.ProjectNamingError(map[string]ManagedProject{"~/work/app": {Skills: off}}, "merge"); err != nil {
		t.Fatalf("managed project: %v", err)
	}
}

func TestTargetConfig_NamingModeConfigError(t *testing.T) {
	off := false
	for _, tc := range []struct {
		name string
		tc   TargetConfig
		mode string
		fix  string // empty when no error is expected
	}{
		{"prefixed inheriting merge", TargetConfig{Skills: &ResourceTargetConfig{TargetNaming: "prefixed"}}, "merge", "set mode: copy on the target"},
		{"prefixed inheriting the default", TargetConfig{Skills: &ResourceTargetConfig{TargetNaming: "prefixed"}}, "", "set mode: copy on the target"},
		{"own copy mode wins", TargetConfig{Skills: &ResourceTargetConfig{TargetNaming: "prefixed", Mode: "copy"}}, "merge", ""},
		{"skills off", TargetConfig{Skills: &ResourceTargetConfig{TargetNaming: "prefixed", Enabled: &off}}, "merge", ""},
		// A managed project's target cannot be edited as a target; its project settings hold the mode.
		{"managed project target", TargetConfig{Skills: &ResourceTargetConfig{TargetNaming: "prefixed"}, projectRoot: "/work/app"}, "merge", "set mode: copy in the project's skills settings"},
	} {
		err := tc.tc.NamingModeConfigError(tc.mode)
		switch {
		case tc.fix == "" && err != nil:
			t.Errorf("%s: unexpected error %v", tc.name, err)
		case tc.fix != "" && (err == nil || !strings.Contains(err.Error(), tc.fix)):
			t.Errorf("%s: error = %v, want fix %q", tc.name, err, tc.fix)
		}
	}
}

func TestValidateConfigForSync_ManagedProjectPrefixedHintPointsAtProject(t *testing.T) {
	problems := validateGlobalTarget("app@claude", TargetConfig{
		Skills:      &ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "skills")},
		projectRoot: "/work/app",
	}, "merge", "prefixed")
	if len(problems) != 1 || !strings.Contains(problems[0], "in the project's skills settings") {
		t.Fatalf("problems = %v, want the project settings hint", problems)
	}
}
