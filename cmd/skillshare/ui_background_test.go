package main

import (
	"reflect"
	"testing"
)

func TestSplitUIModeRecognizesBackgroundSubcommands(t *testing.T) {
	mode, rest := splitUIMode([]string{"start", "--port", "8080"})
	if mode != uiStartMode {
		t.Fatalf("mode = %q, want %q", mode, uiStartMode)
	}
	if !reflect.DeepEqual(rest, []string{"--port", "8080"}) {
		t.Fatalf("rest = %#v", rest)
	}

	mode, rest = splitUIMode([]string{"--port", "8080"})
	if mode != "" {
		t.Fatalf("mode = %q, want empty", mode)
	}
	if !reflect.DeepEqual(rest, []string{"--port", "8080"}) {
		t.Fatalf("rest = %#v", rest)
	}
}

func TestUIChildArgsKeepsExistingUIAsForegroundServer(t *testing.T) {
	opts := uiBackgroundOptions{
		mode:           modeProject,
		host:           "0.0.0.0",
		port:           "8080",
		basePath:       "/studio",
		trustedOrigins: []string{"https://a.example", "https://b.example"},
		noOpen:         true,
	}

	got := uiChildArgs(opts)
	want := []string{"ui", "--no-open", "--host", "0.0.0.0", "--port", "8080", "--base-path", "/studio", "--trusted-origin", "https://a.example", "--trusted-origin", "https://b.example", "--project"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("uiChildArgs() = %#v, want %#v", got, want)
	}
}

func TestUIRestartArgsUseHiddenRestartCommand(t *testing.T) {
	opts := uiBackgroundOptions{mode: modeGlobal, host: "127.0.0.1", port: "19420", basePath: "/app", noOpen: true, clearCache: true}
	got := uiRestartArgs(opts)
	want := []string{"__ui-restart", "--no-open", "--host", "127.0.0.1", "--port", "19420", "--base-path", "/app", "--clear-cache", "--global"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("uiRestartArgs() = %#v, want %#v", got, want)
	}
}

func TestResolveUITrustedOrigins(t *testing.T) {
	t.Setenv("SKILLSHARE_UI_TRUSTED_ORIGINS", "https://env.example, https://env2.example")
	got, err := resolveUITrustedOrigins(nil)
	if err != nil || !reflect.DeepEqual(got, []string{"https://env.example", "https://env2.example"}) {
		t.Fatalf("from env: %#v, %v", got, err)
	}
	got, err = resolveUITrustedOrigins([]string{"HTTPS://Skills.Example.com/"})
	if err != nil || !reflect.DeepEqual(got, []string{"https://skills.example.com"}) {
		t.Fatalf("flag over env: %#v, %v", got, err)
	}
	if _, err := resolveUITrustedOrigins([]string{"skills.example.com"}); err == nil {
		t.Fatal("origin without a scheme accepted")
	}
}

// Stopping the UI does not use trusted origins, so a mistyped variable must not block it.
func TestParseUIBackgroundOptionsIgnoresAMistypedTrustedOrigin(t *testing.T) {
	t.Setenv("SKILLSHARE_UI_TRUSTED_ORIGINS", "skills.example.com")
	if _, err := parseUIBackgroundOptions(modeGlobal, []string{"--port", "19420"}); err != nil {
		t.Fatalf("parseUIBackgroundOptions: %v", err)
	}
}
