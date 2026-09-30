package hooks

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Every command Agent receives its elements exactly as supplied: matchers and fields
// Skillshare has no setting for survive the round trip into the native file.
func TestAdapters_NativePayloadPreservedExactly(t *testing.T) {
	payloads := map[string]string{
		"claude":  `{"PreToolUse":[{"matcher":"Bash|Edit","hooks":[{"type":"command","command":"x","timeout":5,"statusMessage":"s","async":true}]},{"hooks":[{"type":"http","url":"https://example.test/h"}]}]}`,
		"codex":   `{"PreToolUse":[{"matcher":"^shell$","hooks":[{"type":"command","command":"x","commandWindows":"y","additionalContextLimit":10}]}]}`,
		"gemini":  `{"BeforeTool":[{"matcher":"write_.*","sequential":true,"hooks":[{"type":"command","command":"x","name":"n","timeout":60000,"description":"d"}]}]}`,
		"qwen":    `{"PreToolUse":[{"matcher":"Shell","sequential":false,"hooks":[{"type":"command","command":"x","env":{"A":"1"},"shell":"bash"}]}]}`,
		"droid":   `{"PreToolUse":[{"matcher":"Execute","commandRegex":"^git ","hooks":[{"type":"command","command":"x","timeout":60}]}]}`,
		"cursor":  `{"beforeShellExecution":[{"command":"x","matcher":"git","timeout":10,"loop_limit":3,"failClosed":true}]}`,
		"copilot": `{"preToolUse":[{"bash":"x","powershell":"y","matcher":"bash","cwd":"scripts","env":{"A":"1"},"timeoutSec":30}]}`,
	}
	for target, raw := range payloads {
		t.Run(target, func(t *testing.T) {
			e := newEnv(t)
			var events map[string]any
			must(t, json.Unmarshal([]byte(raw), &events))
			save(t, e.service, Mutation{Name: "demo", Entry: &Entry{Bindings: map[string]Binding{target: {Events: events}}}})
			var got map[string]any
			switch target {
			case "copilot":
				var doc struct {
					Hooks map[string]any `json:"hooks"`
				}
				must(t, json.Unmarshal([]byte(read(t, filepath.Join(e.home, ".copilot", "hooks", "skillshare-demo.json"))), &doc))
				got = doc.Hooks
			default:
				path := e.service.Paths()[target]
				got = map[string]any{}
				for event, items := range readEvents(t, path, target != "droid") {
					got[event] = items
				}
			}
			if !reflect.DeepEqual(got, events) {
				t.Fatalf("payload changed:\nwant %v\ngot  %v", events, got)
			}
		})
	}
}

func readEvents(t *testing.T, path string, wrapped bool) map[string]any {
	out := map[string]any{}
	for event, items := range events(t, path, wrapped) {
		out[event] = items
	}
	return out
}

// Copilot handlers are flat and take bash, powershell, command, or exec with args,
// with type optional; http and prompt handlers need no command.
func TestCopilot_OfficialHandlerShapesAccepted(t *testing.T) {
	valid := []string{
		`{"bash":"x"}`,
		`{"type":"command","powershell":"x"}`,
		`{"command":"x","timeoutSec":5}`,
		`{"exec":"node","args":["hook.js"]}`,
		`{"type":"http","url":"https://example.test/hook"}`,
		`{"bash":"x","matcher":"bash"}`,
		`{"bash":"x","powershell":"y","command":"z"}`,
	}
	for _, handler := range valid {
		doc := `{"bindings":{"copilot":{"events":{"preToolUse":[` + handler + `]}}}}`
		if _, err := ParseEntry([]byte(doc)); err != nil {
			t.Errorf("%s rejected: %v", handler, err)
		}
	}
	for _, handler := range []string{
		`{"type":"command"}`, `{"type":"script","bash":"x"}`, `{"args":["x"]}`,
		`{"exec":"node","bash":"x"}`, `{"exec":"node","command":"x"}`, `{"bash":"x","args":["y"]}`,
		`{"exec":"node","args":"hook.js"}`, `{"type":"http"}`, `{"type":"http","url":"ftp://x"}`,
		`{"type":"http","url":"http://example.test"}`, `{"type":"prompt"}`, `{"bash":"x","timeoutSec":-1}`,
	} {
		doc := `{"bindings":{"copilot":{"events":{"preToolUse":[` + handler + `]}}}}`
		if _, err := ParseEntry([]byte(doc)); err == nil {
			t.Errorf("%s accepted", handler)
		}
	}
	// Prompt hooks are sessionStart only; plain http is allowed outside the
	// permission-granting events.
	for doc, ok := range map[string]bool{
		`{"bindings":{"copilot":{"events":{"sessionStart":[{"type":"prompt","prompt":"/status"}]}}}}`:                            true,
		`{"bindings":{"copilot":{"events":{"preToolUse":[{"type":"prompt","prompt":"x"}]}}}}`:                                    false,
		`{"bindings":{"copilot":{"events":{"postToolUse":[{"type":"http","url":"http://localhost:8080/h"}]}}}}`:                  true,
		`{"bindings":{"copilot":{"events":{"postToolUse":[{"type":"http","url":"http://h.test","allowedEnvVars":["TOKEN"]}]}}}}`: false,
	} {
		if _, err := ParseEntry([]byte(doc)); (err == nil) != ok {
			t.Errorf("%s: accepted=%v, want %v (%v)", doc, err == nil, ok, err)
		}
	}
	// PascalCase is Copilot's VS Code compatible form and stays as written.
	if _, err := ParseEntry([]byte(`{"bindings":{"copilot":{"events":{"PreToolUse":[{"bash":"x"}]}}}}`)); err != nil {
		t.Fatalf("PascalCase Copilot event rejected: %v", err)
	}
}

// A user's identical copy inserted before or after Skillshare's makes identity ambiguous:
// the nearest old index would pick the user's copy, so update, disable and remove all
// conflict and never touch the file; replace releases ownership without writing.
func TestLedger_IdenticalCopyInsertedBeforeOrAfterIsAmbiguous(t *testing.T) {
	group := `{"matcher":"Bash","hooks":[{"type":"command","command":"echo guard","timeout":5}]}`
	for name, layout := range map[string]string{
		"before": `{"hooks":{"PreToolUse":[` + group + `,` + group + `]}}`,
		"after":  `{"hooks":{"PreToolUse":[` + group + `,{"hooks":[{"type":"command","command":"echo other"}]},` + group + `]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			path := filepath.Join(e.home, ".claude", "settings.json")
			save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)}) // owned at index 0
			write(t, path, layout)
			off := *entry(t, claudeEntry)
			disabled := false
			off.Enabled = &disabled
			for label, m := range map[string]Mutation{
				"disable": {Name: "guard", Entry: &off},
				"remove":  {Name: "guard", Remove: true},
				"update":  {Name: "guard", Entry: entry(t, strings.Replace(claudeEntry, "echo guard", "echo v2", 1))},
			} {
				p, err := e.service.PreviewMutation(m)
				must(t, err)
				if !p.Blocked {
					t.Fatalf("%s: ambiguous identical copies must conflict: %s", label, actions(p))
				}
				if _, err := e.service.Mutate(m, p.Revision, true); !errors.Is(err, ErrConflict) {
					t.Fatalf("%s: want ErrConflict, got %v", label, err)
				}
			}
			if read(t, path) != layout {
				t.Fatal("a refused sync must leave the file byte for byte")
			}
			inv, err := e.service.List()
			must(t, err)
			if inv.Unmanaged[0].Names[0] != "PreToolUse" {
				t.Fatalf("ambiguous copies are not claimed as owned: %+v", inv.Unmanaged)
			}
			save(t, e.service, Mutation{Name: "guard", Remove: true, Replace: true})
			if read(t, path) != layout {
				t.Fatal("release must not write")
			}
		})
	}
}

// Identical copies all owned by records stay resolvable, and removing one leaves the other.
func TestLedger_OwnedIdenticalCopiesResolve(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	two := `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"x"}]},{"hooks":[{"type":"command","command":"x"}]}]}}}}`
	save(t, e.service, Mutation{Name: "a", Entry: entry(t, two)})
	save(t, e.service, Mutation{Name: "b", Entry: entry(t, `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"x"}]}]}}}}`)})
	if n := len(events(t, path, true)["Stop"]); n != 3 {
		t.Fatalf("three owned copies expected, got %d", n)
	}
	save(t, e.service, Mutation{Name: "a", Remove: true})
	if n := len(events(t, path, true)["Stop"]); n != 1 {
		t.Fatalf("b's copy must remain: %d", n)
	}
	p, err := e.service.Preview()
	must(t, err)
	if p.Blocked || actions(p) != "claude:b:unchanged" {
		t.Fatalf("remaining copy stays owned: %s", actions(p))
	}
}

// Reordering keeps ownership: the owned element is found by record and content,
// not by its old index.
func TestLedger_ReorderedElementsKeepOwnership(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, `{"hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"echo mine"}]}]}}`)
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	var doc map[string]map[string][]any
	must(t, json.Unmarshal([]byte(read(t, path)), &doc))
	items := doc["hooks"]["PreToolUse"]
	items[0], items[1] = items[1], items[0]
	data, _ := json.Marshal(doc)
	write(t, path, string(data))
	save(t, e.service, Mutation{Name: "guard", Remove: true})
	got := events(t, path, true)["PreToolUse"]
	if len(got) != 1 || !strings.Contains(mustJSON(got[0]), "echo mine") {
		t.Fatalf("remove after reorder must prune only the owned group: %v", got)
	}
}

func mustJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// A nonempty revision guards a source-only save exactly like a sync.
func TestMutate_SourceOnlySaveWithRevisionRejectsDrift(t *testing.T) {
	for name, drift := range map[string]func(e *env){
		"native hooks": func(e *env) {
			write(t, filepath.Join(e.home, ".claude", "settings.json"), `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"x"}]}]}}`)
		},
		"config":    func(e *env) { write(t, e.config, "targets: {}\n# edited\n") },
		"ownership": func(e *env) { write(t, e.service.statePath(), `{"version":1,"records":{}}`+"\n") },
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			m := Mutation{Name: "guard", Entry: entry(t, claudeEntry)}
			p, err := e.service.PreviewMutation(m)
			must(t, err)
			drift(e)
			if _, err := e.service.Mutate(m, p.Revision, false); !errors.Is(err, ErrStaleRevision) {
				t.Fatalf("want ErrStaleRevision, got %v", err)
			}
			if source, _ := LoadSource(e.config); source != nil && len(source.Entries) != 0 {
				t.Fatal("a stale save must not write the source")
			}
		})
	}
}

// A symlinked Agent folder inside a project would send writes outside the project.
func TestSync_ProjectAncestorSymlinkRejected(t *testing.T) {
	e := newEnv(t)
	root := filepath.Join(filepath.Dir(e.home), "repo")
	project := e.project(root)
	outside := filepath.Join(filepath.Dir(e.home), "outside")
	must(t, os.MkdirAll(outside, 0755))
	must(t, os.Symlink(outside, filepath.Join(root, ".pi")))
	_, err := project.PreviewMutation(Mutation{Name: "demo", Entry: entry(t, `{"bindings":{"pi":`+adapterEntry["pi"]+`}}`)})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("want symlink refusal, got %v", err)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatal("nothing may be written outside the project")
	}
}

// A crash after the journal but before the native write leaves ownership untouched.
func TestRecovery_InterruptedBeforeWriteKeepsOwnership(t *testing.T) {
	e := newEnv(t)
	m := Mutation{Name: "guard", Entry: entry(t, claudeEntry)}
	source, replace, adopt, err := e.service.draft(m)
	must(t, err)
	must(t, source.save())
	p, err := e.service.previewSource(source, replace, adopt)
	must(t, err)
	f := p.files[0]
	must(t, writeJSONFile(e.service.journalPath(), journal{Path: f.path, After: digest(f.after), State: p.state}))
	plan, err := e.service.Preview()
	must(t, err)
	if !strings.Contains(actions(plan), "claude:guard:add") {
		t.Fatalf("an unwritten file must still be pending: %s", actions(plan))
	}
	sync(t, e.service)
	if exists(e.service.journalPath()) || len(events(t, f.path, true)["PreToolUse"]) != 1 {
		t.Fatal("recovery then sync must write exactly once")
	}
}

// Droid reads settings.json hooks only while hooks.json is absent. Those hooks are what
// Droid runs, so import offers them, the conflict says how to resolve it, and inventory
// lists them only while they are active.
func TestDroid_InlineHooksImportResolveFlow(t *testing.T) {
	e := newEnv(t)
	settings := filepath.Join(e.home, ".factory", "settings.json")
	write(t, settings, `{"model":"x","hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo inline"}]}]}}`)
	candidates, err := e.service.Import(ImportRequest{From: "droid", Name: "inline"})
	must(t, err)
	if len(candidates) != 1 || candidates[0].Entry.Bindings["droid"].Events["Stop"] == nil || len(candidates[0].Warnings) == 0 {
		t.Fatalf("import must offer the active settings.json hooks with guidance: %+v", candidates)
	}
	m := Mutation{Name: "inline", Entry: &candidates[0].Entry}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	if !p.Blocked || !strings.Contains(p.Changes[0].Message, settings) {
		t.Fatalf("conflict must name the settings file to clean up: %+v", p.Changes)
	}
	// Save the definition, then the user removes the inline hooks and syncs.
	_, err = e.service.Mutate(m, p.Revision, false)
	must(t, err)
	write(t, settings, `{"model":"x"}`)
	sync(t, e.service)
	if n := len(events(t, filepath.Join(e.home, ".factory", "hooks.json"), false)["Stop"]); n != 1 {
		t.Fatalf("hooks.json must hold the imported hook: %d", n)
	}
	// With hooks.json present, settings.json hooks are inert and not listed as active.
	write(t, settings, `{"model":"x","hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo ignored"}]}]}}`)
	inv, err := e.service.List()
	must(t, err)
	for _, u := range inv.Unmanaged {
		if u.Path == settings {
			t.Fatalf("Droid ignores settings.json hooks while hooks.json exists: %+v", u)
		}
	}
	// The legacy .factory/hooks/hooks.json still loads and is disclosed.
	legacy := filepath.Join(e.home, ".factory", "hooks", "hooks.json")
	write(t, legacy, `{"SessionStart":[{"hooks":[{"type":"command","command":"echo legacy"}]}]}`)
	inv, err = e.service.List()
	must(t, err)
	found := false
	for _, u := range inv.Unmanaged {
		found = found || u.Path == legacy && strings.Join(u.Names, ",") == "SessionStart"
	}
	if !found {
		t.Fatalf("legacy Droid hooks file must be disclosed: %+v", inv.Unmanaged)
	}
	// Pasted settings.json content, with its hooks wrapper, imports too.
	c, err := e.service.Import(ImportRequest{From: "droid", Content: `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"x"}]}]}}`})
	must(t, err)
	if len(c) != 1 || c[0].Entry.Bindings["droid"].Events["Stop"] == nil {
		t.Fatalf("wrapped Droid content: %+v", c)
	}
}

// Restore validates the whole path, not only the leaf: a parent folder swapped for a
// symlink is refused whether the file there still exists or has to be recreated.
func TestRestore_SymlinkedParentRefusedForExistingAndMissingLeaf(t *testing.T) {
	for _, leaf := range []string{"existing", "missing"} {
		t.Run(leaf, func(t *testing.T) {
			e := newEnv(t)
			m := Mutation{Name: "demo", Entry: entry(t, `{"bindings":{"pi":`+adapterEntry["pi"]+`}}`)}
			r := save(t, e.service, m)
			if leaf == "missing" {
				r = save(t, e.service, Mutation{Name: "demo", Remove: true}) // restoring recreates the file
			}
			dir := filepath.Join(e.home, ".pi", "agent", "extensions")
			outside := filepath.Join(e.home, "outside")
			must(t, os.MkdirAll(outside, 0755))
			if leaf == "existing" {
				must(t, os.Rename(filepath.Join(dir, "skillshare-demo.ts"), filepath.Join(outside, "skillshare-demo.ts")))
			}
			must(t, os.RemoveAll(dir))
			must(t, os.MkdirAll(filepath.Dir(dir), 0755)) // removal pruned the folders it created
			must(t, os.Symlink(outside, dir))
			before, _ := os.ReadDir(outside)
			if _, err := e.service.PreviewRestore(r.BackupIDs[0]); err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("preview restore: want symlink refusal, got %v", err)
			}
			if _, err := e.service.Restore(r.BackupIDs[0], ""); err == nil {
				t.Fatal("restore must refuse")
			}
			after, _ := os.ReadDir(outside)
			if len(after) != len(before) {
				t.Fatal("nothing may be written through the symlink")
			}
		})
	}
}
