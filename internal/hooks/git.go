package hooks

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// collectGitRetired resolves old destinations even when all their entries have
// been disabled/removed. An unavailable root preserves every ownership record.
func (pl *planner) collectGitRetired(d *desired, state ledger) {
	if d.git == nil {
		d.git = map[string]gitWant{}
	}
	seen := map[string]bool{}
	for _, key := range sortedKeys(state.Records) {
		r := state.Records[key]
		if r.Target != "git" || r.Owner != pl.source.ConfigPath || seen[r.Root] {
			continue
		}
		seen[r.Root] = true
		scope := pl.s.scopedFor(r.Root)
		if _, err := scope.gitRunner().Run("", nil, "version"); err != nil {
			d.notes = append(d.notes, Change{Target: "git", Root: r.Root, Name: r.Entry, Action: "inactive", Message: "git not found; ownership preserved"})
			d.skipGitRoot(r.Root)
			continue
		}
		dest, err := scope.gitDestination(r.Root)
		if err != nil {
			d.notes = append(d.notes, Change{Target: "git", Root: r.Root, Name: r.Entry, Action: "inactive", Message: err.Error() + "; ownership preserved"})
			d.skipGitRoot(r.Root)
			continue
		}
		if _, ok := d.git[dest.identity]; !ok {
			d.git[dest.identity] = gitWant{destination: dest}
		}
	}
}

func (d *desired) skipGitRoot(root string) {
	if d.gitSkipped == nil {
		d.gitSkipped = map[string]bool{}
	}
	d.gitSkipped[root] = true
}

func (pl *planner) planGit(w gitWant, state ledger) ([]*filePlan, string, error) {
	d := w.destination
	s := pl.s.scopedFor(d.root)
	owner := pl.source.ConfigPath
	snapshot, err := s.gitSnapshot(d)
	if err != nil {
		return nil, "", fmt.Errorf("read Git hook configuration: %w", err)
	}
	guard := &gitGuard{service: s, destination: d, expected: snapshot}
	pl.p.gitGuards = append(pl.p.gitGuards, guard)
	var plans []*filePlan
	wanted := map[string]record{}
	entries := map[string]bool{}
	for _, c := range w.commands {
		key := elementKey(owner, d.identity, d.hooksFile, "hook."+c.name, c.entry, 0)
		wanted[key] = record{Owner: owner, Target: "git", Path: d.hooksFile, Root: d.root, Entry: c.entry, Event: "hook." + c.name, Hash: elementHash(c.command), GitEvents: c.command.Events}
		entries[c.entry] = true
	}
	for _, r := range state.Records {
		if r.Target == "git" && r.Path == d.hooksFile && r.Owner == owner && r.Event != "" {
			entries[r.Entry] = true
		}
	}
	replace := false
	for name := range entries {
		replace = replace || pl.replaces(d.root, name)
	}
	data, exists, _, err := safeRead(d.hooksFile)
	if err != nil {
		return nil, "", err
	}
	if err := checkPath(d.base, d.hooksFile); err != nil {
		return nil, "", err
	}
	current := ""
	if exists {
		current = digest(data)
	}
	f := &filePlan{path: d.hooksFile, target: "git", root: d.root, base: d.base, kind: kindFile, before: data, after: data, exists: exists, mode: 0644, section: current}
	filekey := fileKey(d.hooksFile)
	r, owned := state.Records[filekey]
	blocked, message := false, ""
	switch {
	case owned && r.Owner != owner && (!ownerGone(r.Owner) || !replace):
		blocked, message = true, "managed by another Skillshare config: "+r.Owner
	case owned && r.Owner == owner && current != r.Hash && !replace:
		blocked, message = true, "changed outside Skillshare; explicitly replace an entry to regenerate the whole hooks file"
	case !owned && exists && !replace:
		blocked, message = true, "a hooks file Skillshare does not manage exists here; explicitly replace to adopt or overwrite it"
	}
	if blocked {
		for _, name := range sortedKeys(entries) {
			pl.note("git", d.hooksFile, d.root, name, "conflict", message)
		}
	} else {
		if len(w.commands) == 0 {
			if exists {
				f.remove, f.after = true, nil
			}
			delete(pl.p.state.Records, filekey)
		} else {
			if !exists || current != digest(w.content) {
				f.write, f.after = w.content, w.content
			}
			pl.p.state.Records[filekey] = record{Owner: owner, Target: "git", Path: d.hooksFile, Root: d.root, Hash: digest(w.content)}
		}
		f.keys = append(f.keys, filekey)
		for _, key := range sortedKeys(wanted) {
			next := wanted[key]
			prior, had := state.Records[key]
			action := "add"
			if had {
				if prior.Hash == next.Hash {
					action = "unchanged"
				} else {
					action = "update"
				}
			}
			if !owned && exists {
				if current == digest(w.content) {
					action = "adopt"
				} else {
					action = "update"
				}
			}
			msg := ""
			if owned && current != r.Hash || !owned && exists && current != digest(w.content) {
				action, msg = "update", "regenerates the whole hooks file; outside edits are discarded after backup"
			}
			pl.p.state.Records[key] = next
			f.keys = append(f.keys, key)
			pl.note("git", d.hooksFile, d.root, next.Entry, action, msg)
			for _, event := range next.GitEvents {
				if action != "unchanged" {
					kind := "added"
					if slices.Contains(prior.GitEvents, event) {
						kind = "updated"
					}
					pl.event("git", d.hooksFile, d.root, next.Entry, event, kind)
				}
			}
			for _, event := range prior.GitEvents {
				if !slices.Contains(next.GitEvents, event) {
					pl.event("git", d.hooksFile, d.root, next.Entry, event, "removed")
				}
			}
		}
		for _, key := range sortedKeys(state.Records) {
			prior := state.Records[key]
			if prior.Target == "git" && prior.Owner == owner && prior.Path == d.hooksFile && prior.Event != "" {
				if _, ok := wanted[key]; !ok {
					delete(pl.p.state.Records, key)
					f.keys = append(f.keys, key)
					pl.note("git", d.hooksFile, d.root, prior.Entry, "remove", "")
					for _, event := range prior.GitEvents {
						pl.event("git", d.hooksFile, d.root, prior.Entry, event, "removed")
					}
				}
			}
		}
	}
	plans = append(plans, f)
	// Detect merges with command/event definitions from every visible source.
	sections := map[string]string{}
	for _, c := range w.commands {
		for _, row := range snapshot.hooks {
			if row.Key == "hook."+c.name+".command" || row.Key == "hook."+c.name+".event" {
				otherOwner := false
				for _, r := range state.Records {
					if r.Target == "git" && sameGitPath(r.Path, row.Origin) && r.Owner != owner && !ownerGone(r.Owner) {
						otherOwner = true
					}
				}
				if pl.replaces(d.root, c.entry) && !otherOwner && sameGitPath(row.Origin, d.includeTarget) && gitWritable(d.includeTarget) {
					sections[c.name] = c.entry
				} else {
					msg := fmt.Sprintf("hook.%s is also defined in %s; Git would merge them; remove that section there", c.name, row.Origin)
					if otherOwner {
						msg = "managed by another Skillshare config: " + row.Origin
					}
					pl.note("git", d.hooksFile, d.root, c.entry, "conflict", msg)
				}
			}
			if (row.Key == "hook."+c.name+".enabled" || commandHasEvent(c.command, row.Key)) && gitConfigFalse(row) {
				pl.p.Warnings = append(pl.p.Warnings, fmt.Sprintf("%s is disabled in %s", strings.TrimSuffix(row.Key, ".enabled"), row.Origin))
			}
		}
		if c.command.Parallel != nil && !gitAtLeast(snapshot.version, 55) {
			pl.p.Warnings = append(pl.p.Warnings, "Git 2.55 or later is required for parallel; older Git cannot honor this setting")
		}
	}
	for _, name := range sortedKeys(sections) {
		section, err := s.gitSectionPlan(d, name)
		if err != nil {
			return nil, "", err
		}
		plans = append(plans, section)
		pl.note("git", d.includeTarget, d.root, sections[name], "update", "remove colliding hook."+name+" section")
	}
	present, conditional := false, false
	for _, row := range snapshot.includes {
		if sameGitPath(s.resolveGitInclude(row), d.hooksFile) {
			present = true
			conditional = conditional || row.Conditional
		}
	}
	if conditional {
		pl.p.Warnings = append(pl.p.Warnings, "Git include is conditional; activation depends on its includeIf condition")
	}
	rows, err := s.gitFileIncludes(d.includeTarget)
	if err != nil {
		return nil, "", err
	}
	includeKey := elementKey(owner, d.identity, d.includeTarget, "include.path", "", 0)
	prior, includeOwned := state.Records[includeKey]
	includeValue := ""
	add := len(w.commands) > 0
	if add && !present && gitWritable(d.includeTarget) {
		includeValue = d.includeValue
		pl.p.state.Records[includeKey] = record{Owner: owner, Target: "git", Path: d.includeTarget, Root: d.root, Event: "include.path", Hash: digest([]byte(d.includeValue))}
	} else if !add && includeOwned {
		count := 0
		for _, row := range rows {
			if row.Key == "include.path" && digest([]byte(row.Value)) == prior.Hash {
				includeValue = row.Value
				count++
			}
		}
		if count != 1 {
			pl.note("git", d.includeTarget, d.root, "", "conflict", "owned include changed or duplicated outside Skillshare")
		} else {
			delete(pl.p.state.Records, includeKey)
		}
	}
	inc := gitIncludePlan(s, d, rows, includeValue, add)
	if includeValue != "" {
		inc.keys = []string{includeKey}
		plans = append(plans, inc)
		action := "remove"
		if add {
			action = "add"
		}
		pl.note("git", d.includeTarget, d.root, "", action, "update Skillshare include.path")
	}
	if add && !present && !gitWritable(d.includeTarget) {
		resolved, _ := filepath.EvalSymlinks(d.includeTarget)
		for _, name := range sortedKeys(entries) {
			pl.note("git", d.hooksFile, d.root, name, "inactive", fmt.Sprintf("include target %s is not writable (resolved: %s); add manually:\n%s", d.includeTarget, resolved, includeLines(d.includeValue)))
		}
	}
	if !gitAtLeast(snapshot.version, 54) {
		if pl.p.gitInactive == nil {
			pl.p.gitInactive = map[string]string{}
		}
		pl.p.gitInactive[d.root] = "Git " + snapshot.version + " cannot run config hooks; Git 2.54 or later is required"
		for _, name := range sortedKeys(entries) {
			pl.note("git", d.hooksFile, d.root, name, "inactive", "Git "+snapshot.version+" cannot run config hooks; Git 2.54 or later is required")
		}
	}
	for _, f := range plans {
		f.gitGuard = guard
	}
	return plans, snapshot.digest() + inc.section, nil
}

func gitConfigFalse(row gitConfigValue) bool {
	// A bare key is true in Git; an explicitly empty value is false.
	if !row.HasValue {
		return false
	}
	switch strings.ToLower(row.Value) {
	case "", "false", "no", "off", "0":
		return true
	}
	return false
}

func commandHasEvent(c GitCommand, key string) bool {
	for _, event := range c.Events {
		if key == "hook."+event+".enabled" {
			return true
		}
	}
	return false
}
