package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// keepBackups bounds the backups kept per native file; every write adds one.
const keepBackups = 20

var backupID = regexp.MustCompile(`^[0-9]+-[a-f0-9]{8}$`)

// ErrUnknownProject is returned for a root hooks.projects does not declare.
var ErrUnknownProject = errors.New("unknown hooks project")

type journal struct {
	Kind        string `json:"kind,omitempty"`
	Value       string `json:"value,omitempty"`
	Add         bool   `json:"add,omitempty"`
	SectionName string `json:"sectionName,omitempty"`
	Path        string `json:"path"`
	After       string `json:"after"` // digest of the written file, "" for a removal
	State       ledger `json:"state"`
}

type backupRecord struct {
	GitInclude *gitIncludeOp `json:"gitInclude,omitempty"`
	GitSection *gitSectionOp `json:"gitSection,omitempty"`
	BeforeMode os.FileMode   `json:"beforeMode,omitempty"`
	AfterMode  os.FileMode   `json:"afterMode,omitempty"`
	ID         string        `json:"id"`
	Owner      string        `json:"owner"`
	Target     string        `json:"target"`
	Path       string        `json:"path"`
	Root       string        `json:"root,omitempty"`
	Kind       string        `json:"kind"`
	// Ops are the element edits of a shared file, with the values before and after.
	Ops []elementOp `json:"ops,omitempty"`
	// Before and After are a whole file's contents; nil means absent.
	Before *string `json:"before,omitempty"`
	After  *string `json:"after,omitempty"`
	// Ownership of the moved records before and after the write; nil means none.
	OwnedBefore map[string]*record `json:"ownedBefore"`
	OwnedAfter  map[string]*record `json:"ownedAfter"`
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if _, _, _, err := safeRead(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".hooks-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(mode); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, append(data, '\n'), 0600)
}

func (s *Service) journalPath() string { return filepath.Join(s.stateDir(), "pending.json") }

// readPending returns an interrupted write's journal and whether its native file was
// written, in which case recovery records the journal's ownership.
func (s *Service) readPending() (*journal, bool, error) {
	data, exists, _, err := safeRead(s.journalPath())
	if err != nil || !exists {
		return nil, false, err
	}
	var pending journal
	if json.Unmarshal(data, &pending) != nil || pending.Path == "" || pending.State.Version != 1 || pending.State.Records == nil {
		return nil, false, fmt.Errorf("hooks recovery journal is invalid; manual recovery required")
	}
	if pending.Kind == kindGitInclude {
		_, exists, _, err := safeRead(pending.Path)
		if err != nil {
			return nil, false, err
		}
		if !exists {
			return &pending, !pending.Add, nil
		}
		rows, err := s.gitFileIncludes(pending.Path)
		if err != nil {
			return nil, false, s.gitRecoveryError(err)
		}
		present := false
		for _, row := range rows {
			if row.Key == "include.path" && row.Value == pending.Value {
				present = true
			}
		}
		return &pending, present == pending.Add, nil
	}
	if pending.Kind == kindGitSection {
		data, _, _, err := safeRead(pending.Path)
		if err != nil {
			return nil, false, err
		}
		rows, err := s.gitParseBytes(data)
		if err != nil {
			return nil, false, s.gitRecoveryError(err)
		}
		return &pending, gitValuesDigest(sectionRows(rows, pending.SectionName, true)) == pending.After, nil
	}
	current, exists, _, err := safeRead(pending.Path)
	if err != nil {
		return nil, false, err
	}
	if pending.After == "" {
		return &pending, !exists, nil
	}
	return &pending, exists && digest(current) == pending.After, nil
}

func (s *Service) gitRecoveryError(err error) error {
	return fmt.Errorf("hooks recovery is blocked; keep journal %s intact, restore Git or repair the config, then retry: %w", s.journalPath(), err)
}

func (s *Service) recoverPending() error {
	pending, written, err := s.readPending()
	if err != nil || pending == nil {
		return err
	}
	if written {
		if err := writeJSONFile(s.statePath(), pending.State); err != nil {
			return err
		}
	}
	// Otherwise the write did not happen, or the file changed again; ownership hashes
	// flag any output that no longer matches, so the journal is obsolete.
	return os.Remove(s.journalPath())
}

// draft applies a mutation to a fresh source and returns the entries it may replace
// and the entries that adopt their identical unmanaged registrations.
func (s *Service) draft(m Mutation) (*Source, map[string]bool, map[string]bool, error) {
	source, err := s.loadSource()
	if err != nil {
		return nil, nil, nil, err
	}
	if m.Adopt && (m.Entry == nil || m.Name == "") {
		return nil, nil, nil, fmt.Errorf("adopt takes a hook name and entry")
	}
	if m.Entry != nil && m.Remove {
		return nil, nil, nil, fmt.Errorf("cannot save and remove the same hook")
	}
	if m.Unmanage && (!m.Remove || m.Name == "") {
		return nil, nil, nil, fmt.Errorf("stop managing applies only to removing a named hook")
	}
	if (m.Entry != nil || m.Remove || m.Replace) && m.Name == "" && m.Project == "" {
		return nil, nil, nil, fmt.Errorf("hook name is required")
	}
	root := ""
	entries := source.Entries
	if m.Project != "" {
		if s.ProjectRoot != "" {
			return nil, nil, nil, fmt.Errorf("hooks.projects belongs in the global config")
		}
		if root, err = projectRoot(m.Project); err != nil {
			return nil, nil, nil, err
		}
		project, exists := source.Projects[root]
		if _, known := source.projectKeys[root]; !known {
			source.projectKeys[root] = m.Project
		}
		switch {
		case m.Remove && m.Name == "":
			if !exists {
				return nil, nil, nil, fmt.Errorf("%w: %s", ErrUnknownProject, m.Project)
			}
			delete(source.Projects, root)
			source.touchedProjects[root] = true
			return source, nil, nil, nil
		case m.Name == "" && m.Entry == nil:
			if !exists {
				source.Projects[root] = Project{Entries: map[string]Entry{}}
				source.touchedProjects[root] = true
			}
			return source, nil, nil, nil
		}
		if project.Entries == nil {
			project.Entries = map[string]Entry{}
		}
		source.Projects[root] = project
		entries = project.Entries
		source.touchedProjects[root] = true
	}
	replace, adopt := map[string]bool{}, map[string]bool{}
	if m.Replace {
		replace[root+"\x00"+m.Name] = true
	}
	if m.Adopt {
		adopt[root+"\x00"+m.Name] = true
	}
	switch {
	case m.Remove:
		if _, ok := entries[m.Name]; !ok {
			return nil, nil, nil, fmt.Errorf("hook %q not found", m.Name)
		}
		if m.Unmanage && len(entries[m.Name].Bindings["git"].Commands) > 0 {
			return nil, nil, nil, fmt.Errorf("Git commands cannot keep files while removing their declaration; copy them to your own include first")
		}
		delete(entries, m.Name)
		if m.Unmanage {
			source.unmanaged[root+"\x00"+m.Name] = true
		}
	case m.Entry != nil:
		entry := *m.Entry
		if err := entry.normalize(); err != nil {
			return nil, nil, nil, err
		}
		if err := s.validateEntry(m.Name, entry, root); err != nil {
			return nil, nil, nil, err
		}
		entries[m.Name] = entry
	}
	if m.Project == "" && (m.Remove || m.Entry != nil) {
		source.touched[m.Name] = true
	}
	return source, replace, adopt, nil
}

// PreviewMutation does not persist a draft or claim ownership.
func (s *Service) PreviewMutation(m Mutation) (*Plan, error) {
	source, replace, adopt, err := s.draft(m)
	if err != nil {
		return nil, err
	}
	return s.previewSource(source, replace, adopt)
}

// Mutate validates before saving. With sync, it refuses to save when native outputs
// conflict, then saves the source and applies the plan. Without sync, it saves the
// source only, even when native sync would be blocked.
func (s *Service) Mutate(m Mutation, revision string, sync bool) (*Result, error) {
	if m.Unmanage && sync {
		return nil, fmt.Errorf("stop managing keeps target files as they are, so it cannot be combined with sync")
	}
	lock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()
	if err := s.recoverPending(); err != nil {
		return nil, err
	}
	source, replace, adopt, err := s.draft(m)
	if err != nil {
		return nil, err
	}
	if !sync && revision == "" && !m.Adopt {
		// Saving the source alone needs only definitions every scope can render; a
		// blocked or unreadable native file must not stop it.
		if _, err := s.render(source); err != nil {
			return nil, err
		}
		if err := source.save(); err != nil {
			return nil, err
		}
		if err := s.forgetUnmanaged(source); err != nil {
			return nil, err
		}
		return &Result{Applied: []string{}, BackupIDs: []string{}}, nil
	}
	p, err := s.previewSource(source, replace, adopt)
	if err != nil {
		return nil, err
	}
	if revision != "" && revision != p.Revision {
		return nil, ErrStaleRevision
	}
	// A project mutation syncs only its own root; pending global or other-project
	// changes, and their conflicts, wait for their own sync.
	var scope *string
	if m.Project != "" {
		root, err := projectRoot(m.Project)
		if err != nil {
			return nil, err
		}
		scope = &root
	}
	if sync && p.blockedIn(scope) {
		return &Result{Plan: p, Applied: []string{}, BackupIDs: []string{}}, ErrConflict
	}
	if err := source.save(); err != nil {
		return nil, err
	}
	if !sync {
		if err := s.recordAdopted(p); err != nil {
			return nil, fmt.Errorf("source saved; recording the imported registrations failed: %w", err)
		}
		if err := s.forgetUnmanaged(source); err != nil {
			return nil, err
		}
		return &Result{Plan: p, Applied: []string{}, BackupIDs: []string{}}, nil
	}
	// The plan was computed from the draft, which is now the saved source.
	result, err := s.applyScoped(p, scope)
	if err != nil && len(source.touched)+len(source.touchedProjects) > 0 {
		return result, fmt.Errorf("source saved; synchronization incomplete: %w", err)
	}
	return result, err
}

// recordAdopted claims the registrations an import adopts without syncing, so the
// next sync adopts them in place instead of reporting a conflict.
func (s *Service) recordAdopted(p *Plan) error {
	if len(p.adopted) == 0 {
		return nil
	}
	state, _, err := s.loadLedger()
	if err != nil {
		return err
	}
	for _, key := range p.adopted {
		r := p.state.Records[key]
		r.Adopted = true
		state.Records[key] = r
	}
	return writeJSONFile(s.statePath(), state)
}

// forgetUnmanaged drops the ownership records of the hooks a draft stops managing, so
// sync neither updates nor removes what they wrote.
func (s *Service) forgetUnmanaged(source *Source) error {
	if len(source.unmanaged) == 0 {
		return nil
	}
	state, _, err := s.loadLedger()
	if err != nil {
		return err
	}
	source.forget(state)
	if err := writeJSONFile(s.statePath(), state); err != nil {
		return fmt.Errorf("source saved; forgetting the hook's registrations failed: %w", err)
	}
	return nil
}

// forget deletes the records of the hooks this draft stops managing.
func (source *Source) forget(state ledger) {
	for key, r := range state.Records {
		if r.Owner == source.ConfigPath && source.unmanaged[r.Root+"\x00"+r.Entry] {
			delete(state.Records, key)
		}
	}
}

// ApplyProject applies only one hooks.projects root's changes. The revision still
// covers the whole plan, so any change anywhere asks for a new preview.
func (s *Service) ApplyProject(revision, root string) (*Result, error) {
	if revision == "" {
		return nil, fmt.Errorf("preview the hooks changes before syncing a project")
	}
	lock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()
	if err := s.recoverPending(); err != nil {
		return nil, err
	}
	p, err := s.Preview()
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	if _, ok := p.source.Projects[root]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownProject, root)
	}
	if revision != p.Revision {
		return nil, ErrStaleRevision
	}
	return s.applyScoped(p, &root)
}

// blockedIn reports a conflict among the plan's changes, or with root set only among
// that root's changes.
func (p *Plan) blockedIn(root *string) bool {
	if root == nil {
		return p.Blocked
	}
	for _, c := range p.Changes {
		if c.Action == "conflict" && c.Root == *root {
			return true
		}
	}
	return false
}

// applyScoped writes the plan's files, or with root set only that root's files, with
// a backup and recovery journal per file.
func (s *Service) applyScoped(p *Plan, root *string) (*Result, error) {
	result := &Result{Plan: p, Applied: []string{}, BackupIDs: []string{}}
	inScope := func(r string) bool { return root == nil || r == *root }
	if p.blockedIn(root) {
		return result, ErrConflict
	}
	if err := p.source.checkUnchanged(); err != nil {
		return result, err
	}
	state, stateBytes, err := s.loadLedger()
	if err != nil {
		return result, err
	}
	if !bytes.Equal(stateBytes, p.stateBytes) {
		return result, fmt.Errorf("hooks ownership changed: %w", ErrStaleRevision)
	}
	// Check every file before the first write, and each again at its write.
	for _, guard := range p.gitGuards {
		if err := guard.check(); err != nil {
			return result, err
		}
	}
	for _, f := range p.files {
		if inScope(f.root) {
			scope := s
			if f.root != "" {
				scope = s.scoped(f.root)
			}
			if scope.accountMissing(f.target) {
				return result, fmt.Errorf("%s: config_dir %s does not exist on this machine", f.target, scope.Accounts[f.target].Dir)
			}
			if _, _, _, _, err := f.refresh(); err != nil {
				return result, err
			}
		}
	}
	for _, f := range p.files {
		if !inScope(f.root) || !f.changed() {
			continue
		}
		if f.gitGuard != nil {
			if err := f.gitGuard.check(); err != nil {
				return result, err
			}
		}
		var finishGit func() error
		var releaseGit func()
		if f.kind == kindGitInclude || f.kind == kindGitSection {
			finishGit, releaseGit, err = f.prepareGitWrite(state.NewFiles[f.path])
			if err != nil {
				return result, err
			}
		}
		data, exists, mode, doc, err := f.refresh()
		if err != nil {
			if releaseGit != nil {
				releaseGit()
			}
			return result, err
		}
		if releaseGit != nil {
			defer releaseGit()
		}
		beforeMode := mode
		if f.target == "git" && f.kind == kindFile {
			mode = f.mode
		}
		after, remove := f.write, f.remove
		if f.kind == kindJSON {
			// Edit the file as it is now, keeping settings the Agent wrote since the preview.
			if after, _, err = doc.edit(f.ops, nil, f.created); err != nil {
				return result, fmt.Errorf("%s: %w", f.path, err)
			}
			// Settings added since the preview keep a file the plan deletes.
			remove = f.remove && skeleton(f.agent, after)
		}
		id := fmt.Sprintf("%d-%s", time.Now().UnixNano(), digest([]byte(f.path))[:8])
		backup := backupRecord{ID: id, Owner: p.source.ConfigPath, Target: f.target, Path: f.path, Root: f.root, Kind: f.kind, Ops: f.ops, OwnedBefore: map[string]*record{}, OwnedAfter: map[string]*record{}, GitInclude: f.include, GitSection: f.gitSection, BeforeMode: beforeMode, AfterMode: mode}
		if f.kind == kindFile {
			if exists {
				before := string(data)
				backup.Before = &before
			}
			if !remove {
				text := string(after)
				backup.After = &text
			}
		}
		for _, key := range f.keys {
			if r, ok := state.Records[key]; ok {
				backup.OwnedBefore[key] = &r
			}
			if r, ok := p.state.Records[key]; ok {
				backup.OwnedAfter[key] = &r
				state.Records[key] = r
			} else {
				delete(state.Records, key)
			}
		}
		if err := writeJSONFile(filepath.Join(s.stateDir(), "backups", id+".json"), backup); err != nil {
			return result, fmt.Errorf("hooks backup failed: %w", err)
		}
		if f.kind == kindJSON || f.kind == kindGitInclude {
			if p.state.Created[f.path] {
				if state.Created == nil {
					state.Created = map[string]bool{}
				}
				state.Created[f.path] = true
			} else {
				delete(state.Created, f.path)
			}
			switch {
			case remove:
				delete(state.NewFiles, f.path)
			case !exists:
				if state.NewFiles == nil {
					state.NewFiles = map[string]bool{}
				}
				state.NewFiles[f.path] = true
			}
		}
		if !remove {
			for _, dir := range append(missingDirs(filepath.Dir(f.path)), f.gitCreatedDirs...) {
				if state.Dirs == nil {
					state.Dirs = map[string]bool{}
				}
				state.Dirs[dir] = true
			}
		}
		pending := journal{Path: f.path, State: state}
		if !remove {
			pending.After = digest(after)
		}
		if f.include != nil {
			pending.Kind, pending.Value, pending.Add = kindGitInclude, f.include.Value, f.include.Add
		}
		if f.gitSection != nil {
			pending.Kind, pending.SectionName = kindGitSection, f.gitSection.Name
			pending.After = f.gitSectionAfter
		}
		if err := writeJSONFile(s.journalPath(), pending); err != nil {
			return result, err
		}
		if finishGit != nil {
			err = finishGit()
			releaseGit()
		} else if remove {
			err = os.Remove(f.path)
			if err == nil {
				if f.target != "git" {
					pruneEmptyDirs(filepath.Dir(f.path))
				}
				pruneCreatedDirs(state.Dirs, filepath.Dir(f.path))
			}
		} else {
			err = atomicWrite(f.path, after, mode)
		}
		if err == nil && f.kind == kindGitInclude && !pathExists(f.path) {
			delete(state.NewFiles, f.path)
			pruneCreatedDirs(state.Dirs, filepath.Dir(f.path))
		}
		if err != nil {
			_ = os.Remove(s.journalPath())
			return result, fmt.Errorf("hooks write failed for %s: %w", f.path, err)
		}
		result.Applied = append(result.Applied, f.path)
		result.BackupIDs = append(result.BackupIDs, id)
		if err := writeJSONFile(s.statePath(), state); err != nil {
			return result, fmt.Errorf("hooks file written but ownership update failed; retry sync to recover: %w", err)
		}
		if err := os.Remove(s.journalPath()); err != nil {
			return result, err
		}
		s.pruneBackups(f.path)
		if f.gitGuard != nil {
			for _, guard := range p.gitGuards {
				if err := guard.completed(f); err != nil {
					return result, err
				}
			}
		}
	}
	// Records that moved without a write, such as an adoption or a refreshed index.
	final := ledger{Version: 1, Records: map[string]record{}, Created: state.Created, NewFiles: state.NewFiles, Dirs: state.Dirs}
	for key, r := range state.Records {
		if !inScope(r.Root) {
			final.Records[key] = r
		}
	}
	for key, r := range p.state.Records {
		if inScope(r.Root) {
			final.Records[key] = r
		}
	}
	finalBytes, _ := json.Marshal(final)
	currentBytes, _ := json.Marshal(state)
	if !bytes.Equal(finalBytes, currentBytes) {
		if err := writeJSONFile(s.statePath(), final); err != nil {
			return result, err
		}
	}
	return result, nil
}

// pruneEmptyDirs removes an entry's script folder and hooks/skillshare once empty.
func pruneEmptyDirs(dir string) {
	for range 2 {
		if filepath.Base(filepath.Dir(dir)) != "skillshare" && filepath.Base(dir) != "skillshare" {
			return
		}
		if os.Remove(dir) != nil { // fails unless empty
			return
		}
		dir = filepath.Dir(dir)
	}
}

// missingDirs lists dir and each missing parent, nearest first: what MkdirAll creates.
func missingDirs(dir string) []string {
	var out []string
	for {
		if _, err := os.Lstat(dir); !os.IsNotExist(err) {
			return out
		}
		out = append(out, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			return out
		}
		dir = parent
	}
}

// pruneCreatedDirs removes dir and its parents while Skillshare created them and they
// are empty, stopping at a folder that existed before or still holds anything.
func pruneCreatedDirs(created map[string]bool, dir string) {
	for created[dir] {
		if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
			return
		}
		delete(created, dir)
		dir = filepath.Dir(dir)
	}
}

// Backups lists this config's backups, newest first, without their contents.
func (s *Service) Backups() ([]Backup, error) {
	owner, err := filepath.Abs(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	records, err := s.backupRecords()
	if err != nil {
		return nil, err
	}
	out := []Backup{}
	for _, r := range records {
		if r.Owner == owner {
			out = append(out, Backup{ID: r.ID, Target: r.Target, Path: r.Path, Root: r.Root, Time: backupTime(r.ID)})
		}
	}
	return out, nil
}

func backupTime(id string) string {
	stamp, _, _ := strings.Cut(id, "-")
	n, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil {
		return ""
	}
	return time.Unix(0, n).UTC().Format(time.RFC3339)
}

// backupRecords returns backups newest first; unreadable ones are skipped.
func (s *Service) backupRecords() ([]backupRecord, error) {
	dir := filepath.Join(s.stateDir(), "backups")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []backupRecord
	for i := len(entries) - 1; i >= 0; i-- {
		name := entries[i].Name()
		data, _, _, err := safeRead(filepath.Join(dir, name))
		var r backupRecord
		if err != nil || json.Unmarshal(data, &r) != nil || !backupID.MatchString(r.ID) || r.ID+".json" != name {
			continue
		}
		records = append(records, r)
	}
	return records, nil
}

// pruneBackups keeps the newest backups of one native file; best effort.
func (s *Service) pruneBackups(path string) {
	dir := filepath.Join(s.stateDir(), "backups")
	entries, _ := os.ReadDir(dir)
	suffix := "-" + digest([]byte(path))[:8] + ".json"
	kept := 0
	for i := len(entries) - 1; i >= 0; i-- {
		if name := entries[i].Name(); strings.HasSuffix(name, suffix) {
			if kept++; kept > keepBackups {
				_ = os.Remove(filepath.Join(dir, name))
			}
		}
	}
}

// PreviewRestore reverts only what one backed-up write changed. Outputs changed
// after the backup are conflicts; unrelated later changes are kept.
func (s *Service) PreviewRestore(id string) (*Plan, error) {
	if !backupID.MatchString(id) {
		return nil, fmt.Errorf("invalid hooks backup ID")
	}
	data, exists, _, err := safeRead(filepath.Join(s.stateDir(), "backups", id+".json"))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("hooks backup not found")
	}
	var b backupRecord
	if json.Unmarshal(data, &b) != nil || b.ID != id {
		return nil, fmt.Errorf("invalid hooks backup")
	}
	source, err := s.loadSource()
	if err != nil {
		return nil, err
	}
	if b.Owner != source.ConfigPath {
		return nil, fmt.Errorf("backup belongs to another config or scope")
	}
	state, stateBytes, err := s.loadLedger()
	if err != nil {
		return nil, err
	}
	p := &Plan{SourcePath: source.ConfigPath, Changes: []Change{}, Warnings: []string{}, source: source, stateBytes: stateBytes, state: ledger{Version: 1, Records: map[string]record{}, Created: state.Created, NewFiles: state.NewFiles, Dirs: state.Dirs}}
	for k, r := range state.Records {
		p.state.Records[k] = r
	}
	scope := s
	if b.Root != "" {
		scope = s.scoped(b.Root)
	}
	if _, ok := targetDef(scope.agentOf(b.Target)); !ok {
		return nil, fmt.Errorf("backup %s belongs to %s, which is not declared in targets; add it back to restore", id, b.Target)
	}
	if scope.accountMissing(b.Target) {
		return nil, fmt.Errorf("backup %s belongs to %s, whose config_dir %s does not exist on this machine; add it back to restore", id, b.Target, scope.Accounts[b.Target].Dir)
	}
	targetScope, agent := scope.forTarget(b.Target)
	dir, err := targetScope.configDir(agent)
	if err != nil {
		return nil, err
	}
	oldDir := filepath.Dir(b.Path)
	if b.Kind == kindFile {
		oldDir = recordDir(record{Path: b.Path})
	}
	if !samePath(oldDir, dir) {
		return nil, fmt.Errorf("backup %s belongs to %s in %s, but %s now resolves to %s; use its former config_dir to restore", id, b.Target, oldDir, b.Target, dir)
	}
	d, err := s.render(source)
	if err != nil {
		return nil, err
	}
	for _, r := range state.Records {
		if r.Owner == source.ConfigPath && s.parkReason(r) == "" {
			d.addOwned(r)
		}
	}
	if b.Kind == kindJSON {
		d.targets[b.Path], d.roots[b.Path] = b.Target, b.Root
	} else {
		d.files[b.Path] = wantFile{target: b.Target, path: b.Path, root: b.Root}
	}
	if err := s.checkDesiredHomes(d); err != nil {
		return nil, err
	}
	base, err := s.base(b.Target, b.Root)
	if err != nil {
		return nil, err
	}
	if b.Kind == kindGitInclude || b.Kind == kindGitSection {
		base = filepath.Dir(b.Path)
	}
	if err := checkPath(base, b.Path); err != nil {
		return nil, err
	}
	current, exists, mode, err := safeRead(b.Path)
	if err != nil {
		return nil, err
	}
	f := &filePlan{path: b.Path, target: b.Target, agent: scope.agentOf(b.Target), root: b.Root, base: base, kind: b.Kind, before: current, exists: exists, mode: mode}
	change := Change{Target: b.Target, Path: b.Path, Root: b.Root, Action: "restore"}
	conflict := func(message string) {
		change.Action, change.Message = "conflict", message
		p.Blocked = true
	}
	sameRecord := func(key string, want *record) bool {
		r, ok := state.Records[key]
		if want == nil {
			return !ok
		}
		return ok && r.Owner == want.Owner && r.Hash == want.Hash && r.Event == want.Event && r.Path == want.Path
	}
	restoreOwnership := func(key string) {
		if r := b.OwnedBefore[key]; r != nil {
			p.state.Records[key] = *r
		} else {
			delete(p.state.Records, key)
		}
		f.keys = append(f.keys, key)
	}
	if b.Kind == kindGitInclude {
		if b.GitInclude == nil {
			return nil, fmt.Errorf("invalid Git include backup")
		}
		rows, err := s.scopedFor(b.Root).gitFileIncludes(b.Path)
		if err != nil {
			return nil, err
		}
		present, count := false, 0
		for _, row := range rows {
			if row.Key == "include.path" && row.Value == b.GitInclude.Value {
				present = true
				count++
			}
		}
		f = gitIncludePlan(s.scopedFor(b.Root), gitDestination{root: b.Root, includeTarget: b.Path}, rows, b.GitInclude.Value, !b.GitInclude.Add)
		if present != b.GitInclude.Add || count > 1 {
			conflict("include changed after the backup")
		}
		for key := range mergeKeys(b.OwnedBefore, b.OwnedAfter) {
			if !sameRecord(key, b.OwnedAfter[key]) {
				conflict("include ownership changed after the backup")
			} else {
				restoreOwnership(key)
			}
		}
	} else if b.Kind == kindGitSection {
		if b.GitSection == nil {
			return nil, fmt.Errorf("invalid Git section backup")
		}
		f.gitService = s.scopedFor(b.Root)
		op := *b.GitSection
		op.Restore = !op.Restore
		f.gitSection = &op
		f.section = gitSectionDigest(current, op.Name)
		if op.Restore {
			destination, err := f.gitService.gitDestination(b.Root)
			if err != nil {
				return nil, err
			}
			for _, r := range state.Records {
				if r.Target == "git" && r.Event == "hook."+op.Name && sameGitPath(r.Path, destination.hooksFile) && !ownerGone(r.Owner) {
					conflict("hook name is still managed; remove its managed command before restoring the foreign section")
				}
			}
			_, err = f.gitService.restoreGitSection(current, op)
			if err != nil {
				conflict(err.Error())
			} else {
				for _, fragment := range op.Fragments {
					f.after = append(f.after, []byte(fragment.Text)...)
				}
				f.before = nil
			}
		} else {
			_, fragments, err := f.gitService.removeGitSection(current, op.Name)
			if err != nil {
				conflict(err.Error())
			} else {
				f.before = nil
				if !reflect.DeepEqual(fragments, op.Fragments) {
					conflict("section changed after the backup")
				}
				for _, fragment := range fragments {
					f.before = append(f.before, []byte(fragment.Text)...)
				}
				f.after = nil
			}
		}
	} else if b.Kind == kindFile {
		f.section = ""
		if exists {
			f.section = digest(current)
		}
		if mode == 0 {
			f.mode = 0644
		}
		if b.BeforeMode != 0 {
			f.mode = b.BeforeMode
		}
		for key := range b.OwnedAfter {
			change.Name = b.OwnedAfter[key].Entry
		}
		for key := range b.OwnedBefore {
			change.Name = b.OwnedBefore[key].Entry
		}
		afterMatches := b.After == nil && !exists || b.After != nil && exists && string(current) == *b.After
		owned := true
		for key, r := range b.OwnedAfter {
			owned = owned && sameRecord(key, r)
		}
		if !afterMatches || !owned {
			conflict("file changed after the backup; restore would overwrite newer changes")
		} else {
			for _, key := range sortedKeys(mergeKeys(b.OwnedBefore, b.OwnedAfter)) {
				restoreOwnership(key)
			}
			if b.Before == nil {
				f.remove = true
			} else {
				f.write = []byte(*b.Before)
			}
		}
	} else {
		doc, err := parseNative(f.agent, current)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", b.Path, err)
		}
		f.section = doc.sectionDigest()
		if mode == 0 {
			f.mode = 0644
		}
		// Locate against every current record of the file: identical copies resolve only
		// when all of them are accounted for. sameRecord below ties these to the backup.
		current := map[string]record{}
		for key, r := range state.Records {
			if r.Path == b.Path && r.Event != "" {
				current[key] = r
			}
		}
		located := locate(doc, current)
		for _, op := range b.Ops {
			if r := b.OwnedAfter[op.Key]; r != nil {
				change.Name = r.Entry
			} else if r := b.OwnedBefore[op.Key]; r != nil {
				change.Name = r.Entry
			}
			if !sameRecord(op.Key, b.OwnedAfter[op.Key]) {
				conflict("hook changed after the backup; restore would overwrite newer changes")
				break
			}
			switch {
			case op.Value == nil: // the write removed it: add it back
				f.ops = append(f.ops, elementOp{Key: op.Key, Event: op.Event, Index: -1, Value: op.Before})
			default:
				i, ok := located[op.Key]
				if !ok {
					conflict("hook changed after the backup; restore would overwrite newer changes")
					break
				}
				undo := elementOp{Key: op.Key, Event: op.Event, Index: i, Before: doc.events[op.Event][i]}
				if op.Index >= 0 { // an update: put the old value back
					undo.Value = op.Before
				}
				f.ops = append(f.ops, undo)
			}
			if change.Action == "conflict" {
				break
			}
			restoreOwnership(op.Key)
		}
		if !p.Blocked {
			after, positions, err := doc.edit(f.ops, nil, false)
			if err != nil {
				return nil, err
			}
			f.after = after
			for key, i := range positions {
				if r, ok := p.state.Records[key]; ok {
					r.Index = i
					p.state.Records[key] = r
				}
			}
		}
	}
	gitRevision := ""
	if !p.Blocked {
		guard, reason, err := s.guardGitRestore(b, f, state)
		if err != nil {
			return nil, err
		}
		if guard != nil {
			f.gitGuard = guard
			p.gitGuards = append(p.gitGuards, guard)
			gitRevision = guard.revision()
		}
		if reason != "" {
			conflict(reason)
		}
	}
	if p.Blocked {
		f.ops, f.write, f.remove, f.keys = nil, nil, false, nil
		f.include, f.gitSection = nil, nil
		p.state = ledger{Version: 1, Records: state.Records, Created: state.Created, NewFiles: state.NewFiles, Dirs: state.Dirs}
	}
	p.Changes = append(p.Changes, change)
	p.files = []*filePlan{f}
	p.Revision = digest([]byte(id + f.section + gitRevision + digest(stateBytes) + digest(source.bytes)))
	return p, nil
}

func mergeKeys[T any](a, b map[string]T) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

// Restore creates a new backup before reverting. The source is unchanged, so a
// later sync may reapply it.
func (s *Service) Restore(id, revision string) (*Result, error) {
	lock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()
	if err := s.recoverPending(); err != nil {
		return nil, err
	}
	p, err := s.PreviewRestore(id)
	if err != nil {
		return nil, err
	}
	if revision != "" && p.Revision != revision {
		return nil, ErrStaleRevision
	}
	if p.Blocked {
		return &Result{Plan: p, Applied: []string{}, BackupIDs: []string{}}, ErrConflict
	}
	return s.applyScoped(p, nil)
}
