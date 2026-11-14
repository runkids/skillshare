package main

import (
	"reflect"
	"testing"

	"skillshare/internal/profile"
)

func TestProfileArgs(t *testing.T) {
	description := "-p"
	for _, tc := range []struct {
		args []string
		want profileOptions
		fail bool
	}{
		{nil, profileOptions{action: "list"}, false},
		{[]string{"show", "web", "--json"}, profileOptions{action: "show", names: []string{"web"}, json: true}, false},
		{[]string{"create", "web", "--description", "-p", "--add-include", "web*"}, profileOptions{
			action: "create", names: []string{"web"},
			mutation: profile.Mutation{Description: &description, AddInclude: []string{"web*"}},
		}, false},
		{[]string{"rename", "web", "work"}, profileOptions{action: "rename", names: []string{"web", "work"}}, false},
		{[]string{"use", "--none", "--json"}, profileOptions{action: "use", names: []string{}, none: true, json: true}, false},
		{[]string{"use", "a", "--none"}, profileOptions{}, true},
		{[]string{"use", "a", "--no-sync", "--discard-edits"}, profileOptions{}, true},
		{[]string{"create", "web", "--add-include"}, profileOptions{}, true},
		{[]string{"show"}, profileOptions{}, true},
		{[]string{"edit", "web", "--allow-empty"}, profileOptions{}, true},
		{[]string{"list", "extra"}, profileOptions{}, true},
		{[]string{"add", "web", "alpha"}, profileOptions{}, true},
	} {
		got, err := parseProfileArgs(tc.args)
		if (err != nil) != tc.fail {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if err == nil && !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%v: got %+v, want %+v", tc.args, got, tc.want)
		}
	}
}

func TestProfileReadOnlyStartup(t *testing.T) {
	for _, tc := range []struct {
		args     []string
		readOnly bool
	}{
		{nil, true}, {[]string{"list", "--json"}, true}, {[]string{"show", "a"}, true},
		{[]string{"use", "a", "--dry-run"}, true}, {[]string{"use", "--none"}, true},
		{[]string{"use", "missing"}, true}, {[]string{"create", "a"}, false},
		{[]string{"use", "a", "--no-sync"}, true}, {[]string{"use", "a", "--none"}, true},
		{[]string{"--help"}, true},
		{[]string{"edit", "a", "--description", "x"}, false},
		{[]string{"rename", "a", "b"}, false}, {[]string{"delete", "a"}, false},
	} {
		if got := profileSkipsMaintenance(tc.args); got != tc.readOnly {
			t.Fatalf("%v: %v", tc.args, got)
		}
	}
}
