package main

import (
	"strings"
	"testing"
)

func TestProfileCompletions(t *testing.T) {
	for name, shell := range shells {
		for _, token := range []string{"profile", "list", "show", "create", "edit", "rename", "delete", "use", "description", "add-include", "remove-include", "add-exclude", "remove-exclude", "discard-edits", "allow-empty", "no-sync", "none"} {
			if !strings.Contains(shell.script, token) {
				t.Errorf("%s lacks %s", name, token)
			}
		}
	}
}

func TestProfileAssignCompletions(t *testing.T) {
	for name, shell := range shells {
		if name == "powershell" {
			for _, command := range []string{"add", "remove"} {
				if !strings.Contains(shell.script, "Name = '"+command+"'; Desc = 'Profile "+command+"'") {
					t.Errorf("%s lacks profile %s", name, command)
				}
			}
		} else if !strings.Contains(shell.script, "list show create edit rename delete use add remove") {
			t.Errorf("%s lacks profile membership commands", name)
		}
	}
}
