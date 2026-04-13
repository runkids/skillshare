package config

import (
	"strings"
	"testing"
)

func TestResolveTargetNameCandidate_PrefersExactMatch(t *testing.T) {
	got, ok, err := resolveTargetNameCandidate("agents", []string{"universal", "agents"}, func(requested, candidate string) bool {
		return sameTargetSpecName(requested, candidate)
	})
	if err != nil {
		t.Fatalf("resolveTargetNameCandidate(exact) error = %v", err)
	}
	if !ok {
		t.Fatal("resolveTargetNameCandidate(exact) = not found, want found")
	}
	if got != "agents" {
		t.Fatalf("resolveTargetNameCandidate(exact) = %q, want %q", got, "agents")
	}
}

func TestResolveTargetNameCandidate_AliasResolvesCanonicalName(t *testing.T) {
	got, ok, err := ResolveTargetNameCandidate("agents", []string{"universal", "codex"})
	if err != nil {
		t.Fatalf("ResolveTargetNameCandidate(alias) error = %v", err)
	}
	if !ok {
		t.Fatal("ResolveTargetNameCandidate(alias) = not found, want found")
	}
	if got != "universal" {
		t.Fatalf("ResolveTargetNameCandidate(alias) = %q, want %q", got, "universal")
	}
}

func TestResolveTargetNameCandidate_MissingDoesNotCrossMatchSharedPathTargets(t *testing.T) {
	_, ok, err := ResolveTargetNameCandidate("agents", []string{"codex"})
	if err != nil {
		t.Fatalf("ResolveTargetNameCandidate(missing) error = %v", err)
	}
	if ok {
		t.Fatal("ResolveTargetNameCandidate(missing) = found, want not found")
	}
}

func TestResolveTargetNameCandidate_AmbiguousMatchFails(t *testing.T) {
	_, ok, err := resolveTargetNameCandidate("agents", []string{"universal", "codex"}, func(string, string) bool {
		return true
	})
	if err == nil {
		t.Fatal("resolveTargetNameCandidate(ambiguous) error = nil, want ambiguity error")
	}
	if ok {
		t.Fatal("resolveTargetNameCandidate(ambiguous) = found, want not found")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("resolveTargetNameCandidate(ambiguous) error = %v, want ambiguity message", err)
	}
}
