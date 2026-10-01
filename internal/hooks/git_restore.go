package hooks

import (
	"fmt"
	"strings"
)

// guardGitRestore applies the same effective-name boundary as sync when a
// restore publishes a config file, restores a foreign section or activates an
// include. Helper-only restores do not need Git or activate registrations.
func (s *Service) guardGitRestore(b backupRecord, f *filePlan, state ledger) (*gitGuard, string, error) {
	if b.Target != "git" {
		return nil, "", nil
	}
	wholeFile := false
	for _, records := range []map[string]*record{b.OwnedBefore, b.OwnedAfter} {
		if r := records[fileKey(b.Path)]; r != nil && r.Entry == "" && r.Event == "" {
			wholeFile = true
		}
	}
	publish := wholeFile && b.Before != nil && !f.remove
	activate := f.include != nil && f.include.Add
	restoreSection := f.gitSection != nil && f.gitSection.Restore
	if !publish && !activate && !restoreSection {
		return nil, "", nil
	}
	scope := s.scopedFor(b.Root)
	d, err := scope.gitDestination(b.Root)
	if err != nil {
		return nil, "", err
	}
	snapshot, err := scope.gitSnapshot(d)
	if err != nil {
		return nil, "", err
	}
	guard := &gitGuard{service: scope, destination: d, expected: snapshot}
	var data []byte
	path := b.Path
	switch {
	case publish:
		data = []byte(*b.Before)
	case activate:
		path = scope.resolveGitInclude(gitConfigValue{Origin: b.Path, Value: f.include.Value})
		var exists bool
		data, exists, _, err = safeRead(path)
		if err != nil {
			return nil, "", err
		}
		guard.activationPath, guard.activationHash, guard.activationExists = path, digest(data), exists
	case restoreSection:
		for _, fragment := range f.gitSection.Fragments {
			data = append(data, fragment.Text...)
		}
	}
	for _, r := range state.Records {
		if r.Target == "git" && sameGitPath(r.Path, path) && r.Owner != b.Owner && !ownerGone(r.Owner) {
			return guard, "managed by another Skillshare config: " + r.Owner, nil
		}
	}
	rows, err := scope.gitParseBytes(data)
	if err != nil {
		return nil, "", err
	}
	names := map[string]bool{}
	for _, row := range rows {
		name, field, ok := gitHookKey(row.Key)
		if ok && (field == "command" || field == "event") {
			names[name] = true
		}
	}
	for _, key := range sortedKeys(state.Records) {
		r := state.Records[key]
		name, named := strings.CutPrefix(r.Event, "hook.")
		// Project snapshots already see global config. Global restores also need
		// live managed names from other destinations, including standalone -p
		// projects whose records have no globally declared Root label.
		if scope.ProjectRoot == "" && r.Target == "git" && named && names[name] && !sameGitPath(r.Path, path) && !ownerGone(r.Owner) {
			return guard, fmt.Sprintf("hook.%s is managed in %s; restore would merge managed definitions", name, r.Path), nil
		}
	}
	collisions := snapshot.hooks
	if restoreSection {
		// Sync excludes its own generated file from foreign observations, but a
		// foreign section restore must check its names even after ledger loss.
		current, exists, _, err := safeRead(d.hooksFile)
		if err != nil {
			return nil, "", err
		}
		guard.activationPath, guard.activationHash, guard.activationExists = d.hooksFile, digest(current), exists
		generated, err := scope.gitParseBytes(current)
		if err != nil {
			return nil, "", err
		}
		for _, row := range generated {
			row.Origin = d.hooksFile
			collisions = append(collisions, row)
		}
	}
	for _, row := range collisions {
		name, field, ok := gitHookKey(row.Key)
		if ok && names[name] && (field == "command" || field == "event") && !sameGitPath(row.Origin, path) {
			return guard, fmt.Sprintf("hook.%s is also defined in %s; restore would merge definitions", name, row.Origin), nil
		}
	}
	// An include must not activate an externally edited generated file.
	if activate && len(names) > 0 {
		r, owned := state.Records[fileKey(path)]
		if !owned || r.Owner != b.Owner || r.Hash != digest(data) {
			return guard, "included hooks file is no longer owned and unchanged; restore it first", nil
		}
	}
	return guard, "", nil
}

func (g *gitGuard) revision() string {
	return g.expected.digest() + g.activationPath + g.activationHash + fmt.Sprint(g.activationExists)
}

func (g *gitGuard) checkActivation() error {
	if g.activationPath == "" {
		return nil
	}
	data, exists, _, err := safeRead(g.activationPath)
	if err != nil {
		return err
	}
	if exists != g.activationExists || digest(data) != g.activationHash {
		return fmt.Errorf("included hooks file changed since preview: %w", ErrStaleRevision)
	}
	return nil
}
