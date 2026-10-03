package main

import (
	"reflect"
	"testing"
)

func TestPulledScopeSyncArgs(t *testing.T) {
	for gitRoot, want := range map[string][][]string{
		"":       {{"--global"}},
		"skills": {{"--global"}},
		"agents": {{"agents", "--global"}},
		"extras": {{"extras", "--global"}},
		"root":   {{"--global"}, {"agents", "--global"}},
	} {
		if got := pulledScopeSyncArgs(gitRoot); !reflect.DeepEqual(got, want) {
			t.Errorf("pulledScopeSyncArgs(%q) = %v, want %v", gitRoot, got, want)
		}
	}
}
