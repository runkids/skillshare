package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/audit"
)

func newRemoteExtraRequest(t *testing.T) RemoteExtraRequest {
	t.Helper()
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(base, "config", "config.yaml"))
	remote, _, _ := setupPinRemote(t)
	source := &Source{Type: SourceTypeGitHTTPS, Raw: remote, CloneURL: remote}
	config := filepath.Join(base, "config.yaml")
	if err := os.WriteFile(config, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	return RemoteExtraRequest{Name: "rules", RawSource: remote, Source: source, ExtrasRoot: filepath.Join(base, "extras"), ConfigPath: config, Opts: InstallOptions{SkipAudit: true},
		Recheck: func() error { return nil }, WriteConfig: func() error { return os.WriteFile(config, []byte("after"), 0644) }, RevertConfig: func() error { return os.WriteFile(config, []byte("before"), 0644) },
		RepairConfig: func(_, _ string) error { return nil }, RefreshAfterRepair: func() error { return nil },
	}
}

func TestInstallRemoteExtra_AcceptedFindingsPublishAndRollback(t *testing.T) {
	req := newRemoteExtraRequest(t)
	if err := os.MkdirAll(req.ExtrasRoot, 0755); err != nil {
		t.Fatal(err)
	}
	store := NewMetadataStore()
	store.AuditAccepted = map[string][]string{"other": {"keep"}}
	if err := store.Save(req.ExtrasRoot); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(req.ExtrasRoot, MetadataFileName))
	if err != nil {
		t.Fatal(err)
	}
	req.Opts.SkipAudit = false
	req.Opts.AuditProjectRoot = t.TempDir()
	req.Opts.AuditOverride = true
	oldScan, oldBoundary := scanExtraCandidate, extraBoundary
	scanExtraCandidate = func(AuditGate) (*AuditGateResult, error) {
		return &AuditGateResult{Overridden: true, Threshold: "HIGH", Audit: &audit.Result{Findings: []audit.Finding{{Severity: "HIGH", RuleID: "synthetic", File: "SKILL.md", Line: 1, Snippet: "synthetic"}}}}, nil
	}
	extraBoundary = func(step string) error {
		if step == "live" {
			got, err := os.ReadFile(filepath.Join(req.ExtrasRoot, MetadataFileName))
			if err != nil {
				return err
			}
			if string(got) != string(before) {
				return errors.New("accepted before publication")
			}
		}
		if step == "metadata" {
			got, _, err := ReadMetadata(req.ExtrasRoot)
			if err != nil {
				return err
			}
			if len(got.AuditAccepted[req.Name]) == 0 {
				return errors.New("accepted finding missing")
			}
		}
		if step == "config" {
			return errors.New("fault")
		}
		return nil
	}
	defer func() { scanExtraCandidate, extraBoundary = oldScan, oldBoundary }()
	if _, err := InstallRemoteExtra(req); err == nil || !strings.Contains(err.Error(), "fault") {
		t.Fatalf("rollback: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(req.ExtrasRoot, MetadataFileName))
	if err != nil || string(after) != string(before) {
		t.Fatalf("accepted snapshot changed: %q %v", after, err)
	}
}

func TestInstallRemoteExtra_InstallAndProtectEdits(t *testing.T) {
	req := newRemoteExtraRequest(t)
	result, err := InstallRemoteExtra(req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "installed" || len(result.Commit) != 40 || !result.AuditSkipped {
		t.Fatalf("result: %+v", result)
	}
	dir := filepath.Join(req.ExtrasRoot, req.Name)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("local edit"), 0644); err != nil {
		t.Fatal(err)
	}
	req.Opts.Force = true
	if _, err := InstallRemoteExtra(req); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("implicit force overwrote edits: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil || string(data) != "local edit" {
		t.Fatalf("edit lost: %q %v", data, err)
	}
}

func TestInstallRemoteExtra_ProtectsEditDuringFetch(t *testing.T) {
	req := newRemoteExtraRequest(t)
	if _, err := InstallRemoteExtra(req); err != nil {
		t.Fatal(err)
	}
	checks := 0
	file := filepath.Join(req.ExtrasRoot, req.Name, "SKILL.md")
	req.Recheck = func() error {
		checks++
		if checks == 2 {
			return os.WriteFile(file, []byte("edit during fetch"), 0644)
		}
		return nil
	}
	if _, err := InstallRemoteExtra(req); err == nil {
		t.Fatal("edit during fetch was overwritten")
	}
	got, err := os.ReadFile(file)
	if err != nil || string(got) != "edit during fetch" {
		t.Fatalf("edit lost: %q %v", got, err)
	}
}

func TestInstallRemoteExtra_RollbackPreservesConcurrentLiveEdit(t *testing.T) {
	req := newRemoteExtraRequest(t)
	old := extraBoundary
	extraBoundary = func(step string) error {
		if step == "live" {
			if err := os.WriteFile(filepath.Join(req.ExtrasRoot, req.Name, "SKILL.md"), []byte("concurrent edit"), 0644); err != nil {
				return err
			}
			return errors.New("fault")
		}
		return nil
	}
	defer func() { extraBoundary = old }()
	if _, err := InstallRemoteExtra(req); err == nil {
		t.Fatal("fault ignored")
	}
	got, err := os.ReadFile(filepath.Join(req.ExtrasRoot, req.Name, "SKILL.md"))
	if err != nil || string(got) != "concurrent edit" {
		t.Fatalf("edit lost: %q %v", got, err)
	}
	if _, err := os.Lstat(extraJournalPath(req.ExtrasRoot, req.Name)); err != nil {
		t.Fatalf("marker lost: %v", err)
	}
}

func TestInstallRemoteExtra_FaultRollsBack(t *testing.T) {
	for _, boundary := range []string{"marker", "incoming", "ignore", "previous", "live", "metadata", "lock", "config"} {
		t.Run(boundary, func(t *testing.T) {
			req := newRemoteExtraRequest(t)
			old := extraBoundary
			extraBoundary = func(step string) error {
				if step == boundary {
					return errors.New("synthetic failure")
				}
				return nil
			}
			defer func() { extraBoundary = old }()
			if _, err := InstallRemoteExtra(req); err == nil {
				t.Fatal("fault accepted")
			}
			data, err := os.ReadFile(req.ConfigPath)
			if err != nil || string(data) != "before" {
				t.Fatalf("config not restored: %q %v", data, err)
			}
			if _, err := os.Lstat(filepath.Join(req.ExtrasRoot, req.Name)); !os.IsNotExist(err) {
				t.Fatalf("directory remains: %v", err)
			}
			if _, err := os.Lstat(extraJournalPath(req.ExtrasRoot, req.Name)); !os.IsNotExist(err) {
				t.Fatalf("marker remains after clean rollback: %v", err)
			}
		})
	}
}

func TestInstallRemoteExtra_RepairUsesJournalAndRefreshesBaseline(t *testing.T) {
	req := newRemoteExtraRequest(t)
	req.Branch = "main"
	req.Source.Branch = "main"
	journalRemote := req.RawSource
	oldBoundary := extraBoundary
	extraBoundary = func(step string) error {
		if step == "live" {
			return errSimulatedDeath
		}
		return nil
	}
	_, err := InstallRemoteExtra(req)
	extraBoundary = oldBoundary
	if !errors.Is(err, errSimulatedDeath) {
		t.Fatalf("expected interrupted install: %v", err)
	}
	other, _, _ := setupPinRemote(t)
	req.RawSource = other
	req.Source = &Source{Type: SourceTypeGitHTTPS, Raw: other, CloneURL: other, Branch: "feature"}
	req.Branch = "feature"
	baseline, refreshed := "old", false
	req.RepairConfig = func(remote, branch string) error {
		if remote != journalRemote || branch != "main" {
			t.Fatalf("repair received requested origin rather than marker: %q %q", remote, branch)
		}
		baseline = "repaired"
		return nil
	}
	req.RefreshAfterRepair = func() error { refreshed = true; baseline = "fresh"; return nil }
	req.Recheck = func() error {
		if baseline != "fresh" {
			return errors.New("stale baseline")
		}
		return errors.New("requested source conflicts")
	}
	_, err = InstallRemoteExtra(req)
	if !refreshed || err == nil {
		t.Fatalf("refresh=%v, requested install error=%v", refreshed, err)
	}
	if _, err := os.Lstat(extraJournalPath(req.ExtrasRoot, req.Name)); !os.IsNotExist(err) {
		t.Fatalf("successful repair left marker: %v", err)
	}
}

func TestInstallRemoteExtra_RepairConfigConflictRetainsMarker(t *testing.T) {
	for _, conflict := range []string{"local", "different origin"} {
		t.Run(conflict, func(t *testing.T) {
			req := newRemoteExtraRequest(t)
			oldBoundary := extraBoundary
			extraBoundary = func(step string) error {
				if step == "live" {
					return errSimulatedDeath
				}
				return nil
			}
			_, err := InstallRemoteExtra(req)
			extraBoundary = oldBoundary
			if !errors.Is(err, errSimulatedDeath) {
				t.Fatalf("expected interruption: %v", err)
			}
			refreshed := false
			req.RepairConfig = func(_, _ string) error { return errors.New(conflict) }
			req.RefreshAfterRepair = func() error { refreshed = true; return nil }
			if _, err := InstallRemoteExtra(req); err == nil || !strings.Contains(err.Error(), conflict) {
				t.Fatalf("conflict accepted: %v", err)
			}
			if refreshed {
				t.Fatal("refreshed after failed repair")
			}
			if _, err := os.Lstat(extraJournalPath(req.ExtrasRoot, req.Name)); err != nil {
				t.Fatalf("marker lost: %v", err)
			}
		})
	}
}

func TestInstallRemoteExtra_RepairsInterrupted(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, replacement := range []bool{false, true} {
			for _, boundary := range []string{"marker", "incoming", "ignore", "previous", "live", "metadata", "lock", "config", "cleanup", "cleaned-previous"} {
				t.Run(fmt.Sprintf("project=%t/replacement=%t/%s", project, replacement, boundary), func(t *testing.T) {
					req := newRemoteExtraRequest(t)
					if project {
						req.ProjectRoot = t.TempDir()
						req.ExtrasRoot = filepath.Join(req.ProjectRoot, ".skillshare", "extras")
						req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
						if err := os.MkdirAll(req.LockDir, 0755); err != nil {
							t.Fatal(err)
						}
					}
					if replacement {
						if _, err := InstallRemoteExtra(req); err != nil {
							t.Fatal(err)
						}
					}
					old := extraBoundary
					extraBoundary = func(step string) error {
						if step == boundary {
							return errSimulatedDeath
						}
						return nil
					}
					_, err := InstallRemoteExtra(req)
					extraBoundary = old
					if !errors.Is(err, errSimulatedDeath) {
						t.Fatalf("death: %v", err)
					}
					if !InspectRemoteExtra(req.ExtrasRoot, req.Name).Interrupted {
						t.Fatal("interruption not visible")
					}
					got, err := InstallRemoteExtra(req)
					if err != nil || !got.Interrupted {
						t.Fatalf("repair: %+v %v", got, err)
					}
					state := InspectRemoteExtra(req.ExtrasRoot, req.Name)
					if state.Err != nil || state.Interrupted || state.Modified || state.Missing || state.Commit != got.Commit {
						t.Fatalf("state: %+v", state)
					}
					if project {
						lock, err := LoadLock(req.LockDir)
						if err != nil || lock.ExtraCommitFor(req.Name, req.RawSource) != got.Commit {
							t.Fatalf("pin: %+v %v", lock, err)
						}
					}
				})
			}
		}
	}
}

func TestInstallRemoteExtra_InterruptedLaterEdits(t *testing.T) {
	for _, change := range []string{"content", "added", "removed", "mode"} {
		t.Run(change, func(t *testing.T) {
			req := newRemoteExtraRequest(t)
			old := extraBoundary
			extraBoundary = func(step string) error {
				if step == "live" {
					return errSimulatedDeath
				}
				return nil
			}
			_, err := InstallRemoteExtra(req)
			extraBoundary = old
			if !errors.Is(err, errSimulatedDeath) {
				t.Fatal(err)
			}
			dir := filepath.Join(req.ExtrasRoot, req.Name)
			file := filepath.Join(dir, "SKILL.md")
			switch change {
			case "content":
				err = os.WriteFile(file, []byte("later edit"), 0644)
			case "added":
				err = os.WriteFile(filepath.Join(dir, "later"), []byte("later"), 0644)
			case "removed":
				err = os.Remove(file)
			case "mode":
				err = os.Chmod(file, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = InstallRemoteExtra(req)
			if change == "mode" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("later edit was replaced")
			}
			if _, err := os.Lstat(extraJournalPath(req.ExtrasRoot, req.Name)); err != nil {
				t.Fatalf("marker lost: %v", err)
			}
			req.Opts.Force = true
			if _, err := InstallRemoteExtra(req); err == nil {
				t.Fatal("implicit Force replaced edits")
			}
			req.Opts.AuditOverride = true
			if _, err := InstallRemoteExtra(req); err != nil {
				t.Fatalf("explicit override: %v", err)
			}
		})
	}
}

func TestInstallRemoteExtra_ConcurrentRecheck(t *testing.T) {
	req := newRemoteExtraRequest(t)
	entered, release, waiting := make(chan struct{}), make(chan struct{}), make(chan struct{})
	oldBoundary, oldWait := extraBoundary, extraLockOnWait
	extraBoundary = func(step string) error {
		if step == "marker" {
			close(entered)
			<-release
		}
		return nil
	}
	extraLockOnWait = func() { close(waiting) }
	defer func() { extraBoundary, extraLockOnWait = oldBoundary, oldWait }()
	second := req
	second.Recheck = func() error {
		data, err := os.ReadFile(req.ConfigPath)
		if err != nil {
			return err
		}
		if string(data) != "before" {
			return errors.New("config changed; retry")
		}
		return nil
	}
	firstResult, secondResult := make(chan error, 1), make(chan error, 1)
	go func() { _, err := InstallRemoteExtra(req); firstResult <- err }()
	<-entered
	go func() { _, err := InstallRemoteExtra(second); secondResult <- err }()
	<-waiting
	close(release)
	if err := <-firstResult; err != nil {
		t.Fatal(err)
	}
	if err := <-secondResult; err == nil || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("concurrent result: %v", err)
	}
	if state := InspectRemoteExtra(req.ExtrasRoot, req.Name); state.Err != nil || state.Modified || state.Interrupted {
		t.Fatalf("state: %+v", state)
	}
}

func TestInstallRemoteExtra_RepairAuditFailureRetainsState(t *testing.T) {
	req := newRemoteExtraRequest(t)
	oldBoundary := extraBoundary
	extraBoundary = func(step string) error {
		if step == "live" {
			return errSimulatedDeath
		}
		return nil
	}
	_, err := InstallRemoteExtra(req)
	extraBoundary = oldBoundary
	if !errors.Is(err, errSimulatedDeath) {
		t.Fatal(err)
	}
	file := filepath.Join(req.ExtrasRoot, req.Name, "SKILL.md")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	oldScan := scanExtraCandidate
	scanExtraCandidate = func(AuditGate) (*AuditGateResult, error) { return nil, errors.New("scanner unavailable") }
	defer func() { scanExtraCandidate = oldScan }()
	req.Opts.SkipAudit = false
	req.Opts.AuditProjectRoot = t.TempDir()
	req.Opts.AuditOverride = true
	if _, err := InstallRemoteExtra(req); err == nil {
		t.Fatal("repair bypassed scan failure")
	}
	after, err := os.ReadFile(file)
	if err != nil || string(after) != string(before) {
		t.Fatalf("live changed: %q %v", after, err)
	}
	if _, err := os.Lstat(extraJournalPath(req.ExtrasRoot, req.Name)); err != nil {
		t.Fatalf("marker lost: %v", err)
	}
}

func TestInstallRemoteExtra_RepairUnavailableCommitPreservesState(t *testing.T) {
	req := newRemoteExtraRequest(t)
	old := extraBoundary
	extraBoundary = func(step string) error {
		if step == "live" {
			return errSimulatedDeath
		}
		return nil
	}
	_, err := InstallRemoteExtra(req)
	extraBoundary = old
	if !errors.Is(err, errSimulatedDeath) {
		t.Fatal(err)
	}
	j, err := readExtraJournal(req.ExtrasRoot, req.Name)
	if err != nil {
		t.Fatal(err)
	}
	j.Commit = strings.Repeat("f", 40)
	data, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	marker := extraJournalPath(req.ExtrasRoot, req.Name)
	if err := os.WriteFile(marker, data, 0600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(req.ExtrasRoot, req.Name, "SKILL.md")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InstallRemoteExtra(req); err == nil {
		t.Fatal("unavailable commit accepted")
	}
	after, err := os.ReadFile(file)
	if err != nil || string(after) != string(before) {
		t.Fatalf("live changed: %q %v", after, err)
	}
	gotMarker, err := os.ReadFile(marker)
	if err != nil || string(gotMarker) != string(data) {
		t.Fatalf("marker changed: %v", err)
	}
}

func TestInstallRemoteExtra_InterruptedPreviousEdit(t *testing.T) {
	req := newRemoteExtraRequest(t)
	if _, err := InstallRemoteExtra(req); err != nil {
		t.Fatal(err)
	}
	old := extraBoundary
	extraBoundary = func(step string) error {
		if step == "live" {
			return errSimulatedDeath
		}
		return nil
	}
	_, err := InstallRemoteExtra(req)
	extraBoundary = old
	if !errors.Is(err, errSimulatedDeath) {
		t.Fatal(err)
	}
	file := filepath.Join(req.ExtrasRoot, extraStagingDir, "previous-"+req.Name, "SKILL.md")
	if err := os.WriteFile(file, []byte("later previous edit"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallRemoteExtra(req); err == nil {
		t.Fatal("previous edit discarded")
	}
	got, err := os.ReadFile(file)
	if err != nil || string(got) != "later previous edit" {
		t.Fatalf("previous lost: %q %v", got, err)
	}
	if _, err := os.Lstat(extraJournalPath(req.ExtrasRoot, req.Name)); err != nil {
		t.Fatalf("marker lost: %v", err)
	}
}

func TestInstallRemoteExtra_FaultRestoresParticipants(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		for _, boundary := range []string{"marker", "incoming", "ignore", "previous", "live", "metadata", "lock", "config", "cleanup"} {
			t.Run(fmt.Sprintf("replacement=%t/%s", replacement, boundary), func(t *testing.T) {
				req := newRemoteExtraRequest(t)
				req.ProjectRoot = t.TempDir()
				req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
				req.ExtrasRoot = filepath.Join(req.LockDir, "extras")
				if err := os.MkdirAll(req.LockDir, 0755); err != nil {
					t.Fatal(err)
				}
				if replacement {
					if _, err := InstallRemoteExtra(req); err != nil {
						t.Fatal(err)
					}
				}
				paths := []string{req.ConfigPath, filepath.Join(req.ExtrasRoot, MetadataFileName), filepath.Join(req.LockDir, LockFileName), filepath.Join(req.LockDir, ".gitignore")}
				before := make([]extraFileSnapshot, len(paths))
				for i, p := range paths {
					var err error
					before[i], err = snapshotExtraFile(p)
					if err != nil {
						t.Fatal(err)
					}
				}
				old := extraBoundary
				extraBoundary = func(step string) error {
					if step == boundary {
						return errors.New("fault")
					}
					return nil
				}
				_, err := InstallRemoteExtra(req)
				extraBoundary = old
				if err == nil {
					t.Fatal("fault ignored")
				}
				for i, p := range paths {
					after, err := snapshotExtraFile(p)
					if err != nil || before[i].exists != after.exists || string(before[i].data) != string(after.data) || before[i].mode != after.mode {
						t.Fatalf("participant %s changed: %+v %+v %v", p, before[i], after, err)
					}
				}
				_, err = os.Lstat(filepath.Join(req.ExtrasRoot, req.Name))
				if replacement && err != nil || !replacement && !os.IsNotExist(err) {
					t.Fatalf("previous state: %v", err)
				}
				if _, err := os.Lstat(extraJournalPath(req.ExtrasRoot, req.Name)); !os.IsNotExist(err) {
					t.Fatalf("marker: %v", err)
				}
			})
		}
	}
}

func TestInstallRemoteExtra_UnknownAndDifferentOrigin(t *testing.T) {
	req := newRemoteExtraRequest(t)
	dir := filepath.Join(req.ExtrasRoot, req.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "local"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallRemoteExtra(req); err == nil {
		t.Fatal("unknown directory accepted")
	}
	req.Opts.AuditOverride = true
	if _, err := InstallRemoteExtra(req); err != nil {
		t.Fatal(err)
	}
	other, _, _ := setupPinRemote(t)
	req.RawSource = other
	req.Source = &Source{Type: SourceTypeGitHTTPS, Raw: other, CloneURL: other}
	if _, err := InstallRemoteExtra(req); err == nil || !strings.Contains(err.Error(), "different origin") {
		t.Fatalf("different origin: %v", err)
	}
}

func TestInstallRemoteExtra_ProjectIgnoreAndOutsideWarning(t *testing.T) {
	for _, outside := range []bool{false, true} {
		t.Run(fmt.Sprintf("outside=%t", outside), func(t *testing.T) {
			req := newRemoteExtraRequest(t)
			req.ProjectRoot = t.TempDir()
			req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
			if err := os.MkdirAll(req.LockDir, 0755); err != nil {
				t.Fatal(err)
			}
			if !outside {
				req.ExtrasRoot = filepath.Join(req.LockDir, "extras")
			}
			result, err := InstallRemoteExtra(req)
			if err != nil {
				t.Fatal(err)
			}
			if outside {
				if len(result.Warnings) == 0 {
					t.Fatal("missing outside-root warning")
				}
				return
			}
			data, err := os.ReadFile(filepath.Join(req.LockDir, ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range []string{"extras/rules/", "extras/.skillshare-staging/", "extras/.metadata.json"} {
				if !strings.Contains(string(data), entry) {
					t.Errorf("missing %s", entry)
				}
			}
		})
	}
}

func TestInstallRemoteExtra_RollbackPreservesConcurrentConfig(t *testing.T) {
	req := newRemoteExtraRequest(t)
	req.RevertConfig = func() error { return os.WriteFile(req.ConfigPath, []byte("before\nother"), 0644) }
	old := extraBoundary
	extraBoundary = func(step string) error {
		if step == "config" {
			if err := os.WriteFile(req.ConfigPath, []byte("after\nother"), 0644); err != nil {
				return err
			}
			return errors.New("fault")
		}
		return nil
	}
	defer func() { extraBoundary = old }()
	_, err := InstallRemoteExtra(req)
	if err == nil || !strings.Contains(err.Error(), req.ConfigPath) || !strings.Contains(err.Error(), "changed by another writer") {
		t.Fatalf("fault: %v", err)
	}
	got, err := os.ReadFile(req.ConfigPath)
	if err != nil || string(got) != "before\nother" {
		t.Fatalf("concurrent change lost: %q %v", got, err)
	}
}

func TestInstallRemoteExtra_WriteFailureAfterPartialConfig(t *testing.T) {
	req := newRemoteExtraRequest(t)
	req.WriteConfig = func() error {
		if err := os.WriteFile(req.ConfigPath, []byte("partial"), 0644); err != nil {
			return err
		}
		return errors.New("write failed")
	}
	if _, err := InstallRemoteExtra(req); err == nil {
		t.Fatal("write failure accepted")
	}
	got, err := os.ReadFile(req.ConfigPath)
	if err != nil || string(got) != "before" {
		t.Fatalf("partial config not restored: %q %v", got, err)
	}
	if _, err := os.Lstat(extraJournalPath(req.ExtrasRoot, req.Name)); !os.IsNotExist(err) {
		t.Fatalf("marker after rollback: %v", err)
	}
}

func TestInstallRemoteExtra_RollbackFailureKeepsMarker(t *testing.T) {
	req := newRemoteExtraRequest(t)
	oldBoundary, oldCleanup := extraBoundary, extraCleanup
	extraBoundary = func(step string) error {
		if step == "live" {
			return errors.New("fault")
		}
		return nil
	}
	extraCleanup = func(string) error { return errors.New("cleanup failed") }
	defer func() { extraBoundary, extraCleanup = oldBoundary, oldCleanup }()
	_, err := InstallRemoteExtra(req)
	if err == nil || !strings.Contains(err.Error(), "rollback incomplete") {
		t.Fatalf("rollback: %v", err)
	}
	if _, err := os.Lstat(extraJournalPath(req.ExtrasRoot, req.Name)); err != nil {
		t.Fatalf("marker lost: %v", err)
	}
}

func TestInstallRemoteExtra_InterruptedEditsAtBoundaries(t *testing.T) {
	for _, boundary := range []string{"marker", "incoming", "ignore", "previous", "live", "metadata", "lock", "config", "cleanup", "cleaned-previous"} {
		for _, tree := range []string{"live", "previous"} {
			t.Run(boundary+"/"+tree, func(t *testing.T) {
				req := newRemoteExtraRequest(t)
				if _, err := InstallRemoteExtra(req); err != nil {
					t.Fatal(err)
				}
				old := extraBoundary
				extraBoundary = func(step string) error {
					if step == boundary {
						return errSimulatedDeath
					}
					return nil
				}
				_, err := InstallRemoteExtra(req)
				extraBoundary = old
				if !errors.Is(err, errSimulatedDeath) {
					t.Fatal(err)
				}
				dir := filepath.Join(req.ExtrasRoot, req.Name)
				if tree == "previous" {
					dir = filepath.Join(req.ExtrasRoot, extraStagingDir, "previous-"+req.Name)
				}
				if _, err := os.Lstat(dir); os.IsNotExist(err) {
					return
				} else if err != nil {
					t.Fatal(err)
				}
				file := filepath.Join(dir, "later")
				if err := os.WriteFile(file, []byte("keep later edit"), 0644); err != nil {
					t.Fatal(err)
				}
				if _, err := InstallRemoteExtra(req); err == nil {
					t.Fatal("later edit was overwritten")
				}
				got, err := os.ReadFile(file)
				if err != nil || string(got) != "keep later edit" {
					t.Fatalf("later edit lost: %q %v", got, err)
				}
				if _, err := os.Lstat(extraJournalPath(req.ExtrasRoot, req.Name)); err != nil {
					t.Fatalf("marker lost: %v", err)
				}
			})
		}
	}
}

func TestInstallRemoteExtra_WriteHooksFailClosed(t *testing.T) {
	for _, hook := range []string{"marker", "ignore", "previous", "live", "metadata", "lock", "config", "cleanup"} {
		t.Run(hook, func(t *testing.T) {
			req := newRemoteExtraRequest(t)
			req.ProjectRoot = t.TempDir()
			req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
			req.ExtrasRoot = filepath.Join(req.LockDir, "extras")
			if err := os.MkdirAll(req.LockDir, 0755); err != nil {
				t.Fatal(err)
			}
			if _, err := InstallRemoteExtra(req); err != nil {
				t.Fatal(err)
			}
			marker, rename, meta, lock, config, ignore, cleanup := extraWriteMarker, extraRename, extraWriteMeta, extraWriteLock, extraWriteConfig, extraWriteIgnore, extraCleanup
			defer func() {
				extraWriteMarker, extraRename, extraWriteMeta, extraWriteLock, extraWriteConfig, extraWriteIgnore, extraCleanup = marker, rename, meta, lock, config, ignore, cleanup
			}()
			fault := errors.New("hook failure")
			switch hook {
			case "marker":
				extraWriteMarker = func(string, extraJournal) error { return fault }
			case "ignore":
				extraWriteIgnore = func(string, []string) error { return fault }
			case "previous", "live":
				count := 0
				extraRename = func(from, to string) error {
					count++
					if hook == "previous" && count == 1 || hook == "live" && count == 2 {
						return fault
					}
					return rename(from, to)
				}
			case "metadata":
				extraWriteMeta = func(*MetadataStore, string) error { return fault }
			case "lock":
				extraWriteLock = func(*Lock, string) error { return fault }
			case "config":
				extraWriteConfig = func(func() error) error { return fault }
			case "cleanup":
				extraCleanup = func(name string) error {
					if strings.HasSuffix(name, "previous-"+req.Name) {
						return fault
					}
					return cleanup(name)
				}
			}
			if _, err := InstallRemoteExtra(req); err == nil {
				t.Fatal("hook failure accepted")
			}
			state := InspectRemoteExtra(req.ExtrasRoot, req.Name)
			if hook == "cleanup" {
				if !state.Interrupted {
					t.Fatal("cleanup failure lost marker")
				}
				return
			}
			if state.Interrupted || state.Missing || state.Modified || state.Err != nil {
				t.Fatalf("rollback state: %+v", state)
			}
		})
	}
}

func TestInstallRemoteExtra_RecheckStopsBeforePublication(t *testing.T) {
	req := newRemoteExtraRequest(t)
	req.Recheck = func() error { return errors.New("config changed") }
	if _, err := InstallRemoteExtra(req); err == nil || !strings.Contains(err.Error(), "config changed") {
		t.Fatalf("recheck: %v", err)
	}
	if _, err := os.Lstat(req.ExtrasRoot); !os.IsNotExist(err) {
		t.Fatalf("created extras root: %v", err)
	}
}

func TestInstallRemoteExtra_DryRunDoesNotRepairOrWrite(t *testing.T) {
	req := newRemoteExtraRequest(t)
	old := extraBoundary
	extraBoundary = func(step string) error {
		if step == "marker" {
			return errSimulatedDeath
		}
		return nil
	}
	_, err := InstallRemoteExtra(req)
	extraBoundary = old
	if !errors.Is(err, errSimulatedDeath) {
		t.Fatal(err)
	}
	marker := extraJournalPath(req.ExtrasRoot, req.Name)
	before, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	req.Opts.DryRun = true
	result, err := InstallRemoteExtra(req)
	if err != nil || !result.Interrupted || result.Action != "would install" {
		t.Fatalf("dry run: %+v %v", result, err)
	}
	after, err := os.ReadFile(marker)
	if err != nil || string(after) != string(before) {
		t.Fatalf("marker changed: %v", err)
	}
}

func TestInstallRemoteExtra_RollbackPreservesConcurrentLock(t *testing.T) {
	req := newRemoteExtraRequest(t)
	req.ProjectRoot = t.TempDir()
	req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
	req.ExtrasRoot = filepath.Join(req.LockDir, "extras")
	if err := os.MkdirAll(req.LockDir, 0755); err != nil {
		t.Fatal(err)
	}
	old := extraBoundary
	extraBoundary = func(step string) error {
		if step == "lock" {
			lock, err := LoadLock(req.LockDir)
			if err != nil {
				return err
			}
			lock.Skills = map[string]LockEntry{"other": {Source: "other", Commit: strings.Repeat("a", 40)}}
			if err := lock.Save(req.LockDir); err != nil {
				return err
			}
			return errors.New("fault")
		}
		return nil
	}
	defer func() { extraBoundary = old }()
	if _, err := InstallRemoteExtra(req); err == nil || !strings.Contains(err.Error(), filepath.Join(req.LockDir, LockFileName)) || !strings.Contains(err.Error(), "changed by another writer") {
		t.Fatalf("rollback: %v", err)
	}
	lock, err := LoadLock(req.LockDir)
	if err != nil || lock.Skills["other"].Source != "other" || len(lock.Extras) != 0 {
		t.Fatalf("lock: %+v %v", lock, err)
	}
}

func TestInstallRemoteExtra_RollbackPreservesConcurrentIgnore(t *testing.T) {
	req := newRemoteExtraRequest(t)
	req.ProjectRoot = t.TempDir()
	req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
	req.ExtrasRoot = filepath.Join(req.LockDir, "extras")
	if err := os.MkdirAll(req.LockDir, 0755); err != nil {
		t.Fatal(err)
	}
	old := extraBoundary
	extraBoundary = func(step string) error {
		if step == "ignore" {
			f, err := os.OpenFile(filepath.Join(req.LockDir, ".gitignore"), os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			_, err = f.WriteString("other/\n")
			if closeErr := f.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				return err
			}
			return errors.New("fault")
		}
		return nil
	}
	defer func() { extraBoundary = old }()
	if _, err := InstallRemoteExtra(req); err == nil || !strings.Contains(err.Error(), filepath.Join(req.LockDir, ".gitignore")) || !strings.Contains(err.Error(), "changed by another writer") {
		t.Fatalf("rollback: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(req.LockDir, ".gitignore"))
	if err != nil || !strings.Contains(string(data), "other/") || strings.Contains(string(data), "extras/rules/") {
		t.Fatalf("ignore: %q %v", data, err)
	}
}

func TestInstallRemoteExtra_SymlinkedConfigRestored(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires symlink privilege")
	}
	req := newRemoteExtraRequest(t)
	alias := filepath.Join(filepath.Dir(req.ConfigPath), "config-link.yaml")
	if err := os.Symlink(req.ConfigPath, alias); err != nil {
		t.Fatal(err)
	}
	req.WriteConfig = func() error { return os.WriteFile(alias, []byte("after"), 0644) }
	old := extraBoundary
	extraBoundary = func(step string) error {
		if step == "config" {
			return errors.New("fault")
		}
		return nil
	}
	defer func() { extraBoundary = old }()
	if _, err := InstallRemoteExtra(req); err == nil {
		t.Fatal("fault ignored")
	}
	info, err := os.Lstat(alias)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("config link lost: %v", err)
	}
	got, err := os.ReadFile(alias)
	if err != nil || string(got) != "before" {
		t.Fatalf("config not restored: %q %v", got, err)
	}
}

func TestInstallRemoteExtra_RepairConflictLeavesOldLiveTree(t *testing.T) {
	req := newRemoteExtraRequest(t)
	req.Branch = "main"
	if _, err := InstallRemoteExtra(req); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(req.ExtrasRoot, req.Name, "SKILL.md")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	req.Branch = "feature"
	req.Opts.Update = true
	old := extraBoundary
	extraBoundary = func(step string) error {
		if step == "marker" {
			return errSimulatedDeath
		}
		return nil
	}
	_, err = InstallRemoteExtra(req)
	extraBoundary = old
	if !errors.Is(err, errSimulatedDeath) {
		t.Fatal(err)
	}
	req.RepairConfig = func(_, _ string) error { return errors.New("local extra conflict") }
	if _, err := InstallRemoteExtra(req); err == nil || !strings.Contains(err.Error(), "local extra conflict") {
		t.Fatalf("conflict: %v", err)
	}
	after, err := os.ReadFile(file)
	if err != nil || string(after) != string(before) {
		t.Fatalf("conflict changed live tree: %q %v", after, err)
	}
	if _, err := os.Lstat(extraJournalPath(req.ExtrasRoot, req.Name)); err != nil {
		t.Fatalf("marker lost: %v", err)
	}
}

func TestInstallRemoteExtra_RepairRefusesLinkedStateFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires symlink privilege")
	}
	for _, target := range []string{"metadata", "lock", "ignore"} {
		t.Run(target, func(t *testing.T) {
			req := newRemoteExtraRequest(t)
			req.ProjectRoot = t.TempDir()
			req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
			req.ExtrasRoot = filepath.Join(req.LockDir, "extras")
			if err := os.MkdirAll(req.LockDir, 0755); err != nil {
				t.Fatal(err)
			}
			old := extraBoundary
			extraBoundary = func(step string) error {
				if step == "live" {
					return errSimulatedDeath
				}
				return nil
			}
			_, err := InstallRemoteExtra(req)
			extraBoundary = old
			if !errors.Is(err, errSimulatedDeath) {
				t.Fatal(err)
			}
			stateFile := filepath.Join(req.ExtrasRoot, MetadataFileName)
			if target == "lock" {
				stateFile = filepath.Join(req.LockDir, LockFileName)
			}
			if target == "ignore" {
				stateFile = filepath.Join(req.LockDir, ".gitignore")
			}
			if err := os.Remove(stateFile); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			sentinel := filepath.Join(t.TempDir(), "sentinel")
			if err := os.WriteFile(sentinel, []byte("do not touch"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(sentinel, stateFile); err != nil {
				t.Fatal(err)
			}
			if _, err := InstallRemoteExtra(req); err == nil {
				t.Fatal("linked state accepted")
			}
			got, err := os.ReadFile(sentinel)
			if err != nil || string(got) != "do not touch" {
				t.Fatalf("sentinel changed: %q %v", got, err)
			}
			if _, err := os.Lstat(extraJournalPath(req.ExtrasRoot, req.Name)); err != nil {
				t.Fatalf("marker lost: %v", err)
			}
		})
	}
}

func TestInstallRemoteExtra_MetadataChangedKeepsMarker(t *testing.T) {
	req := newRemoteExtraRequest(t)
	meta := filepath.Join(req.ExtrasRoot, MetadataFileName)
	old := extraBoundary
	extraBoundary = func(step string) error {
		if step == "metadata" {
			if err := os.WriteFile(meta, []byte("other writer"), 0644); err != nil {
				return err
			}
			return errors.New("fault")
		}
		return nil
	}
	defer func() { extraBoundary = old }()
	_, err := InstallRemoteExtra(req)
	if err == nil || !strings.Contains(err.Error(), "metadata changed") || !strings.Contains(err.Error(), "marker retained") {
		t.Fatalf("rollback: %v", err)
	}
	got, err := os.ReadFile(meta)
	if err != nil || string(got) != "other writer" {
		t.Fatalf("metadata overwritten: %q %v", got, err)
	}
	if _, err := os.Lstat(extraJournalPath(req.ExtrasRoot, req.Name)); err != nil {
		t.Fatalf("marker lost: %v", err)
	}
}

func TestInstallRemoteExtra_RollbackCompareFailureKeepsMarker(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires symlink privilege")
	}
	req := newRemoteExtraRequest(t)
	sentinel := filepath.Join(t.TempDir(), "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	old := extraBoundary
	extraBoundary = func(step string) error {
		if step == "config" {
			if err := os.Remove(req.ConfigPath); err != nil {
				return err
			}
			if err := os.Symlink(sentinel, req.ConfigPath); err != nil {
				return err
			}
			return errors.New("fault")
		}
		return nil
	}
	defer func() { extraBoundary = old }()
	_, err := InstallRemoteExtra(req)
	if err == nil || !strings.Contains(err.Error(), req.ConfigPath) || !strings.Contains(err.Error(), "marker retained") {
		t.Fatalf("rollback: %v", err)
	}
	got, err := os.ReadFile(sentinel)
	if err != nil || string(got) != "keep" {
		t.Fatalf("sentinel: %q %v", got, err)
	}
}
