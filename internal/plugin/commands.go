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
		return fmt.Errorf("command bin changed; restore %s before cleanup", bin)
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
	owners := map[string][]string{}
	for _, r := range []*commandReceipt{local.Applied, local.Pending} {
		if r != nil {
			for name, path := range r.Links {
				owners[name] = append(owners[name], path)
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
		if exists && !slices.Contains(owners[name], link) && !removal {
			conflicts = append(conflicts, fmt.Errorf("command %s conflicts with an operator entry in %s", name, bin))
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
			b.Pending = local.Pending.Action
		}
		bin, err := s.commandBin()
		if err != nil || bin != receipt.Bin {
			b.ContextChanged = true
		}
		if _, err := s.observeCommands(receipt.Bin, local, receipt, false); err != nil {
			b.Pending = "conflict"
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
	if r.nativeOnly || !slices.Contains([]string{"add", "sync", "check", "update", "remove"}, r.Action) || r.Action != "add" && (r.Commands != nil && !*r.Commands || r.Commands == nil && len(r.Targets) > 0) {
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
		if p.Source == "" {
			p = packageDeclaration{Source: addDiscovery.Source, SourceRef: addDiscovery.SourceRef, Plugin: addCandidate.Name, Entry: r.Entry}
		}
		names = map[string]packageDeclaration{name: p}
	}
	changes := []Change{}
	for _, name := range slices.Sorted(maps.Keys(names)) {
		recipe := names[name]
		bin, _ = s.commandBin()
		local := d.state.Commands[name]
		before := local.Applied
		if before == nil {
			before = local.Pending
		}
		remove := r.Action == "remove" || recipe.Source == "" || local.Pending != nil && local.Pending.Action == "remove" && r.Action == "sync"
		c := Change{Name: name, Commands: true, Action: "install"}
		cp := &commandPlan{Before: cloneCommandReceipt(before)}
		c.command = cp
		fail := func(err error) { c.Action = "skip"; c.Failed = true; c.Message = err.Error() }
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
			if r.Action == "update" && r.SourceRef != "" {
				recipe.SourceRef = r.SourceRef
			}
			// A native preview already discovered moving sources. When it did not,
			// inspect its owned reviewed copy before downloading a commandless recipe.
			if before == nil && r.Action != "add" && s.reviewedWithoutCommands(d, name, recipe) {
				continue
			}

			var after *commandReceipt
			if r.Action == "sync" && before != nil && before.Intent == packageFingerprint(recipe) {
				after = cloneCommandReceipt(before)
				if local.Pending != nil {
					after = cloneCommandReceipt(local.Pending)
				}
			} else {
				var disc *Discovery
				var candidate *Candidate
				if addDiscovery != nil && recipe.Source == addDiscovery.Source && recipe.SourceRef == addDiscovery.SourceRef && recipe.Plugin == addCandidate.Name {
					disc = addDiscovery
					candidate = addCandidate
				} else {
					disc, err = s.discover(ctx, recipe.Source, recipe.SourceRef, recipe.Entry)
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
					if before == nil {
						continue
					}
					fail(err)
				} else if candidate == nil {
					fail(fmt.Errorf("command provider %s no longer exists", recipe.Plugin))
				} else if candidate.CommandProblem != "" {
					fail(errors.New(candidate.CommandProblem))
				} else {
					if len(candidate.Commands) == 0 && before == nil {
						continue
					}
					b := Binding{Source: disc.Source, SourceRef: disc.SourceRef, Commit: disc.Commit, Digest: disc.Digest, Plugin: candidate.Name, Entry: recipe.Entry, Version: candidate.Version}
					after = &commandReceipt{Intent: packageFingerprint(recipe), Bin: bin, Path: candidate.Path, Binding: b, Links: map[string]string{}}
					for command, path := range candidate.Commands {
						after.Links[command] = filepath.Join(s.snapshotPath(b, ""), "content", filepath.FromSlash(candidate.Path), filepath.FromSlash(path))
					}
				}
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
		if runtime.GOOS == "windows" {
			fail(fmt.Errorf("plugin command publication is unsupported on Windows"))
		}
		if r.Action == "check" && (c.Action == "install" || c.Action == "update") {
			c.Action = "update-available"
		}
		if c.Message == "" {
			c.Message = fmt.Sprintf("Manage package commands in %s; add this directory to PATH to invoke them.", bin)
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
		return fmt.Errorf("command destination changed: %s", path)
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
		return fmt.Errorf("command destination changed: %s", path)
	}
	return os.Rename(staged, path)
}

func (s *Service) applyCommands(ctx context.Context, c Change, d *document) (out Outcome, resultErr error) {
	out = Outcome{Name: c.Name, Commands: true, Status: "failed"}
	defer func() {
		if resultErr != nil {
			out.Status = "failed"
			out.Message = resultErr.Error()
		}
	}()
	if c.Action == "skip" {
		return out, errors.New(c.Message)
	}
	if c.command == nil {
		return out, fmt.Errorf("command review missing")
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
		return out, fmt.Errorf("command destinations changed; preview again")
	}
	if err := d.checkIntent(s.ConfigPath); err != nil {
		return out, err
	}
	if c.Action == "noop" {
		out.Status = "unchanged"
		out.Message = c.Message
		return out, nil
	}
	pending := cloneCommandReceipt(after)
	if pending == nil {
		pending = cloneCommandReceipt(before)
	}
	pending.Action = c.Action
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
					err = fmt.Errorf("operator changed command during rollback: %s", name)
				}
				resultErr = errors.Join(resultErr, err)
			}
		}
		if snapshot != "" {
			resultErr = errors.Join(resultErr, finishSnapshot(snapshot, resultErr == nil))
		}
	}()
	if after != nil {
		snapshot = s.snapshotPath(after.Binding, "")
		if err := noSymlink(s.managedRoot(), snapshot); err != nil {
			return out, err
		}
		if _, err := os.Stat(snapshot + ".previous"); err == nil {
			snapshot = ""
			return out, fmt.Errorf("unfinished command snapshot replacement; inspect before retrying")
		}
		if _, err := s.materialize(ctx, after.Binding, ""); err != nil {
			return out, err
		}
		root := filepath.Join(snapshot, "content", filepath.FromSlash(after.Path))
		commands := map[string]string{}
		for name, path := range after.Links {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return out, err
			}
			commands[name] = filepath.ToSlash(rel)
		}
		if err := validateCommands(root, commands); err != nil {
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
			if after == nil {
				preserved = append(preserved, name)
				continue
			}
			return out, fmt.Errorf("command %s was replaced by an operator", name)
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
		return out, fmt.Errorf("recording command result failed; pending evidence retained: %w", err)
	}
	out.Status = "installed"
	out.Message = c.Message
	if after == nil {
		out.Status = "removed"
		out.Message = "Removed owned package commands."
	}
	if len(preserved) > 0 {
		out.Message += " Preserved operator replacements: " + strings.Join(preserved, ", ") + "."
		out.Guidance = true
	}
	return out, nil
}

func commandBindingEqual(a, b Binding) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func (s *Service) ownedCommandSnapshot(b Binding) bool {
	// ownedSnapshot requires a native ID; command receipts intentionally have none.
	path := s.snapshotPath(b, "")
	if err := noSymlink(s.managedRoot(), path); err != nil {
		return false
	}
	owner, err := os.ReadFile(filepath.Join(path, ".skillshare-owner"))
	if err != nil || string(owner) != b.Source+"\n"+b.Plugin {
		return false
	}
	digest, err := treeDigest(filepath.Join(path, "content"))
	return err == nil && b.Digest != "" && digest == b.Digest
}

func (s *Service) reviewedWithoutCommands(d *document, name string, p packageDeclaration) bool {
	key := p.Source + "\x00" + p.SourceRef + "\x00" + p.Entry
	s.reads.sources.mu.Lock()
	_, discovered := s.reads.sources.calls[key]
	s.reads.sources.mu.Unlock()
	if discovered {
		return false
	}
	original := d.intent[name]
	for target, b := range d.packages[name].Bindings {
		if b.Source != original.Source || b.SourceRef != original.SourceRef || b.Plugin != original.Plugin || b.Entry != original.Entry || !s.ownedSnapshot(b, target) {
			continue
		}
		disc, err := discoverRoot(filepath.Join(s.snapshotPath(b, target), "content"), b.Source, b.Entry)
		if err != nil {
			continue
		}
		for _, candidate := range disc.Candidates {
			if candidate.Name == b.Plugin {
				return len(candidate.Commands) == 0 && candidate.CommandProblem == ""
			}
		}
	}
	return false
}
