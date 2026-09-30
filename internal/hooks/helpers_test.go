package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tailscale/hujson"
)

type env struct {
	t       *testing.T
	home    string
	config  string
	service *Service
}

// newEnv is a global scope with an isolated home, state and config.
func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	config := filepath.Join(dir, "skillshare", "config.yaml")
	must(t, os.MkdirAll(home, 0755))
	write(t, config, "targets: {}\n")
	return &env{t: t, home: home, config: config, service: &Service{ConfigPath: config, Home: home, StateDir: filepath.Join(dir, "state"), ConfigDirs: map[string]string{}}}
}

// project is a project scope sharing e's state directory.
func (e *env) project(root string) *Service {
	config := filepath.Join(root, ".skillshare", "config.yaml")
	write(e.t, config, "targets: []\n")
	return &Service{ConfigPath: config, ProjectRoot: root, Home: e.home, StateDir: e.service.StateDir}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	must(t, os.MkdirAll(filepath.Dir(path), 0755))
	must(t, os.WriteFile(path, []byte(content), 0644))
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	must(t, err)
	return string(data)
}

func entry(t *testing.T, doc string) *Entry {
	t.Helper()
	e, err := ParseEntry([]byte(doc))
	must(t, err)
	return &e
}

// save previews then saves a mutation with sync, as the CLI does.
func save(t *testing.T, s *Service, m Mutation) *Result {
	t.Helper()
	p, err := s.PreviewMutation(m)
	must(t, err)
	r, err := s.Mutate(m, p.Revision, true)
	if err != nil {
		t.Fatalf("mutate: %v (changes %+v)", err, p.Changes)
	}
	return r
}

func sync(t *testing.T, s *Service) *Result { return save(t, s, Mutation{}) }

func actions(p *Plan) string {
	var out []string
	for _, c := range p.Changes {
		out = append(out, c.Target+":"+c.Name+":"+c.Action)
	}
	return strings.Join(out, ",")
}

// events decodes a shared hooks file's event map.
func events(t *testing.T, path string, wrapped bool) map[string][]any {
	t.Helper()
	v, err := hujson.Parse([]byte(read(t, path)))
	must(t, err)
	v.Standardize()
	var doc map[string]any
	must(t, json.Unmarshal(v.Pack(), &doc))
	section := doc
	if wrapped {
		section, _ = doc["hooks"].(map[string]any)
	}
	out := map[string][]any{}
	for k, v := range section {
		out[k], _ = v.([]any)
	}
	return out
}

const claudeEntry = `{"bindings":{"claude":{"events":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo guard","timeout":5}]}]}}}}`
