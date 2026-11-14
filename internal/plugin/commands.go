package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/gofrs/flock"
)

type commandPlan struct {
	Before, After       *commandReceipt
	DestinationRevision string
}

func (s *Service) commandBin() (string, error) {
	if s.ProjectRoot != "" {
		return filepath.Join(s.ProjectRoot, ".skillshare", "bin"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

func (s *Service) commandParents(bin string) error {
	root := s.ProjectRoot
	if root == "" {
		var err error
		root, err = os.UserHomeDir()
		if err != nil {
			return err
		}
	}
	expected, err := s.commandBin()
	if err != nil {
		return err
	}
	if bin != expected {
		return agentError{key: "plugins.error.commandBinChanged", message: "Command bin changed; restore " + bin + " before cleanup", args: map[string]string{"bin": bin}}
	}
	return noSymlink(root, filepath.Join(bin, ".skillshare-plugin-commands.lock"))
}

func cloneCommandReceipt(r *commandReceipt) *commandReceipt {
	if r == nil {
		return nil
	}
	copy := *r
	copy.Links = maps.Clone(r.Links)
	return &copy
}

func commandOwners(local localCommands) map[string][]string {
	owners := maps.Clone(local.Owned)
	if owners == nil {
		owners = map[string][]string{}
	}
	for _, r := range []*commandReceipt{local.Applied, local.Pending} {
		if r != nil {
			for name, path := range r.Links {
				if !slices.Contains(owners[name], path) {
					owners[name] = append(owners[name], path)
				}
			}
		}
	}
	return owners
}

func commandLink(path string) (string, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", true, nil
	}
	link, err := os.Readlink(path)
	return link, true, err
}

// Observe final entries without following foreign or dangling links. Parent checks
// keep the fixed bin from redirecting writes through a linked descendant.
func (s *Service) observeCommands(bin string, local localCommands, after *commandReceipt, removal bool) (string, error) {
	if err := s.commandParents(bin); err != nil {
		return "", err
	}
	owners := commandOwners(local)
	names := map[string]bool{}
	for name := range owners {
		names[name] = true
	}
	if after != nil {
		for name := range after.Links {
			names[name] = true
		}
	}
	observation := map[string]any{}
	var conflicts []error
	for _, name := range slices.Sorted(maps.Keys(names)) {
		link, exists, err := commandLink(filepath.Join(bin, name))
		if err != nil {
			return "", err
		}
		observation[name] = []any{link, exists}
		if exists && !slices.Contains(owners[name], link) && !removal && after != nil && after.Links[name] != "" {
			conflicts = append(conflicts, agentError{key: "plugins.error.commandConflict", message: "Command " + name + " conflicts with an operator entry in " + bin, args: map[string]string{"name": name, "bin": bin}})
		}
	}
	raw, _ := json.Marshal(observation)
	return hash(raw), errors.Join(conflicts...)
}

func (s *Service) composeCommands(d *document) {
	for name, local := range d.state.Commands {
		receipt := local.Applied
		if receipt == nil {
			receipt = local.Pending
		}
		if receipt == nil {
			continue
		}
		pack := d.packages[name]
		if pack.Bindings == nil {
			pack.Bindings = map[string]Binding{}
		}
		b := receipt.Binding
		decl, declared := d.intent[name]
		b.Cleanup = !declared || receipt.Intent != packageFingerprint(decl)
		if local.Pending != nil {
			if local.Pending.Intent == packageFingerprint(decl) {
				b.Pending = local.Pending.Action
			} else {
				b.Pending = "update"
			}
		}
		bin, err := s.commandBin()
		if err != nil || bin != receipt.Bin {
			b.ContextChanged = true
		}
		if _, err := s.observeCommands(receipt.Bin, local, receipt, false); err != nil {
			b.Pending = "blocked"
		} else {
			for command := range receipt.Links {
				if _, exists, _ := commandLink(filepath.Join(receipt.Bin, command)); !exists && b.Pending == "" {
					b.Pending = "install"
				}
			}
		}
		pack.Commands = &CommandInfo{Binding: b, Bin: receipt.Bin, Names: slices.Sorted(maps.Keys(receipt.Links))}
		d.packages[name] = pack
	}
}

// planCommands operates once on the package's default recipe, independently of
// native distribution selection. An existing sync receipt pins reviewed bytes.
func (s *Service) planCommands(ctx context.Context, r Request, d *document) ([]Change, error) {
	if r.nativeOnly || !slices.Contains([]string{"add", "sync", "check", "update", "remove"}, r.Action) || r.Action != "add" && (r.Commands != nil && !*r.Commands || r.Commands == nil && len(r.Targets) > 0 && r.Action != "check" && r.Action != "update") {
		return nil, nil
	}
	bin, err := s.commandBin()
	if err != nil {
		return nil, err
	}
	names := map[string]packageDeclaration{}
	for name, p := range d.intent {
		if r.Name == "" || r.Name == name {
			names[name] = p
		}
	}
	for name := range d.state.Commands {
		if r.Name == "" || r.Name == name {
			if _, ok := names[name]; !ok {
				names[name] = packageDeclaration{}
			}
		}
	}
	var addDiscovery *Discovery
	var addCandidate *Candidate
	if r.Action == "add" {
		if isNpmSource(r.Source) {
			return nil, nil
		}
		addDiscovery, err = s.discover(ctx, r.Source, r.SourceRef, r.Entry)
		if err != nil {
			return nil, err
		}
		for _, c := range addDiscovery.Candidates {
			if c.Name == r.Plugin || r.Plugin == "" && len(addDiscovery.Candidates) == 1 {
				cc := c
				addCandidate = &cc
				break
			}
		}
		if addCandidate == nil {
			return nil, nil
		}
		name := r.Name
		if name == "" {
			name = addCandidate.Name
		}
		p := d.intent[name]
		if p.Source == "" || len(r.Targets) == 0 {
			p = packageDeclaration{Source: addDiscovery.Source, SourceRef: addDiscovery.SourceRef, Plugin: addCandidate.Name, Entry: r.Entry}
		}
		names = map[string]packageDeclaration{name: p}
	}
	changes := []Change{}
	for _, name := range slices.Sorted(maps.Keys(names)) {
		var err error
		recipe := names[name]
		if r.Commands == nil && len(r.Targets) > 0 && r.Action != "add" {
			// CLI filters include package commands only when a selected native
			// binding follows the same effective default recipe, including ref.
			followsDefault := slices.ContainsFunc(r.Targets, func(target string) bool {
				decl, ok := recipe.Bindings[target]
				if r.Action == "update" && r.SourceRef != "" {
					decl.SourceRef = r.SourceRef
				}
				return ok && d.packages[name].Bindings[target].Selected() && decl.fingerprint() == packageFingerprint(recipe)
			})
			if !followsDefault {
				continue
			}
		}
		bin, _ = s.commandBin()
		local := d.state.Commands[name]
		before := local.Applied
		if before == nil {
			before = local.Pending
		}
		remove := r.Action == "remove" || recipe.Source == "" || local.Pending != nil && local.Pending.Action == "remove" && local.Pending.Intent == packageFingerprint(recipe) && r.Action == "sync"
		c := Change{Name: name, Commands: true, Action: "install"}
		cp := &commandPlan{Before: cloneCommandReceipt(before)}
		c.command = cp
		fail := func(err error) {
			c.Action = "skip"
			c.Failed = true
			c.Message = err.Error()
			c.MessageKey, c.MessageArgs = ErrorKey(err)
			if c.MessageKey == "" {
				c.MessageKey, c.MessageArgs = "plugins.error.commandFailure", map[string]string{"detail": err.Error()}
			}
		}
		if remove {
			if before == nil {
				continue
			}
			c.Action = "remove"
			c.Binding = before.Binding
			bin = before.Bin
		} else if isNpmSource(recipe.Source) {
			continue
		} else {
			localRef := ""
			if r.Action == "update" && r.SourceRef != "" {
				if info, statErr := os.Stat(recipe.Source); statErr == nil && info.IsDir() {
					// A ref for an alternate native distribution does not affect a
					// commandless local default. Inspect its current metadata first.
					localRef = r.SourceRef
				} else {
					recipe.SourceRef = r.SourceRef
				}
			}

			var after *commandReceipt
			if r.Action == "sync" {
				for _, receipt := range []*commandReceipt{local.Pending, local.Applied} {
					if receipt != nil && receipt.Intent == packageFingerprint(recipe) && receipt.Action != "remove" {
						after = cloneCommandReceipt(receipt)
						break
					}
				}
			}
			if after == nil {
				var disc *Discovery
				var candidate *Candidate
				if addDiscovery != nil && recipe.Source == addDiscovery.Source && recipe.SourceRef == addDiscovery.SourceRef && recipe.Plugin == addCandidate.Name {
					disc = addDiscovery
					candidate = addCandidate
				} else {
					if r.Action == "check" || r.Action == "sync" {
						bindings := map[string]Binding{}
						maps.Copy(bindings, d.packages[name].Bindings)
						if before != nil {
							bindings[""] = before.Binding
						}
						for _, b := range bindings {
							if s.sourceUnmoved(ctx, b, recipe.Source, recipe.SourceRef, recipe.Plugin, recipe.Entry) {
								disc = s.discoverSnapshot(ctx, name, recipe.Source, recipe.SourceRef, recipe.Entry)
								break
							}
						}
					}
					if disc == nil {
						disc, err = s.discover(ctx, recipe.Source, recipe.SourceRef, recipe.Entry)
					}
					if err == nil {
						for _, found := range disc.Candidates {
							if found.Name == recipe.Plugin || recipe.Plugin == "" && len(disc.Candidates) == 1 {
								cc := found
								candidate = &cc
								break
							}
						}
					}
				}
				if err != nil {
					fail(err)
				} else if candidate == nil {
					fail(agentError{key: "plugins.error.commandProvider", message: "Command provider " + recipe.Plugin + " no longer exists", args: map[string]string{"name": recipe.Plugin}})
				} else if candidate.CommandProblem != "" {
					fail(agentError{key: candidate.CommandProblemKey, args: candidate.CommandProblemArgs, message: candidate.CommandProblem})
				} else {
					if len(candidate.Commands) == 0 && before == nil {
						continue
					}
					if localRef != "" {
						_, err = s.discover(ctx, recipe.Source, localRef, recipe.Entry)
						fail(err)
						changes = append(changes, c)
						continue
					}
					b := Binding{Source: disc.Source, SourceRef: disc.SourceRef, Commit: disc.Commit, Digest: disc.Digest, Plugin: candidate.Name, Entry: recipe.Entry, Version: candidate.Version}
					after = &commandReceipt{Intent: packageFingerprint(recipe), Bin: bin, Binding: b, Links: map[string]string{}}
					for command, path := range candidate.Commands {
						after.Links[command] = filepath.Join(s.snapshotPath(b, "", name), "content", filepath.FromSlash(candidate.Path), filepath.FromSlash(path))
					}
				}
			}
			if after != nil {
				c.sourceVersion = &after.Binding.Version
			}
			if after != nil && len(after.Links) == 0 {
				remove, c.Action, c.Binding = true, "remove", after.Binding
				after = nil
			}
			if after != nil {
				cp.After = after
				c.Binding = after.Binding
				if before != nil {
					c.Action = "update"
				}
				if before != nil && commandBindingEqual(before.Binding, after.Binding) && maps.Equal(before.Links, after.Links) && local.Pending == nil {
					c.Action = "noop"
					for command := range after.Links {
						if _, exists, _ := commandLink(filepath.Join(bin, command)); !exists {
							c.Action = "install"
						}
					}
				}
			}
		}
		if c.Action != "skip" {
			cp.DestinationRevision, err = s.observeCommands(bin, local, cp.After, remove)
			if err != nil {
				fail(err)
			}
		}
		platform := s.commandPlatform
		if platform == "" {
			platform = runtime.GOOS
		}
		if platform == "windows" {
			fail(agentError{key: "plugins.error.commandsWindows", message: "plugin command publication is unsupported on Windows"})
		}
		if r.Action == "check" && (c.Action == "install" || c.Action == "update" || c.Action == "remove") {
			c.Action = "update-available"
		}
		if c.Message == "" {
			c.Message = fmt.Sprintf("Manage package commands in %s; add this directory to PATH to invoke them.", bin)
			c.MessageKey, c.MessageArgs = "plugins.note.commandsBin", map[string]string{"bin": bin}
		}
		changes = append(changes, c)
	}
	return changes, nil
}

// replaceCommand never replaces an entry unless its exact link text was proved
// by a local receipt. New names use exclusive symlink creation.
func replaceCommand(path, previous, next string, exists bool) error {
	current, present, err := commandLink(path)
	if err != nil {
		return err
	}
	if present != exists || present && current != previous {
		return agentError{key: "plugins.error.commandDestination", message: "Command destination changed: " + path, args: map[string]string{"path": path}}
	}
	if next == "" {
		if !present {
			return nil
		}
		return os.Remove(path)
	}
	if !present {
		return os.Symlink(next, path)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".command-")
	if err != nil {
		return err
	}
	staged := temp.Name()
	_ = temp.Close()
	if err := os.Remove(staged); err != nil {
		return err
	}
	defer os.Remove(staged)
	if err := os.Symlink(next, staged); err != nil {
		return err
	}
	current, present, err = commandLink(path)
	if err != nil {
		return err
	}
	if !present || current != previous {
		return agentError{key: "plugins.error.commandDestination", message: "Command destination changed: " + path, args: map[string]string{"path": path}}
	}
	return os.Rename(staged, path)
}

// finishCommandRotations resolves retained cleanup before its original proofs change.
func (s *Service) finishCommandRotations(name string, local localCommands) error {
	resolved := map[string]bool{}
	for _, receipt := range []*commandReceipt{local.Applied, local.Pending} {
		if receipt == nil {
			continue
		}
		snapshot := s.snapshotPath(receipt.Binding, "", name)
		if resolved[snapshot] {
			continue
		}
		resolved[snapshot] = true
		if err := noSymlink(s.managedRoot(), snapshot); err != nil {
			return err
		}
		if _, err := os.Stat(snapshot + ".previous"); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		currentOwned := false
		for _, proof := range []*commandReceipt{local.Applied, local.Pending} {
			if proof != nil && s.snapshotPath(proof.Binding, "", name) == snapshot && s.ownedCommandSnapshot(name, proof.Binding) {
				currentOwned = true
			}
		}
		if !currentOwned {
			return agentError{key: "plugins.error.commandSnapshot", message: "Unfinished command snapshot replacement; inspect before retrying"}
		}
		finish := s.commandFinish
		if finish == nil {
			finish = finishSnapshot
		}
		if err := finish(snapshot, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) applyCommands(ctx context.Context, c Change, d *document) (out Outcome, resultErr error) {
	out = Outcome{Name: c.Name, Commands: true, Status: "failed"}
	defer func() {
		if resultErr != nil {
			out.Status = "failed"
			out.Message = resultErr.Error()
			out.MessageKey, out.MessageArgs = ErrorKey(resultErr)
			if out.MessageKey == "" {
				out.MessageKey, out.MessageArgs = "plugins.error.commandFailure", map[string]string{"detail": resultErr.Error()}
			}
			if c.Action == "skip" {
				out.MessageKey, out.MessageArgs = c.MessageKey, c.MessageArgs
			}
		}
	}()
	if c.Action == "skip" {
		return out, errors.New(c.Message)
	}
	before, after := c.command.Before, c.command.After
	bin := ""
	if after != nil {
		bin = after.Bin
	} else if before != nil {
		bin = before.Bin
	}
	if err := s.commandParents(bin); err != nil {
		return out, err
	}
	if err := os.MkdirAll(bin, 0755); err != nil {
		return out, err
	}
	lock := flock.New(filepath.Join(bin, ".skillshare-plugin-commands.lock"))
	if err := lock.Lock(); err != nil {
		return out, err
	}
	defer lock.Unlock()
	local := d.state.Commands[c.Name]
	revision, err := s.observeCommands(bin, local, after, c.Action == "remove")
	if err != nil {
		return out, err
	}
	if revision != c.command.DestinationRevision {
		return out, agentError{key: "plugins.error.commandDestination", message: "Command destinations changed; preview again", args: map[string]string{"path": bin}}
	}
	if err := d.checkIntent(s.ConfigPath); err != nil {
		return out, err
	}
	if err := s.finishCommandRotations(c.Name, local); err != nil {
		return out, err
	}
	if c.Action == "noop" {
		out.Status = "unchanged"
		out.Message = c.Message
		out.MessageKey, out.MessageArgs = c.MessageKey, c.MessageArgs
		return out, nil
	}
	pending := cloneCommandReceipt(after)
	if pending == nil {
		pending = cloneCommandReceipt(before)
		pending.Intent = packageFingerprint(d.intent[c.Name])
	}
	pending.Action = c.Action
	// Persist every exact prior proof before replacing the interrupted intent.
	local.Owned = commandOwners(local)
	local.Pending = pending
	d.state.Commands[c.Name] = local
	if err := s.saveState(d); err != nil {
		return out, err
	}
	// Keep both the old snapshot and each actual link until the Applied save succeeds.
	type priorLink struct {
		text      string
		exists    bool
		published string
	}
	journal := map[string]priorLink{}
	snapshot := ""
	defer func() {
		if resultErr != nil {
			for _, name := range slices.Sorted(maps.Keys(journal)) {
				prior := journal[name]
				link, exists, err := commandLink(filepath.Join(bin, name))
				if err == nil && (!exists || link == prior.published) {
					if prior.exists {
						err = replaceCommand(filepath.Join(bin, name), link, prior.text, exists)
					} else if exists {
						err = replaceCommand(filepath.Join(bin, name), link, "", true)
					}
				} else if err == nil {
					err = agentError{key: "plugins.error.commandRollback", message: "Operator changed command during rollback: " + name, args: map[string]string{"name": name}}
				}
				resultErr = errors.Join(resultErr, err)
			}
		}
		if snapshot != "" {
			finish := s.commandFinish
			if finish == nil {
				finish = finishSnapshot
			}
			cleanupErr := finish(snapshot, resultErr == nil)
			if cleanupErr != nil && resultErr == nil {
				saved := d.state.Commands[c.Name]
				saved.Pending = pending
				d.state.Commands[c.Name] = saved
				cleanupErr = errors.Join(cleanupErr, s.saveState(d))
			}
			resultErr = errors.Join(resultErr, cleanupErr)
		}
	}()
	if after != nil {
		snapshot = s.snapshotPath(after.Binding, "", c.Name)
		if err := noSymlink(s.managedRoot(), snapshot); err != nil {
			return out, err
		}

		if _, err := s.materialize(ctx, after.Binding, "", c.Name); err != nil {
			return out, err
		}
	}
	if err := d.checkIntent(s.ConfigPath); err != nil {
		return out, err
	}
	owners := commandOwners(local)
	names := map[string]bool{}
	for name := range owners {
		names[name] = true
	}
	if after != nil {
		for name := range after.Links {
			names[name] = true
		}
	}
	var preserved []string
	for _, name := range slices.Sorted(maps.Keys(names)) {
		path := filepath.Join(bin, name)
		link, exists, err := commandLink(path)
		if err != nil {
			return out, err
		}
		if exists && !slices.Contains(owners[name], link) {
			if after == nil || after.Links[name] == "" {
				preserved = append(preserved, name)
				continue
			}
			return out, agentError{key: "plugins.error.commandConflict", message: "Command " + name + " was replaced by an operator in " + bin, args: map[string]string{"name": name, "bin": bin}}
		}
		next := ""
		if after != nil {
			next = after.Links[name]
		}
		if exists && link == next || !exists && next == "" {
			continue
		}
		publish := s.commandPublish
		if publish == nil {
			publish = replaceCommand
		}
		if err := publish(path, link, next, exists); err != nil {
			return out, err
		}
		journal[name] = priorLink{link, exists, next}
	}
	if after == nil {
		delete(d.state.Commands, c.Name)
	} else {
		applied := cloneCommandReceipt(after)
		applied.Action = ""
		d.state.Commands[c.Name] = localCommands{Applied: applied}
	}
	if err := s.saveState(d); err != nil {
		d.state.Commands[c.Name] = local
		return out, agentError{key: "plugins.error.commandRecord", message: "Recording command result failed; pending evidence retained: " + err.Error(), args: map[string]string{"detail": err.Error()}, cause: err}
	}
	out.Status = "installed"
	out.Message = c.Message
	out.MessageKey, out.MessageArgs = c.MessageKey, c.MessageArgs
	if after == nil {
		out.Status = "removed"
		out.Message = "Removed owned package commands."
		out.MessageKey, out.MessageArgs = "plugins.note.commandsRemoved", nil
	}
	if len(preserved) > 0 {
		out.Message += " Preserved operator replacements: " + strings.Join(preserved, ", ") + "."
		out.Guidance = true
		out.MessageKey, out.MessageArgs = "plugins.note.commandsPreserved", map[string]string{"names": strings.Join(preserved, ", ")}
	}
	return out, nil
}

func commandBindingEqual(a, b Binding) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func (s *Service) ownedCommandSnapshot(name string, b Binding) bool {
	return s.ownedSnapshotContent(b, s.snapshotPath(b, "", name))
}
