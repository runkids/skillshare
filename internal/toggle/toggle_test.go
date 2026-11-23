package toggle

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func source(t *testing.T, agent bool, files map[string]string) Source {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return Source{Agent: agent, Root: root}
}

func lines(t *testing.T, src Source, arg string, enable bool) []string {
	t.Helper()
	set, err := src.Discover()
	if err != nil {
		t.Fatal(err)
	}
	got, _ := set.Lines(arg, enable)
	return got
}

func TestApply_BlockedNamesTheFileThatDecides(t *testing.T) {
	src := source(t, false, map[string]string{
		"frontend/react-ui/SKILL.md": "# ui",
		".skillignore":               "frontend/\n",
		"_team/.git/HEAD":            "ref: refs/heads/main\n",
		"_team/foo/SKILL.md":         "# foo",
		"_team/.skillignore":         "foo\n",
	})
	res, err := src.Apply(true, nil, []string{"frontend/react-ui", "_team/foo"})
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"another rule in .skillignore or .skillignore.local", "_team/.skillignore"} {
		if msg := res.Items[i].Blocked; !strings.Contains(msg, want) {
			t.Errorf("%s: Blocked = %q, want it to name %q", res.Items[i].RelPath, msg, want)
		}
	}
}

func TestApply_DisableOverriddenByNegation(t *testing.T) {
	src := source(t, true, map[string]string{"mine.md": "# m", ".agentignore.local": "!mine.md\n"})
	res, err := src.Apply(false, []string{"mine.md"}, []string{"mine.md"})
	if err != nil {
		t.Fatal(err)
	}
	if msg := res.Items[0].Blocked; !strings.Contains(msg, "still enabled") {
		t.Errorf("Blocked = %q, want still enabled", msg)
	}
}

func TestApply_ReportsTheLinesThatChangedAnItem(t *testing.T) {
	src := source(t, false, map[string]string{"group/inner/SKILL.md": "# i", ".skillignore": "group/inner\n"})
	res, err := src.Apply(true, []string{"inner", "group/inner"}, []string{"group/inner"})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Items[0]; got.Blocked != "" || !slices.Equal(got.Lines, []string{"group/inner"}) {
		t.Errorf("outcome = %+v, want enabled by the group/inner line", got)
	}
}

func TestLines(t *testing.T) {
	skills := map[string]string{
		"frontend/react-ui/SKILL.md": "# ui",
		"react-ui/SKILL.md":          "# ui",
		"archive/old/SKILL.md":       "# old",
		"group/inner/SKILL.md":       "# i",
		"other/inner/SKILL.md":       "# i",
	}
	agents := map[string]string{
		"reviewer.md":      "# r",
		"team/helper.md":   "# h",
		"rev.md":           "---\nname: critic\n---\n",
		"sub/reviewer2.md": "# r2",
	}
	for _, tc := range []struct {
		name   string
		agent  bool
		arg    string
		enable bool
		want   []string
	}{
		{"glob is written as typed", false, "draft-*", false, []string{"draft-*"}},
		{"folder name that matches is written as typed", false, "react-ui", false, []string{"react-ui"}},
		{"skill flat name writes the relPath", false, "archive__old", false, []string{"archive/old"}},
		{"enable removes the relPath line of every skill a folder name names", false, "inner", true, []string{"inner", "group/inner", "other/inner"}},
		{"exact relPath names that skill alone", false, "react-ui", true, []string{"react-ui"}},
		{"agent without .md gets .md", true, "reviewer", false, []string{"reviewer.md"}},
		{"nested agent without .md gets .md", true, "team/helper", false, []string{"team/helper.md"}},
		{"agent frontmatter name writes the relPath", true, "critic", false, []string{"rev.md"}},
		{"agent flat name writes the relPath", true, "sub__reviewer2.md", false, []string{"sub/reviewer2.md"}},
		{"nested agent file name with .md names it", true, "reviewer2.md", true, []string{"reviewer2.md", "sub/reviewer2.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := skills
			if tc.agent {
				files = agents
			}
			if got := lines(t, source(t, tc.agent, files), tc.arg, tc.enable); !slices.Equal(got, tc.want) {
				t.Errorf("Lines(%q) = %v, want %v", tc.arg, got, tc.want)
			}
		})
	}
}
