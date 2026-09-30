package hooks

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// allAgents is one entry that binds every Agent.
func allAgents(t *testing.T) *Entry {
	t.Helper()
	var bindings []string
	for _, target := range sortedKeys(adapterEntry) {
		bindings = append(bindings, `"`+target+`":`+adapterEntry[target])
	}
	return entry(t, `{"bindings":{`+strings.Join(bindings, ",")+`}}`)
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	list, err := os.ReadDir(dir)
	must(t, err)
	var names []string
	for _, e := range list {
		names = append(names, e.Name())
	}
	return names
}

// A file Skillshare created is deleted once removing the last hook leaves only what
// Skillshare put there, and so are the directories it created; the preview shows
// each deletion as a file whose new text is empty.
func TestRemove_DeletesFilesAndFoldersSkillshareCreated(t *testing.T) {
	e := newEnv(t)
	save(t, e.service, Mutation{Name: "demo", Entry: allAgents(t)})
	p, err := e.service.PreviewMutation(Mutation{Name: "demo", Remove: true})
	must(t, err)
	shared := 0
	for _, f := range p.Files() {
		if f.After != "" {
			t.Errorf("%s: remove must delete the created file, got %q", f.Path, f.After)
		}
		if strings.HasSuffix(f.Path, ".json") && !strings.Contains(f.Path, "copilot") {
			shared++
		}
	}
	if shared != 7 {
		t.Fatalf("every created shared hooks file must be a delete in the preview, got %d: %+v", shared, p.Files())
	}
	save(t, e.service, Mutation{Name: "demo", Remove: true})
	if left := entries(t, e.home); len(left) != 0 {
		t.Fatalf("created files and folders must be gone, left %v", left)
	}
}

func TestRemove_ProjectDeletesOnlyWhatSkillshareCreated(t *testing.T) {
	e := newEnv(t)
	root := t.TempDir()
	s := e.project(root)
	save(t, s, Mutation{Name: "demo", Entry: allAgents(t)})
	save(t, s, Mutation{Name: "demo", Remove: true})
	if left := entries(t, root); !slices.Equal(left, []string{".skillshare"}) {
		t.Fatalf("project must be back to its own config only, left %v", left)
	}
}

// Anything the user owns stays: a file or folder that existed before, and a created
// file that gained other settings or a comment.
func TestRemove_KeepsWhatTheUserOwns(t *testing.T) {
	e := newEnv(t)
	gemini := filepath.Join(e.home, ".gemini", "settings.json")
	write(t, gemini, "{}\n")
	must(t, os.MkdirAll(filepath.Join(e.home, ".cursor"), 0755))
	both := entry(t, `{"bindings":{"claude":`+adapterEntry["claude"]+`,"gemini":`+adapterEntry["gemini"]+`,"cursor":{"events":{"stop":[{"command":"echo stop"}]}},"codex":`+adapterEntry["codex"]+`}}`)
	save(t, e.service, Mutation{Name: "demo", Entry: both})
	claude := filepath.Join(e.home, ".claude", "settings.json")
	write(t, claude, strings.Replace(read(t, claude), "{", `{"theme": "dark",`, 1))
	codex := filepath.Join(e.home, ".codex", "hooks.json")
	write(t, codex, "// mine\n"+read(t, codex))
	save(t, e.service, Mutation{Name: "demo", Remove: true})

	if got := read(t, gemini); got != "{}\n" {
		t.Errorf("a file that existed before must stay, got %q", got)
	}
	if got := read(t, claude); !strings.Contains(got, `"theme"`) || strings.Contains(got, "hooks") {
		t.Errorf("a created file with user settings keeps them and loses only the hooks: %q", got)
	}
	if got := read(t, codex); !strings.Contains(got, "// mine") {
		t.Errorf("a created file with a user comment must stay: %q", got)
	}
	if _, err := os.Stat(filepath.Join(e.home, ".cursor", "hooks.json")); !os.IsNotExist(err) {
		t.Errorf("the created Cursor file must be deleted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(e.home, ".cursor")); err != nil {
		t.Errorf("a folder that existed before must stay: %v", err)
	}
}
