package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestExtrasTUIDetail_Remote(t *testing.T) {
	model := extrasListTUIModel{}
	got := model.renderExtrasDetail(extrasListEntry{Name: "rules", Remote: "example-org/ai-hub", Branch: "main", Commit: strings.Repeat("a", 40), State: "interrupted"})
	for _, text := range []string{"example-org/ai-hub", "main", "aaaaaaa", "interrupted"} {
		if !strings.Contains(got, text) {
			t.Fatalf("missing %q: %s", text, got)
		}
	}
}

func TestCheckExtras_ReadOnlyMissingAndUnsupportedFlags(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "config.yaml"))
	cfg := &config.Config{Source: filepath.Join(root, "skills"), Extras: []config.ExtraConfig{{Name: "rules", Remote: "example-org/ai-hub"}, {Name: "local"}}}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(config.ReadOnlyConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() {
		if err := cmdCheckExtras(modeGlobal, "", []string{"--json"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, `"state": "missing"`) || strings.Contains(output, `"name": "local"`) || !strings.Contains(output, "remote revision not checked") {
		t.Fatal(output)
	}
	if err := cmdCheckExtras(modeGlobal, "", []string{"--exit-code"}); err == nil {
		t.Fatal("missing state ignored")
	}
	for _, args := range [][]string{{"--all"}, {"--diff", "rules"}, {"--group", "rules"}} {
		if err := cmdCheckExtras(modeGlobal, "", args); err == nil {
			t.Fatal("unsupported args accepted", args)
		}
	}
	after, err := os.ReadFile(config.ReadOnlyConfigPath())
	if err != nil || string(after) != string(before) {
		t.Fatal("local report changed config")
	}
}

func TestStatusDoctorExtras_RemoteMissingAndInterrupted(t *testing.T) {
	root := t.TempDir()
	entries := []config.ExtraConfig{{Name: "rules", Remote: "example-org/ai-hub", Branch: "main"}}
	source := filepath.Join(root, "rules")
	for _, interrupted := range []bool{false, true} {
		if interrupted {
			stage := filepath.Join(root, ".skillshare-staging")
			if err := os.MkdirAll(stage, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(stage, "publishing-rules.json"), []byte(`{}`), 0644); err != nil {
				t.Fatal(err)
			}
		}
		outputs := []string{captureStdout(t, func() { printExtrasStatus(entries, func(config.ExtraConfig) string { return source }, false) }), captureStdout(t, func() { checkExtras(entries, &doctorResult{}, false, "", root, "", "", func() error { return nil }) })}
		for _, output := range outputs {
			if !strings.Contains(output, "example-org/ai-hub") {
				t.Fatal("origin missing", output)
			}
			if interrupted && !strings.Contains(output, "interrupted") {
				t.Fatal("interruption missing", output)
			}
			if !interrupted && !strings.Contains(output, "--kind extra --name rules --branch main -g") {
				t.Fatal("restore command missing", output)
			}
		}
	}
}
