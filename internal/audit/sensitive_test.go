package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sensitiveRules(t testing.TB) []rule {
	t.Helper()
	rules, err := loadBuiltinRules()
	if err != nil {
		t.Fatal(err)
	}
	var selected []rule
	for _, r := range rules {
		if strings.HasPrefix(r.ID, "ssrf-") || strings.HasPrefix(r.ID, "unsafe-deserialization-") {
			selected = append(selected, r)
		}
	}
	if len(selected) == 0 {
		t.Fatal("missing sensitive built-in rules")
	}
	return selected
}

func TestSensitiveEvidence(t *testing.T) {
	rules := sensitiveRules(t)
	cases := []struct{ file, code, id, severity string }{
		{"guide.md", "Metadata: http://169.254.169.254/latest", "ssrf-metadata-0", SeverityHigh},
		{"x.txt", "Metadata endpoint fd00:ec2::254: it serves credentials", "ssrf-metadata-0", SeverityHigh},
		{"x.txt", "addr:fd00:ec2::254", "ssrf-metadata-0", SeverityHigh},
		{"x.txt", "fd00:ec2::254:", "ssrf-metadata-0", SeverityHigh},
		{"x.txt", "169.254.169.254:80", "ssrf-metadata-0", SeverityHigh},
		{"data.txt", "metadata.google.internal", "ssrf-metadata-0", SeverityHigh},
		{"x.json", `"http://[fd00:ec2::254]/latest"`, "ssrf-metadata-0", SeverityHigh},
		{"x.sh", "curl http://169.254.170.2/creds", "ssrf-metadata-0", SeverityCritical},
		{"x.py", `requests.get("http://100.100.100.200/latest")`, "ssrf-metadata-0", SeverityCritical},
		{"x.py", `requests.get("http://127.0.0.1:8080")`, "ssrf-internal-0", SeverityMedium},
		{"x.js", `fetch("https://10.0.0.1/admin")`, "ssrf-internal-0", SeverityMedium},
		{"x.ts", `axios.get("http://172.31.255.255")`, "ssrf-internal-0", SeverityMedium},
		{"x.rb", `Net::HTTP.get(URI("http://192.168.1.2"))`, "ssrf-internal-0", SeverityMedium},
		{"x.php", `file_get_contents("http://localhost/admin");`, "ssrf-internal-0", SeverityMedium},
		{"x.sh", `curl 'http://[::1]/admin'`, "ssrf-internal-0", SeverityMedium},
		{"x.js", `fetch("http://[fd12::1]/")`, "ssrf-internal-0", SeverityMedium},
		{"x.py", `httpx.get("http://service.internal/")`, "ssrf-internal-0", SeverityMedium},
		{"x.py", `requests.get(f"https://{host}/admin")`, "ssrf-dynamic-0", SeverityMedium},
		{"x.js", "fetch(`https://${host}/admin`)", "ssrf-dynamic-0", SeverityMedium},
		{"x.js", `fetch(req.query.url)`, "ssrf-dynamic-0", SeverityMedium},
		{"x.py", `requests.get(request.args["url"])`, "ssrf-dynamic-0", SeverityMedium},
		{"x.php", `file_get_contents($_GET['url']);`, "ssrf-dynamic-0", SeverityMedium},
		{"x.rb", `URI.open(params[:url])`, "ssrf-dynamic-0", SeverityMedium},
		{"x.PY", `pickle.load(stream)`, "unsafe-deserialization-python", SeverityHigh},
		{"x.PHTML", `unserialize($data)`, "unsafe-deserialization-php", SeverityHigh},
		{"x.py", `marshal.load(stream)`, "unsafe-deserialization-python", SeverityHigh},
		{"x.py", `marshal.loads(data)`, "unsafe-deserialization-python", SeverityHigh},
		{"x.py", `dill.load(stream)`, "unsafe-deserialization-python", SeverityHigh},
		{"x.py", `dill.loads(data)`, "unsafe-deserialization-python", SeverityHigh},
		{"x.py", `yaml.safe_load(a); yaml.load(b)`, "unsafe-deserialization-yaml", SeverityHigh},
		{"x.py", `torch.load(f(weights_only=True))`, "unsafe-deserialization-torch", SeverityHigh},
		{"x.py", `yaml.unsafe_load(data, Loader=yaml.SafeLoader)`, "unsafe-deserialization-yaml", SeverityHigh},
		{"x.py", `requests.get("http://[fe80::1]/")`, "ssrf-internal-0", SeverityMedium},
		{"x.js", `fetch("http://[::ffff:127.0.0.1]/")`, "ssrf-internal-0", SeverityMedium},
		{"x.rb", `Marshal.restore(data)`, "unsafe-deserialization-ruby", SeverityHigh},
		{"x.py", `pickle.load(stream)`, "unsafe-deserialization-python", SeverityHigh},
		{"x.py", `pickle.loads(data); marshal.loads(data); dill.load(stream)`, "unsafe-deserialization-python", SeverityHigh},
		{"x.py", `yaml.load(data)`, "unsafe-deserialization-yaml", SeverityHigh},
		{"x.py", `yaml.load(data, Loader=yaml.UnsafeLoader)`, "unsafe-deserialization-yaml", SeverityHigh},
		{"x.py", `yaml.load(a, Loader=yaml.SafeLoader); yaml.load(b)`, "unsafe-deserialization-yaml", SeverityHigh},
		{"x.py", `yaml.load(a); yaml.load(b, Loader=yaml.SafeLoader)`, "unsafe-deserialization-yaml", SeverityHigh},
		{"x.py", `yaml.load("""a", Loader=yaml.SafeLoader, "b""")`, "unsafe-deserialization-yaml", SeverityHigh},
		{"x.py", `torch.load("""a", weights_only=True, "b""")`, "unsafe-deserialization-torch", SeverityHigh},
		{"x.py", `yaml.load("Loader=yaml.SafeLoader")`, "unsafe-deserialization-yaml", SeverityHigh},
		{"x.py", `yaml.load(f(Loader=yaml.SafeLoader))`, "unsafe-deserialization-yaml", SeverityHigh},
		{"x.py", `torch.load(path)`, "unsafe-deserialization-torch", SeverityHigh},
		{"x.py", `torch.load(a, weights_only=True); torch.load(b, weights_only=False)`, "unsafe-deserialization-torch", SeverityHigh},
		{"x.py", `torch.load("weights_only=True")`, "unsafe-deserialization-torch", SeverityHigh},
		{"x.php", `unserialize($data);`, "unsafe-deserialization-php", SeverityHigh},
		{"x.rb", `Marshal.load(data)`, "unsafe-deserialization-ruby", SeverityHigh},
		{"x.js", `require('node-serialize').unserialize(data)`, "unsafe-deserialization-js", SeverityHigh},
	}
	for _, tc := range cases {
		t.Run(tc.file+tc.code, func(t *testing.T) {
			fs := ScanContentWithRules([]byte(tc.code), tc.file, rules)
			for _, f := range fs {
				if f.RuleID == tc.id {
					if f.Severity != tc.severity {
						t.Fatalf("severity %s", f.Severity)
					}
					if tc.id == "ssrf-dynamic-0" && f.Confidence >= .95 {
						t.Fatal("dynamic evidence must have lower confidence")
					}
					return
				}
			}
			t.Fatalf("missing %s: %+v", tc.id, fs)
		})
	}
	negatives := []struct{ file, code string }{
		{"x.py", `yaml.safe_load(data); yaml.load(data, Loader=yaml.SafeLoader); yaml.load(data, Loader=SafeLoader)`},
		{"x.py", `yaml.load(data, Loader=yaml.CSafeLoader); torch.load(path, weights_only=True)`},
		{"x.py", `yaml.load("""data""", Loader=yaml.SafeLoader); torch.load("""path""", weights_only=True)`},
		{"x.py", `json.loads(data)`}, {"x.js", "fetch(`https://example.com/${path}`)"}, {"x.rb", `JSON.parse(data)`}, {"x.php", `json_decode($data)`},
		{"x.js", `JSON.parse(data); fetch(url); fetch(config.url)`},
		{"x.md", `pickle.load(data); yaml.load(data); fetch(req.query.url); fetch("http://localhost")`},
		{"x.js", `fetch("https://example.com"); fetch("http://172.32.0.1"); fetch("http://192.169.1.1")`},
		{"x.py", `message = "http://localhost"`},
		{"x.txt", `metadata.google.internal.example.com 169.254.169.254.example.com`},
		{"x.txt", `metadata.google.internal.example.com. 169.254.169.254.example.com. 169.254.169.2540.`},
		{"x.txt", `[fd00:ec2::254:1] [1:fd00:ec2::254]`},
	}
	for _, tc := range negatives {
		if fs := ScanContentWithRules([]byte(tc.code), tc.file, rules); len(fs) != 0 {
			t.Errorf("benign %s %s: %+v", tc.file, tc.code, fs)
		}
	}
}

func TestSensitiveSkillCollection(t *testing.T) {
	for _, ext := range []string{".php", ".php3", ".php4", ".php5", ".phtml", ".PHP", ".PHTML"} {
		if !isScannable("unsafe" + ext) {
			t.Errorf("PHP script skipped: %s", ext)
		}
	}
	dir := t.TempDir()
	for name, code := range map[string]string{"SKILL.md": "# Test", "x.PHP": "<?php unserialize($data);", "x.py": "yaml.load(data)", "guide.md": "pickle.load(data)"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(code), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := ScanSkillWithRules(dir, sensitiveRules(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("want two language findings, got %+v", result.Findings)
	}
}

func TestMetadataContextSeverity(t *testing.T) {
	var rules []rule
	for _, r := range sensitiveRules(t) {
		if r.ID == "ssrf-metadata-0" {
			rules = append(rules, r)
		}
	}
	for _, tc := range []struct{ name, file, content, severity string }{
		{"prose warning", "SKILL.md", "Never call the instance metadata service at 169.254.169.254", SeverityHigh},
		{"sentence period", "SKILL.md", "Never call the instance metadata service at 169.254.169.254.", SeverityHigh},
		{"alias sentence period", "notes.txt", "Avoid metadata.google.internal.", SeverityHigh},
		{"plain text", "notes.txt", "Metadata: 169.254.169.254", SeverityHigh},
		{"configuration", "settings.yaml", "endpoint: http://169.254.169.254", SeverityHigh},
		{"json data", "settings.json", `{"endpoint":"http://169.254.169.254"}`, SeverityHigh},
		{"untyped text", "NOTES", "169.254.169.254", SeverityHigh},
		{"extensionless script", "creds", "#!/bin/bash\nT=$(curl -X PUT \"http://169.254.169.254/latest/api/token\" -H \"X-aws-ec2-metadata-token-ttl-seconds: 60\")", SeverityCritical},
		{"extensionless long flags", "creds", "#!/bin/bash\ncurl --silent -H \"X-aws-ec2-metadata-token: $T\" http://169.254.169.254/latest/meta-data/iam/security-credentials/", SeverityCritical},
		{"BOM shebang", "creds", "\xef\xbb\xbf#!/bin/bash\ncurl -X GET http://169.254.169.254/latest", SeverityCritical},
		{"later shebang is prose", "NOTES", "Example script:\n#!/bin/bash\nAvoid 169.254.169.254", SeverityHigh},
		{"source", "client.PY", `requests.get("http://169.254.169.254/latest")`, SeverityCritical},
		{"shell", "client.sh", "curl http://169.254.169.254/latest", SeverityCritical},
		{"markdown request", "SKILL.md", "Run `curl http://169.254.169.254/latest`", SeverityCritical},
		{"markdown bare host request", "SKILL.md", "Run `curl http://169.254.169.254`", SeverityCritical},
		{"text request", "notes.txt", `requests.get("http://169.254.169.254/latest")`, SeverityCritical},
		{"fenced request", "SKILL.md", "```sh\ncurl http://169.254.169.254/latest\n```", SeverityCritical},
		{"fenced source", "SKILL.md", "```python\nhost = '169.254.169.254'\n```", SeverityCritical},
		{"fenced makefile", "SKILL.md", "```makefile\nHOST = 169.254.169.254\n```", SeverityCritical},
		{"fenced dockerfile", "SKILL.md", "```dockerfile\nENV HOST=169.254.169.254\n```", SeverityCritical},
		{"fenced prose", "SKILL.md", "```text\nNever call 169.254.169.254\n```", SeverityHigh},
		{"fenced config", "SKILL.md", "```json\n{\"host\":\"169.254.169.254\"}\n```", SeverityHigh},
		{"http fence", "SKILL.md", "```http\nGET http://169.254.169.254/latest HTTP/1.1\n```", SeverityHigh},
		{"powershell fence", "SKILL.md", "```powershell\nInvoke-WebRequest http://169.254.169.254/latest\n```", SeverityHigh},
		{"after fence", "SKILL.md", "```sh\necho done\n```\nNever call 169.254.169.254", SeverityHigh},
		{"unrelated request", "SKILL.md", "`fetch(\"https://example.com\")`; avoid 169.254.169.254", SeverityHigh},
		{"public request with metadata path", "SKILL.md", `requests.get("https://ipinfo.io/169.254.169.254/json")`, SeverityHigh},
		{"public request with metadata query", "SKILL.md", `requests.get("https://ipinfo.io/lookup?ip=169.254.169.254")`, SeverityHigh},
		{"public request path in source", "client.py", `requests.get("https://ipinfo.io/169.254.169.254/json")`, SeverityHigh},
		{"public request path in code fence", "SKILL.md", "```python\nrequests.get(\"https://ipinfo.io/169.254.169.254/json\")\n```", SeverityHigh},
		{"public URL path in source variable", "client.py", `url = "https://ipinfo.io/169.254.169.254/json"`, SeverityHigh},
		{"public URL query in source variable", "client.py", `url = "https://ipinfo.io/lookup?ip=169.254.169.254"`, SeverityHigh},
		{"public URL path in code fence variable", "SKILL.md", "```python\nurl = \"https://ipinfo.io/169.254.169.254/json\"\n```", SeverityHigh},
		{"public template URL with metadata path in source", "client.js", "fetch(`https://example.com/169.254.169.254`)", SeverityHigh},
		{"adjacent public and metadata template requests in source", "client.js", "fetch(`https://example.com/`);fetch(`http://169.254.169.254/latest`);", SeverityCritical},
		{"metadata URL in source variable", "client.py", `url = "http://169.254.169.254/latest"`, SeverityCritical},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scans := []func([]byte, string, []rule) []Finding{ScanContentWithRules}
			if tc.file == "SKILL.md" {
				scans = append(scans, ScanMarkdownContentWithRules)
			}
			for _, scan := range scans {
				findings := scan([]byte(tc.content), tc.file, rules)
				if len(findings) != 1 || findings[0].Severity != tc.severity {
					t.Fatalf("want one %s metadata finding, got %+v", tc.severity, findings)
				}
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.file), []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := ScanSkillWithRules(dir, rules)
			if err != nil || len(result.Findings) != 1 || result.Findings[0].Severity != tc.severity {
				t.Fatalf("skill scan want one %s metadata finding: %+v, %v", tc.severity, result, err)
			}
			result, err = scanFileImpl(filepath.Join(dir, tc.file), rules, nil)
			if err != nil || len(result.Findings) != 1 || result.Findings[0].Severity != tc.severity {
				t.Fatalf("file scan want one %s metadata finding: %+v, %v", tc.severity, result, err)
			}
		})
	}
}

func TestMetadataExtensionlessCredentialsScript(t *testing.T) {
	// Store shell syntax as data only: neither request is ever executed.
	script := `#!/bin/bash
T=$(curl -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 60")
curl --silent -H "X-aws-ec2-metadata-token: $T" http://169.254.169.254/latest/meta-data/iam/security-credentials/
`
	for _, tc := range []struct {
		name, content string
		want          int
	}{
		{"metadata", script, 2},
		{"public twin", strings.ReplaceAll(script, "169.254.169.254", "example.com"), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "scripts"), 0700); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(dir, "scripts", "creds")
			if err := os.WriteFile(file, []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := ScanSkillWithRules(dir, sensitiveRules(t))
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Findings) != tc.want || result.HasCritical() != (tc.want > 0) {
				t.Fatalf("want %d critical findings, got %+v", tc.want, result.Findings)
			}
			for _, f := range result.Findings {
				if f.RuleID != "ssrf-metadata-0" || f.Severity != SeverityCritical || filepath.ToSlash(f.File) != "scripts/creds" {
					t.Fatalf("unexpected finding: %+v", f)
				}
			}
		})
	}
}

func TestMetadataContextOverrides(t *testing.T) {
	base, err := parseBuiltinRulesYAML()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, overlay, severity string }{
		{"partial inherits context", "- id: ssrf-metadata-0\n  files: ['*.md']", SeverityHigh},
		{"rule severity", "- id: ssrf-metadata-0\n  severity: CRITICAL", SeverityCritical},
		{"pattern severity", "- pattern: ssrf-metadata\n  severity: MEDIUM", SeverityMedium},
		{"custom regex", "- id: ssrf-metadata-0\n  severity: CRITICAL\n  pattern: custom\n  regex: '169\\.254\\.169\\.254'", SeverityCritical},
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
			for _, scan := range []func([]byte, string, []rule) []Finding{ScanContentWithRules, ScanMarkdownContentWithRules} {
				findings := scan([]byte("Never call 169.254.169.254"), "SKILL.md", rules)
				if len(findings) != 1 || findings[0].Severity != tc.severity {
					t.Fatalf("want %s, got %+v", tc.severity, findings)
				}
			}
		})
	}
}

func TestRuleFilesApplicability(t *testing.T) {
	raw := `rules:
- id: scoped
  severity: HIGH
  pattern: scoped
  message: scoped
  regex: DANGER
  files: ['*.py', 'scripts/*.php']
`
	yr, err := parseRulesYAML([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := compileRules(yr)
	if err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]int{"x.py": 1, "X.PY": 1, "nested/x.py": 1, "scripts/x.php": 1, "nested/scripts/x.php": 0, "x.md": 0, "": 0} {
		if got := len(ScanContentWithRules([]byte("DANGER"), file, rules)); got != want {
			t.Errorf("%q got %d want %d", file, got, want)
		}
		if got := len(ScanMarkdownContentWithRules([]byte("DANGER"), file, rules)); got != want {
			t.Errorf("markdown %q got %d want %d", file, got, want)
		}
	}
	overlay, err := parseRulesYAML([]byte("rules:\n- pattern: scoped\n  files: ['*.rb']\n- id: scoped\n  files: ['*.php']\n"))
	if err != nil {
		t.Fatal(err)
	}
	merged := mergeYAMLRules(yr, overlay)
	listed, err := buildCompiledList(yr, nil, overlay)
	if err != nil {
		t.Fatal(err)
	}
	// Listing must expose the effective applicability, including overrides.
	data, err := json.Marshal(listed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"files":["*.php"]`) {
		t.Fatal(string(data))
	}
	if len(listed) != 1 {
		t.Fatal(listed)
	}
	rules, err = compileRules(merged)
	if err != nil {
		t.Fatal(err)
	}
	if len(ScanContentWithRules([]byte("DANGER"), "x.php", rules)) != 1 || len(ScanContentWithRules([]byte("DANGER"), "x.py", rules)) != 0 {
		t.Fatal("files overlay not applied")
	}
	for _, files := range []string{"['[']", "['']", "['/tmp/*.py']", "['../*.py']", "['**/*.py']", "['a\\b']", "'*.py'"} {
		data := strings.Replace(raw, "['*.py', 'scripts/*.php']", files, 1)
		if err := ValidateRulesYAML(data); err == nil {
			t.Errorf("accepted malformed files %s", files)
		}
		data += "\n- id: disabled\n  enabled: false\n  files: ['[']\n"
		if _, err := parseRulesYAML([]byte(data)); err == nil {
			t.Errorf("load accepted malformed files %s", files)
		}
	}
}

// A safe call must never hide an unsafe sibling, regardless of string contents.
func FuzzSensitiveCallLocalSafety(f *testing.F) {
	for _, s := range []string{"", "weights_only=True", "Loader=yaml.SafeLoader", ")", "#"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		rules := sensitiveRules(t)
		for _, pair := range [][2]string{{`yaml.load(%q, Loader=yaml.SafeLoader)`, `yaml.load(data)`}, {`torch.load(%q, weights_only=True)`, `torch.load(data)`}} {
			safe := fmt.Sprintf(pair[0], s)
			for _, code := range []string{safe + "; " + pair[1], pair[1] + "; " + safe} {
				if fs := ScanContentWithRules([]byte(code), "x.py", rules); len(fs) == 0 {
					t.Fatalf("unsafe call suppressed: %s", code)
				}
			}
		}
	})
}

func TestRuleFilesLayers(t *testing.T) {
	resetForTest()
	t.Cleanup(resetForTest)
	global := filepath.Join(t.TempDir(), "audit-rules.yaml")
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(filepath.Dir(global), "config.yaml"))
	project := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(global, "rules:\n- id: unsafe-deserialization-python\n  files: ['*.txt']\n")
	globalRules, err := Rules()
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(ScanContentWithRules([]byte("pickle.load(x)"), "x.txt", globalRules), "unsafe-deserialization-python") {
		t.Fatal("global filter lost")
	}
	write(ProjectAuditRulesPath(project), "rules:\n- pattern: unsafe-deserialization\n  files: ['*.php']\n- id: unsafe-deserialization-python\n  files: []\n")
	projectRules, err := RulesWithProject(project)
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(ScanContentWithRules([]byte("pickle.load(x)"), "x.md", projectRules), "unsafe-deserialization-python") {
		t.Fatal("empty files must clear filter")
	}
	listed, err := ListRulesWithProject(project)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range listed {
		if r.ID == "unsafe-deserialization-python" && (len(r.Files) != 0 || r.Source != "project") {
			t.Fatal(r)
		}
	}
	if err := SetSeverity(ProjectAuditRulesPath(project), "unsafe-deserialization-php", SeverityMedium); err != nil {
		t.Fatal(err)
	}
	projectRules, err = RulesWithProject(project)
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(ScanContentWithRules([]byte("pickle.load(x)"), "x.md", projectRules), "unsafe-deserialization-python") {
		t.Fatal("mutation lost files reset")
	}
	for _, path := range []string{global, ProjectAuditRulesPath(project)} {
		write(path, "rules:\n- pattern: missing-pattern\n  files: ['[']\n")
		ResetGlobalCache()
		if _, err := RulesWithProject(project); err == nil {
			t.Fatalf("invalid layer accepted %s", path)
		}
		if _, err := ListRulesWithProject(project); err == nil {
			t.Fatalf("invalid listing layer accepted %s", path)
		}
		write(path, "rules: []\n")
	}
}

func hasRule(findings []Finding, id string) bool {
	for _, f := range findings {
		if f.RuleID == id {
			return true
		}
	}
	return false
}

func TestRuleFilesLinkAndSkillPaths(t *testing.T) {
	yr, err := parseRulesYAML([]byte("rules:\n- id: link\n  severity: HIGH\n  pattern: external-link\n  regex: example\n  files: ['docs/*.md']\n"))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := compileRules(yr)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range []string{"SKILL.md", "docs/guide.md", "guide.md"} {
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("[a](https://example.com)"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := ScanSkillWithRules(dir, rules)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 1 || filepath.ToSlash(result.Findings[0].File) != "docs/guide.md" {
		t.Fatal(result.Findings)
	}
}

func TestSensitiveMaintainerRegressions(t *testing.T) {
	rules := sensitiveRules(t)
	for _, tc := range []struct {
		file, line, id string
		want           bool
	}{
		{"x.py", `yaml.load(data, yaml.SafeLoader)`, "unsafe-deserialization-yaml", false},
		{"x.py", `yaml.load(data, yaml.CSafeLoader)`, "unsafe-deserialization-yaml", false},
		{"x.py", `yaml.load(data, yaml.UnsafeLoader)`, "unsafe-deserialization-yaml", true},
		{"x.py", `yaml.unsafe_load(data, yaml.SafeLoader)`, "unsafe-deserialization-yaml", true},
		{"x.sh", `curl --silent http://localhost:8080`, "ssrf-internal-0", true},
		{"x.sh", `curl -O http://localhost/file`, "ssrf-internal-0", true},
		{"x.sh", `curl --url=http://localhost/file`, "ssrf-internal-0", true},
		{"x.sh", `curl --fail http://10.0.0.5/`, "ssrf-internal-0", true},
		{"x.sh", `curl -H "X: y" http://localhost`, "ssrf-internal-0", true},
		{"x.sh", `[ "$(curl -s http://127.0.0.1:8000)" ]`, "ssrf-internal-0", true},
		{"x.sh", `curl http://localhost:3000|jq`, "ssrf-internal-0", true},
		{"x.sh", `curl --silent https://example.com`, "ssrf-internal-0", false},
		{"x.sh", `curl -H "Referer: http://localhost" https://example.com`, "ssrf-internal-0", false},
		{"x.sh", `curl -H "Referer: http://localhost" http://10.0.0.5/`, "ssrf-internal-0", true},
		{"x.sh", `curl -x http://localhost:8080 https://example.com`, "ssrf-internal-0", false},
		{"x.sh", `curl --proxy http://localhost:8080 http://10.0.0.1`, "ssrf-internal-0", true},
		{"x.sh", `curl -e http://localhost https://example.com`, "ssrf-internal-0", false},
		{"x.sh", `curl -T http://localhost/file https://example.com`, "ssrf-internal-0", false},
		{"x.sh", `curl --resolve http://localhost https://example.com`, "ssrf-internal-0", false},
		{"x.php", `public function unserialize(string $data) {}`, "unsafe-deserialization-php", false},
		{"x.php", `public function &unserialize(string $data) {}`, "unsafe-deserialization-php", false},
		{"x.php", `$value = unserialize($input);`, "unsafe-deserialization-php", true},
	} {
		if got := hasRule(ScanContentWithRules([]byte(tc.line), tc.file, rules), tc.id); got != tc.want {
			t.Errorf("%s: %s present=%v, want %v", tc.file, tc.line, got, tc.want)
		}
	}
}
