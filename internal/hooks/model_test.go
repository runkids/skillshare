package hooks

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGitValidation(t *testing.T) {
	valid := `{"commands":{"check.lint":{"events":["pre-commit"],"command":"echo ok"}}}`
	for _, tc := range []struct {
		name, binding string
		bad           bool
	}{
		{"command", valid, false},
		{"helper", `{"commands":{"check.lint":{"events":["pre-push"],"command":"{files}/check","parallel":false}},"files":{"check":"#!/bin/sh\nexit 0\n"}}`, false},
		{"unknown event", strings.ReplaceAll(valid, "pre-commit", "custom-event"), false},
		{"no commands", `{}`, true},
		{"scripts deferred", `{"scripts":{"pre-commit":"#!/bin/sh\n"}}`, true},
		{"mixed events", `{"events":{},"commands":{"x":{"events":["pre-commit"],"command":"echo ok"}}}`, true},
		{"mixed code", `{"code":"x","commands":{"x":{"events":["pre-commit"],"command":"echo ok"}}}`, true},
		{"bad name", strings.ReplaceAll(valid, "check.lint", "../check"), true},
		{"trailing dot", strings.ReplaceAll(valid, "check.lint", "check."), true},
		{"double dot", strings.ReplaceAll(valid, "check.lint", "check..lint"), true},
		{"reserved name", strings.ReplaceAll(valid, "check.lint", "pre-commit"), true},
		{"long name", strings.ReplaceAll(valid, "check.lint", strings.Repeat("x", 129)), true},
		{"empty events", strings.ReplaceAll(valid, `["pre-commit"]`, `[]`), true},
		{"duplicate events", strings.ReplaceAll(valid, `["pre-commit"]`, `["pre-commit","pre-commit"]`), true},
		{"invalid event", strings.ReplaceAll(valid, "pre-commit", "PreCommit"), true},
		{"empty command", strings.ReplaceAll(valid, "echo ok", " "), true},
		{"newline", strings.ReplaceAll(valid, "echo ok", `echo\nok`), true},
		{"carriage return", strings.ReplaceAll(valid, "echo ok", `echo\rok`), true},
		{"nul", strings.ReplaceAll(valid, "echo ok", `echo\u0000ok`), true},
		{"compound fi", strings.ReplaceAll(valid, "echo ok", "if true; then echo ok; fi"), true},
		{"compound done", strings.ReplaceAll(valid, "echo ok", "while true; do :; done"), true},
		{"compound esac", strings.ReplaceAll(valid, "echo ok", "case x in x) :;; esac"), true},
		{"compound brace", strings.ReplaceAll(valid, "echo ok", "{ :; }"), true},
		{"compound paren", strings.ReplaceAll(valid, "echo ok", "(echo ok)"), true},
		{"missing files", strings.ReplaceAll(valid, "echo ok", "{files}/check"), true},
		{"bad file", `{"commands":{"check.lint":{"events":["pre-commit"],"command":"echo ok"}},"files":{"../check":"bad"}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, err := ParseEntry([]byte(`{"bindings":{"git":` + tc.binding + `}}`))
			if (err != nil) != tc.bad {
				t.Fatalf("bad=%t, error=%v", tc.bad, err)
			}
			if err == nil {
				data, err := json.Marshal(e)
				must(t, err)
				_, err = ParseEntry(data)
				must(t, err)
			}
		})
	}
	for _, agent := range []string{"claude", "pi"} {
		_, err := ParseEntry([]byte(`{"bindings":{"` + agent + `":` + valid + `}}`))
		if err == nil {
			t.Fatalf("%s accepted Git commands", agent)
		}
	}
}

// Control bytes forbidden by the source contract must never reach a Git command.
func FuzzGitValidation(f *testing.F) {
	for _, s := range []string{"", "echo ok", "'", "\n", "\r", "\x00", "if :; then :; fi"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, command string) {
		data, err := json.Marshal(map[string]any{"bindings": map[string]any{"git": map[string]any{"commands": map[string]any{"check.lint": map[string]any{"events": []string{"pre-commit"}, "command": command}}}}})
		must(t, err)
		_, err = ParseEntry(data)
		if strings.ContainsAny(command, "\n\r\x00") && err == nil {
			t.Fatal("accepted forbidden control byte")
		}
	})
}
