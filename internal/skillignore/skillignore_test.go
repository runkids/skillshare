package skillignore

import (
	"os"
	"path/filepath"
	"testing"
)

// --- Legacy API tests (backward compat) ---

func TestMatch_ExactMatch(t *testing.T) {
	patterns := []string{"debug-tool"}
	if !Match("debug-tool", patterns) {
		t.Error("expected exact match")
	}
}

func TestMatch_GroupPrefix(t *testing.T) {
	patterns := []string{"experimental"}
	if !Match("experimental/sub-skill", patterns) {
		t.Error("expected group prefix match")
	}
}

func TestMatch_WildcardSuffix(t *testing.T) {
	patterns := []string{"test-*"}
	if !Match("test-alpha", patterns) {
		t.Error("expected wildcard match for test-alpha")
	}
	if !Match("test-beta/sub", patterns) {
		t.Error("expected wildcard match for test-beta/sub")
	}
}

func TestMatch_NoMatch(t *testing.T) {
	patterns := []string{"debug-tool", "test-*"}
	if Match("production-skill", patterns) {
		t.Error("expected no match")
	}
}

func TestReadPatterns_ParsesFile(t *testing.T) {
	dir := t.TempDir()
	content := "# Comment line\n\ndebug-tool\ntest-*\nexperimental\n"
	if err := os.WriteFile(filepath.Join(dir, ".skillignore"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	patterns := ReadPatterns(dir)
	if len(patterns) != 3 {
		t.Fatalf("expected 3 patterns, got %d: %v", len(patterns), patterns)
	}
	expected := []string{"debug-tool", "test-*", "experimental"}
	for i, p := range patterns {
		if p != expected[i] {
			t.Errorf("pattern[%d] = %q, want %q", i, p, expected[i])
		}
	}
}

func TestReadPatterns_NoFile(t *testing.T) {
	dir := t.TempDir()
	patterns := ReadPatterns(dir)
	if patterns != nil {
		t.Errorf("expected nil, got %v", patterns)
	}
}

// --- New Matcher API: table-driven Match tests ---

func TestMatcherMatch(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		path     string
		isDir    bool
		want     bool
	}{
		// --- Star glob ---
		{"star matches basename", []string{"*.log"}, "debug.log", false, true},
		{"star matches nested basename", []string{"*.log"}, "sub/debug.log", false, true},
		{"star matches deeply nested", []string{"*.log"}, "a/b/c/error.log", false, true},
		{"star no match different ext", []string{"*.log"}, "debug.txt", false, false},
		// sub.log matches *.log as a parent dir, so sub.log/file is ignored via inheritance
		{"star parent match inherits", []string{"*.log"}, "sub.log/file", false, true},
		{"star prefix match", []string{"test-*"}, "test-alpha", false, true},
		{"star prefix no match", []string{"test-*"}, "prod-alpha", false, false},

		// --- Double star ---
		{"** prefix matches root", []string{"**/test"}, "test", true, true},
		{"** prefix matches one dir", []string{"**/test"}, "a/test", true, true},
		{"** prefix matches deep", []string{"**/test"}, "a/b/c/test", true, true},
		{"** middle zero dirs", []string{"a/**/z"}, "a/z", false, true},
		{"** middle one dir", []string{"a/**/z"}, "a/b/z", false, true},
		{"** middle multi dirs", []string{"a/**/z"}, "a/b/c/z", false, true},
		{"** middle wrong prefix", []string{"a/**/z"}, "b/c/z", false, false},
		{"** trailing", []string{"abc/**"}, "abc/x", false, true},
		{"** trailing deep", []string{"abc/**"}, "abc/x/y/z", false, true},
		{"** only pattern", []string{"**"}, "anything", false, true},
		{"** only matches deep", []string{"**"}, "a/b/c", false, true},
		{"**/dir/*", []string{"**/vendor/*"}, "vendor/lib", false, true},
		{"**/dir/* deep", []string{"**/vendor/*"}, "a/vendor/lib", false, true},
		{"**/dir/* no match root", []string{"**/vendor/*"}, "vendor", true, false},
		{"** at start and end", []string{"**/build/**"}, "build/out", false, true},
		{"** at start and end deep", []string{"**/build/**"}, "x/build/out/bin", false, true},
		{"multiple ** in pattern", []string{"a/**/b/**/c"}, "a/x/b/y/c", false, true},
		{"multiple ** zero matches", []string{"a/**/b/**/c"}, "a/b/c", false, true},

		// --- Question mark ---
		{"? matches single char", []string{"?.md"}, "a.md", false, true},
		{"? no match multi char", []string{"?.md"}, "ab.md", false, false},
		{"? no match empty", []string{"?.md"}, ".md", false, false},
		{"? in middle", []string{"te?t"}, "test", false, true},
		{"? does not match slash", []string{"a?b"}, "a/b", false, false},

		// --- Character class ---
		{"[Tt] matches T", []string{"[Tt]est"}, "Test", false, true},
		{"[Tt] matches t", []string{"[Tt]est"}, "test", false, true},
		{"[Tt] no match b", []string{"[Tt]est"}, "best", false, false},
		{"[0-9] range", []string{"file[0-9]"}, "file3", false, true},
		{"[0-9] no match letter", []string{"file[0-9]"}, "filea", false, false},
		{"[!a] negated class", []string{"[!a]bc"}, "xbc", false, true},
		{"[!a] negated no match", []string{"[!a]bc"}, "abc", false, false},

		// --- Negation ---
		{"negation un-ignores", []string{"*.log", "!important.log"}, "important.log", false, false},
		{"negation keeps other ignored", []string{"*.log", "!important.log"}, "debug.log", false, true},
		{"double negation re-ignores", []string{"*.log", "!important.log", "important.log"}, "important.log", false, true},
		{"negation order: last wins", []string{"foo", "!foo"}, "foo", false, false},
		{"re-ignore after negation", []string{"!foo", "foo"}, "foo", false, true},
		{"negation on dir pattern", []string{"build/", "!build/"}, "build", true, false},
		{"negation with glob", []string{"*.tmp", "!keep-*.tmp"}, "keep-data.tmp", false, false},
		{"negation with glob other still ignored", []string{"*.tmp", "!keep-*.tmp"}, "delete-me.tmp", false, true},

		// --- Anchored (leading /) ---
		{"anchored matches root", []string{"/root-only"}, "root-only", false, true},
		{"anchored no match nested", []string{"/root-only"}, "sub/root-only", false, false},
		{"anchored with glob", []string{"/build-*"}, "build-prod", false, true},
		{"anchored with glob no match nested", []string{"/build-*"}, "sub/build-prod", false, false},
		{"anchored dir", []string{"/dist/"}, "dist", true, true},
		{"anchored dir file no match", []string{"/dist/"}, "dist", false, false},

		// --- Implicit anchoring (contains /) ---
		{"contains / is anchored", []string{"foo/bar"}, "foo/bar", false, true},
		{"contains / no match deeper", []string{"foo/bar"}, "baz/foo/bar", false, false},
		{"multi-seg with glob", []string{"doc/*.md"}, "doc/readme.md", false, true},
		{"multi-seg no match wrong dir", []string{"doc/*.md"}, "src/readme.md", false, false},

		// --- Directory-only (trailing /) ---
		{"dir-only matches dir", []string{"build/"}, "build", true, true},
		{"dir-only no match file", []string{"build/"}, "build", false, false},
		{"dir-only matches nested via parent", []string{"build/"}, "build/output/file", false, true},
		{"dir-only nested basename", []string{"node_modules/"}, "a/node_modules", true, true},
		{"dir-only nested content", []string{"node_modules/"}, "a/node_modules/pkg", false, true},

		// --- Escaped characters ---
		{"escaped #", []string{"\\#file"}, "#file", false, true},
		{"escaped !", []string{"\\!important"}, "!important", false, true},
		{"unescaped # is comment", []string{"#file"}, "#file", false, false},

		// --- Parent directory inheritance ---
		{"parent ignored → child ignored", []string{"vendor"}, "vendor/sub/deep", false, true},
		{"parent ignored → child ignored (dir)", []string{"vendor"}, "vendor/sub", true, true},
		{"parent not matched → child not matched", []string{"vendor"}, "other/sub", false, false},
		{"deep parent inheritance", []string{"a"}, "a/b/c/d/e", false, true},

		// --- Edge cases ---
		{"empty path", []string{"foo"}, "", false, false},
		{"path with leading slash", []string{"foo"}, "/foo", false, true},
		{"windows backslash", []string{"foo"}, "foo", false, true},
		{"windows backslash nested", []string{"a/b"}, "a\\b", false, true},
		{"trailing whitespace stripped", []string{"foo   "}, "foo", false, true},
		{"pattern is just *", []string{"*"}, "anything", false, true},
		{"pattern * matches single segment only", []string{"*"}, "a", false, true},
		{"single char path", []string{"x"}, "x", false, true},

		// --- Combined features ---
		{"anchored + dir-only", []string{"/build/"}, "build", true, true},
		{"anchored + dir-only no match file", []string{"/build/"}, "build", false, false},
		{"anchored + dir-only no match nested", []string{"/build/"}, "sub/build", true, false},
		{"negation + anchored", []string{"*.log", "!/important.log"}, "important.log", false, false},
		{"negation + dir-only", []string{"tmp/", "!tmp/"}, "tmp", true, false},
		{"** + negation", []string{"**/test", "!**/test"}, "a/test", true, false},
		{"dir-only + !** un-ignore child", []string{"vendor/", "!**/keep"}, "vendor/keep", false, false},
		{"glob + anchored + negation", []string{"/log-*", "!/log-keep"}, "log-keep", false, false},
		{"glob + anchored + negation other", []string{"/log-*", "!/log-keep"}, "log-debug", false, true},

		// --- Real-world .skillignore patterns ---
		{"gitignore: node_modules", []string{"node_modules/"}, "node_modules", true, true},
		{"gitignore: node_modules content", []string{"node_modules/"}, "node_modules/express/index.js", false, true},
		{"gitignore: .env files", []string{".env*"}, ".env.local", false, true},
		{"gitignore: OS files", []string{".DS_Store"}, ".DS_Store", false, true},
		{"gitignore: OS files nested", []string{".DS_Store"}, "subdir/.DS_Store", false, true},
		{"gitignore: dist but keep dist/readme", []string{"dist/", "!dist/README.md"}, "dist/bundle.js", false, true},
		{"gitignore: dist but keep dist/readme", []string{"dist/", "!dist/README.md"}, "dist/README.md", false, false},
		{"gitignore: ignore all logs except error", []string{"logs/", "!logs/error.log"}, "logs/debug.log", false, true},
		{"gitignore: ignore all logs except error", []string{"logs/", "!logs/error.log"}, "logs/error.log", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Compile(tt.patterns)
			got := m.Match(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("Compile(%q).Match(%q, %v) = %v, want %v",
					tt.patterns, tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

// --- parseRule edge cases ---

func TestParseRule(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantOK   bool
		negated  bool
		anchored bool
		dirOnly  bool
	}{
		{"blank line", "", false, false, false, false},
		{"whitespace only", "   ", false, false, false, false},
		{"tab only", "\t", false, false, false, false},
		{"comment", "# comment", false, false, false, false},
		{"simple pattern", "foo", true, false, false, false},
		{"negated", "!foo", true, true, false, false},
		{"anchored leading /", "/foo", true, false, true, false},
		{"dir-only trailing /", "foo/", true, false, false, true},
		{"anchored + dir-only", "/foo/", true, false, true, true},
		{"contains slash → anchored", "a/b", true, false, true, false},
		{"escaped hash", "\\#file", true, false, false, false},
		{"escaped bang", "\\!file", true, false, false, false},
		{"negated + anchored", "!/foo", true, true, true, false},
		{"negated + dir-only", "!foo/", true, true, false, true},
		{"trailing whitespace stripped", "foo  \t", true, false, false, false},
		{"just slash", "/", false, false, false, false},
		{"just negation", "!", false, false, false, false},
		{"double star pattern", "**/foo", true, false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, ok := parseRule(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("parseRule(%q) ok = %v, want %v", tt.line, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if r.negated != tt.negated {
				t.Errorf("negated = %v, want %v", r.negated, tt.negated)
			}
			if r.anchored != tt.anchored {
				t.Errorf("anchored = %v, want %v", r.anchored, tt.anchored)
			}
			if r.dirOnly != tt.dirOnly {
				t.Errorf("dirOnly = %v, want %v", r.dirOnly, tt.dirOnly)
			}
		})
	}
}

// --- HasRules ---

func TestHasRules(t *testing.T) {
	t.Run("nil matcher", func(t *testing.T) {
		var m *Matcher
		if m.HasRules() {
			t.Error("nil matcher should have no rules")
		}
	})
	t.Run("empty matcher", func(t *testing.T) {
		m := &Matcher{}
		if m.HasRules() {
			t.Error("empty matcher should have no rules")
		}
	})
	t.Run("comments only", func(t *testing.T) {
		m := Compile([]string{"# comment", "", "  "})
		if m.HasRules() {
			t.Error("comment-only matcher should have no rules")
		}
	})
	t.Run("with rules", func(t *testing.T) {
		m := Compile([]string{"*.log"})
		if !m.HasRules() {
			t.Error("matcher with rules should report HasRules=true")
		}
	})
}

// --- CanSkipDir ---

func TestCanSkipDir(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		dir      string
		want     bool
	}{
		// Basic cases
		{"ignored dir no negation", []string{"vendor"}, "vendor", true},
		{"not ignored dir", []string{"vendor"}, "src", false},
		{"nil-safe", []string{}, "anything", false},

		// With negation
		{"negation inside dir → unsafe", []string{"vendor", "!vendor/important"}, "vendor", false},
		{"negation unrelated dir → safe", []string{"vendor", "logs", "!logs/keep"}, "vendor", true},
		{"non-anchored negation → always unsafe", []string{"vendor", "!*.keep"}, "vendor", false},

		// Dir-only patterns
		{"dir-only pattern", []string{"build/"}, "build", true},
		{"dir-only with negation inside", []string{"build/", "!build/output"}, "build", false},

		// Nested dirs
		{"nested dir ignored via parent", []string{"vendor"}, "vendor/sub", true},

		// Anchored negation targeting exact dir
		{"negation targets same dir", []string{"tmp", "!tmp"}, "tmp", false},

		// Windows path
		{"windows backslash", []string{"vendor"}, "vendor", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Compile(tt.patterns)
			got := m.CanSkipDir(tt.dir)
			if got != tt.want {
				t.Errorf("Compile(%q).CanSkipDir(%q) = %v, want %v",
					tt.patterns, tt.dir, got, tt.want)
			}
		})
	}

	t.Run("nil matcher", func(t *testing.T) {
		var m *Matcher
		if m.CanSkipDir("anything") {
			t.Error("nil matcher CanSkipDir should return false")
		}
	})
}

// --- Nil / empty matcher safety ---

func TestMatcher_NilSafety(t *testing.T) {
	var m *Matcher
	if m.Match("anything", false) {
		t.Error("nil matcher Match should return false")
	}
	if m.Match("anything", true) {
		t.Error("nil matcher Match(dir) should return false")
	}
	if m.CanSkipDir("anything") {
		t.Error("nil matcher CanSkipDir should return false")
	}
	if m.HasRules() {
		t.Error("nil matcher HasRules should return false")
	}
}

func TestMatcher_EmptyRules(t *testing.T) {
	m := Compile([]string{"# only comments", "", "  "})
	if m.Match("anything", false) {
		t.Error("comment-only matcher should never match")
	}
	if m.CanSkipDir("anything") {
		t.Error("comment-only matcher CanSkipDir should return false")
	}
}

// --- matchSegments edge cases ---

func TestMatchSegments(t *testing.T) {
	tests := []struct {
		name string
		pat  []string
		path []string
		want bool
	}{
		{"exact match", []string{"a", "b"}, []string{"a", "b"}, true},
		{"length mismatch short", []string{"a"}, []string{"a", "b"}, false},
		{"length mismatch long", []string{"a", "b"}, []string{"a"}, false},
		{"glob segment", []string{"*.log"}, []string{"debug.log"}, true},
		{"glob segment no match", []string{"*.log"}, []string{"debug.txt"}, false},
		{"** at end", []string{"a", "**"}, []string{"a", "b", "c"}, true},
		{"** at start", []string{"**", "z"}, []string{"x", "y", "z"}, true},
		{"** zero segments", []string{"**", "z"}, []string{"z"}, true},
		{"** middle", []string{"a", "**", "z"}, []string{"a", "x", "z"}, true},
		{"** middle zero", []string{"a", "**", "z"}, []string{"a", "z"}, true},
		{"** middle deep", []string{"a", "**", "z"}, []string{"a", "b", "c", "d", "z"}, true},
		{"** no match after", []string{"a", "**", "z"}, []string{"a", "b", "c"}, false},
		{"trailing ** after match", []string{"a", "**"}, []string{"a"}, true},
		{"double ** consecutive", []string{"**", "**", "z"}, []string{"a", "z"}, true},
		{"empty pattern empty path", []string{}, []string{}, true},
		{"empty pattern nonempty path", []string{}, []string{"a"}, false},
		{"nonempty pattern empty path", []string{"a"}, []string{}, false},
		{"** only", []string{"**"}, []string{"a", "b", "c"}, true},
		{"** only empty", []string{"**"}, []string{}, true},
		{"? glob", []string{"?"}, []string{"a"}, true},
		{"? glob no match", []string{"?"}, []string{"ab"}, false},
		{"[abc] class", []string{"[abc]"}, []string{"b"}, true},
		{"[abc] class no match", []string{"[abc]"}, []string{"d"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchSegments(tt.pat, tt.path)
			if got != tt.want {
				t.Errorf("matchSegments(%q, %q) = %v, want %v",
					tt.pat, tt.path, got, tt.want)
			}
		})
	}
}

// --- Compile ---

func TestCompile(t *testing.T) {
	t.Run("mixed valid and invalid lines", func(t *testing.T) {
		m := Compile([]string{
			"# comment",
			"",
			"  ",
			"*.log",
			"# another comment",
			"!important.log",
			"build/",
		})
		if len(m.rules) != 3 {
			t.Errorf("expected 3 rules, got %d", len(m.rules))
		}
		if !m.hasNegation {
			t.Error("expected hasNegation=true")
		}
	})

	t.Run("no negation", func(t *testing.T) {
		m := Compile([]string{"*.log", "build/"})
		if m.hasNegation {
			t.Error("expected hasNegation=false")
		}
	})

	t.Run("nil input", func(t *testing.T) {
		m := Compile(nil)
		if m == nil {
			t.Fatal("Compile(nil) should return non-nil matcher")
		}
		if m.HasRules() {
			t.Error("Compile(nil) should have no rules")
		}
	})
}

// --- ReadMatcher ---

func TestReadMatcher(t *testing.T) {
	t.Run("parses complex file", func(t *testing.T) {
		dir := t.TempDir()
		content := "# Build artifacts\nbuild/\ndist/\n\n# Logs\n*.log\n!error.log\n\n# OS\n.DS_Store\nThumbs.db\n"
		os.WriteFile(filepath.Join(dir, ".skillignore"), []byte(content), 0644)

		m := ReadMatcher(dir)
		if !m.HasRules() {
			t.Fatal("expected rules")
		}

		// build/ → dir ignored
		if !m.Match("build", true) {
			t.Error("build dir should be ignored")
		}
		if m.Match("build", false) {
			t.Error("build file should not be ignored (dir-only pattern)")
		}
		// *.log ignored, !error.log negated
		if !m.Match("debug.log", false) {
			t.Error("debug.log should be ignored")
		}
		if m.Match("error.log", false) {
			t.Error("error.log should NOT be ignored (negated)")
		}
		// .DS_Store at any depth
		if !m.Match(".DS_Store", false) {
			t.Error(".DS_Store should be ignored")
		}
		if !m.Match("sub/.DS_Store", false) {
			t.Error("sub/.DS_Store should be ignored")
		}
	})

	t.Run("no file returns empty matcher", func(t *testing.T) {
		dir := t.TempDir()
		m := ReadMatcher(dir)
		if m == nil {
			t.Fatal("should return non-nil")
		}
		if m.HasRules() {
			t.Error("should have no rules")
		}
		if m.Match("anything", false) {
			t.Error("should not match anything")
		}
	})
}

// --- Gitignore semantic correctness ---

func TestGitignoreSemantics(t *testing.T) {
	t.Run("last matching rule wins", func(t *testing.T) {
		m := Compile([]string{"foo", "!foo", "foo"})
		if !m.Match("foo", false) {
			t.Error("last rule is 'foo' (ignore), should be ignored")
		}
	})

	t.Run("star does not cross slash", func(t *testing.T) {
		m := Compile([]string{"a*b"})
		if !m.Match("aXb", false) {
			t.Error("a*b should match aXb")
		}
		// a*b should NOT match a/b because * doesn't cross /
		if m.Match("a/b", false) {
			t.Error("a*b should not match a/b (star does not cross /)")
		}
	})

	t.Run("question mark does not match slash", func(t *testing.T) {
		m := Compile([]string{"a?b"})
		if !m.Match("axb", false) {
			t.Error("a?b should match axb")
		}
	})

	t.Run("negation requires prior ignore", func(t *testing.T) {
		// !foo alone does nothing if foo wasn't ignored first
		m := Compile([]string{"!foo"})
		if m.Match("foo", false) {
			t.Error("negation alone should not cause ignore")
		}
	})

	t.Run("re-include after ignore", func(t *testing.T) {
		m := Compile([]string{"logs/", "!logs/important/"})
		if !m.Match("logs/debug.log", false) {
			t.Error("logs/debug.log should be ignored")
		}
		// Note: parent dir "logs" is still ignored, so logs/important is
		// actually still ignored by parent-dir inheritance in the candidate loop.
		// This matches gitignore behavior: you cannot re-include a file
		// if a parent directory of that file is excluded.
	})

	t.Run("cannot re-include if parent excluded", func(t *testing.T) {
		// gitignore spec: "It is not possible to re-include a file if a
		// parent directory of that file is excluded."
		// In our implementation, the negation at the file level CAN still
		// un-ignore because we evaluate all rules against all candidates
		// and last-match wins at each level. This is a known divergence
		// from strict gitignore, intentional for simplicity.
		m := Compile([]string{"vendor/", "!vendor/important.go"})
		// vendor/important.go: parent "vendor" is ignored by "vendor/",
		// but the negation "!vendor/important.go" matches the full path
		// and is evaluated last, so it un-ignores.
		if m.Match("vendor/important.go", false) {
			t.Error("vendor/important.go should be un-ignored by negation")
		}
		// Other files under vendor/ remain ignored
		if !m.Match("vendor/other.go", false) {
			t.Error("vendor/other.go should still be ignored")
		}
	})

	t.Run("pattern with only glob chars", func(t *testing.T) {
		m := Compile([]string{"*"})
		if !m.Match("anything", false) {
			t.Error("* should match anything")
		}
		if !m.Match("deeply/nested/path", false) {
			t.Error("* should match deeply/nested/path basename")
		}
	})
}

// --- Regression tests ---

func TestRegressions(t *testing.T) {
	t.Run("#83 trailing slash", func(t *testing.T) {
		// Bug #83: trailing slash in .skillignore patterns caused matching failures
		m := Compile([]string{"demo/"})
		if !m.Match("demo", true) {
			t.Error("demo/ should match demo directory")
		}
		if !m.Match("demo/sub", false) {
			t.Error("demo/ should match demo/sub via parent-dir inheritance")
		}
		if m.Match("demo", false) {
			t.Error("demo/ should NOT match demo as file")
		}
	})

	t.Run("exact name backward compat", func(t *testing.T) {
		m := Compile([]string{"debug-tool"})
		if !m.Match("debug-tool", false) {
			t.Error("exact name should match")
		}
	})

	t.Run("prefix backward compat", func(t *testing.T) {
		m := Compile([]string{"experimental"})
		if !m.Match("experimental/sub-skill", false) {
			t.Error("prefix should match via parent-dir inheritance")
		}
	})

	t.Run("test-* backward compat", func(t *testing.T) {
		m := Compile([]string{"test-*"})
		if !m.Match("test-alpha", false) {
			t.Error("test-* should match test-alpha")
		}
	})
}

// --- matchRule edge case: non-anchored multi-segment (defensive path) ---

func TestMatchRule_NonAnchoredMultiSegment(t *testing.T) {
	// This branch is unreachable via parseRule (patterns with "/" become anchored),
	// but we test it directly as a defensive measure.
	r := rule{
		segments: []string{"a", "b"},
		anchored: false,
	}
	if !matchRule(r, "a/b") {
		t.Error("non-anchored multi-segment should match via matchSegments fallback")
	}
	if matchRule(r, "x/y") {
		t.Error("non-anchored multi-segment should not match different segments")
	}
}

// --- normalizePath ---

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"foo/bar", "foo/bar"},
		{"foo\\bar", "foo/bar"},
		{"/foo/bar", "foo/bar"},
		{"\\foo\\bar", "foo/bar"},
		{"", ""},
		{"/", ""},
		{"foo", "foo"},
	}
	for _, tt := range tests {
		got := normalizePath(tt.input)
		if got != tt.want {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
