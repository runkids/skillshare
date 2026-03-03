package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAuditConfig_RoundTrip(t *testing.T) {
	input := "block_threshold: HIGH\nprofile: strict\ndedupe_mode: global\n"
	var ac AuditConfig
	if err := yaml.Unmarshal([]byte(input), &ac); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ac.BlockThreshold != "HIGH" {
		t.Errorf("block_threshold = %q, want HIGH", ac.BlockThreshold)
	}
	if ac.Profile != "strict" {
		t.Errorf("profile = %q, want strict", ac.Profile)
	}
	if ac.DedupeMode != "global" {
		t.Errorf("dedupe_mode = %q, want global", ac.DedupeMode)
	}

	out, err := yaml.Marshal(&ac)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "profile: strict") {
		t.Errorf("marshal missing profile: %s", s)
	}
	if !strings.Contains(s, "dedupe_mode: global") {
		t.Errorf("marshal missing dedupe_mode: %s", s)
	}
}

func TestAuditConfig_EmptyOmitsNewFields(t *testing.T) {
	ac := AuditConfig{BlockThreshold: "CRITICAL"}
	out, _ := yaml.Marshal(&ac)
	s := string(out)
	if strings.Contains(s, "profile") {
		t.Errorf("empty profile should be omitted: %s", s)
	}
	if strings.Contains(s, "dedupe_mode") {
		t.Errorf("empty dedupe_mode should be omitted: %s", s)
	}
}
