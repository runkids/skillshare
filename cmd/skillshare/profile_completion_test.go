package main

import (
	"slices"
	"strings"
	"testing"
)

func TestProfileCompletions(t *testing.T) {
	for name, shell := range shells {
		t.Run(name, func(t *testing.T) {
			section := func(script, start, end string) string {
				t.Helper()
				_, rest, found := strings.Cut(script, start)
				block, _, closed := strings.Cut(rest, end)
				if !found || !closed {
					t.Fatalf("missing completion section %q ... %q", start, end)
				}
				return block
			}
			require := func(block, fragment string) {
				t.Helper()
				if !strings.Contains(block, fragment) {
					t.Errorf("completion section lacks %q", fragment)
				}
			}
			script := shell.script
			var commands, flags string
			switch name {
			case "bash":
				registered := section(script, `local commands="`, `"`)
				if !slices.Contains(strings.Fields(registered), "profile") {
					t.Error("profile is not registered as a top-level command")
				}
				require(section(script, `case "${cword}" in`, "    esac"), `COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))`)
				block := section(script, `if [[ "${cmd}" == profile ]]; then`, "\n    fi")
				require(block, `if [[ ${cword} -eq 2 ]]; then`)
				require(block, `elif [[ "${cur}" == -* ]]; then`)
				require(block, `COMPREPLY=($(compgen -W "${profile_flags}" -- "${cur}"))`)
				commands = section(block, `compgen -W "`, `"`)
				flags = section(block, `local profile_flags="`, `"`)
				require(flags, `${global_flags}`)
				flags += " " + section(script, `local global_flags="`, `"`)
			case "zsh":
				require(section(script, "    commands=(", "\n    )"), "'profile:")
				require(script, "command)\n            _describe 'command' commands")
				require(script, "case ${words[1]} in")
				block := section(script, "\n                profile)\n", "\n                    ;;")
				require(block, "_arguments")
				commands = section(block, "'1:command:(", ")'")
				require(block, "$global_flags")
				flags = block + section(script, "    global_flags=(", "\n    )")
			case "fish":
				require(script, "complete -c skillshare -n __fish_skillshare_no_subcommand -a profile ")
				prefix := "complete -c skillshare -n '__fish_skillshare_using_command profile' "
				commands = section(script, prefix+"-a '", "'")
				for _, line := range strings.Split(script, "\n") {
					if rest, ok := strings.CutPrefix(line, prefix+"-l "); ok {
						flags += " --" + strings.Fields(rest)[0]
					}
				}
			case "powershell":
				require(section(script, "        '' = @(\n", "\n        )"), "@{ Name = 'profile';")
				commands = section(script, "$commands['profile'] = @(\n", "\n    )")
				require(commands, "Name = 'add'; Desc = 'Profile add'")
				require(commands, "Name = 'remove'; Desc = 'Profile remove'")
				flags = section(script, "$flags['profile'] = ", "\n")
				require(script, "$key = if ($elements.Count -le 1 -or ($elements.Count -eq 2 -and $wordToComplete -ne '')) { '' } else { $cmd }")
				require(script, "$commands[$key] | Where-Object")
				require(script, "$commands[$cmd] | Where-Object")
				require(script, "$flagKey = if ($flags.ContainsKey($nested)) { $nested } elseif ($flags.ContainsKey($cmd)) { $cmd } else { '' }")
				require(script, "if ($flags.ContainsKey($flagKey)) {")
				require(script, "$flags[$flagKey] | Where-Object")
			case "nushell":
				require(section(script, `def "nu-complete skillshare commands" [] {`, "\n}"), `{ value: "profile",`)
				require(section(script, `export extern "skillshare" [`, "\n]"), `command?: string@"nu-complete skillshare commands"`)
				commands = section(script, `def "nu-complete skillshare profile" [] { [`, "] }")
				flags = section(script, `export extern "skillshare profile" [`, "\n]")
				require(flags, `subcommand?: string@"nu-complete skillshare profile"`)
			default:
				t.Fatalf("unreviewed shell %q", name)
			}
			for _, command := range []string{"list", "show", "create", "edit", "rename", "delete", "use", "add", "remove"} {
				if name == "powershell" {
					require(commands, "@{ Name = '"+command+"';")
				} else if !slices.Contains(strings.Fields(commands), command) {
					t.Errorf("profile command definition lacks %q", command)
				}
			}
			// Exact tokens prevent another flag or a description from satisfying one.
			tokens := strings.FieldsFunc(flags, func(r rune) bool {
				return !(r >= 'a' && r <= 'z' || r == '-')
			})
			for _, flag := range []string{"description", "add-include", "remove-include", "add-exclude", "remove-exclude", "dry-run", "no-sync", "none", "discard-edits", "allow-empty", "json", "help", "global", "project"} {
				if !slices.Contains(tokens, "--"+flag) {
					t.Errorf("profile flag definition lacks --%s", flag)
				}
			}
			if name != "fish" {
				for _, short := range []string{"-n", "-h", "-g", "-p"} {
					if !slices.Contains(tokens, short) {
						t.Errorf("profile flag definition lacks %s", short)
					}
				}
			}
		})
	}
}
