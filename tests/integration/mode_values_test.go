//go:build !online

package integration

import (
	"testing"

	"skillshare/internal/testutil"
)

// Every command with value-taking options must leave mode-looking values for
// its own parser. Opposite real/value modes make the old bug unambiguous.
func TestModeFlagsInOptionValues(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    string
		success bool
	}{
		{"install", []string{"install", "missing", "--kind"}, "--kind must be", false},
		{"new", []string{"new", "demo", "--pattern"}, "unknown pattern: -p", false},
		{"list", []string{"list", "--type"}, "invalid type \"-p\"", false},
		{"audit", []string{"audit", "--format"}, "unknown format: -p", false},
		{"audit-rules-pattern", []string{"audit", "rules", "list", "--format", "json", "--pattern"}, "\"rules\": null", true},
		{"audit-rules-severity", []string{"audit", "rules", "list", "--format", "json", "--severity"}, "\"rules\": [", true},
		{"enable", []string{"enable", "demo", "--kind"}, "--kind must be 'skill' or 'agent', got \"-p\"", false},
		{"disable", []string{"disable", "demo", "--kind"}, "--kind must be 'skill' or 'agent', got \"-p\"", false},
		{"backup-delete", []string{"backup", "--dry-run", "--delete"}, "--delete requires a backup timestamp", false},
		{"check", []string{"check", "--group"}, "group '-p' not found", false},
		{"update", []string{"update", "--group"}, "group '-p' not found", false},
		{"uninstall", []string{"uninstall", "--group"}, "group '-p' not found", false},
		{"analyze", []string{"analyze", "--filter"}, "No skills to analyze", true},
		{"search", []string{"search", "--limit"}, "--limit must be a positive number", false},
		{"log", []string{"log", "--tail"}, "No operations log entries", true},
		{"backup", []string{"backup", "--target"}, "target '-p' not found", false},
		{"restore", []string{"restore", "--from"}, "usage: skillshare restore", false},
		{"init", []string{"init", "--mode"}, "invalid --mode value \"-p\"", false},
		{"target", []string{"target", "claude", "--mode"}, "--mode requires a value", false},
		{"hub-add", []string{"hub", "add", "https://example.invalid/index.json", "--label"}, "Added hub \"-p\"", true},
		{"hub-index", []string{"hub", "index", "--source"}, "stat -p:", false},
		{"ui", []string{"ui", "--port"}, "unknown port", false},
		{"ui-restart", []string{"__ui-restart", "--port"}, "--port must be a number", false},
		{"mcp", []string{"mcp", "add", "demo", "--pi-options"}, "--pi-options takes a JSON object", false},
		{"mcp-check", []string{"mcp", "check", "--live", "--timeout"}, "got \"-p\"", false},
		{"hooks", []string{"hooks", "import", "--from"}, "unsupported hooks Agent \"-p\"", false},
		{"plugin", []string{"plugin", "inspect", "--target"}, "No plugins managed yet", true},
		{"extras-init", []string{"extras", "init", "demo", "--mode"}, "at least one --target is required", false},
		{"extras-collect", []string{"extras", "collect", "demo", "--from"}, "extra \"demo\" not found", false},
		{"extras-add-target", []string{"extras", "demo", "--add-target"}, "extra \"demo\" not found", false},
		{"extras-remove-target", []string{"extras", "demo", "--remove-target"}, "extra \"demo\" not found", false},
		{"extras-mode", []string{"extras", "demo", "--mode"}, "invalid mode \"-p\"", false},
		{"extras-memory", []string{"extras", "memory", "instructions", "--update-mode"}, "memory update mode must be passive or active", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			args := append(append([]string{}, tc.args...), "-p", "-g")
			result := sb.RunCLIInDir(sb.Root, args...)
			if tc.success {
				result.AssertSuccess(t)
			} else {
				result.AssertFailure(t)
			}
			result.AssertOutputContains(t, tc.want)
		})
	}
}

// Preserve install name validation after mode extraction; unlike link names,
// installed skill names must start with a letter or number.
func TestInstall_ModeFlagAsName(t *testing.T) {
	for _, name := range []string{"-g", "-p"} {
		t.Run(name, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			source := sb.CreateSkill("original", map[string]string{"SKILL.md": "---\nname: original\ndescription: Example\n---\n# Example"})
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			result := sb.RunCLIInDir(sb.Root, "install", source, "--name", name, "--global", "--yes")
			result.AssertFailure(t)
			result.AssertOutputContains(t, "invalid skill name '"+name+"'")
		})
	}
}
