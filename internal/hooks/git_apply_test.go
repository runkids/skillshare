package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGitLifecycleAndOrder(t *testing.T) {
	e := gitEnv(t)
	r := save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	d, err := e.service.gitDestination("")
	must(t, err)
	helper := filepath.Join(d.base, "skillshare", "files", "guard", "check.sh")
	if len(r.Applied) != 3 || r.Applied[0] != helper || r.Applied[1] != d.hooksFile || r.Applied[2] != d.includeTarget {
		t.Fatalf("add order: %v", r.Applied)
	}
	if !strings.Contains(read(t, d.includeTarget), d.includeValue) {
		t.Fatal("include not registered")
	}
	info, err := os.Stat(helper)
	must(t, err)
	if info.Mode().Perm() != 0755 {
		t.Fatalf("helper mode %v", info.Mode())
	}
	if len(sync(t, e.service).Applied) != 0 {
		t.Fatal("sync not idempotent")
	}
	write(t, d.includeTarget, read(t, d.includeTarget)+"[user]\n name = later\n")
	r = save(t, e.service, Mutation{Name: "guard", Remove: true})
	if len(r.Applied) != 3 || r.Applied[0] != d.includeTarget || r.Applied[1] != d.hooksFile || r.Applied[2] != helper {
		t.Fatalf("remove order: %v", r.Applied)
	}
	if pathExists(d.hooksFile) || pathExists(helper) || strings.Contains(read(t, d.includeTarget), d.includeValue) || !strings.Contains(read(t, d.includeTarget), "name = later") {
		t.Fatal("remove affected unrelated settings or left owned output")
	}
	// Restore the helper's deletion, with executable mode and only its ownership.
	p, err := e.service.PreviewRestore(r.BackupIDs[2])
	must(t, err)
	_, err = e.service.Restore(r.BackupIDs[2], p.Revision)
	must(t, err)
	info, err = os.Stat(helper)
	must(t, err)
	if info.Mode().Perm() != 0755 {
		t.Fatal("restore lost executable mode")
	}
}

func TestGitIncludeRestorePrivacyAndRecovery(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	write(t, d.includeTarget, "[credential]\n helper = SECRET\n")
	r := save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	id := r.BackupIDs[len(r.BackupIDs)-1]
	b := read(t, filepath.Join(e.service.stateDir(), "backups", id+".json"))
	if strings.Contains(b, "SECRET") {
		t.Fatal("private config backed up")
	}
	write(t, d.includeTarget, read(t, d.includeTarget)+"[user]\n name = later\n")
	p, err := e.service.PreviewRestore(id)
	must(t, err)
	for _, f := range p.Files() {
		if strings.Contains(f.Before+f.After, "SECRET") {
			t.Fatal("private config in restore preview")
		}
	}
	_, err = e.service.Restore(id, p.Revision)
	must(t, err)
	if strings.Contains(read(t, d.includeTarget), d.includeValue) || !strings.Contains(read(t, d.includeTarget), "SECRET") || !strings.Contains(read(t, d.includeTarget), "name = later") {
		t.Fatal("include restore changed unrelated data")
	}
	state, _, err := e.service.loadLedger()
	must(t, err)
	key := elementKey(e.config, d.identity, d.includeTarget, "include.path", "", 0)
	state.Records[key] = record{Owner: e.config, Target: "git", Path: d.includeTarget, Event: "include.path", Hash: digest([]byte(d.includeValue))}
	gitSetup(t, e.home, "config", "--file", d.includeTarget, "--add", "include.path", d.includeValue)
	must(t, writeJSONFile(e.service.journalPath(), journal{Kind: kindGitInclude, Path: d.includeTarget, Value: d.includeValue, Add: true, State: state}))
	must(t, e.service.recoverPending())
	recovered, _, err := e.service.loadLedger()
	must(t, err)
	if _, ok := recovered.Records[key]; !ok {
		t.Fatal("include journal lost completed ownership")
	}
}

func TestGitSectionReplaceRestoreAndLock(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	original := "[user]\n name = private\n[hook \"check\"]\n command = echo foreign\n unknown\n[remote \"origin\"]\n url = private\n[hook \"check\"]\n event = pre-commit\n"
	write(t, d.includeTarget, original)
	r := save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry), Replace: true})
	var id string
	for _, bid := range r.BackupIDs {
		data := read(t, filepath.Join(e.service.stateDir(), "backups", bid+".json"))
		var b backupRecord
		must(t, json.Unmarshal([]byte(data), &b))
		if b.Kind == kindGitSection {
			id = bid
			if strings.Contains(data, "private") {
				t.Fatal("private unrelated section in backup")
			}
		}
	}
	if id == "" || strings.Contains(read(t, d.includeTarget), "echo foreign") {
		t.Fatal("replacement did not remove foreign section")
	}
	p, err := e.service.PreviewRestore(id)
	must(t, err)
	if !p.Blocked {
		t.Fatal("restore must refuse merging with live managed same name")
	}
	save(t, e.service, Mutation{Name: "guard", Remove: true})
	write(t, d.includeTarget, read(t, d.includeTarget)+"[credential]\n helper = later\n")
	p, err = e.service.PreviewRestore(id)
	must(t, err)
	if p.Blocked {
		t.Fatalf("removed managed name permits restore: %+v", p.Changes)
	}
	write(t, d.includeTarget+".lock", "user lock")
	_, err = e.service.Restore(id, p.Revision)
	if err == nil {
		t.Fatal("must respect native lock")
	}
	must(t, os.Remove(d.includeTarget+".lock"))
	p, err = e.service.PreviewRestore(id)
	must(t, err)
	_, err = e.service.Restore(id, p.Revision)
	must(t, err)
	if read(t, d.includeTarget) != original+"[credential]\n helper = later\n" {
		t.Fatalf("section fragments/order changed:\n%s", read(t, d.includeTarget))
	}
}

func TestGitStaleApplyAndPartialRecovery(t *testing.T) {
	e := gitEnv(t)
	p := gitDraft(t, e, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	d, err := e.service.gitDestination("")
	must(t, err)
	write(t, d.includeTarget, "[hook \"check\"]\n command = echo intervening\n")
	_, err = e.service.applyScoped(p, nil)
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("collision stale at apply: %v", err)
	}
	if pathExists(d.hooksFile) {
		t.Fatal("stale plan wrote hooks")
	}
	write(t, d.includeTarget, "")
	p, err = e.service.Preview()
	must(t, err)
	write(t, d.includeTarget+".lock", "user lock")
	r, err := e.service.applyScoped(p, nil)
	if err == nil || len(r.Applied) != 2 || !pathExists(d.hooksFile) {
		t.Fatalf("partial result: %+v / %v", r, err)
	}
	state, _, err := e.service.loadLedger()
	must(t, err)
	if _, ok := state.Records[fileKey(d.hooksFile)]; !ok {
		t.Fatal("partial result lost completed ownership")
	}
	must(t, os.Remove(d.includeTarget+".lock"))
	r = sync(t, e.service)
	if len(r.Applied) != 1 || r.Applied[0] != d.includeTarget {
		t.Fatalf("retry repeated completed writes: %v", r.Applied)
	}
}

func TestGitManualIncludeAndOwnedDirectories(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	write(t, d.includeTarget, includeLines(d.includeValue))
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	save(t, e.service, Mutation{Name: "guard", Remove: true})
	if read(t, d.includeTarget) != includeLines(d.includeValue) {
		t.Fatal("manual include removed")
	}
	if pathExists(filepath.Join(d.base, "skillshare")) {
		t.Fatal("owned empty directories not pruned")
	}
}

func TestGitProjectsLinkedIdentityAndRegistration(t *testing.T) {
	requireGitVersion(t, 54)
	e := gitEnv(t)
	root := t.TempDir()
	gitSetup(t, root, "init", "--quiet")
	linked := filepath.Join(t.TempDir(), "linked")
	gitSetup(t, root, "worktree", "add", "--orphan", "-b", "linked", linked)
	r := save(t, e.service, Mutation{Project: root, Name: "guard", Entry: entry(t, gitEntry)})
	d, err := e.service.scoped(root).gitDestination(root)
	must(t, err)
	if len(r.Applied) != 3 || r.Applied[1] != d.hooksFile || r.Applied[2] != d.includeTarget {
		t.Fatalf("project outputs: %v", r.Applied)
	}
	for _, repo := range []string{root, linked} {
		out, err := e.service.gitRunner().Run(repo, nil, "hook", "list", "pre-commit")
		must(t, err)
		if !strings.Contains(string(out), "check") {
			t.Fatalf("not registered in linked repository: %s", out)
		}
	}
	// Relocate the declaration between worktrees in one draft. Canonical keys stay
	// the same while source roots remain selectors for ApplyProject.
	source, err := LoadSource(e.config)
	must(t, err)
	source.Projects[linked] = source.Projects[root]
	delete(source.Projects, root)
	p, err := e.service.previewSource(source, nil, nil)
	must(t, err)
	if p.Blocked {
		t.Fatalf("linked relocation conflicted: %+v", p.Changes)
	}
	state, _, err := e.service.loadLedger()
	must(t, err)
	for key, r := range state.Records {
		if r.Event == "hook.check" {
			if _, ok := p.state.Records[key]; !ok {
				t.Fatal("canonical name identity changed")
			}
			if p.state.Records[key].Root != linked {
				t.Fatal("apply selector not updated")
			}
		}
	}
	source.Projects[root] = source.Projects[linked]
	if _, err := e.service.render(source); err == nil {
		t.Fatal("two declarations sharing common directory must be rejected")
	}
}

func TestGitIncludeDuplicateAndWholeFileRestoreOwnership(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	r := save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	fileBackup := r.BackupIDs[1]
	write(t, d.includeTarget, read(t, d.includeTarget)+includeLines(d.includeValue))
	p, err := e.service.PreviewMutation(Mutation{Name: "guard", Remove: true})
	must(t, err)
	if !p.Blocked {
		t.Fatal("duplicate owned include must not be guessed or removed")
	}
	// Restoring a whole-file backup checks every name's ownership, even when its
	// bytes still match. Altering one name record makes restore unsafe.
	state, _, err := e.service.loadLedger()
	must(t, err)
	for key, r := range state.Records {
		if r.Event == "hook.check" {
			r.Hash = "intervening"
			state.Records[key] = r
		}
	}
	must(t, writeJSONFile(e.service.statePath(), state))
	p, err = e.service.PreviewRestore(fileBackup)
	must(t, err)
	if !p.Blocked {
		t.Fatal("whole-file restore ignored changed name ownership")
	}
}

func TestGitKeepFilesRefusedInDomain(t *testing.T) {
	e := gitEnv(t)
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	_, err := e.service.PreviewMutation(Mutation{Name: "guard", Remove: true, Unmanage: true})
	if err == nil || !strings.Contains(err.Error(), "cannot keep files") {
		t.Fatalf("keep-files should be refused: %v", err)
	}
}

func TestGitMainCheckoutOriginOutsideWorkingDirectory(t *testing.T) {
	for _, declared := range []bool{false, true} {
		for _, replace := range []bool{false, true} {
			t.Run(fmt.Sprintf("declared=%v/replace=%v", declared, replace), func(t *testing.T) {
				e := gitEnv(t)
				root := t.TempDir()
				gitSetup(t, root, "init", "--quiet")
				s := e.service
				m := Mutation{Name: "guard", Project: root, Entry: entry(t, gitEntry), Replace: replace}
				if !declared {
					s = e.project(root)
					m.Project = ""
				}
				if replace {
					config := filepath.Join(root, ".git", "config")
					write(t, config, read(t, config)+"[hook \"check\"]\n command = echo foreign\n event = pre-commit\n")
				}
				save(t, s, m)
				p, err := s.Preview()
				must(t, err)
				if p.Blocked {
					t.Fatalf("own output became a foreign collision: %+v", p.Changes)
				}
				if len(sync(t, s).Applied) != 0 {
					t.Fatal("main checkout sync was not idempotent")
				}
				for _, c := range p.Changes {
					if c.Action == "inactive" {
						t.Fatalf("main checkout include inactive: %+v", c)
					}
				}
			})
		}
	}
}

func TestGitHelperRestorePreservesPriorMode(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	helper := filepath.Join(d.base, "skillshare", "files", "guard", "check.sh")
	original := "#!/bin/sh\nprintf original\n"
	write(t, helper, original)
	must(t, os.Chmod(helper, 0600))
	r := save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry), Replace: true})
	p, err := e.service.PreviewRestore(r.BackupIDs[0])
	must(t, err)
	_, err = e.service.Restore(r.BackupIDs[0], p.Revision)
	must(t, err)
	info, err := os.Stat(helper)
	must(t, err)
	if read(t, helper) != original || info.Mode().Perm() != 0600 {
		t.Fatalf("restore changed original bytes/mode: %s / %v", read(t, helper), info.Mode())
	}
}

func TestGitPublicMutationRejectsChangedReviewedOutput(t *testing.T) {
	for _, section := range []bool{false, true} {
		t.Run(fmt.Sprintf("foreign-section=%v", section), func(t *testing.T) {
			e := gitEnv(t)
			save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
			d, err := e.service.gitDestination("")
			must(t, err)
			path := d.hooksFile
			if section {
				path = d.includeTarget
				write(t, path, read(t, path)+"[hook \"check\"]\n # reviewed comment\n command = echo foreign\n event = pre-commit\n")
			}
			m := Mutation{Name: "guard", Entry: entry(t, gitEntry), Replace: true}
			p, err := e.service.PreviewMutation(m)
			must(t, err)
			changed := strings.ReplaceAll(read(t, path), "reviewed comment", "new comment")
			if !section {
				changed += "# edited after preview\n"
			}
			write(t, path, changed)
			_, err = e.service.Mutate(m, p.Revision, true)
			if !errors.Is(err, ErrStaleRevision) || read(t, path) != changed {
				t.Fatalf("stale mutation overwrote reviewed output: %v / %s", err, read(t, path))
			}
		})
	}
}

func TestGitRestoreRejectsForeignNamesWithManualInclude(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	write(t, d.includeTarget, includeLines(d.includeValue))
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	r := save(t, e.service, Mutation{Name: "guard", Remove: true})
	write(t, d.includeTarget, read(t, d.includeTarget)+"[hook \"check\"]\n command = echo foreign\n event = pre-commit\n")
	var id string
	for _, bid := range r.BackupIDs {
		data := read(t, filepath.Join(e.service.stateDir(), "backups", bid+".json"))
		var backup backupRecord
		must(t, json.Unmarshal([]byte(data), &backup))
		if backup.Path == d.hooksFile {
			id = bid
		}
	}
	if id == "" {
		t.Fatal("generated-file removal backup missing")
	}
	p, err := e.service.PreviewRestore(id)
	must(t, err)
	if !p.Blocked {
		t.Fatalf("restore would activate colliding commands: %+v", p.Changes)
	}
}

func TestGitRestoreRevisionIncludesForeignConfiguration(t *testing.T) {
	for _, ownedInclude := range []bool{false, true} {
		t.Run(fmt.Sprintf("owned-include=%v", ownedInclude), func(t *testing.T) {
			e := gitEnv(t)
			d, err := e.service.gitDestination("")
			must(t, err)
			if !ownedInclude {
				write(t, d.includeTarget, includeLines(d.includeValue))
			}
			save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
			r := save(t, e.service, Mutation{Name: "guard", Remove: true})
			ids := map[string]string{}
			for _, bid := range r.BackupIDs {
				var backup backupRecord
				must(t, json.Unmarshal([]byte(read(t, filepath.Join(e.service.stateDir(), "backups", bid+".json"))), &backup))
				ids[backup.Path] = bid
			}
			id := ids[d.hooksFile]
			if ownedInclude {
				p, err := e.service.PreviewRestore(id)
				must(t, err)
				_, err = e.service.Restore(id, p.Revision)
				must(t, err)
				id = ids[d.includeTarget]
			}
			p, err := e.service.PreviewRestore(id)
			must(t, err)
			if p.Blocked {
				t.Fatalf("initial restore unexpectedly blocked: %+v", p.Changes)
			}
			config := ""
			if pathExists(d.includeTarget) {
				config = read(t, d.includeTarget)
			}
			write(t, d.includeTarget, config+"[hook \"check\"]\n command = echo foreign\n event = pre-commit\n")
			_, err = e.service.Restore(id, p.Revision)
			if !errors.Is(err, ErrStaleRevision) {
				t.Fatalf("restore accepted newly colliding definition: %v", err)
			}
			q, err := e.service.PreviewRestore(id)
			must(t, err)
			if !q.Blocked {
				t.Fatal("fresh restore did not report collision")
			}
		})
	}
}

func TestGitSyncGlobalAndDeclaredProjectTogether(t *testing.T) {
	e := gitEnv(t)
	root := t.TempDir()
	gitSetup(t, root, "init", "--quiet")
	_, err := e.service.Mutate(Mutation{Name: "global", Entry: entry(t, gitEntry)}, "", false)
	must(t, err)
	local := strings.ReplaceAll(gitEntry, `"check":`, `"local.check":`)
	_, err = e.service.Mutate(Mutation{Name: "local", Project: root, Entry: entry(t, local)}, "", false)
	must(t, err)
	sync(t, e.service)
	if len(sync(t, e.service).Applied) != 0 {
		t.Fatal("combined scope sync is not idempotent")
	}
}

func TestGitCombinedScopesRejectMergedFriendlyNamesBeforeWrite(t *testing.T) {
	e := gitEnv(t)
	root := t.TempDir()
	gitSetup(t, root, "init", "--quiet")
	_, err := e.service.Mutate(Mutation{Name: "global", Entry: entry(t, gitEntry)}, "", false)
	must(t, err)
	_, err = e.service.Mutate(Mutation{Name: "local", Project: root, Entry: entry(t, gitEntry)}, "", false)
	must(t, err)
	p, err := e.service.Preview()
	must(t, err)
	if !p.Blocked {
		t.Fatal("combined scopes would publish merged hook.check definitions")
	}
	d, err := e.service.gitDestination("")
	must(t, err)
	_, err = e.service.applyScoped(p, nil)
	if err == nil || pathExists(d.hooksFile) {
		t.Fatal("colliding combined plan wrote Git output")
	}
}

func TestGitForeignSectionRestoreChecksUnrecordedGeneratedNames(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	write(t, d.includeTarget, "[hook \"check\"]\n command = echo foreign\n event = pre-commit\n")
	r := save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry), Replace: true})
	var id string
	for _, bid := range r.BackupIDs {
		var b backupRecord
		must(t, json.Unmarshal([]byte(read(t, filepath.Join(e.service.stateDir(), "backups", bid+".json"))), &b))
		if b.Kind == kindGitSection {
			id = bid
		}
	}
	if id == "" {
		t.Fatal("foreign section backup missing")
	}
	state, _, err := e.service.loadLedger()
	must(t, err)
	for key, record := range state.Records {
		if record.Path == d.hooksFile {
			delete(state.Records, key)
		}
	}
	must(t, writeJSONFile(e.service.statePath(), state))
	p, err := e.service.PreviewRestore(id)
	must(t, err)
	if !p.Blocked {
		t.Fatal("unrecorded generated hook.check would merge with restored foreign section")
	}
}

func TestGitForeignSectionRestoreGuardsInactiveGeneratedFile(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	write(t, d.includeTarget, "[hook \"check\"]\n command = echo foreign\n event = pre-commit\n")
	r := save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry), Replace: true})
	var id string
	for _, bid := range r.BackupIDs {
		var b backupRecord
		must(t, json.Unmarshal([]byte(read(t, filepath.Join(e.service.stateDir(), "backups", bid+".json"))), &b))
		if b.Kind == kindGitSection {
			id = bid
		}
	}
	save(t, e.service, Mutation{Name: "guard", Remove: true})
	p, err := e.service.PreviewRestore(id)
	must(t, err)
	if p.Blocked {
		t.Fatal("restoring removed generated name is blocked")
	}
	write(t, d.hooksFile, "[hook \"check\"]\n command = echo changed\n event = pre-commit\n")
	_, err = e.service.Restore(id, p.Revision)
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("new unrecorded generated name bypassed restore preview: %v", err)
	}
}

func TestGitIncludeRecoveryWhenTargetIsAbsent(t *testing.T) {
	for _, add := range []bool{false, true} {
		t.Run(fmt.Sprintf("add=%v", add), func(t *testing.T) {
			e := gitEnv(t)
			d, err := e.service.gitDestination("")
			must(t, err)
			state, _, err := e.service.loadLedger()
			must(t, err)
			key := elementKey(e.config, d.identity, d.includeTarget, "include.path", "", 0)
			state.Records[key] = record{Owner: e.config, Target: "git", Path: d.includeTarget, Event: "include.path", Hash: digest([]byte(d.includeValue))}
			must(t, writeJSONFile(e.service.statePath(), state))
			next := ledger{Version: 1, Records: map[string]record{}}
			must(t, writeJSONFile(e.service.journalPath(), journal{Kind: kindGitInclude, Path: d.includeTarget, Value: d.includeValue, Add: add, State: next}))
			e.service.Git = versionGit{missing: true}
			must(t, e.service.recoverPending())
			recovered, _, err := e.service.loadLedger()
			must(t, err)
			_, kept := recovered.Records[key]
			if kept != add || pathExists(e.service.journalPath()) {
				t.Fatalf("missing-file recovery lost completion boundary: kept=%v add=%v", kept, add)
			}
		})
	}
}

func TestGitRecoveryErrorPreservesJournalUntilGitReturns(t *testing.T) {
	for _, kind := range []string{kindGitInclude, kindGitSection} {
		t.Run(kind, func(t *testing.T) {
			e := gitEnv(t)
			d, err := e.service.gitDestination("")
			must(t, err)
			write(t, d.includeTarget, "[include]\n path = "+gitQuote(d.includeValue)+"\n[credential]\n helper = PRIVATE_CONFIG_SENTINEL\n")
			previous := ledger{Version: 1, Records: map[string]record{"previous": {Owner: e.config, Target: "git", Path: d.hooksFile}}}
			must(t, writeJSONFile(e.service.statePath(), previous))
			next := ledger{Version: 1, Records: map[string]record{"recovered": {Owner: e.config, Target: "git", Path: d.hooksFile}}}
			// The include is already added, or the named section already removed.
			pending := journal{Kind: kind, Path: d.includeTarget, Value: d.includeValue, Add: true, SectionName: "check", After: digest([]byte("[]")), State: next}
			must(t, writeJSONFile(e.service.journalPath(), pending))
			beforeConfig, beforeJournal, beforeState := read(t, d.includeTarget), read(t, e.service.journalPath()), read(t, e.service.statePath())
			workingGit := e.service.gitRunner()
			e.service.Git = versionGit{missing: true}
			_, _, err = e.service.loadLedger()
			if err == nil || !strings.Contains(err.Error(), e.service.journalPath()) || !strings.Contains(err.Error(), "restore Git") || !strings.Contains(err.Error(), "retry") || !strings.Contains(err.Error(), "keep") {
				t.Fatalf("missing actionable journal recovery guidance: %v", err)
			}
			if !errors.Is(err, exec.ErrNotFound) {
				t.Fatalf("recovery error lost Git cause: %v", err)
			}
			if strings.Contains(err.Error(), "PRIVATE_CONFIG_SENTINEL") {
				t.Fatal("private config exposed in recovery guidance")
			}
			if read(t, d.includeTarget) != beforeConfig || read(t, e.service.journalPath()) != beforeJournal || read(t, e.service.statePath()) != beforeState {
				t.Fatal("failed recovery changed config, journal or ownership")
			}
			e.service.Git = workingGit
			must(t, e.service.recoverPending())
			recovered, _, err := e.service.loadLedger()
			must(t, err)
			if _, ok := recovered.Records["recovered"]; !ok || pathExists(e.service.journalPath()) || read(t, d.includeTarget) != beforeConfig {
				t.Fatal("recovery did not complete safely after Git returned")
			}
		})
	}
}

func TestGitGlobalRestoreRejectsManagedProjectName(t *testing.T) {
	for _, standalone := range []bool{false, true} {
		t.Run(fmt.Sprintf("standalone=%v", standalone), func(t *testing.T) {
			e := gitEnv(t)
			d, err := e.service.gitDestination("")
			must(t, err)
			write(t, d.includeTarget, includeLines(d.includeValue))
			save(t, e.service, Mutation{Name: "global", Entry: entry(t, gitEntry)})
			r := save(t, e.service, Mutation{Name: "global", Remove: true})
			var id string
			for _, bid := range r.BackupIDs {
				var b backupRecord
				must(t, json.Unmarshal([]byte(read(t, filepath.Join(e.service.stateDir(), "backups", bid+".json"))), &b))
				if b.Path == d.hooksFile {
					id = bid
				}
			}
			if id == "" {
				t.Fatal("missing global file backup")
			}
			root := t.TempDir()
			gitSetup(t, root, "init", "--quiet")
			if standalone {
				local := e.project(root)
				save(t, local, Mutation{Name: "local", Entry: entry(t, gitEntry)})
			} else {
				save(t, e.service, Mutation{Name: "local", Project: root, Entry: entry(t, gitEntry)})
			}
			p, err := e.service.PreviewRestore(id)
			must(t, err)
			if !p.Blocked {
				t.Fatal("restore would merge global and project hook.check definitions")
			}
			if _, err := e.service.Restore(id, p.Revision); err == nil || pathExists(d.hooksFile) {
				t.Fatal("blocked restore published the global hooks file")
			}
		})
	}
}

func TestGitSyncRepairsOwnedHelperExecuteBit(t *testing.T) {
	e := gitEnv(t)
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	d, err := e.service.gitDestination("")
	must(t, err)
	helper := filepath.Join(d.base, "skillshare", "files", "guard", "check.sh")
	before := read(t, helper)
	must(t, os.Chmod(helper, 0644))
	r := sync(t, e.service)
	info, err := os.Stat(helper)
	must(t, err)
	if info.Mode().Perm() != 0755 || read(t, helper) != before || len(r.Applied) != 1 || r.Applied[0] != helper {
		t.Fatalf("sync did not repair only the helper execute bit: mode=%v applied=%v", info.Mode(), r.Applied)
	}
	if len(sync(t, e.service).Applied) != 0 {
		t.Fatal("mode repair is not idempotent")
	}
}

func TestGitProjectRestoreKeepsDifferentProjectsIsolated(t *testing.T) {
	e := gitEnv(t)
	first, second := t.TempDir(), t.TempDir()
	gitSetup(t, first, "init", "--quiet")
	gitSetup(t, second, "init", "--quiet")
	save(t, e.service, Mutation{Name: "first", Project: first, Entry: entry(t, gitEntry)})
	d, err := e.service.scopedFor(first).gitDestination(first)
	must(t, err)
	r := save(t, e.service, Mutation{Name: "first", Project: first, Remove: true})
	var id string
	for _, bid := range r.BackupIDs {
		var b backupRecord
		must(t, json.Unmarshal([]byte(read(t, filepath.Join(e.service.stateDir(), "backups", bid+".json"))), &b))
		if b.Path == d.hooksFile {
			id = bid
		}
	}
	if id == "" {
		t.Fatal("missing project file backup")
	}
	save(t, e.service, Mutation{Name: "second", Project: second, Entry: entry(t, gitEntry)})
	p, err := e.service.PreviewRestore(id)
	must(t, err)
	if p.Blocked {
		t.Fatalf("independent project names cannot merge: %+v", p.Changes)
	}
	_, err = e.service.Restore(id, p.Revision)
	must(t, err)
	if !pathExists(d.hooksFile) {
		t.Fatal("isolated project file was not restored")
	}
}

func TestGitStandaloneProjectRestoreKeepsOtherProjectIsolated(t *testing.T) {
	e := gitEnv(t)
	first, second := t.TempDir(), t.TempDir()
	gitSetup(t, first, "init", "--quiet")
	gitSetup(t, second, "init", "--quiet")
	local := e.project(first)
	save(t, local, Mutation{Name: "first", Entry: entry(t, gitEntry)})
	d, err := local.gitDestination("")
	must(t, err)
	r := save(t, local, Mutation{Name: "first", Remove: true})
	var id string
	for _, bid := range r.BackupIDs {
		var b backupRecord
		must(t, json.Unmarshal([]byte(read(t, filepath.Join(local.stateDir(), "backups", bid+".json"))), &b))
		if b.Path == d.hooksFile {
			id = bid
		}
	}
	if id == "" {
		t.Fatal("missing standalone project backup")
	}
	save(t, e.service, Mutation{Name: "second", Project: second, Entry: entry(t, gitEntry)})
	p, err := local.PreviewRestore(id)
	must(t, err)
	if p.Blocked {
		t.Fatalf("standalone and declared projects are isolated: %+v", p.Changes)
	}
	_, err = local.Restore(id, p.Revision)
	must(t, err)
}

func TestGitConfigCommitDoesNotReleaseAnotherWritersLock(t *testing.T) {
	e := gitEnv(t)
	p := gitDraft(t, e, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	for _, f := range p.files {
		if f.include == nil {
			continue
		}
		commit, release, err := f.prepareGitWrite(false)
		must(t, err)
		defer release()
		must(t, commit())
		write(t, f.path+".lock", "another writer")
		release()
		if got := read(t, f.path+".lock"); got != "another writer" {
			t.Fatalf("replacement lock changed: %q", got)
		}
		return
	}
	t.Fatal("include operation missing")
}

func TestGitHelperPermissionsSettleOnWindows(t *testing.T) {
	e := gitEnv(t)
	e.service.Platform = "windows"
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	d, err := e.service.gitDestination("")
	must(t, err)
	helper := filepath.Join(d.base, "skillshare", "files", "guard", "check.sh")
	// Model the permission bits reported by Windows on every read.
	must(t, os.Chmod(helper, 0644))
	result := sync(t, e.service)
	if len(result.Applied) != 0 || len(result.BackupIDs) != 0 {
		t.Fatalf("unchanged Windows helper rewritten: %+v", result)
	}
	if runtime.GOOS == "windows" {
		return
	}
	e.service.Platform = "linux"
	result = sync(t, e.service)
	if len(result.Applied) != 1 || result.Applied[0] != helper {
		t.Fatalf("POSIX executable permission not restored: %+v", result)
	}
}

func TestGitInactiveOutputsRetainTheirActions(t *testing.T) {
	for _, version := range []string{"2.39.5", "2.53.0"} {
		t.Run(version, func(t *testing.T) {
			e := gitEnv(t)
			e.service.Git = versionGit{GitRunner: e.service.gitRunner(), version: version}
			p := gitDraft(t, e, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
			for _, c := range p.Changes {
				if c.Target == "git" && (c.Action != "add" || !strings.Contains(c.InactiveReason, version)) {
					t.Fatalf("inactive status hid an add: %+v", c)
				}
			}
			result := sync(t, e.service)
			if len(result.Applied) != 3 {
				t.Fatalf("inactive Git outputs not generated: %+v", result)
			}
			p, err := e.service.Preview()
			must(t, err)
			if len(p.Changes) != 2 {
				t.Fatalf("inactive output status missing after sync: %+v", p.Changes)
			}
			if len(p.Warnings) == 0 {
				t.Fatal("inactive Git warning missing")
			}
			for _, c := range p.Changes {
				if c.Target == "git" && (c.Action != "unchanged" || !strings.Contains(c.InactiveReason, version)) {
					t.Fatalf("inactive Git stays pending after sync: %+v", c)
				}
			}
			if len(sync(t, e.service).Applied) != 0 {
				t.Fatal("inactive sync is not idempotent")
			}
		})
	}
}

func TestGitConfigFailedCommitReleasesItsLock(t *testing.T) {
	e := gitEnv(t)
	p := gitDraft(t, e, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	for _, f := range p.files {
		if f.include == nil {
			continue
		}
		commit, release, err := f.prepareGitWrite(false)
		must(t, err)
		defer release()
		// A directory at the destination makes the rename fail on every platform.
		must(t, os.Mkdir(f.path, 0755))
		if err := commit(); err == nil {
			t.Fatal("commit unexpectedly succeeded")
		}
		release()
		if pathExists(f.path + ".lock") {
			t.Fatal("failed commit left its lock behind")
		}
		return
	}
	t.Fatal("include operation missing")
}

func TestGitUnwritableIncludePreservesActionsAfterSync(t *testing.T) {
	e := gitEnv(t)
	target := filepath.Join(e.home, ".gitconfig")
	referent := filepath.Join(e.home, "dotfiles.gitconfig")
	write(t, referent, "[user]\n name = unchanged\n")
	must(t, os.Symlink(referent, target))
	e.service.Git = versionGit{GitRunner: e.service.gitRunner(), version: "2.55.0"}
	p := gitDraft(t, e, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	for _, c := range p.Changes {
		if c.Action != "add" {
			t.Fatalf("inactive include hid addition: %+v", c)
		}
	}
	result := sync(t, e.service)
	if len(result.Applied) != 2 {
		t.Fatalf("manual include should write only outputs: %+v", result)
	}
	p, err := e.service.Preview()
	must(t, err)
	for _, c := range p.Changes {
		if c.Action != "unchanged" || c.InactiveReason == "" || !strings.Contains(c.InactiveReason, "add manually") {
			t.Fatalf("manual include status: %+v", c)
		}
	}
	if read(t, referent) != "[user]\n name = unchanged\n" {
		t.Fatal("user-owned include target changed")
	}
}

func TestGitBindingIgnoresSameNamedAgentAccount(t *testing.T) {
	for _, present := range []bool{false, true} {
		t.Run(fmt.Sprintf("account-present=%v", present), func(t *testing.T) {
			e := gitEnv(t)
			dir := filepath.Join(e.home, "account")
			if present {
				must(t, os.MkdirAll(dir, 0755))
			}
			e.service.Accounts = map[string]Account{"git": {Agent: "codex", Dir: dir}}
			result := save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
			if len(result.Applied) != 3 {
				t.Fatalf("Git publication redirected to account: %+v", result)
			}
			inv, err := e.service.List()
			must(t, err)
			count := 0
			for _, def := range inv.Targets {
				if def.Name == "git" {
					count++
					if def.Kind != KindGit {
						t.Fatalf("Git target reinterpreted: %+v", def)
					}
				}
			}
			if count != 1 {
				t.Fatalf("duplicate Git targets: %+v", inv.Targets)
			}
			d, err := e.service.gitDestination("")
			must(t, err)
			if inv.Paths["git"] != d.hooksFile {
				t.Fatalf("Git path redirected: %v", inv.Paths)
			}
			p, err := e.service.Preview()
			must(t, err)
			if !strings.Contains(strings.Join(p.Warnings, "\n"), "target git is a codex account") {
				t.Fatalf("same-named account ignored without warning: %v", p.Warnings)
			}
		})
	}
}

func TestGitAccountWithoutGitBindingIsQuiet(t *testing.T) {
	e := gitEnv(t)
	e.service.Accounts = map[string]Account{"git": {Agent: "codex", Dir: filepath.Join(e.home, "account")}}
	p := gitDraft(t, e, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	if strings.Contains(strings.Join(p.Warnings, "\n"), "target git") {
		t.Fatalf("account warning without a git binding: %v", p.Warnings)
	}
}

func TestGitInactiveReasonReportedOnce(t *testing.T) {
	e := gitEnv(t)
	e.service.Git = versionGit{GitRunner: e.service.gitRunner(), version: "2.39.5"}
	root := t.TempDir()
	gitSetup(t, root, "init", "--quiet")
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	projectEntry := strings.Replace(gitEntry, `"check":`, `"project-check":`, 1)
	p := gitDraft(t, e, Mutation{Project: root, Name: "guard", Entry: entry(t, projectEntry)})
	count := 0
	for _, w := range p.Warnings {
		if strings.Contains(w, "2.39.5") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("want the inactive reason once in warnings: %v", p.Warnings)
	}
	for _, c := range p.Changes {
		if strings.Contains(c.Message, "2.39.5") {
			t.Fatalf("inactive reason repeated in a change message: %+v", c)
		}
	}
}
