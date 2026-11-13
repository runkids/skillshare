package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/audit"
)

func extraGateEnvironment(t *testing.T) (staged, root, dest string) {
	t.Helper()
	base := t.TempDir()
	t.Setenv("HOME", base)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "xdg"))
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(base, "config", "config.yaml"))
	audit.ResetGlobalCache()
	t.Cleanup(audit.ResetGlobalCache)
	staged = filepath.Join(base, "candidate", "staged")
	root = filepath.Join(base, "extras")
	if err := os.MkdirAll(staged, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	return staged, root, filepath.Join(root, "rules")
}

func TestGateExtraCandidate_ScanErrorBlocksEvenWithForce(t *testing.T) {
	staged, root, dest := extraGateEnvironment(t)
	old := scanExtraCandidate
	scanExtraCandidate = func(g AuditGate) (*AuditGateResult, error) {
		return nil, &AuditGateError{ScanErr: errors.New("synthetic scanner failure"), Rollback: CheckoutUnchanged}
	}
	defer func() { scanExtraCandidate = old }()
	for _, opts := range []InstallOptions{{}, {Force: true}, {AuditOverride: true}, {Force: true, AuditOverride: true}} {
		_, err := gateExtraCandidate(staged, root, dest, strings.Repeat("a", 40), opts)
		if !errors.Is(err, audit.ErrBlocked) {
			t.Errorf("opts %+v: got %v", opts, err)
		}
	}
}

func TestGateExtraCandidate_BadRulesBlock(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			staged, root, dest := extraGateEnvironment(t)
			projectRoot := ""
			path := audit.GlobalAuditRulesPath()
			if project {
				projectRoot = filepath.Join(t.TempDir(), "project")
				path = filepath.Join(projectRoot, ".skillshare", "audit-rules.yaml")
			}
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("rules: [invalid"), 0644); err != nil {
				t.Fatal(err)
			}
			for _, override := range []bool{false, true} {
				audit.ResetGlobalCache()
				_, err := gateExtraCandidate(staged, root, dest, strings.Repeat("a", 40), InstallOptions{AuditProjectRoot: projectRoot, AuditOverride: override})
				if err == nil {
					t.Fatalf("project=%v override=%v: accepted invalid rules", project, override)
				}
			}
		})
	}
}

func TestGateExtraCandidate_FindingOverrideAndNoPersistence(t *testing.T) {
	staged, root, dest := extraGateEnvironment(t)
	rulePath := audit.GlobalAuditRulesPath()
	if err := os.MkdirAll(filepath.Dir(rulePath), 0755); err != nil {
		t.Fatal(err)
	}
	rule := "rules:\n  - id: synthetic-finding\n    severity: CRITICAL\n    pattern: synthetic\n    message: synthetic finding\n    regex: 'SYNTHETIC_BAD_CONTENT'\n"
	if err := os.WriteFile(rulePath, []byte(rule), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged, "rules.md"), []byte("SYNTHETIC_BAD_CONTENT\n"), 0644); err != nil {
		t.Fatal(err)
	}
	audit.ResetGlobalCache()
	for _, tc := range []struct{ force, override, blocked bool }{{false, false, true}, {true, false, true}, {false, true, false}} {
		result, err := gateExtraCandidate(staged, root, dest, strings.Repeat("a", 40), InstallOptions{Force: tc.force, AuditOverride: tc.override})
		if tc.blocked {
			if !errors.Is(err, audit.ErrBlocked) {
				t.Fatalf("force=%v: %v", tc.force, err)
			}
		} else if err != nil || result == nil || !result.Overridden {
			t.Fatalf("override: result=%+v err=%v", result, err)
		}
		if _, err := os.Stat(filepath.Join(root, ".metadata.json")); !os.IsNotExist(err) {
			t.Fatalf("gate wrote metadata: %v", err)
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Fatalf("gate wrote destination: %v", err)
		}
	}
}

func TestGateExtraCandidate_Skip(t *testing.T) {
	staged, root, dest := extraGateEnvironment(t)
	called := false
	old := scanExtraCandidate
	scanExtraCandidate = func(AuditGate) (*AuditGateResult, error) { called = true; return nil, errors.New("unexpected scan") }
	defer func() { scanExtraCandidate = old }()
	res, err := gateExtraCandidate(staged, root, dest, "", InstallOptions{SkipAudit: true})
	if err != nil || res == nil || !res.Skipped || called {
		t.Fatalf("skip: %+v %v called=%v", res, err, called)
	}
}

func TestGateExtraCandidate_DoesNotMigrateExistingMetadata(t *testing.T) {
	staged, root, dest := extraGateEnvironment(t)
	if err := os.Mkdir(dest, 0755); err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(dest, MetaFileName)
	original := []byte(`{"source":"https://example.com/org/repo","type":"github"}`)
	if err := os.WriteFile(sidecar, original, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := gateExtraCandidate(staged, root, dest, strings.Repeat("a", 40), InstallOptions{AuditOverride: true})
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(sidecar)
	if err != nil || string(after) != string(original) {
		t.Fatalf("sidecar changed: %q %v", after, err)
	}
	if _, err := os.Stat(filepath.Join(root, MetadataFileName)); !os.IsNotExist(err) {
		t.Fatalf("metadata created: %v", err)
	}
}

func TestGateExtraCandidate_ExistingMetadataUnchanged(t *testing.T) {
	staged, root, dest := extraGateEnvironment(t)
	original := []byte(`{"version":1,"entries":{"rules":{"source":"https://example.com/repo"}},"audit_accepted":{"other":["existing"]}}`)
	path := filepath.Join(root, MetadataFileName)
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := gateExtraCandidate(staged, root, dest, strings.Repeat("a", 40), InstallOptions{AuditOverride: true})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(original) {
		t.Fatalf("metadata changed: %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(staged), MetadataFileName)); !os.IsNotExist(err) {
		t.Fatalf("candidate metadata written: %v", err)
	}
}

func TestGateExtraCandidate_UsesCandidateAndExplicitOverride(t *testing.T) {
	staged, root, dest := extraGateEnvironment(t)
	old := scanExtraCandidate
	scanExtraCandidate = func(g AuditGate) (*AuditGateResult, error) {
		if g.SourceDir != root || g.RepoPath != staged || g.AcceptPath != dest || g.BeforeHash != strings.Repeat("a", 40) || !g.Candidate || !g.ReadOnlyMetadata || g.Override || g.Threshold != audit.SeverityHigh {
			t.Errorf("bad candidate gate: %+v", g)
		}
		return &AuditGateResult{Audit: &audit.Result{}, Threshold: g.Threshold}, nil
	}
	defer func() { scanExtraCandidate = old }()
	if _, err := gateExtraCandidate(staged, root, dest, strings.Repeat("a", 40), InstallOptions{Force: true, AuditThreshold: audit.SeverityHigh}); err != nil {
		t.Fatal(err)
	}
}
