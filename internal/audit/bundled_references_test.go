package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledReferences(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "references"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "references", "ok.md"), []byte("safe"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"100%.md", "a%20b.md", "space name.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("safe"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	outside := t.TempDir()
	for name, target := range map[string]string{"drive-relative": "C:outside.md", "internal": "references", "external": outside, "absolute-internal": filepath.Join(root, "references"), "loop": "loop", "broken": "absent"} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct{ name, text, rule string }{
		{"missing link", "[read](references/missing.md)", "dangling-link"},
		{"percent literal", "[read](100%.md)", ""},
		{"encoded literal fallback", "[read](a%20b.md)", ""},
		{"decoded markdown link", "[read](space%20name.md)", ""},
		{"code literal percent", "Read `path:a%20b.md`", ""},
		{"fence literal percent", "```skillshare-paths\na%20b.md\n```", ""},
		{"existing link", "[read](references/ok.md#part)", ""},
		{"missing image", "![preview](assets/missing.png)", "dangling-link"},
		{"existing image", "![preview](references/ok.md)", ""},
		{"linked image escape", "[![preview](../outside.png)](https://example.invalid)", "reference-escapes-root"},
		{"linked image inside root", "[![preview](references/ok.md)](https://example.invalid)", ""},
		{"multiline image escape", "![preview]\n(../outside.png)", "reference-escapes-root"},
		{"multiline image inside root", "![preview]\n(references/ok.md)", ""},
		{"reference image", "![preview][asset]\n\n[asset]: assets/missing.png", "dangling-link"},
		{"shortcut reference image escape", "![preview]\n\n[preview]: ../outside.png", "reference-escapes-root"},
		{"shortcut reference image inside root", "![preview]\n\n[preview]: references/ok.md", ""},
		{"shortcut reference image missing", "![preview]\n\n[preview]: references/missing.png", "dangling-link"},
		{"shortcut reference link escape", "[read]\n\n[read]: ../outside.md", "reference-escapes-root"},
		{"shortcut reference link inside root", "[read]\n\n[read]: references/ok.md", ""},
		{"shortcut reference link missing", "[read]\n\n[read]: references/missing.md", "dangling-link"},
		{"inline link takes precedence over same-label definition", "[read](references/ok.md)\n\n[read]: ../outside.md", ""},
		{"inline image takes precedence over same-label definition", "![preview](references/ok.md)\n\n[preview]: ../outside.png", ""},
		{"shortcut-looking link title is not a reference", "[open](references/ok.md \"[read]\")\n\n[read]: ../outside.md", ""},
		{"shortcut-looking image title is not a reference", "![preview](references/ok.md \"[read]\")\n\n[read]: ../outside.png", ""},
		{"shortcut-looking multiline link title is not a reference", "[open]\n(references/ok.md \"[read]\")\n\n[read]: ../outside.md", ""},
		{"shortcut-looking multiline image title is not a reference", "![preview]\n(references/ok.md \"[read]\")\n\n[read]: ../outside.png", ""},
		{"active shortcut beside inline title is still checked", "[open](references/ok.md \"[read]\") [read]\n\n[read]: ../outside.md", "reference-escapes-root"},
		{"escaped shortcut link remains literal", "\\[read]\n\n[read]: ../outside.md", ""},
		{"escaped backslash leaves shortcut link active", "\\\\[read]\n\n[read]: ../outside.md", "reference-escapes-root"},
		{"escaped image marker leaves shortcut link active", "\\![preview]\n\n[preview]: ../outside.png", "reference-escapes-root"},
		{"code path", "Read `./missing.md`.", "dangling-link"},
		{"common directory", "Read `references/missing.md`.", "dangling-link"},
		{"explicit path", "Read `path:missing.md`.", "dangling-link"},
		{"prefix examples", "Use `./`, `../`, `scripts/`, `references/`, `assets/`.", ""},
		{"extensionless example", "Use `./path`.", ""},
		{"glob example", "Use `./*.md`.", ""},
		{"template example", "Use `./<name>.md`.", ""},
		{"explicit directory", "Read `path:references/`.", ""},
		{"explicit directory escape", "Read `path:../`.", "reference-escapes-root"},
		{"explicit extensionless", "Read `path:missing`.", "dangling-link"},
		{"code benign twin", "Use `missing.md` or `some.identifier`.", ""},
		{"path fence", "```skillshare-paths\nreferences/missing.md\n```", "dangling-link"},
		{"ordinary fence", "```sh\n./missing.md\n```", ""},
		{"absolute example", "Example: `/example/config.json`", ""},
		{"required absolute", "[read](/example/config.json)", "reference-escapes-root"},
		{"explicit absolute", "Read `path:/example/config.json`", "reference-escapes-root"},
		{"parent escape", "[read](../missing.md)", "reference-escapes-root"},
		{"encoded parent escape", "[read](%2e%2e/missing.md)", "reference-escapes-root"},
		{"home escape", "[read](~/.synthetic/config)", "reference-escapes-root"},
		{"drive relative escape", "[read](C:outside.md)", "reference-escapes-root"},
		{"windows escape", "[read](C:/synthetic/config)", "reference-escapes-root"},
		{"symlinked parent", "[read](external/missing.md)", "reference-escapes-root"},
		{"symlink then parent", "[read](external/../references/ok.md)", "reference-escapes-root"},
		{"absolute internal symlink", "[read](absolute-internal/ok.md)", ""},
		{"absolute symlink then parent", "[read](" + root + "/external/../references/ok.md)", "reference-escapes-root"},
		{"absolute internal path", "[read](" + filepath.Join(root, "references", "ok.md") + ")", ""},
		{"windows backslash escape", "[read](C:\\synthetic\\config)", "reference-escapes-root"},
		{"internal symlink", "[read](internal/ok.md)", ""},
		{"broken symlink", "[read](broken)", "dangling-link"},
		{"drive relative symlink", "[read](drive-relative)", "reference-escapes-root"},
		{"symlink cycle", "[read](loop)", "reference-check-incomplete"},
		{"external exceptions", "[web](https://example.invalid/path) [anchor](#here) [uri](vscode:example)", ""},
	}
	if filepath.Separator == '\\' {
		cases = append(cases, struct{ name, text, rule string }{"native code path", "Read `references\\missing.md`", "dangling-link"})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := checkBundledReferences(root, []mdFileInfo{{relPath: "SKILL.md", absDir: root, data: []byte(tc.text)}}, nil)
			if tc.rule == "" {
				if len(findings) != 0 {
					t.Fatalf("unexpected: %+v", findings)
				}
				return
			}
			if len(findings) != 1 || findings[0].RuleID != tc.rule {
				t.Fatalf("want %s, got %+v", tc.rule, findings)
			}
			f := findings[0]
			if f.File != "SKILL.md" || f.Line != 1 && tc.name != "path fence" || !strings.Contains(tc.text, f.Snippet) {
				t.Fatalf("lost evidence: %+v", f)
			}
			if tc.rule == "reference-escapes-root" && f.Severity != SeverityHigh {
				t.Fatalf("escape severity: %+v", f)
			}
		})
	}
}

func TestBundledReferencesControlsAndNestedRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "references"), 0755); err != nil {
		t.Fatal(err)
	}
	files := []mdFileInfo{{relPath: "references/guide.md", absDir: filepath.Join(root, "references"), data: []byte("[inside](../SKILL.md) [outside](../../outside)")}}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, disabled := range []map[string]bool{nil, {"reference-escapes-root": true}, {"dangling-link": true}} {
		ctx := &AnalyzeContext{SkillPath: root, MDFiles: files, DisabledIDs: disabled}
		findings, err := (&structureAnalyzer{}).Analyze(ctx)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if disabled["reference-escapes-root"] {
			want = 0
		}
		if len(findings) != want {
			t.Fatalf("disabled=%v: %+v", disabled, findings)
		}
	}
}

func TestBundledReferencesNestedCodePaths(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	for _, dir := range []string{"references", "scripts", "assets"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"references/auth.md", "scripts/query_design.py", "assets/preview.png"} {
		if err := os.WriteFile(filepath.Join(root, file), []byte("synthetic fixture"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "references", "external.md")); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ target, rule string }{
		{"references/auth.md", ""},
		{"scripts/query_design.py", ""},
		{"assets/preview.png", ""},
		{"../scripts/query_design.py", ""},
		{"./auth.md", ""},
		{"./missing.md", "dangling-link"},
		{"../scripts/missing.py", "dangling-link"},
		{"references/missing.md", "dangling-link"},
		{"../../outside.md", "reference-escapes-root"},
		{"./external.md", "reference-escapes-root"},
		{"./external.md/../auth.md", "reference-escapes-root"},
		{"references/external.md", "reference-escapes-root"},
	}
	if filepath.Separator == '\\' {
		cases = append(cases, []struct{ target, rule string }{
			{`..\scripts\query_design.py`, ""},
			{`.\auth.md`, ""},
			{`..\scripts\missing.py`, "dangling-link"},
			{`..\..\outside.md`, "reference-escapes-root"},
		}...)
	}
	for _, tc := range cases {
		for _, text := range []string{"See `" + tc.target + "`.", "See `path:" + tc.target + "`.", "```skillshare-paths\n" + tc.target + "\n```"} {
			t.Run(text, func(t *testing.T) {
				files := []mdFileInfo{{relPath: "references/app-flow.md", data: []byte(text)}}
				findings := checkBundledReferences(root, files, nil)
				want := tc.rule
				if want == "reference-escapes-root" && strings.HasPrefix(text, "See `") && !strings.HasPrefix(text, "See `path:") {
					want = ""
				}
				if want == "" {
					if len(findings) != 0 {
						t.Fatalf("unexpected bundle reference: %+v", findings)
					}
				} else if len(findings) != 1 || findings[0].RuleID != want {
					t.Fatalf("want %s, got %+v", want, findings)
				}
			})
		}
	}
	// Markdown links/images retain document-relative resolution, even when
	// the same target is also declared as a code path on the same source line.
	files := []mdFileInfo{{relPath: "references/app-flow.md", data: []byte("[local](auth.md) ![image](../assets/preview.png)\n`references/auth.md` [missing](references/auth.md)")}}
	findings := checkBundledReferences(root, files, nil)
	if len(findings) != 1 || findings[0].RuleID != "dangling-link" || findings[0].Line != 2 || findings[0].Message != `broken local link: "references/auth.md" not found` {
		t.Fatalf("Markdown resolution or missing-target message changed: %+v", findings)
	}
}

func TestBundledReferencesSymlinkedRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "bundle")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "ok.md")
	if err := os.WriteFile(target, []byte("synthetic fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "internal")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{target, filepath.Join(alias, "ok.md"), "internal", root + "/../outside.md"} {
		want := ""
		if strings.Contains(path, "/../") {
			want = "reference-escapes-root"
		}
		if got := inspectBundledPath(alias, path); got != want {
			t.Errorf("symlinked root: %q classified %q, want %q", path, got, want)
		}
	}
}

func TestBundledReferencesSymlinkedParentSpellings(t *testing.T) {
	parent := t.TempDir()
	physicalParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(physicalParent, "physical", "bundle")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ok.md"), []byte("synthetic fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	parentAlias := filepath.Join(parent, "parent-alias")
	for link, dest := range map[string]string{
		parentAlias: filepath.Join(physicalParent, "physical"),
		filepath.Join(physicalParent, "physical", "source-alias"): root,
		filepath.Join(root, "internal"):                           filepath.Join(parentAlias, "bundle", "ok.md"),
		filepath.Join(root, "external"):                           t.TempDir(),
	} {
		if err := os.Symlink(dest, link); err != nil {
			t.Fatal(err)
		}
	}
	checker := newBundledPathChecker(filepath.Join(parentAlias, "source-alias"))
	for _, spelling := range []string{
		filepath.Join(parentAlias, "bundle"),
		filepath.Join(parent, "physical", "bundle"),
		root,
	} {
		for _, tc := range []struct{ suffix, rule string }{
			{"ok.md", ""}, {"internal", ""}, {"missing.md", "dangling-link"},
			{"../outside.md", "reference-escapes-root"},
			{"external/../ok.md", "reference-escapes-root"},
		} {
			// Keep symlink/.. components intact for the containment walker.
			target := spelling + "/" + tc.suffix
			if got := checker.inspect(target); got != tc.rule {
				t.Errorf("%q classified %q, want %q", target, got, tc.rule)
			}
		}
	}
}

func FuzzBundledReferenceContainment(f *testing.F) {
	for _, p := range []string{"../escape", "external/../safe", "./ok", "%2e%2e/escape"} {
		f.Add(p)
	}
	root := f.TempDir()
	alias := filepath.Join(f.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		f.Fatal(err)
	}
	checker := newBundledPathChecker(alias)
	f.Fuzz(func(t *testing.T, target string) {
		// Equivalent root spellings must preserve the component walk's result,
		// including missing paths, traversal, and unsupported path syntax.
		absolute := root + "/" + target
		if !filepath.IsAbs(target) && !isDriveReference(target) && !strings.HasPrefix(target, "~/") && len(absolute) <= maxReferencePath {
			if got, want := checker.inspect(absolute), checker.inspect(target); got != want {
				t.Fatalf("root spelling changed %q: %s != %s", target, got, want)
			}
		}
		// Lexical parent escapes must never become existence checks outside root.
		if !strings.HasPrefix(target, "../") || strings.ContainsAny(target, "\x00?#%\\") {
			return
		}
		status := inspectBundledPath(root, target)
		if status != "reference-escapes-root" {
			t.Fatalf("escape %q classified %q", target, status)
		}
	})
}

func TestBundledReferenceLimits(t *testing.T) {
	root := t.TempDir()
	for _, target := range []string{strings.Repeat("x", maxReferencePath+1), strings.Repeat("./", maxReferenceSteps+1)} {
		if got := inspectBundledPath(root, target); got != "reference-check-incomplete" {
			t.Errorf("limit/unsupported path classified %q", got)
		}
	}
	if got := inspectBundledPath(root, `unsupported\path`); filepath.Separator == '\\' {
		if got != "dangling-link" {
			t.Errorf("native Windows path classified %q, want dangling-link", got)
		}
	} else if got != "reference-check-incomplete" {
		t.Errorf("unsupported path classified %q, want reference-check-incomplete", got)
	}
	if got := inspectBundledPath(filepath.Join(root, "missing-root"), "file"); got != "reference-check-incomplete" {
		t.Fatalf("unavailable root: %s", got)
	}
}

func TestExtractMarkdownLinksMultilineDestinationKeepsSiblingReference(t *testing.T) {
	data := []byte("[open]\n(references/ok.md) ([read][ref])\n\n[ref]: ../outside.md\n")
	got := map[string]bool{}
	for _, link := range extractMarkdownLinks(data) {
		got[link.target] = true
	}
	for _, want := range []string{"references/ok.md", "../outside.md"} {
		if !got[want] {
			t.Errorf("missing link target %q in %v", want, got)
		}
	}
}

func TestBundledReferenceMaintainerCases(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "foo.py"), []byte("pass"), 0644); err != nil {
		t.Fatal(err)
	}
	text := "Text[^1]\n\n[^1]: Some note text\n\nEdit `../package.json` and see `../../CLAUDE.md`.\nRead `scripts/foo.py:12`.\n"
	files := []mdFileInfo{{relPath: "SKILL.md", absDir: root, data: []byte(text)}}
	if findings := checkBundledReferences(root, files, nil); len(findings) != 0 {
		t.Fatalf("ordinary docs: %+v", findings)
	}
}
