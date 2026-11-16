package hooks

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// nativeEdit previews then applies edits, as the dashboard does.
func nativeEdit(t *testing.T, s *Service, edits ...NativeEdit) *Result {
	t.Helper()
	p, err := s.PreviewNativeEdits(edits)
	must(t, err)
	r, err := s.ApplyNativeEdits(edits, p.Revision)
	must(t, err)
	return r
}

// guardOnWrite is trialSettings' PreToolUse hook with its matcher changed.
func guardOnWrite(t *testing.T) map[string]any {
	t.Helper()
	var out map[string]any
	must(t, json.Unmarshal([]byte(`{"PreToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"~/.claude/guard.sh"}]}]}`), &out))
	return out
}

func unmanagedNames(t *testing.T, s *Service) string {
	t.Helper()
	inventory, err := s.List()
	must(t, err)
	var names []string
	for _, u := range inventory.Unmanaged {
		names = append(names, u.Names...)
	}
	return strings.Join(names, ",")
}

func TestNativeEdit_RewritesAnUnmanagedHookAndLeavesItUnmanaged(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, trialSettings)
	c := importOne(t, e.service, "claude", "claude-pretooluse")
	if !c.Editable {
		t.Fatal("a Claude event must be editable in place")
	}
	before := c.Entry.Bindings["claude"].Events
	after := guardOnWrite(t)
	edits := []NativeEdit{{Target: "claude", Before: before, After: after}}
	p, err := e.service.PreviewNativeEdits(edits)
	must(t, err)
	if files := p.Files(); len(files) != 1 || !strings.Contains(files[0].After, `"matcher":"Write"`) {
		t.Fatalf("preview must show the rewritten file: %+v", files)
	}
	if read(t, path) != trialSettings {
		t.Fatal("preview must not write")
	}
	if _, err := e.service.ApplyNativeEdits(edits, p.Revision); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	if !strings.Contains(got, `"matcher":"Write"`) || strings.Contains(got, `"matcher":"Bash"`) || !strings.Contains(got, "afplay") || !strings.Contains(got, `"model":"opus"`) {
		t.Fatalf("only the edited hook may change:\n%s", got)
	}
	if names := unmanagedNames(t, e.service); names != "PreToolUse,Stop" {
		t.Fatalf("edited hooks must stay unmanaged: %s", names)
	}
}

func TestNativeEdit_RemovesAnUnmanagedHook(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, trialSettings)
	c := importOne(t, e.service, "claude", "claude-stop")
	nativeEdit(t, e.service, NativeEdit{Target: "claude", Before: c.Entry.Bindings["claude"].Events})
	got := read(t, path)
	if strings.Contains(got, "Stop") || !strings.Contains(got, "guard.sh") {
		t.Fatalf("only Stop may be removed:\n%s", got)
	}
}

func TestNativeEdit_BackupRestoresTheFile(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, trialSettings)
	stop := importOne(t, e.service, "claude", "claude-stop").Entry.Bindings["claude"].Events
	guard := importOne(t, e.service, "claude", "claude-pretooluse").Entry.Bindings["claude"].Events
	r := nativeEdit(t, e.service,
		NativeEdit{Target: "claude", Before: stop},
		NativeEdit{Target: "claude", Before: guard, After: guardOnWrite(t)},
	)
	if len(r.BackupIDs) != 1 {
		t.Fatalf("one file, one backup: %v", r.BackupIDs)
	}
	p, err := e.service.PreviewRestore(r.BackupIDs[0])
	must(t, err)
	if p.Blocked {
		t.Fatalf("restore blocked: %+v", p.Changes)
	}
	_, err = e.service.Restore(r.BackupIDs[0], p.Revision)
	must(t, err)
	got := read(t, path)
	if !strings.Contains(got, "afplay") || !strings.Contains(got, `"matcher":"Bash"`) || strings.Contains(got, `"matcher":"Write"`) {
		t.Fatalf("restore must bring both hooks back:\n%s", got)
	}
}

func TestNativeEdit_RefusesAManagedHook(t *testing.T) {
	e := newEnv(t)
	write(t, filepath.Join(e.home, ".claude", "settings.json"), trialSettings)
	c := importOne(t, e.service, "claude", "claude-stop")
	m := Mutation{Name: c.Name, Entry: &c.Entry, Adopt: true}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	_, err = e.service.Mutate(m, p.Revision, false)
	must(t, err)
	_, err = e.service.PreviewNativeEdits([]NativeEdit{{Target: "claude", Before: c.Entry.Bindings["claude"].Events}})
	if err == nil || !strings.Contains(err.Error(), "managed by Skillshare") {
		t.Fatalf("a managed hook must be refused: %v", err)
	}
}

func TestNativeEdit_StaleFileIsRefused(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, trialSettings)
	c := importOne(t, e.service, "claude", "claude-stop")
	edits := []NativeEdit{{Target: "claude", Before: c.Entry.Bindings["claude"].Events}}
	p, err := e.service.PreviewNativeEdits(edits)
	must(t, err)
	write(t, path, strings.Replace(trialSettings, "guard.sh", "other.sh", 1))
	if _, err := e.service.ApplyNativeEdits(edits, p.Revision); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("want ErrStaleRevision, got %v", err)
	}
}

func TestNativeEdit_RefusesFileAndBlockTargets(t *testing.T) {
	for _, target := range []string{"pi", "copilot", "antigravity"} {
		e := newEnv(t)
		_, err := e.service.PreviewNativeEdits([]NativeEdit{{Target: target, Before: map[string]any{"x": []any{map[string]any{}}}}})
		if err == nil || !strings.Contains(err.Error(), "cannot be edited in place") {
			t.Fatalf("%s hooks must be refused: %v", target, err)
		}
	}
}

// Every Agent that keeps hooks as events of one file: the edit lands in that file's own shape.
func TestNativeEdit_EveryEditableFileShape(t *testing.T) {
	files := map[string]string{
		"claude": `{"model":"opus","hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo mine"}]}]}}`,
		"gemini": `{"theme":"x","hooks":{"BeforeTool":[{"matcher":"write_.*","hooks":[{"type":"command","command":"echo mine"}]}]}}`,
		"qwen":   `{"hooks":{"PreToolUse":[{"matcher":"Shell","hooks":[{"type":"command","command":"echo mine"}]}]}}`,
		"codex":  `{"hooks":{"PreToolUse":[{"matcher":"^shell$","hooks":[{"type":"command","command":"echo mine"}]}]}}`,
		"cursor": `{"version":1,"hooks":{"beforeShellExecution":[{"command":"echo mine","matcher":"git"}]}}`,
		"droid":  `{"PreToolUse":[{"matcher":"Execute","hooks":[{"type":"command","command":"echo mine"}]}]}`,
	}
	for target, content := range files {
		t.Run(target, func(t *testing.T) {
			e := newEnv(t)
			path := e.service.Paths()[target]
			write(t, path, content)
			c, err := e.service.Import(ImportRequest{From: target})
			must(t, err)
			if len(c) != 1 || !c[0].Editable {
				t.Fatalf("want one editable candidate: %+v", c)
			}
			before := c[0].Entry.Bindings[target].Events
			raw, err := json.Marshal(before)
			must(t, err)
			var after map[string]any
			must(t, json.Unmarshal([]byte(strings.Replace(string(raw), "echo mine", "echo edited", 1)), &after))
			nativeEdit(t, e.service, NativeEdit{Target: target, Before: before, After: after})
			got := read(t, path)
			if !strings.Contains(got, "echo edited") || strings.Contains(got, "echo mine") {
				t.Fatalf("not rewritten:\n%s", got)
			}
			again, err := e.service.Import(ImportRequest{From: target})
			must(t, err)
			if len(again) != 1 || !reflect.DeepEqual(again[0].Entry.Bindings[target].Events, after) {
				t.Fatalf("the file must read back as the edited hook: %+v", again)
			}
		})
	}
}

// managedBeside saves a managed Stop hook next to an unmanaged one and returns the unmanaged events.
func managedBeside(t *testing.T, e *env, path string) map[string]any {
	t.Helper()
	save(t, e.service, Mutation{Name: "m", Entry: entry(t, `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"echo managed"}]}]}}}}`)})
	var doc map[string]map[string][]any
	must(t, json.Unmarshal([]byte(read(t, path)), &doc))
	var mine any
	must(t, json.Unmarshal([]byte(`{"hooks":[{"type":"command","command":"echo mine"}]}`), &mine))
	doc["hooks"]["Stop"] = append([]any{mine}, doc["hooks"]["Stop"]...)
	raw, err := json.Marshal(doc)
	must(t, err)
	write(t, path, string(raw))
	return importOne(t, e.service, "claude", "claude-stop").Entry.Bindings["claude"].Events
}

func TestNativeEdit_LeavesAManagedHookOfTheSameEventManaged(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	nativeEdit(t, e.service, NativeEdit{Target: "claude", Before: managedBeside(t, e, path)})
	if got := read(t, path); strings.Contains(got, "echo mine") || !strings.Contains(got, "echo managed") {
		t.Fatalf("only the unmanaged hook may go:\n%s", got)
	}
	p, err := e.service.PreviewMutation(Mutation{})
	must(t, err)
	if p.Blocked || unmanagedNames(t, e.service) != "" {
		t.Fatalf("the managed hook must still be located: %+v", p.Changes)
	}
}

func TestNativeEdit_RefusesToWriteACopyOfAManagedHook(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	before := managedBeside(t, e, path)
	var after map[string]any
	must(t, json.Unmarshal([]byte(`{"Stop":[{"hooks":[{"type":"command","command":"echo managed"}]}]}`), &after))
	_, err := e.service.PreviewNativeEdits([]NativeEdit{{Target: "claude", Before: before, After: after}})
	if err == nil || !strings.Contains(err.Error(), "managed by Skillshare") {
		t.Fatalf("a second copy of a managed hook must be refused: %v", err)
	}
}

func TestNativeEdit_RemovesOneOfTwoIdenticalHooks(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	one := `{"hooks":[{"type":"command","command":"echo twin"}]}`
	write(t, path, `{"hooks":{"Stop":[`+one+`,`+one+`]}}`)
	var before map[string]any
	must(t, json.Unmarshal([]byte(`{"Stop":[`+one+`]}`), &before))
	nativeEdit(t, e.service, NativeEdit{Target: "claude", Before: before})
	if n := strings.Count(read(t, path), "echo twin"); n != 1 {
		t.Fatalf("want one copy left, got %d:\n%s", n, read(t, path))
	}
}

func TestNativeEdit_RestorePreviewNamesTheEvent(t *testing.T) {
	e := newEnv(t)
	write(t, filepath.Join(e.home, ".claude", "settings.json"), trialSettings)
	stop := importOne(t, e.service, "claude", "claude-stop").Entry.Bindings["claude"].Events
	r := nativeEdit(t, e.service, NativeEdit{Target: "claude", Before: stop})
	p, err := e.service.PreviewRestore(r.BackupIDs[0])
	must(t, err)
	if len(p.Changes) != 1 || p.Changes[0].Name != "Stop" {
		t.Fatalf("the restore must say which hook comes back: %+v", p.Changes)
	}
}

// A backup stays listed after it is restored, so restoring it again must not add the hook twice.
func TestNativeEdit_RestoringARemovalTwiceIsRefused(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, trialSettings)
	stop := importOne(t, e.service, "claude", "claude-stop").Entry.Bindings["claude"].Events
	r := nativeEdit(t, e.service, NativeEdit{Target: "claude", Before: stop})
	p, err := e.service.PreviewRestore(r.BackupIDs[0])
	must(t, err)
	_, err = e.service.Restore(r.BackupIDs[0], p.Revision)
	must(t, err)
	again, err := e.service.PreviewRestore(r.BackupIDs[0])
	must(t, err)
	if !again.Blocked {
		t.Fatalf("a second restore must be refused: %+v", again.Changes)
	}
	if n := strings.Count(read(t, path), "afplay"); n != 1 {
		t.Fatalf("want the hook once, got %d", n)
	}
}
