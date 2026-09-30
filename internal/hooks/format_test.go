package hooks

import (
	"path/filepath"
	"strings"
	"testing"
)

const stopEntry = `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"echo stop"}]}]}}}}`

func TestSync_CompactFileStaysCompact(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, `{"model":"opus","hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"g"}]}]}}`)
	save(t, e.service, Mutation{Name: "stop", Entry: entry(t, stopEntry)})
	want := `{"model":"opus","hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"g"}]}],"Stop":[{"hooks":[{"command":"echo stop","type":"command"}]}]}}` + "\n"
	if got := read(t, path); got != want {
		t.Fatalf("added:\n%s", got)
	}
	save(t, e.service, Mutation{Name: "stop", Remove: true})
	if got := read(t, path); got != `{"model":"opus","hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"g"}]}]}}`+"\n" {
		t.Fatalf("removed:\n%s", got)
	}
}

func TestSync_EmptiedHooksKeyFollowsWhoCreatedIt(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, "{\n    \"model\": \"opus\"\n}\n")
	save(t, e.service, Mutation{Name: "stop", Entry: entry(t, stopEntry)})
	if got := read(t, path); !strings.Contains(got, "\n    \"hooks\": {\n        \"Stop\": [\n            {\n") {
		t.Fatalf("indentation must follow the file:\n%s", got)
	}
	save(t, e.service, Mutation{Name: "stop", Remove: true})
	if got := read(t, path); got != "{\n    \"model\": \"opus\"\n}\n" {
		t.Fatalf("Skillshare created hooks, so it removes the emptied key:\n%q", got)
	}

	write(t, path, `{"model":"opus","hooks":{}}`)
	save(t, e.service, Mutation{Name: "stop", Entry: entry(t, stopEntry)})
	save(t, e.service, Mutation{Name: "stop", Remove: true})
	if got := read(t, path); got != `{"model":"opus","hooks":{}}`+"\n" {
		t.Fatalf("a hooks key the user had stays:\n%q", got)
	}
}

func TestPlanFiles_AfterIsWhatSyncWrites(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, `{"model":"opus"}`)
	m := Mutation{Name: "stop", Entry: entry(t, `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"echo stop"}]}]}},"opencode":{"code":"export default {}\n"}}}`)}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	files := p.Files()
	if len(files) != 2 || files[0].Before != `{"model":"opus"}` || files[1].Before != "" {
		t.Fatalf("files: %+v", files)
	}
	save(t, e.service, m)
	for _, f := range files {
		if got := read(t, f.Path); got != f.After {
			t.Fatalf("%s: preview after differs from the written file:\n%q\n%q", f.Target, f.After, got)
		}
	}
	p, err = e.service.Preview()
	must(t, err)
	if len(p.Files()) != 0 {
		t.Fatalf("unchanged files are skipped: %+v", p.Files())
	}
}

func TestSync_OneLineArraysStayOnOneLine(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, "{\n  \"hooks\": {\n    \"Stop\": [{\"hooks\": [{\"type\": \"command\", \"command\": \"mine\"}]}]\n  }\n}\n")
	save(t, e.service, Mutation{Name: "stop", Entry: entry(t, stopEntry)})
	want := "{\n  \"hooks\": {\n    \"Stop\": [{\"hooks\": [{\"type\": \"command\", \"command\": \"mine\"}]}, {\"hooks\": [{\"command\": \"echo stop\", \"type\": \"command\"}]}]\n  }\n}\n"
	if got := read(t, path); got != want {
		t.Fatalf("appended:\n%s", got)
	}
}

// An element replaced in place keeps its key order and the separators of the line it is on.
func TestSync_ReplacedElementKeepsKeyOrderAndSpacing(t *testing.T) {
	for _, tc := range []struct{ name, file string }{
		{"spaced", "{\n  \"hooks\": {\n    \"Stop\": [{\"hooks\": [{\"type\": \"command\", \"command\": \"chime\", \"timeout\": 5}]}]\n  }\n}\n"},
		{"compact", `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"chime","timeout":5}]}]}}` + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			path := filepath.Join(e.home, ".claude", "settings.json")
			write(t, path, tc.file)
			c := importOne(t, e.service, "claude", "claude-stop")
			save(t, e.service, Mutation{Name: c.Name, Entry: &c.Entry, Adopt: true})
			save(t, e.service, Mutation{Name: c.Name, Entry: entry(t, `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"chime","timeout":10}]}]}}}}`)})
			if got, want := read(t, path), strings.Replace(tc.file, "5}", "10}", 1); got != want {
				t.Fatalf("replaced:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}
