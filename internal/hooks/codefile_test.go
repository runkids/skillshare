package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const piCodeFileConfig = "targets: {}\nhooks:\n  entries:\n    lint:\n      bindings:\n        pi:\n          codeFile: hooks/lint.pi.ts\n"

func TestCodeFile_SyncsFileContentsAndSavesTheReference(t *testing.T) {
	e := newEnv(t)
	write(t, e.config, piCodeFileConfig)
	write(t, filepath.Join(filepath.Dir(e.config), "hooks", "lint.pi.ts"), "export default function () {}\n")
	sync(t, e.service)
	out := filepath.Join(e.home, ".pi", "agent", "extensions", "skillshare-lint.ts")
	if got := read(t, out); got != "export default function () {}\n" {
		t.Fatalf("extension = %q", got)
	}

	source, err := LoadSource(e.config)
	must(t, err)
	disabled := source.Entries["lint"]
	off := false
	disabled.Enabled = &off
	save(t, e.service, Mutation{Name: "lint", Entry: &disabled})
	if got := read(t, e.config); !strings.Contains(got, "codeFile: hooks/lint.pi.ts") || strings.Contains(got, "code:") {
		t.Fatalf("saving must keep codeFile and leave out the code:\n%s", got)
	}
}

func TestCodeFile_ChangedFileChangesRevision(t *testing.T) {
	e := newEnv(t)
	write(t, e.config, piCodeFileConfig)
	code := filepath.Join(filepath.Dir(e.config), "hooks", "lint.pi.ts")
	write(t, code, "export default function () {}\n")
	before, err := e.service.PreviewMutation(Mutation{})
	must(t, err)
	write(t, code, "export default function (pi) {}\n")
	if _, err := e.service.Mutate(Mutation{}, before.Revision, true); err != ErrStaleRevision {
		t.Fatalf("err = %v, want ErrStaleRevision", err)
	}
}

func TestCodeFile_RejectsInvalidReferences(t *testing.T) {
	for name, binding := range map[string]string{
		"both":    "code: x\n          codeFile: hooks/lint.pi.ts",
		"escape":  "codeFile: ../lint.pi.ts",
		"missing": "codeFile: hooks/missing.ts",
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			write(t, e.config, strings.Replace(piCodeFileConfig, "codeFile: hooks/lint.pi.ts", binding, 1))
			write(t, filepath.Join(filepath.Dir(e.config), "hooks", "lint.pi.ts"), "export default function () {}\n")
			if _, err := LoadSource(e.config); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestCodeFile_RefusesSymlinks(t *testing.T) {
	e := newEnv(t)
	write(t, e.config, piCodeFileConfig)
	secret := filepath.Join(e.home, "secret")
	write(t, secret, "token\n")
	dir := filepath.Join(filepath.Dir(e.config), "hooks")
	must(t, os.MkdirAll(dir, 0755))
	must(t, os.Symlink(secret, filepath.Join(dir, "lint.pi.ts")))
	if _, err := LoadSource(e.config); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("err = %v, want a symlink refusal", err)
	}
}

func TestCodeFile_EchoedEntryMustMatchTheFile(t *testing.T) {
	e := newEnv(t)
	write(t, e.config, piCodeFileConfig)
	write(t, filepath.Join(filepath.Dir(e.config), "hooks", "lint.pi.ts"), "export default function () {}\n")
	// list --json and the dashboard carry both code and codeFile.
	echoed := entry(t, `{"bindings":{"pi":{"codeFile":"hooks/lint.pi.ts","code":"export default function () {}\n"}}}`)
	save(t, e.service, Mutation{Name: "lint", Entry: echoed})
	edited := entry(t, `{"bindings":{"pi":{"codeFile":"hooks/lint.pi.ts","code":"edited"}}}`)
	if _, err := e.service.PreviewMutation(Mutation{Name: "lint", Entry: edited}); err == nil || !strings.Contains(err.Error(), "differ") {
		t.Fatalf("err = %v, want code that differs from the file refused", err)
	}
}

func TestFilesFrom_SyncsContentsAndSavesTheReference(t *testing.T) {
	e := newEnv(t)
	write(t, e.config, "targets: {}\nhooks:\n  entries:\n    guard:\n      bindings:\n        cursor:\n          events:\n            beforeShellExecution:\n              - command: ./guard.sh\n          files:\n            guard.sh:\n              from: hooks/guard.sh\n            inline.sh: \"echo inline\\n\"\n")
	write(t, filepath.Join(filepath.Dir(e.config), "hooks", "guard.sh"), "#!/bin/sh\necho guard\n")
	sync(t, e.service)
	if got := read(t, filepath.Join(e.home, ".cursor", "hooks", "skillshare", "guard", "guard.sh")); got != "#!/bin/sh\necho guard\n" {
		t.Fatalf("script = %q", got)
	}

	source, err := LoadSource(e.config)
	must(t, err)
	disabled := source.Entries["guard"]
	off := false
	disabled.Enabled = &off
	save(t, e.service, Mutation{Name: "guard", Entry: &disabled})
	if got := read(t, e.config); !strings.Contains(got, "guard.sh:\n              from: hooks/guard.sh") || strings.Contains(got, "echo guard") || !strings.Contains(got, "echo inline") {
		t.Fatalf("saving must keep the reference and the inline file:\n%s", got)
	}
}

func TestFilesFrom_RejectsUnknownReferenceFields(t *testing.T) {
	_, err := ParseEntry([]byte("bindings:\n  cursor:\n    events: {beforeShellExecution: [{command: x}]}\n    files:\n      guard.sh: {path: hooks/guard.sh}\n"))
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestFilesFrom_KeepsAnchorsSharedAcrossBindings(t *testing.T) {
	e := newEnv(t)
	write(t, e.config, "targets: {}\nhooks:\n  entries:\n    guard:\n      bindings:\n        claude:\n          events: &ev\n            Stop:\n              - hooks:\n                  - {type: command, command: echo done}\n        codex:\n          events: *ev\n          files:\n            guard.sh: {from: hooks/guard.sh}\n")
	write(t, filepath.Join(filepath.Dir(e.config), "hooks", "guard.sh"), "echo guard\n")
	source, err := LoadSource(e.config)
	must(t, err)
	if b := source.Entries["guard"].Bindings["codex"]; len(b.Events) != 1 || b.Files["guard.sh"] != "echo guard\n" {
		t.Fatalf("codex binding = %+v", b)
	}
}

func TestFilesFrom_DropsTheReferenceOfARemovedFile(t *testing.T) {
	e := newEnv(t)
	removed := entry(t, `{"bindings":{"cursor":{"events":{"stop":[{"command":"echo"}]},"filesFrom":{"guard.sh":"hooks/guard.sh"}}}}`)
	save(t, e.service, Mutation{Name: "guard", Entry: removed})
	if got := read(t, e.config); strings.Contains(got, "guard.sh") {
		t.Fatalf("a removed file must not come back:\n%s", got)
	}
}

func TestFilesFrom_KeepsTheReferenceInAnAliasedBinding(t *testing.T) {
	e := newEnv(t)
	write(t, e.config, "targets: {}\nhooks:\n  entries:\n    a:\n      bindings:\n        cursor: &b\n          events: {stop: [{command: ./check.sh}]}\n          files:\n            check.sh: {from: hooks/check.sh}\n    b:\n      bindings:\n        cursor: *b\n")
	write(t, filepath.Join(filepath.Dir(e.config), "hooks", "check.sh"), "echo check\n")
	source, err := LoadSource(e.config)
	must(t, err)
	if got := source.Entries["b"].Bindings["cursor"].Files["check.sh"]; got != "echo check\n" {
		t.Fatalf("aliased check.sh = %q", got)
	}
}

func TestFilesFrom_RefusesAnEchoedEmptyFile(t *testing.T) {
	e := newEnv(t)
	write(t, filepath.Join(filepath.Dir(e.config), "hooks", "guard.sh"), "echo guard\n")
	cleared := entry(t, `{"bindings":{"cursor":{"events":{"stop":[{"command":"echo"}]},"files":{"guard.sh":""},"filesFrom":{"guard.sh":"hooks/guard.sh"}}}}`)
	if _, err := e.service.PreviewMutation(Mutation{Name: "guard", Entry: cleared}); err == nil || !strings.Contains(err.Error(), "differ") {
		t.Fatalf("err = %v, want a cleared file refused", err)
	}
}

func TestRenderNative_ReadsReferencesInsteadOfSentContents(t *testing.T) {
	e := newEnv(t)
	dir := filepath.Dir(e.config)
	write(t, filepath.Join(dir, "hooks", "lint.pi.ts"), "export default function (pi) {}\n")
	write(t, filepath.Join(dir, "hooks", "guard.sh"), "echo new\n")
	// The dashboard sends contents cached before the files changed.
	stale := entry(t, `{"bindings":{"pi":{"codeFile":"hooks/lint.pi.ts","code":"old"},"cursor":{"events":{"stop":[{"command":"echo"}]},"files":{"guard.sh":"echo old\n"},"filesFrom":{"guard.sh":"hooks/guard.sh"}}}}`)
	files, err := e.service.RenderNative(Mutation{Name: "lint", Entry: stale})
	must(t, err)
	var got []string
	for _, f := range files {
		got = append(got, f.Content)
	}
	if all := strings.Join(got, "\n"); !strings.Contains(all, "export default function (pi) {}") || !strings.Contains(all, "echo new") || strings.Contains(all, "old") {
		t.Fatalf("rendered %q", got)
	}
}

func TestFilesFrom_KeepsAnAnchoredReference(t *testing.T) {
	e := newEnv(t)
	write(t, e.config, "targets: {}\nhooks:\n  entries:\n    a:\n      bindings:\n        cursor:\n          events: {stop: [{command: ./check.sh}]}\n          files:\n            check.sh: &check {from: hooks/check.sh}\n        claude:\n          events: {Stop: [{hooks: [{type: command, command: ./check.sh}]}]}\n          files:\n            check.sh: *check\n")
	write(t, filepath.Join(filepath.Dir(e.config), "hooks", "check.sh"), "echo check\n")
	source, err := LoadSource(e.config)
	must(t, err)
	if got := source.Entries["a"].Bindings["claude"].Files["check.sh"]; got != "echo check\n" {
		t.Fatalf("aliased check.sh = %q", got)
	}
}

func TestFilesFrom_AcceptsReferencesInJSONEntries(t *testing.T) {
	e := newEnv(t)
	write(t, filepath.Join(filepath.Dir(e.config), "hooks", "guard.sh"), "echo guard\n")
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, `{"bindings":{"cursor":{"events":{"stop":[{"command":"echo"}]},"files":{"guard.sh":{"from":"hooks/guard.sh"}}}}}`)})
	if got := read(t, e.config); !strings.Contains(got, "from: hooks/guard.sh") {
		t.Fatalf("config:\n%s", got)
	}
	if _, err := ParseEntry([]byte(`{"bindings":{"cursor":{"files":{"guard.sh":{"path":"x"}}}}}`)); err == nil {
		t.Fatal("an unknown reference field must be refused")
	}
}
