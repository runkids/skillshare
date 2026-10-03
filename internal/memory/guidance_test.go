package memory

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInstructionsProjectPathIsRelativeToProjectRoot(t *testing.T) {
	project := filepath.Join(t.TempDir(), "repo")
	got := Instructions(filepath.Join(project, ".skillshare", "extras", "memory"), project)
	if !strings.Contains(got, "`.skillshare/extras/memory` (relative to the project root)") || strings.Contains(got, project) {
		t.Fatalf("project guidance = %q", got)
	}
	outside := filepath.Join(t.TempDir(), "notes")
	if got := Instructions(outside, project); !strings.Contains(got, "`"+filepath.ToSlash(outside)+"`") || strings.Contains(got, "relative") {
		t.Fatalf("outside guidance = %q", got)
	}
	if got := Instructions(outside, ""); !strings.Contains(got, "scope=global") || !strings.Contains(got, "## Shared memory") {
		t.Fatalf("global guidance = %q", got)
	}
}

func TestApplyKeepsOutsideContentAndIsIdempotent(t *testing.T) {
	want := Instructions("/notes", "")
	before := "# Mine\n\nKeep this.\n"
	after, err := Apply(before, want)
	if err != nil || !strings.HasPrefix(after, before) || Inspect(after, want) != StateConfigured {
		t.Fatalf("after = %q, %v", after, err)
	}
	if again, err := Apply(after, want); err != nil || again != after {
		t.Fatalf("second apply changed the file: %q, %v", again, err)
	}
}

func TestApplyReplacesOnlyOutdatedBlock(t *testing.T) {
	content := "top\r\n\r\n" + strings.ReplaceAll(Instructions("/old", ""), "\n", "\r\n") + "bottom\r\n"
	want := Instructions("/new", "")
	if Inspect(content, want) != StateOutdated {
		t.Fatalf("state = %s", Inspect(content, want))
	}
	after, err := Apply(content, want)
	if err != nil || !strings.HasPrefix(after, "top\r\n\r\n") || !strings.HasSuffix(after, "-->\r\nbottom\r\n") || !strings.Contains(after, "`/new`") || strings.Contains(after, "/old") {
		t.Fatalf("after = %q, %v", after, err)
	}
	if Inspect(after, want) != StateConfigured {
		t.Fatalf("state after = %s", Inspect(after, want))
	}
}

func TestApplyRefusesUserChangedBlocks(t *testing.T) {
	want := Instructions("/notes", "")
	modified := strings.Replace(want, "Read only", "Always read", 1)
	unclosed := strings.Replace(want, blockEnd+"\n", "", 1)
	twice := want + "\n" + want
	for name, content := range map[string]string{StateModified: modified, StateMalformed: unclosed, StateMalformed + "-twice": twice} {
		state := strings.TrimSuffix(name, "-twice")
		if got := Inspect(content, want); got != state {
			t.Errorf("%s: state = %s", name, got)
		}
		if _, err := Apply(content, want); err == nil {
			t.Errorf("%s: apply succeeded", name)
		}
	}
}

func TestInspectOwnsOnlyMarkedBlocksOfItsScope(t *testing.T) {
	want := Instructions("/notes", "")
	manual := "## Shared memory\n\nShared notes directory: `/notes`\n"
	project := Instructions("notes", "/repo")
	fenced := "```\n" + want + "```\n"
	for _, content := range []string{manual, project, fenced} {
		if got := Inspect(content, want); got != StateUnconfigured {
			t.Errorf("Inspect(%q) = %s", content, got)
		}
	}
}

func TestInspectIgnoresMarkersInCodeExamples(t *testing.T) {
	want := Instructions("/notes", "")
	indented := "    " + strings.ReplaceAll(strings.TrimSuffix(want, "\n"), "\n", "\n    ") + "\n"
	for name, content := range map[string]string{
		"tilde":         "~~~markdown\n" + want + "~~~\n",
		"long backtick": "````\n```\n" + want + "```\n````\n",
		"tilde in ```":  "```\n~~~\n```\n" + "text\n",
		"indented":      "Example:\n\n" + indented,
		"tab":           "\t" + strings.SplitN(want, "\n", 2)[0] + "\n",
	} {
		if got := Inspect(content, want); got != StateUnconfigured {
			t.Errorf("%s: state = %s", name, got)
		}
	}
	after := "~~~\n" + want + "~~~\n\n" + want
	if got := Inspect(after, want); got != StateConfigured {
		t.Errorf("block after a closed fence: state = %s", got)
	}
	if out, err := Apply("~~~\n"+want+"~~~\n", want); err != nil || !strings.HasPrefix(out, "~~~\n"+want+"~~~\n\n<!--") {
		t.Errorf("apply beside example = %q, %v", out, err)
	}
}
