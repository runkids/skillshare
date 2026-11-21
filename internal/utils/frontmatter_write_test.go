package utils

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSetFrontmatterList_SetMetadataTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\n---\n# Content\nBody text here"), 0644)

	err := SetFrontmatterList(path, "metadata.targets", []string{"claude"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "targets:") {
		t.Error("expected targets field in output")
	}
	if !strings.Contains(content, "claude") {
		t.Error("expected 'claude' in output")
	}
	if !strings.Contains(content, "# Content") {
		t.Error("body content should be preserved")
	}
	if !strings.Contains(content, "Body text here") {
		t.Error("body text should be preserved")
	}
	result := ParseFrontmatterList(path, "targets")
	if len(result) != 1 || result[0] != "claude" {
		t.Errorf("re-parsed targets = %v, want [claude]", result)
	}
}

func TestSetFrontmatterList_RemoveTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\nmetadata:\n  targets:\n    - claude\n---\n# Content"), 0644)

	err := SetFrontmatterList(path, "metadata.targets", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := ParseFrontmatterList(path, "targets")
	if result != nil {
		t.Errorf("expected nil targets after removal, got %v", result)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "name: my-skill") {
		t.Error("name field should be preserved")
	}
}

func TestSetFrontmatterList_RemovesLegacyTopLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\ntargets: [cursor]\n---\n# Content"), 0644)

	err := SetFrontmatterList(path, "metadata.targets", []string{"claude"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := ParseFrontmatterList(path, "targets")
	if len(result) != 1 || result[0] != "claude" {
		t.Errorf("re-parsed targets = %v, want [claude]", result)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "# Content") {
		t.Error("body should be preserved")
	}
}

func TestSetFrontmatterList_RemoveAll_CleansLegacyToo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\ntargets: [cursor]\nmetadata:\n  targets: [claude]\n---\n# Content"), 0644)

	err := SetFrontmatterList(path, "metadata.targets", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := ParseFrontmatterList(path, "targets")
	if result != nil {
		t.Errorf("expected nil after remove, got %v", result)
	}
}

func TestSetFrontmatterList_PreservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\ndescription: A great skill\nmetadata:\n  pattern: tool-wrapper\n---\n# Content"), 0644)

	err := SetFrontmatterList(path, "metadata.targets", []string{"claude", "cursor"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "name: my-skill") {
		t.Error("name should be preserved")
	}
	if !strings.Contains(content, "description: A great skill") {
		t.Error("description should be preserved")
	}
	if !strings.Contains(content, "pattern: tool-wrapper") {
		t.Error("metadata.pattern should be preserved")
	}
	result := ParseFrontmatterList(path, "targets")
	if len(result) != 2 {
		t.Errorf("expected 2 targets, got %v", result)
	}
}

func TestSetFrontmatterList_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("# Just content"), 0644)

	err := SetFrontmatterList(path, "metadata.targets", []string{"claude"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := ParseFrontmatterList(path, "targets")
	if len(result) != 1 || result[0] != "claude" {
		t.Errorf("expected [claude], got %v", result)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "# Just content") {
		t.Error("body should be preserved")
	}
}

func TestSetFrontmatterList_FileNotFound(t *testing.T) {
	err := SetFrontmatterList("/nonexistent/SKILL.md", "metadata.targets", []string{"claude"})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestSetFrontmatterList_MultipleTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\n---\n# Content"), 0644)

	err := SetFrontmatterList(path, "metadata.targets", []string{"claude", "cursor", "copilot"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := ParseFrontmatterList(path, "targets")
	if len(result) != 3 {
		t.Errorf("expected 3 targets, got %v", result)
	}
}

func TestSetFrontmatterList_TwoSpaceIndent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\n---\n"), 0644)

	if err := SetFrontmatterList(path, "metadata.targets", []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "metadata:\n  targets:\n    - claude\n") {
		t.Errorf("frontmatter not indented with 2 spaces:\n%s", data)
	}
}

func TestRewriteTargetsPlacementsAndScalars(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, bom := range []string{"", "\xef\xbb\xbf"} {
			for _, stale := range []string{"2026-06-30", "2026-06-30 12:34:56"} {
				for _, legacy := range []string{"", "targets: [legacy]\n"} {
					for _, target := range []string{"", "  targets: [old]\n"} {
						for _, values := range [][]string{nil, {"claude", "cursor"}} {
							fixed := "  sources: [{resource: a.md, usage_count: 012}]\n  status: experimental\n  superseded_by: docs/new.md\n  generated: {by: 'team:data', at: '2026-06-30T12:34:56+02:00'}\n  verified: {by: 'custom:actor', at: 2026-06-30T12:34:56+02:00}\n  x-extra: {items: [one, {key: value}]}\n  custom: \"012\\nquoted: value\"\n  stale_after: " + stale + "\n  note: |\n    text\n\n"
							input := bom + strings.ReplaceAll("---\nname: demo\n"+legacy+"metadata:\n"+fixed+target+"---\nbody\n", "\n", newline)
							before, err := ParseFrontmatterMap([]byte(input))
							if err != nil {
								t.Fatal(err)
							}
							out, err := RewriteFrontmatterList([]byte(input), "metadata.targets", values)
							if err != nil {
								t.Fatal(err)
							}
							after, err := ParseFrontmatterMap(out)
							if err != nil {
								t.Fatal(err)
							}
							delete(before, "targets")
							delete(after, "targets")
							delete(before["metadata"].(map[string]any), "targets")
							md := after["metadata"].(map[string]any)
							got, present := md["targets"]
							if values == nil && present || values != nil && (!present || !reflect.DeepEqual(got, []any{"claude", "cursor"})) {
								t.Fatalf("targets = %#v in %s", got, out)
							}
							delete(md, "targets")
							if !reflect.DeepEqual(before, after) || !strings.Contains(string(out), strings.ReplaceAll(fixed, "\n", newline)) || !strings.HasPrefix(string(out), bom+"---"+newline) || !strings.HasSuffix(string(out), "body"+newline) {
								t.Fatalf("unrelated data changed: %s", out)
							}
						}
					}
				}
			}
		}
	}
}

func TestRewriteTargetsInvalidYAML(t *testing.T) {
	if _, err := RewriteFrontmatterList([]byte("---\nname: [broken\n---\nbody\n"), "metadata.targets", []string{"claude"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRewriteTargetsRemovalKeepsPrecedingScalar(t *testing.T) {
	for _, tc := range []struct {
		header string
		want   map[string]any
	}{
		{"name: demo\nmetadata:\n  note: |+\n    keep\n  targets: [old]\n\n\n", map[string]any{"name": "demo", "metadata": map[string]any{"note": "keep\n"}}},
		{"name: demo\ndescription: |+\n  keep\ntargets: [old]\n\n\n", map[string]any{"name": "demo", "description": "keep\n"}},
		{"name: demo\ndescription: |+\n  keep\nmetadata:\n  targets: [old]\n\n\n", map[string]any{"name": "demo", "description": "keep\n"}},
		{"name: demo\ndescription: |+\n  keep\nmetadata: {targets: [old]}\n\n\n", map[string]any{"name": "demo", "description": "keep\n"}},
	} {
		input := "---\n" + tc.header + "---\nbody\n"
		out, err := RewriteFrontmatterList([]byte(input), "metadata.targets", nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ParseFrontmatterMap(out)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("header %q: got %#v want %#v", tc.header, got, tc.want)
		}
	}
}

func TestRewriteTargetsAllowsUnrelatedAliases(t *testing.T) {
	input := "---\nname: demo\ncommon: &common {status: stable}\nother: *common\nmetadata: {status: ok}\n---\nbody\n"
	out, err := RewriteFrontmatterList([]byte(input), "metadata.targets", []string{"claude"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "other: *common") {
		t.Fatalf("lost unrelated alias: %s", out)
	}
}

func TestRewriteTargetsFlow(t *testing.T) {
	type flowCase struct {
		field, header, body string
		keep                []string
	}
	cases := []flowCase{
		{"metadata.targets", `{name: demo, metadata: {status: "A\u0041",}, targets: [old], description: ok}`, "body\n", []string{`"A\u0041"`, "description: ok"}},
		{"metadata.targets", "{name: demo, metadata: {\n  # keep comment\n  status: \"A\\u0041\",\n  targets: [old],\n  other: [one, {x: y}]\n}, description: ok}", "body\n", []string{`"A\u0041"`, "description: ok", "# keep comment"}},
		{"metadata.targets", "name: demo\nmetadata:\n  {status: \"A\\u0041\", targets: [old]}\ndescription: ok", "body\n", []string{`"A\u0041"`, "description: ok"}},
		{"metadata.targets", `{name: demo, metadata: {status: "A\u0041"}, description: ok}`, "body\n", []string{`"A\u0041"`, "description: ok"}},
		{"metadata.targets", "name: demo\nmetadata: {sources: [{resource: a.md, usage_count: 012}], stale_after: 2026-06-30 12:34:56, targets: [old]}\ndescription: |\n  text\n", "body\n", []string{"usage_count: 012", "2026-06-30 12:34:56", "description: |"}},
		{"metadata.targets", `{name: demo, metadata: {sources: [{usage_count: 012}], stale_after: 2026-06-30 12:34:56, targets: [old]}}`, "body\n", []string{"usage_count: 012", "2026-06-30 12:34:56"}},
		{"metadata.targets", "name: demo\ndescription: Demo\nmetadata: # keep\n  # flow value\n  {status: stable, targets: [cursor]}\nother: retained", "body\n", []string{"metadata: # keep\n  # flow value\n", "other: retained"}},
		{"metadata.targets", `{name: demo, metadata: {status: "A\u0041", stale_after: 2026-06-30 12:34:56, targets: [old]}}`, "body\r\n", []string{`"A\u0041"`, "stale_after: 2026-06-30 12:34:56"}},
		{"metadata.targets", "name: demo\nmetadata: {status: \"A\\u0041\", stale_after: 2026-06-30 12:34:56, targets: [old]}", "body\r\n", []string{`"A\u0041"`, "stale_after: 2026-06-30 12:34:56"}},
	}
	for _, field := range []string{"targets", "metadata.targets"} {
		add := func(entries string, keep ...string) {
			header := "{name: demo, " + entries + "}"
			if field == "metadata.targets" {
				header = "name: demo\nmetadata: {" + entries + "}"
			}
			cases = append(cases, flowCase{field, header, "body\n", keep})
		}
		add("status: stable, # about status\n  targets: [cursor] # about targets\n", "# about status", "# about targets")
		add("status: stable, # about status\n  targets: [cursor], # about targets\n  k: v # about k\n", "# about status", "# about targets", "# about k")
		sibling := `later: {note: "}, ]", nested: [x, {text: 'it''s, }'}]}`
		add(sibling, sibling)
		add("targets: [old], "+sibling, sibling)
	}
	for _, tc := range cases {
		for _, newline := range []string{"\n", "\r\n"} {
			for _, values := range [][]string{nil, {"claude"}} {
				input := strings.ReplaceAll("---\n"+tc.header+"\n---\n", "\n", newline) + tc.body
				want, err := ParseFrontmatterMap([]byte(input))
				if err != nil {
					t.Fatal(err)
				}
				parent := want
				if tc.field == "metadata.targets" {
					delete(want, "targets")
					parent = want["metadata"].(map[string]any)
				}
				if values == nil {
					delete(parent, "targets")
				} else {
					parent["targets"] = []any{"claude"}
				}
				out, err := RewriteFrontmatterList([]byte(input), tc.field, values)
				if err != nil {
					t.Fatalf("%q: %v", tc.header, err)
				}
				got, err := ParseFrontmatterMap(out)
				if err != nil || !reflect.DeepEqual(want, got) {
					t.Fatalf("unexpected result: %s: %v", out, err)
				}
				for _, part := range tc.keep {
					if !strings.Contains(string(out), strings.ReplaceAll(part, "\n", newline)) {
						t.Errorf("lost %q: %s", part, out)
					}
				}
				header, ok := strings.CutSuffix(string(out), "---"+newline+tc.body)
				if !ok || newline == "\n" && strings.Contains(header, "\r\n") {
					t.Errorf("header/body changed: %q", out)
				}
			}
		}
	}
}

func TestRewriteTargetsPreservesAdjacentContent(t *testing.T) {
	for _, tc := range []struct{ header, keep string }{
		{"name: demo\nmetadata:\n  note: hello\n  targets: [old]\ndescription: retained\n", "description: retained\n"},
		{"name: demo\nmetadata:\n  targets: [old]\n  # about next key\n  z: 1\n", "  # about next key\n  z: 1\n"},
	} {
		for _, values := range [][]string{nil, {"claude"}} {
			input := "---\n" + tc.header + "---\nbody\n"
			out, err := RewriteFrontmatterList([]byte(input), "metadata.targets", values)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), tc.keep) {
				t.Errorf("lost adjacent content: %s", out)
			}
		}
	}
}

func TestRewriteTargetsPreservesEmptyHeaders(t *testing.T) {
	for _, content := range []string{"---\n# keep me\n---\nbody\n", "---\n---\nbody\n"} {
		for _, values := range [][]string{nil, {"claude"}} {
			out, err := RewriteFrontmatterList([]byte(content), "metadata.targets", values)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(content, "# keep me") && !strings.Contains(string(out), "# keep me\n") {
				t.Errorf("lost comment: %q", out)
			}
			if values == nil && string(out) != content {
				t.Errorf("clear changed header: %q -> %q", content, out)
			}
			if !strings.HasSuffix(string(out), "---\nbody\n") {
				t.Errorf("lost body: %q", out)
			}
		}
	}
}

func TestRewriteTargetsMetadataShapesAndExplicitKeys(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, md := range []string{"legacy-string", "null", "~", "", "{}"} {
			input := strings.ReplaceAll("---\nname: demo\nmetadata: "+md+"\n---\nbody\n", "\n", newline)
			clear, err := RewriteFrontmatterList([]byte(input), "metadata.targets", nil)
			if err != nil {
				t.Fatal(err)
			}
			if string(clear) != input {
				t.Errorf("%q clear modified metadata: %q", md, clear)
			}
			set, err := RewriteFrontmatterList([]byte(input), "metadata.targets", []string{"claude"})
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := ParseFrontmatterMap(set)
			if err != nil {
				t.Fatal(err)
			}
			meta, ok := parsed["metadata"].(map[string]any)
			if !ok || !reflect.DeepEqual(meta["targets"], []any{"claude"}) {
				t.Errorf("%q unexpected result: %s", md, set)
			}
		}
	}

	for _, header := range []string{
		"name: demo\nmetadata:\n  targets: [old]\ndescription: kept",
		"name: demo\nmetadata: {targets: [old]}\ndescription: kept",
		"{name: demo, metadata: {targets: [old]}, targets: [legacy], description: kept}",
	} {
		input := "---\n" + header + "\n---\nbody\n"
		out, err := RewriteFrontmatterList([]byte(input), "metadata.targets", nil)
		if err != nil || strings.Contains(string(out), "metadata:") || strings.Contains(string(out), "targets:") || !strings.Contains(string(out), "description: kept") {
			t.Fatalf("unexpected output: %s: %v", out, err)
		}
	}

	{

		for _, field := range []string{"metadata.targets", "targets"} {
			header := "? targets\n: [old]\n"
			if field == "metadata.targets" {
				header = "metadata:\n  ? targets\n  : [old]\n"
			}
			input := []byte("---\nname: demo\n" + header + "---\nbody\n")
			for _, values := range [][]string{nil, {"claude"}} {
				out, err := RewriteFrontmatterList(input, field, values)
				if err != nil {
					t.Fatalf("%s %v: %v", field, values, err)
				}
				parsed, err := ParseFrontmatterMap(out)
				if err != nil {
					t.Fatal(err)
				}
				parent := parsed
				if field == "metadata.targets" {
					parent, _ = parsed["metadata"].(map[string]any)
				}
				got, present := parent["targets"]
				if values == nil && present || values != nil && (!present || !reflect.DeepEqual(got, []any{"claude"})) {
					t.Errorf("%s: unexpected targets %#v in %s", field, got, out)
				}
			}
		}

	}
}

func TestRewriteRootTargetsBlockAndFlow(t *testing.T) {
	for _, bom := range []string{"", "\xef\xbb\xbf"} {
		for _, newline := range []string{"\n", "\r\n"} {
			for _, header := range []string{
				"name: tutor\ndate: 2026-06-30\nsources: [{usage_count: 012}]\ntargets: [old]\n# next\nstale_after: 2026-06-30 12:34:56",
				`{name: tutor, date: 2026-06-30, sources: [{usage_count: 012}], targets: [old], stale_after: 2026-06-30 12:34:56}`,
			} {
				for _, values := range [][]string{nil, {"claude"}} {
					input := bom + strings.ReplaceAll("---\n"+header+"\n---\nbody\n", "\n", newline)
					before, err := ParseFrontmatterMap([]byte(input))
					if err != nil {
						t.Fatal(err)
					}
					out, err := RewriteFrontmatterList([]byte(input), "targets", values)
					if err != nil {
						t.Fatal(err)
					}
					after, err := ParseFrontmatterMap(out)
					if err != nil {
						t.Fatal(err)
					}
					got, present := after["targets"]
					if values == nil && present || values != nil && (!present || !reflect.DeepEqual(got, []any{"claude"})) {
						t.Errorf("targets %#v in %s", got, out)
					}
					delete(before, "targets")
					delete(after, "targets")
					if !reflect.DeepEqual(before, after) {
						t.Errorf("metadata changed: %s", out)
					}
					for _, part := range []string{"usage_count: 012", "date: 2026-06-30", "stale_after: 2026-06-30 12:34:56"} {
						if !strings.Contains(string(out), part) {
							t.Errorf("lost %q in %s", part, out)
						}
					}
					if strings.Contains(header, "# next") && !strings.Contains(string(out), "# next"+newline) {
						t.Errorf("lost comment: %s", out)
					}
					if !strings.HasPrefix(string(out), bom+"---"+newline) || !strings.HasSuffix(string(out), "---"+newline+"body"+newline) {
						t.Errorf("body changed: %s", out)
					}
				}
			}
		}

	}

	{

		input := "---\nname: demo\nmetadata:\n  status: ok\n---\nbody\r\n"
		out, err := RewriteFrontmatterList([]byte(input), "metadata.targets", []string{"claude"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.Split(string(out), "---\nbody")[0], "\r\n") {
			t.Errorf("mixed header EOL: %q", out)
		}

	}
}

func TestRewriteTargetsClearsAbsentAliasedTarget(t *testing.T) {
	for _, input := range []string{
		"---\ncommon: &m {status: stable}\nmetadata: *m\n---\nbody\n",
		"---\n{common: &m {status: stable}, metadata: *m}\n---\nbody\n",
		"---\ncommon: &m {1: stable}\nmetadata: *m\n---\nbody\n",
	} {
		got, err := RewriteFrontmatterList([]byte(input), "metadata.targets", nil)
		if err != nil || string(got) != input {
			t.Fatalf("clear changed %q to %q: %v", input, got, err)
		}
	}
}

func TestRewriteTargetsRefusesUnsupportedSource(t *testing.T) {
	for _, header := range []string{
		"  name: demo\n  metadata: {status: stable}",
		`{metadata: {targets: ["cur\u0073or"]}}`,
		"metadata: {1: value, targets: [old]}",
	} {
		input := []byte("---\n" + header + "\n---\nbody\n")
		if _, err := RewriteFrontmatterList(input, "metadata.targets", []string{"claude"}); !errors.Is(err, ErrTargetFrontmatter) {
			t.Fatalf("%q: expected a typed refusal, got %v", header, err)
		}
	}
}

func TestRewriteTargetsRefusesAliasedTargets(t *testing.T) {
	for _, header := range []string{
		"common: &m {status: stable, targets: [old]}\nmetadata: *m", "metadata: &m {status: stable, targets: [old]}\nother: *m", "metadata:\n  targets: &t [old]\nother: *t", "targets: &t [old]\nother: *t",
	} {
		for _, values := range [][]string{nil, {"claude"}} {
			input := "---\nname: demo\n" + header + "\n---\nbody\n"
			if _, err := RewriteFrontmatterList([]byte(input), "metadata.targets", values); err == nil {
				t.Errorf("unsafe alias accepted: %s", input)
			}
		}
	}
}

// yaml.v3 counts literal Unicode separators as line breaks even inside quotes.
func TestTargetEditsWithUnicodeSeparators(t *testing.T) {
	for _, sep := range []string{"\u2028", "\u0085"} {
		desc := "description: \"first" + sep + "second\"\n"
		multi := "description: \"first\n  second" + sep + "third\"\n"
		for _, tc := range []struct {
			name, field, before, after string
			values                     []string
			refuse                     bool
		}{
			{"root insert", "targets", desc, desc + "targets:\n  - claude\n", []string{"claude"}, false},
			{"root replace", "targets", desc + "targets: [old]\n", desc + "targets:\n  - claude\n", []string{"claude"}, false},
			{"root clear", "targets", desc + "targets: [old]\n", desc, nil, false},
			{"root unchanged", "targets", desc, desc, nil, false},
			{"insert nested", "metadata.targets", multi + "metadata:\n  status: stable\n", multi + "metadata:\n  targets:\n    - claude\n  status: stable\n", []string{"claude"}, false},
			{"nested unchanged", "metadata.targets", desc + "metadata:\n  status: stable\n", desc + "metadata:\n  status: stable\n", nil, false},
			{"migrate", "metadata.targets", desc + "targets: [old]\nmetadata:\n  targets: [old]\n  status: stable\n", desc + "metadata:\n  targets:\n    - claude\n  status: stable\n", []string{"claude"}, false},
			{"remove parent", "metadata.targets", desc + "metadata:\n  targets: [old]\n", desc, nil, false},
			{"flow", "metadata.targets", desc + "metadata: {status: \"A\\u0041\", targets: [old]}\n", desc + "metadata: {status: \"A\\u0041\", targets: [claude]}\n", []string{"claude"}, false},
			{"same physical line", "targets", "name: demo" + sep + "targets: [old]\n", "", []string{"claude"}, true},
		} {
			for _, eol := range []string{"\n", "\r\n"} {
				t.Run(fmt.Sprintf("%s/%U/%q", tc.name, []rune(sep)[0], eol), func(t *testing.T) {
					file := func(raw string) []byte { return []byte(strings.ReplaceAll("---\n"+raw+"---\nbody\n", "\n", eol)) }
					before, want := file(tc.before), file(tc.after)
					path := filepath.Join(t.TempDir(), "SKILL.md")
					if err := os.WriteFile(path, before, 0644); err != nil {
						t.Fatal(err)
					}
					err := SetFrontmatterList(path, tc.field, tc.values)
					if tc.refuse {
						if !errors.Is(err, ErrTargetFrontmatter) {
							t.Fatalf("expected refusal, got %v", err)
						}
						want = before
					} else if err != nil {
						t.Fatal(err)
					}
					if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
						t.Fatalf("got %q, want %q: %v", got, want, err)
					}
				})
			}
		}
	}
}
