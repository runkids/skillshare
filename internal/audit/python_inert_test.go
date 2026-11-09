package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func credentialRules(t testing.TB, rules []rule) []rule {
	t.Helper()
	var selected []rule
	for _, r := range rules {
		if r.Pattern == "credential-access" {
			selected = append(selected, r)
		}
	}
	if len(selected) == 0 {
		t.Fatal("missing credential-access rules")
	}
	return selected
}

func TestPythonInertCredentialSeverity(t *testing.T) {
	builtin, err := loadBuiltinRules()
	if err != nil {
		t.Fatal(err)
	}
	rules := credentialRules(t, builtin)
	const cmd = "cat ~/.aws/credentials"
	for _, tc := range []struct{ name, file, content, severity string }{
		{"function docstring", "hook.py", "def path_matches(pattern, text):\n    \"\"\"Match a pattern.\n\n    Shell commands such as " + cmd + " start mid-text.\n    \"\"\"\n    return True", SeverityHigh},
		{"module docstring", "hook.py", "#!/usr/bin/env python3\n\"\"\"Blocks " + cmd + ".\"\"\"", SeverityHigh},
		{"raw single-quoted docstring", "hook.py", "def f():\n    r'''" + cmd + "'''  # example", SeverityHigh},
		{"escaped quote inside docstring", "hook.py", "\"\"\"a \\\"\"\" still open\n" + cmd + "\n\"\"\"", SeverityHigh},
		{"single-quoted docstring", "hook.py", "def f():\n    \"Blocks " + cmd + ".\"", SeverityHigh},
		{"CRLF docstring", "hook.py", "def f():\r\n    \"\"\"Example:\r\n    " + cmd + "\r\n    \"\"\"\r\n", SeverityHigh},
		{"comment", "hook.PY", "x = 1\n    # e.g. " + cmd, SeverityHigh},
		{"comment inside call", "hook.py", "run(\n    # " + cmd + "\n    args,\n)", SeverityHigh},
		{"call", "hook.py", "os.system(\"" + cmd + "\")", SeverityCritical},
		{"trailing comment after code", "hook.py", "x = 1  # " + cmd, SeverityCritical},
		{"string argument", "hook.py", "subprocess.run(\"\"\"" + cmd + "\"\"\", shell=True)", SeverityCritical},
		{"string argument on its own line", "hook.py", "subprocess.run(\n    \"\"\"" + cmd + "\"\"\",\n    shell=True,\n)", SeverityCritical},
		{"assigned string", "hook.py", "CMD = \"\"\"\n" + cmd + "\n\"\"\"", SeverityCritical},
		{"comment line inside assigned string", "hook.py", "CMD = \"\"\"\n# " + cmd + "\n\"\"\"", SeverityCritical},
		{"continued assignment", "hook.py", "CMD = \\\n    \"\"\"" + cmd + "\"\"\"", SeverityCritical},
		{"string used after close", "hook.py", "\"\"\"\n" + cmd + "\n\"\"\".split()", SeverityCritical},
		{"f-string evaluates", "hook.py", "f\"\"\"{os.system('" + cmd + "')}\"\"\"", SeverityCritical},
		{"unclosed string", "hook.py", "\"\"\"\n" + cmd, SeverityCritical},
		{"backslash-continued string", "hook.py", "x = '\\\n# '; os.system('" + cmd + "')", SeverityCritical},
		{"multi-line f-string field", "hook.py", "c = f\"{\n'''" + cmd + "'''\n}\"", SeverityCritical},
		{"docstring read back", "hook.py", "\"\"\"" + cmd + "\"\"\"\nimport os; os.system(__doc__)", SeverityCritical},
		{"shell comment keeps severity", "hook.sh", "# " + cmd, SeverityCritical},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check := func(path string, findings []Finding) {
				t.Helper()
				if len(findings) != 1 || findings[0].RuleID != "credential-access-aws-credentials" || findings[0].Severity != tc.severity {
					t.Fatalf("%s: want one %s aws finding, got %+v", path, tc.severity, findings)
				}
			}
			check("content", ScanContentWithRules([]byte(tc.content), tc.file, rules))
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.file), []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := ScanSkillWithRules(dir, rules)
			if err != nil {
				t.Fatal(err)
			}
			check("skill", result.Findings)
			result, err = scanFileImpl(filepath.Join(dir, tc.file), rules, nil)
			if err != nil {
				t.Fatal(err)
			}
			check("file", result.Findings)
		})
	}
}

func TestPythonInertCredentialOverrides(t *testing.T) {
	base, err := parseBuiltinRulesYAML()
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("# cat ~/.aws/credentials")
	for _, tc := range []struct{ name, overlay, severity string }{
		{"partial inherits context", "- id: credential-access-aws-credentials\n  files: ['*.py']", SeverityHigh},
		{"rule severity", "- id: credential-access-aws-credentials\n  severity: CRITICAL", SeverityCritical},
		{"pattern severity", "- pattern: credential-access\n  severity: CRITICAL", SeverityCritical},
		{"custom regex", "- id: credential-access-aws-credentials\n  severity: CRITICAL\n  pattern: credential-access\n  regex: 'cat ~/\\.aws/credentials'", SeverityCritical},
	} {
		t.Run(tc.name, func(t *testing.T) {
			overlay, err := parseRulesYAML([]byte("rules:\n" + tc.overlay))
			if err != nil {
				t.Fatal(err)
			}
			rules, err := compileRules(mergeYAMLRules(base, overlay))
			if err != nil {
				t.Fatal(err)
			}
			found := ScanContentWithRules(content, "hook.py", credentialRules(t, rules))
			if len(found) != 1 || found[0].Severity != tc.severity {
				t.Fatalf("want %s, got %+v", tc.severity, found)
			}
		})
	}
}
