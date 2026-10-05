package hooks

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func allAdapters(t *testing.T) *Entry {
	t.Helper()
	var parts []string
	for target, binding := range adapterEntry {
		parts = append(parts, `"`+target+`":`+binding)
	}
	return entry(t, `{"enabled":false,"bindings":{`+strings.Join(parts, ",")+`}}`)
}

func rendered(t *testing.T, s *Service, m Mutation) map[string]RenderedFile {
	t.Helper()
	files, err := s.RenderNative(m)
	must(t, err)
	out := map[string]RenderedFile{}
	for _, f := range files {
		out[f.Path] = f
	}
	return out
}

func TestRender_AllAdaptersNativeShapesWithoutWriting(t *testing.T) {
	e := newEnv(t)
	// Existing settings and an unrelated invalid source entry are not part of the preview.
	write(t, filepath.Join(e.home, ".claude", "settings.json"), `{"model":"x","hooks":{"Stop":[{"hooks":[{"type":"command","command":"mine"}]}]}}`)
	write(t, e.config, "hooks:\n  entries:\n    broken:\n      bindings:\n        nope: {}\n")
	got := rendered(t, e.service, Mutation{Name: "demo", Entry: allAdapters(t)})
	want := map[string]string{
		".claude/settings.json": "claude", ".codex/hooks.json": "codex", ".gemini/settings.json": "gemini",
		".qwen/settings.json": "qwen", ".factory/hooks.json": "droid", ".cursor/hooks.json": "cursor",
		".cursor/hooks/skillshare/demo/guard.sh": "cursor",
		".copilot/hooks/skillshare-demo.json":    "copilot", ".pi/agent/extensions/skillshare-demo.ts": "pi", ".omp/agent/extensions/skillshare-demo.ts": "omp",
		".config/amp/plugins/skillshare-demo.ts": "amp", ".config/opencode/plugins/skillshare-demo.ts": "opencode",
		".gemini/config/hooks.json": "antigravity",
	}
	if len(got) != len(want) {
		t.Fatalf("rendered %d files: %+v", len(got), got)
	}
	for rel, target := range want {
		f, ok := got[filepath.Join(e.home, rel)]
		if !ok || f.Target != target || f.Error != "" {
			t.Fatalf("%s: %+v", rel, f)
		}
		var b Binding
		must(t, json.Unmarshal([]byte(adapterEntry[target]), &b))
		switch {
		case strings.HasSuffix(rel, ".ts"):
			if f.Content != b.Code {
				t.Fatalf("%s: code must be verbatim: %q", rel, f.Content)
			}
			continue
		case strings.HasSuffix(rel, ".sh"):
			if f.Content != b.Files["guard.sh"] {
				t.Fatalf("script content: %q", f.Content)
			}
			continue
		}
		var doc map[string]any
		must(t, json.Unmarshal([]byte(f.Content), &doc))
		section := doc
		switch target {
		case "droid":
		case "antigravity":
			section, _ = doc["demo"].(map[string]any)
			if len(doc) != 1 {
				t.Fatalf("%s: one block named after the hook: %s", rel, f.Content)
			}
		default:
			section, _ = doc["hooks"].(map[string]any)
		}
		for event := range b.Events {
			if items, _ := section[event].([]any); len(items) != 1 {
				t.Fatalf("%s: %s missing from %s", rel, event, f.Content)
			}
		}
		if len(section) != len(b.Events) || (target == "cursor" || target == "copilot") != (doc["version"] == float64(1)) {
			t.Fatalf("%s: only this hook in its native wrapper: %s", rel, f.Content)
		}
	}
	if strings.Contains(got[filepath.Join(e.home, ".claude", "settings.json")].Content, "mine") {
		t.Fatal("existing native content must not be shown")
	}
	// Only the fixture files exist: nothing written, no state created.
	var written []string
	must(t, filepath.WalkDir(e.home, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			written = append(written, path)
		}
		return err
	}))
	if len(written) != 1 || exists(e.service.StateDir) {
		t.Fatalf("render must write nothing: %v", written)
	}
}

func TestRender_ProjectScopes(t *testing.T) {
	e := newEnv(t)
	root := filepath.Join(filepath.Dir(e.home), "repo")
	m := Mutation{Project: root, Name: "demo", Entry: entry(t, `{"bindings":{"factory":`+adapterEntry["droid"]+`,"claude":`+adapterEntry["claude"]+`}}`)}
	got := rendered(t, e.service, m)
	for _, rel := range []string{".factory/hooks.json", ".claude/settings.json"} {
		if _, ok := got[filepath.Join(root, rel)]; !ok {
			t.Fatalf("hooks.projects root paths: %+v", got)
		}
	}
	project := e.project(root)
	m.Project = ""
	if got := rendered(t, project, m); got[filepath.Join(root, ".claude", "settings.json")].Target != "claude" {
		t.Fatalf("project mode paths: %+v", got)
	}
	m.Project = root
	if _, err := project.RenderNative(m); err == nil {
		t.Fatal("project mode must reject hooks.projects roots")
	}
}

func TestRender_RejectsNonRenderRequests(t *testing.T) {
	e := newEnv(t)
	ok := entry(t, claudeEntry)
	for name, m := range map[string]Mutation{
		"remove":   {Name: "demo", Entry: ok, Remove: true},
		"replace":  {Name: "demo", Entry: ok, Replace: true},
		"no name":  {Entry: ok},
		"no entry": {Name: "demo"},
		"invalid":  {Name: "demo", Entry: &Entry{Bindings: map[string]Binding{"claude": {Events: map[string]any{"PreToolUse": map[string]any{}}}}}},
		"relative": {Project: "repo", Name: "demo", Entry: ok},
	} {
		if _, err := e.service.RenderNative(m); err == nil {
			t.Fatalf("%s must be rejected", name)
		}
	}
}

func TestRender_UnsafePathIsReportedPerFile(t *testing.T) {
	e := newEnv(t)
	outside := filepath.Join(filepath.Dir(e.home), "outside")
	must(t, os.MkdirAll(outside, 0755))
	must(t, os.MkdirAll(filepath.Join(e.home, ".cursor"), 0755))
	must(t, os.Symlink(outside, filepath.Join(e.home, ".cursor", "hooks")))
	got := rendered(t, e.service, Mutation{Name: "demo", Entry: entry(t, `{"bindings":{"cursor":`+adapterEntry["cursor"]+`}}`)})
	if f := got[filepath.Join(e.home, ".cursor", "hooks", "skillshare", "demo", "guard.sh")]; f.Error == "" || f.Content == "" {
		t.Fatalf("a script behind a symlink must carry an error: %+v", f)
	}
	if f := got[filepath.Join(e.home, ".cursor", "hooks.json")]; f.Error != "" || f.Content == "" {
		t.Fatalf("the hooks file is still previewed: %+v", f)
	}
}

func TestRender_EmptyScriptKeepsItsContentInJSON(t *testing.T) {
	e := newEnv(t)
	got := rendered(t, e.service, Mutation{Name: "demo", Entry: entry(t, `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"empty.sh"}]}]},"files":{"empty.sh":""}}}}`)})
	path := filepath.Join(e.home, ".claude", "hooks", "skillshare", "demo", "empty.sh")
	f, ok := got[path]
	if !ok || f.Error != "" || f.Content != "" {
		t.Fatalf("empty script: %+v", got)
	}
	data, err := json.Marshal(f)
	must(t, err)
	if !strings.Contains(string(data), `"content":""`) {
		t.Fatalf("an empty file's content must be serialized: %s", data)
	}
	if exists(filepath.Join(e.home, ".claude")) {
		t.Fatal("render must write nothing")
	}
}
