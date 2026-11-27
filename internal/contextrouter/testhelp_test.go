package contextrouter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type goldenSizes struct {
	Kernel *int            `json:"kernel"`
	Topics map[string]*int `json:"topics"`
}

type goldenCase struct {
	Name    string            `json:"name"`
	Compact bool              `json:"compact"`
	Files   map[string]string `json:"files"`
	Expect  struct {
		goldenSizes
		// Descriptions are what the reference's `list` prints; Order is the config's.
		Descriptions map[string]*string `json:"descriptions"`
		Order        []string           `json:"order"`
	} `json:"expect"`
	// Legacy holds what older router scripts print for the same folder, by version.
	Legacy map[string]goldenSizes `json:"legacy"`
}

// withScript adds a router script that declares version to the case's folder.
func (c goldenCase) withScript(t *testing.T, version string) string {
	t.Helper()
	dir := c.materialize(t)
	writeFile(t, filepath.Join(dir, filepath.FromSlash(ScriptRel)), "#!/usr/bin/env python3\nVERSION = '"+version+"'  # x-release-please-version\n")
	return dir
}

func loadCases(t *testing.T) []goldenCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Reference string       `json:"reference"`
		Cases     []goldenCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if file.Reference != Reference {
		t.Fatalf("goldens come from %s, the port follows %s", file.Reference, Reference)
	}
	return file.Cases
}

func goldenNamed(t *testing.T, name string) goldenCase {
	t.Helper()
	for _, c := range loadCases(t) {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no golden case %q", name)
	return goldenCase{}
}

// materialize writes the case's folder to a temporary directory.
func (c goldenCase) materialize(t *testing.T) string {
	t.Helper()
	return writeTree(t, c.Files)
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, text := range files {
		writeFile(t, filepath.Join(dir, filepath.FromSlash(rel)), text)
	}
	return dir
}

func sameSize(got, want *int) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}

func show(n *int) any {
	if n == nil {
		return nil
	}
	return *n
}
