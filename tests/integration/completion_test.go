//go:build !online

package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

func TestCompletion_Bash_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "bash")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "complete -F _skillshare skillshare")
	for _, cmd := range []string{"sync", "install", "list", "target", "commit", "completion"} {
		result.AssertOutputContains(t, cmd)
	}
}

func TestCompletion_PluginUnimport_AllShells(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, shell := range []string{"bash", "zsh", "fish", "powershell", "nushell"} {
		t.Run(shell, func(t *testing.T) {
			result := sb.RunCLI("completion", shell)
			result.AssertSuccess(t)
			result.AssertOutputContains(t, "unimport")
		})
	}
}

func TestCompletion_Zsh_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "zsh")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "#compdef skillshare")
	result.AssertOutputContains(t, "_skillshare")
}

func TestCompletion_Fish_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "fish")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "complete -c skillshare")
	result.AssertOutputContains(t, "__fish_skillshare_no_subcommand")
}

func TestCompletion_PowerShell_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "powershell")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Register-ArgumentCompleter")
}

func TestCompletion_Nushell_OutputsScript(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "nushell")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "export extern \"skillshare\"")
}

func TestCompletion_BackupCleanupDescription(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, shell := range []string{"fish", "zsh", "nushell"} {
		t.Run(shell, func(t *testing.T) {
			result := sb.RunCLI("completion", shell)
			result.AssertSuccess(t)
			result.AssertOutputContains(t, "Remove old backups and abandoned incomplete directories")
		})
	}
}

func TestCompletion_MCPCheck_AllShells(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	for shell, subcommands := range map[string]string{
		"bash":       "add check edit import list remove restore serve",
		"zsh":        "(add check edit import list remove restore serve)",
		"fish":       "-a 'add check edit import list remove restore serve'",
		"powershell": "@{ Name = 'serve'; Desc = 'MCP serve' }",
		"nushell":    "[add check edit import list remove restore serve]",
	} {
		result := sb.RunCLI("completion", shell)
		result.AssertSuccess(t)
		result.AssertOutputContains(t, subcommands)
		result.AssertOutputContains(t, "no-dns")
		result.AssertOutputContains(t, "live")
		result.AssertOutputContains(t, "accept-tools")
		result.AssertOutputContains(t, "timeout")
		for _, flag := range []string{"tools-allow", "tools-deny"} {
			result.AssertOutputContains(t, flag)
		}
		result.AssertOutputNotContains(t, "direct-tools")
		result.AssertOutputNotContains(t, "tools-expose")
	}
}

func TestCompletion_CheckDiff_AllShells(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	for shell, flag := range map[string]string{
		"bash":       `check_flags="--json --diff `,
		"zsh":        "'--diff[Show changes since install]'",
		"fish":       "check' -l diff",
		"powershell": "'check' = '--json', '--diff'",
		"nushell":    "--diff                   # Show changes since install",
	} {
		result := sb.RunCLI("completion", shell)
		result.AssertSuccess(t)
		result.AssertOutputContains(t, flag)
	}
}

func TestCompletion_TargetCLIFlag_AllShells(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	for shell, flag := range map[string]string{
		"bash":       "--config-dir --cli",
		"zsh":        "'--cli[",
		"fish":       "-l cli -r",
		"powershell": "'--config-dir', '--cli'",
		"nushell":    "--cli: string",
	} {
		result := sb.RunCLI("completion", shell)
		result.AssertSuccess(t)
		result.AssertOutputContains(t, flag)
	}
}

func TestCompletion_PushPullFlag_AllShells(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	for shell, flag := range map[string]string{
		"bash":       `push_flags="--dry-run -n --pull`,
		"zsh":        "'--pull[",
		"fish":       "using_command push' -l pull",
		"powershell": "'push' = '--dry-run', '-n', '--pull'",
		"nushell":    "--pull                   # Merge remote",
	} {
		result := sb.RunCLI("completion", shell)
		result.AssertSuccess(t)
		result.AssertOutputContains(t, flag)
	}
}

func TestCompletion_Subcommands_AllShells(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	for shell, subcommands := range map[string][]string{
		"bash": {
			`backup_subcmds="files agents"`,
			`backup_files_subcmds="list show restore"`,
			`extras_subcmds="init list remove collect source memory"`,
			`audit_rules_subcmds="disable enable severity reset init"`,
			`ui_subcmds="start stop"`,
		},
		"zsh": {
			"'files:Versions of single files skillshare rewrote'",
			"'1:files command:(list show restore)'",
			"'1:rules command:(disable enable severity reset init)'",
			"'1:subcommand:(start stop)'",
		},
		"fish": {
			"backup' -a files",
			"backup files' -a 'list show restore'",
			"audit rules' -a 'disable enable severity reset init'",
			"ui' -a 'start stop'",
		},
		"powershell": {
			"'backup files' = @(",
			"'audit rules' = @(",
			"@{ Name = 'start'; Desc = 'Start a background UI server' }",
			"@{ Name = 'plugins'; Desc = 'Sync plugins' }",
		},
		"nushell": {
			`export extern "skillshare backup files"`,
			`export extern "skillshare audit rules"`,
			`export extern "skillshare status"`,
			`subcommand?: string@"nu-complete skillshare ui"`,
		},
	} {
		result := sb.RunCLI("completion", shell)
		result.AssertSuccess(t)
		for _, s := range subcommands {
			result.AssertOutputContains(t, s)
		}
		// Completed once, but not in the CLI: backup restore, extras mode, hub index --audit-skills.
		result.AssertOutputNotContains(t, "Restore from backup")
		result.AssertOutputNotContains(t, "Change sync mode or flatten")
		result.AssertOutputNotContains(t, "audit-skills")
	}
}

func TestCompletion_Hooks_AllShells(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	for shell, subcommands := range map[string]string{
		"bash":       "add disable edit enable import list remove restore sync",
		"zsh":        "(add disable edit enable import list remove restore sync)",
		"fish":       "-a 'add disable edit enable import list remove restore sync'",
		"powershell": "@{ Name = 'sync'; Desc = 'Hooks sync' }",
		"nushell":    "[add disable edit enable import list remove restore sync]",
	} {
		result := sb.RunCLI("completion", shell)
		result.AssertSuccess(t)
		result.AssertOutputContains(t, subcommands)
	}
}

func TestCompletion_UnsupportedShell_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "tcsh")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "unsupported shell")
}

func TestCompletion_NoArgs_ShowsUsage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Usage  skillshare completion")
}

func TestCompletion_Install_WritesFile(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("completion", "bash", "--install")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "installed")

	destPath := filepath.Join(sb.Home, ".local", "share", "bash-completion", "completions", "skillshare")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Errorf("expected completion script at %s, but file does not exist", destPath)
	}

	assertCandidates(t, completeIn(t, "bash", "--noprofile", "--norc", "-c", `
source "$1"
COMP_WORDS=(skillshare hooks '')
COMP_CWORD=2
_skillshare
printf '%s\n' "${COMPREPLY[@]}"
`, "bash", destPath), "add", "disable", "edit", "enable", "import", "list", "remove", "restore", "sync")
	assertCandidates(t, completeIn(t, "bash", "--noprofile", "--norc", "-c", `
source "$1"
COMP_WORDS=(skillshare sync '')
COMP_CWORD=2
_skillshare
printf '%s\n' "${COMPREPLY[@]}"
`, "bash", destPath), "agents", "extras", "mcp", "hooks", "plugins")
	assertCandidates(t, completeIn(t, "bash", "--noprofile", "--norc", "-c", `
source "$1"
COMP_WORDS=(skillshare mcp check --)
COMP_CWORD=3
_skillshare
printf '%s\n' "${COMPREPLY[@]}"
`, "bash", destPath), "--live", "--accept-tools", "--timeout", "--no-dns")
}

// zshCapture completes a command line in an interactive zsh under zpty and prints each
// candidate compadd receives. Arguments: the fpath directory, then the command line.
const zshCapture = `zmodload zsh/zpty
zpty z zsh -f -i
zpty -w z "PS1=''; fpath=($1 \$fpath); autoload -Uz compinit; compinit -u -d $1/.zcompdump"
zpty -w z "bindkey '^I' complete-word; comppostfuncs=( exit )"
zpty -w z 'compadd () {
  if [[ ${@[1,(i)(-|--)]} == *-(O|A|D)\ * ]]; then builtin compadd "$@"; return $?; fi
  typeset -a __hits
  builtin compadd -A __hits "$@"
  local h; for h in $__hits; do print -r -- "HIT:$h"; done
}'
zpty -w z "$2"$'\t'
local line
while zpty -r z line; do
  line=${line%$'\r'}
  # The first candidate can share a line with the echoed input; skip the echoed definition.
  [[ $line == *HIT:* && $line != *'HIT:$h'* ]] && print -r -- ${line##*HIT:}
done
zpty -d z
`

// completeIn runs a real shell and returns its candidates, one per line; it skips when
// the shell is not installed, as on most CI images.
func completeIn(t *testing.T, shell string, args ...string) []string {
	t.Helper()
	if _, err := exec.LookPath(shell); err != nil {
		t.Skipf("%s is not installed", shell)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", shell, args, err, out)
	}
	var candidates []string
	for _, line := range strings.Split(string(out), "\n") {
		// fish prints "candidate<TAB>description".
		if name, _, _ := strings.Cut(strings.TrimSpace(line), "\t"); name != "" {
			candidates = append(candidates, name)
		}
	}
	return candidates
}

func assertCandidates(t *testing.T, got []string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("missing completion %q in %q", w, got)
		}
	}
}

func TestCompletion_Fish_CompletesMCPCheck(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	result := sb.RunCLI("completion", "fish")
	result.AssertSuccess(t)
	script := filepath.Join(sb.Home, "skillshare.fish")
	if err := os.WriteFile(script, []byte(result.Stdout), 0644); err != nil {
		t.Fatal(err)
	}

	assertCandidates(t, completeIn(t, "fish", "--no-config", "-c", "source "+script+"; complete -C 'skillshare mcp '"), "add", "check", "edit", "import", "list", "remove", "restore")
	assertCandidates(t, completeIn(t, "fish", "--no-config", "-c", "source "+script+"; complete -C 'skillshare mcp check --'"), "--no-dns", "--live", "--accept-tools", "--timeout")
	assertCandidates(t, completeIn(t, "fish", "--no-config", "-c", "source "+script+"; complete -C 'skillshare mcp add --'"), "--url", "--target", "--sync", "--replace", "--dry-run", "--json", "--no-tui")
	assertCandidates(t, completeIn(t, "fish", "--no-config", "-c", "source "+script+"; complete -C 'skillshare backup '"), "files", "agents")
	assertCandidates(t, completeIn(t, "fish", "--no-config", "-c", "source "+script+"; complete -C 'skillshare hub index --'"), "--audit", "--full")
	assertCandidates(t, completeIn(t, "fish", "--no-config", "-c", "source "+script+"; complete -C 'skillshare extras '"), "init", "list", "remove", "collect", "source", "memory")
	assertCandidates(t, completeIn(t, "fish", "--no-config", "-c", "source "+script+"; complete -C 'skillshare extras memory '"), "init", "list", "show", "write", "delete", "instructions")
	assertCandidates(t, completeIn(t, "fish", "--no-config", "-c", "source "+script+"; complete -C 'skillshare extras memory write --'"), "--from", "--version")
	assertCandidates(t, completeIn(t, "fish", "--no-config", "-c", "source "+script+"; complete -C 'skillshare trash '"), "agents", "list", "restore", "delete", "empty")
}

func TestCompletion_Zsh_CompletesMCPCheck(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	result := sb.RunCLI("completion", "zsh")
	result.AssertSuccess(t)
	dir := filepath.Join(sb.Home, "zsh-functions")
	capture := filepath.Join(sb.Home, "capture.zsh")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_skillshare"), []byte(result.Stdout), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(capture, []byte(zshCapture), 0644); err != nil {
		t.Fatal(err)
	}

	completeIn(t, "zsh", "-n", filepath.Join(dir, "_skillshare"))
	assertCandidates(t, completeIn(t, "zsh", "-f", capture, dir, "skillshare mcp "), "add", "check", "edit", "import", "list", "remove", "restore")
	assertCandidates(t, completeIn(t, "zsh", "-f", capture, dir, "skillshare mcp check --"), "--no-dns", "--live", "--accept-tools", "--timeout")
	assertCandidates(t, completeIn(t, "zsh", "-f", capture, dir, "skillshare backup "), "files", "agents")
	assertCandidates(t, completeIn(t, "zsh", "-f", capture, dir, "skillshare backup files "), "list", "show", "restore")
	assertCandidates(t, completeIn(t, "zsh", "-f", capture, dir, "skillshare hub index --"), "--audit", "--full")
	assertCandidates(t, completeIn(t, "zsh", "-f", capture, dir, "skillshare extras "), "init", "list", "remove", "collect", "source", "memory")
	assertCandidates(t, completeIn(t, "zsh", "-f", capture, dir, "skillshare extras memory "), "init", "list", "show", "write", "delete", "instructions")
	assertCandidates(t, completeIn(t, "zsh", "-f", capture, dir, "skillshare extras memory write --"), "--from", "--version")
	assertCandidates(t, completeIn(t, "zsh", "-f", capture, dir, "skillshare trash "), "agents", "list", "restore", "delete", "empty")
}

func TestCompletion_Memory_AllShells(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for shell, commands := range map[string]string{
		"bash":       "init list show write delete instructions",
		"zsh":        "(init list show write delete instructions)",
		"fish":       "-a 'init list show write delete instructions'",
		"powershell": "'extras memory' = @(",
		"nushell":    "[init list show write delete instructions]",
	} {
		t.Run(shell, func(t *testing.T) {
			result := sb.RunCLI("completion", shell)
			result.AssertSuccess(t)
			result.AssertOutputContains(t, commands)
			for _, flag := range []string{"version", "search", "from", "update-mode"} {
				result.AssertOutputContains(t, flag)
			}
		})
	}
}

func TestCompletion_LinkUnlink_AllShells(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	for shell, want := range map[string][]string{
		"bash":       {"uninstall link unlink", `link_flags="--name --enable`},
		"zsh":        {"'link:", "'unlink:", "'--enable["},
		"fish":       {"-a link ", "-a unlink ", "using_command link' -l enable"},
		"powershell": {"Name = 'link'", "Name = 'unlink'", "'link' = '--name', '--enable'"},
		"nushell":    {`export extern "skillshare link"`, `export extern "skillshare unlink"`},
	} {
		result := sb.RunCLI("completion", shell)
		result.AssertSuccess(t)
		for _, s := range want {
			result.AssertOutputContains(t, s)
		}
	}
}

func TestCompletion_Move_AllShells(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	for shell, want := range map[string][]string{
		"bash":       {"unlink move list", `move_flags="--dry-run -n --force -f --json`},
		"zsh":        {"'move:", "'--force[Accept a target name collision]'"},
		"fish":       {"-a move ", "using_command move' -l force"},
		"powershell": {"Name = 'move'", "'move' = '--dry-run', '-n', '--force', '-f', '--json'"},
		"nushell":    {`export extern "skillshare move"`, "--force(-f)"},
	} {
		result := sb.RunCLI("completion", shell)
		result.AssertSuccess(t)
		for _, s := range want {
			result.AssertOutputContains(t, s)
		}
	}
}

// These assertions are scoped to project so another command cannot satisfy them.
func TestCompletion_Project_AllShells(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, shell := range []string{"bash", "zsh", "fish", "powershell", "nushell"} {
		t.Run(shell, func(t *testing.T) {
			result := sb.RunCLI("completion", shell)
			result.AssertSuccess(t)
			start := strings.Index(result.Stdout, "# Project command")
			end := strings.Index(result.Stdout, "# End project command")
			if start < 0 || end < start {
				t.Fatal("missing project completion block")
			}
			block := result.Stdout[start:end]
			registration := map[string]string{
				"bash":       "target project upgrade",
				"zsh":        "'project:Manage project folders'",
				"fish":       "-a project -d 'Manage project folders'",
				"powershell": "@{ Name = 'project'; Desc = 'Manage project folders' }",
				"nushell":    `{ value: "project", description: "Manage project folders" }`,
			}
			if !strings.Contains(result.Stdout, registration[shell]) {
				t.Error("missing top-level project command")
			}
			for _, flag := range []string{"target", "name", "skills", "no-skills", "agents", "no-agents", "skills-mode", "agents-mode", "skills-naming", "skills-include", "skills-exclude", "agents-include", "agents-exclude", "sync", "dry-run", "yes", "json", "help", "global"} {
				pattern := "--" + flag
				if shell == "fish" {
					pattern = "-l " + flag
				}
				// Delimit the flag so --skills-mode cannot satisfy --skills.
				if !strings.Contains(block, pattern+" ") && !strings.Contains(block, pattern+"\n") && !strings.Contains(block, pattern+"'") && !strings.Contains(block, pattern+"[") && !strings.Contains(block, pattern+":") && !strings.Contains(block, pattern+"(") {
					t.Errorf("project completions missing flag %q", flag)
				}
			}
			// Tokenize values and short flags so xcode-codex cannot satisfy
			// codex, nor --no-skills satisfy -n.
			tokens := strings.FieldsFunc(block, func(r rune) bool {
				return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-')
			})
			for _, short := range []string{"n", "h", "g"} {
				found := slices.Contains(tokens, "-"+short)
				if shell == "fish" {
					found = strings.Contains(block, "-s "+short+"\n")
				}
				if !found {
					t.Errorf("project completions missing short flag -%s", short)
				}
			}
			if shell == "nushell" {
				for _, flag := range []string{"skills-mode", "agents-mode"} {
					if !strings.Contains(block, "--"+flag+`: string@"nu-complete skillshare sync-mode"`) {
						t.Errorf("missing %s mode completer", flag)
					}
				}
				start := strings.Index(result.Stdout, `def "nu-complete skillshare sync-mode"`)
				if start < 0 {
					t.Fatal("missing sync-mode completer")
				}
				end := strings.Index(result.Stdout[start:], "}")
				if end < 0 {
					t.Fatal("unterminated sync-mode completer")
				}
				modeTokens := strings.FieldsFunc(result.Stdout[start:start+end], func(r rune) bool { return !(r >= 'a' && r <= 'z') })
				tokens = append(tokens, modeTokens...)
			}
			for _, word := range []string{"list", "show", "add", "edit", "sync", "remove", "merge", "symlink", "copy", "flat", "standard", "prefixed"} {
				if !slices.Contains(tokens, word) {
					t.Errorf("project completions missing %q", word)
				}
			}
			for name := range config.ProjectTargets() {
				if !slices.Contains(tokens, name) {
					t.Errorf("project target completions missing %q", name)
				}
			}
		})
	}
}

func TestCompletion_Project_RealShells(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			if _, err := exec.LookPath(shell); err != nil {
				t.Skipf("%s is not installed", shell)
			}
			result := sb.RunCLI("completion", shell)
			result.AssertSuccess(t)
			dir := t.TempDir()
			script := filepath.Join(dir, "_skillshare")
			capture := filepath.Join(dir, "capture.zsh")
			if err := os.WriteFile(script, []byte(result.Stdout), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(capture, []byte(zshCapture), 0644); err != nil {
				t.Fatal(err)
			}
			complete := func(line string) []string {
				switch shell {
				case "bash":
					return completeIn(t, shell, "--noprofile", "--norc", "-c", `source "$1"; read -ra COMP_WORDS <<< "$2"; [[ "$2" == *" " ]] && COMP_WORDS+=(""); COMP_CWORD=$((${#COMP_WORDS[@]}-1)); _skillshare; printf '%s\n' "${COMPREPLY[@]}"`, "bash", script, line)
				case "zsh":
					return completeIn(t, shell, "-f", capture, dir, line)
				default:
					return completeIn(t, shell, "--no-config", "-c", `source "$argv[1]"; complete -C "$argv[2]"`, script, line)
				}
			}
			assertCandidates(t, complete("skillshare "), "project")
			assertCandidates(t, complete("skillshare project "), "list", "show", "add", "edit", "sync", "remove")
			for _, action := range []string{"add", "edit"} {
				prefix := "skillshare project " + action + " example "
				assertCandidates(t, complete(prefix+"--"), "--target", "--name", "--skills", "--no-skills", "--agents", "--no-agents", "--skills-mode", "--agents-mode", "--skills-naming", "--skills-include", "--skills-exclude", "--agents-include", "--agents-exclude", "--sync", "--dry-run", "--json", "--help", "--global")
				assertCandidates(t, complete(prefix+"-"), "-n", "-h", "-g")
				for _, flag := range []string{"--skills-mode", "--agents-mode"} {
					assertCandidates(t, complete(prefix+flag+" "), "merge", "symlink", "copy")
				}
				assertCandidates(t, complete(prefix+"--skills-naming "), "flat", "standard", "prefixed")
				targets := complete(prefix + "--target ")
				for name := range config.ProjectTargets() {
					assertCandidates(t, targets, name)
				}
			}
			assertCandidates(t, complete("skillshare project remove example --"), "--yes", "--dry-run", "--json")
		})
	}
}
