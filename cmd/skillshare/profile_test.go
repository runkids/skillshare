package main

import "testing"

func TestProfileArgs(t *testing.T) {
	for _, tc := range []struct { args []string; action string; fail bool }{
		{nil, "list", false},
		{[]string{"show", "web", "--json"}, "show", false},
		{[]string{"create", "web", "--description", "-p", "--add-include", "web*"}, "create", false},
		{[]string{"rename", "web", "work"}, "rename", false},
		{[]string{"use", "--none", "--json"}, "use", false},
		{[]string{"use", "a", "--none"}, "", true},
		{[]string{"use", "a", "--no-sync", "--discard-edits"}, "", true},
		{[]string{"create", "web", "--add-include"}, "", true},
		{[]string{"show"}, "", true},
		{[]string{"edit", "web", "--allow-empty"}, "", true},
		{[]string{"list", "extra"}, "", true},
		{[]string{"add", "web", "alpha"}, "", true},
	} {
		got, err := parseProfileArgs(tc.args)
		if (err != nil) != tc.fail { t.Fatalf("%v: %v", tc.args, err) }
		if err == nil && got.action != tc.action { t.Fatalf("%v: %s", tc.args, got.action) }
	}
}

func TestProfileReadOnlyStartup(t *testing.T) {
	for _, tc := range []struct { args []string; readOnly bool }{
		{nil, true}, {[]string{"list", "--json"}, true}, {[]string{"show", "a"}, true},
		{[]string{"use", "a", "--dry-run"}, true}, {[]string{"use", "--none"}, true},
		{[]string{"use", "missing"}, true}, {[]string{"create", "a"}, false},
		{[]string{"use", "a", "--no-sync"}, true}, {[]string{"use", "a", "--none"}, true},
		{[]string{"--help"}, true},
		{[]string{"edit", "a", "--description", "x"}, false},
		{[]string{"rename", "a", "b"}, false}, {[]string{"delete", "a"}, false},
	} {
		if got := profileSkipsMaintenance(tc.args); got != tc.readOnly { t.Fatalf("%v: %v", tc.args, got) }
	}
}
