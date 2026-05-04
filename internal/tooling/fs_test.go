package tooling

import (
	"strings"
	"testing"
)

func TestEnsureManagedTOMLBoolMatchesQuotedHeaderWithComment(t *testing.T) {
	initial := `
[plugins.'demo@skillshare']   # existing plugin
enabled = false
`
	got := EnsureManagedTOMLBool(initial, []string{"plugins", `"demo@skillshare"`}, "enabled", true)
	if strings.Count(got, "enabled = true") != 1 {
		t.Fatalf("expected one updated enabled line, got:\n%s", got)
	}
	if strings.Contains(got, `[plugins."demo@skillshare"]`+"\nenabled = true\n\n[plugins.'demo@skillshare']") {
		t.Fatalf("expected existing table to be updated instead of duplicated:\n%s", got)
	}
}

func TestEnsureManagedTOMLBoolMatchesWhitespaceWrappedHeader(t *testing.T) {
	initial := `
 [ features ]   # keep comment
codex_hooks = false
`
	got := EnsureManagedTOMLBool(initial, []string{"features"}, "codex_hooks", true)
	if strings.Count(got, "codex_hooks = true") != 1 {
		t.Fatalf("expected updated codex_hooks line, got:\n%s", got)
	}
	if strings.Count(got, "[features]") > 1 || strings.Count(got, "[ features ]") > 1 {
		t.Fatalf("expected a single features table, got:\n%s", got)
	}
}
