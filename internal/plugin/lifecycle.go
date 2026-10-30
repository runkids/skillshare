package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

func (s *Service) Apply(ctx context.Context, r Request, revision string) (*Result, error) {
	if revision == "" {
		return nil, fmt.Errorf("preview the plugin changes before applying")
	}
	unlock, err := s.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	p, err := s.Preview(ctx, r)
	if err != nil {
		return nil, err
	}
	if p.Revision != revision {
		return nil, fmt.Errorf("plugin state changed since preview; preview again")
	}
	return s.applyPlan(ctx, r, p)
}

// Lock serializes CLI and UI mutations across processes, not just HTTP requests.
func (s *Service) Lock() (unlock func(), err error) {
	path := s.operationLockPath()
	if err := noSymlink(s.StateDir, path); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("another plugin operation is running (lock: %s)", path)
	}
	_ = lock.Close()
	return func() { _ = os.Remove(path) }, nil
}

// applyPlan persists declarations once, then records all native work privately.
func (s *Service) applyPlan(ctx context.Context, r Request, p *Plan) (*Result, error) {
	if p.Blocked {
		return nil, fmt.Errorf("resolve blocked plugin targets before applying")
	}
	if r.Action == "check" {
		return nil, fmt.Errorf("check is read-only")
	}
	d, err := s.load()
	if err != nil {
		return nil, err
	}
	// Reuse the inventory already observed by this apply's fresh Preview.
	s.adoptLegacy(d, func(target string) Host {
		if h, ok := p.hosts[target]; ok {
			return h
		}
		return s.host(ctx, target)
	})
	before, _ := yaml.Marshal(d.intent)
	for _, c := range p.Changes {
		if c.Action == "skip" || c.Action == "blocked" {
			continue
		}
		pack := d.intent[c.Name]
		if pack.Bindings == nil {
			pack.Bindings = map[string]declaration{}
		}
		switch r.Action {
		case "add":
			decl := declarationForAdd(r, c)
			if old, exists := pack.Bindings[c.Target]; exists {
				decl.Sync = old.Sync
			}
			if c.Target == "" || pack.Source == "" {
				pack.Source, pack.SourceRef, pack.Plugin, pack.Entry = decl.Source, decl.SourceRef, decl.Plugin, decl.Entry
			}
			if c.Target != "" {
				pack.Bindings[c.Target] = decl
			}
			d.intent[c.Name] = pack
		case "import":
			// Importing an existing declaration records only local evidence.
			if old, exists := pack.Bindings[c.Target]; !exists {
				pack.Bindings[c.Target] = declaration{Imported: true, Sync: old.Sync}
				d.intent[c.Name] = pack
			}
		case "enable", "disable":
			b := pack.Bindings[c.Target]
			selected := r.Action == "enable"
			b.Sync = &selected
			pack.Bindings[c.Target] = b
			d.intent[c.Name] = pack
		case "remove":
			if len(r.Targets) == 0 {
				delete(d.intent, c.Name)
			} else {
				delete(pack.Bindings, c.Target)
				if len(pack.Bindings) == 0 && pack.Source == "" {
					delete(d.intent, c.Name)
				} else {
					d.intent[c.Name] = pack
				}
			}
		case "update":
			if r.SourceRef != "" && c.Target != "" {
				b := pack.Bindings[c.Target]
				b.SourceRef = r.SourceRef
				pack.Bindings[c.Target] = b
				d.intent[c.Name] = pack
			}
		}
	}
	after, _ := yaml.Marshal(recordPackageVersions(d, r, p))
	if !bytes.Equal(before, after) {
		// Preserve verified receipts and unresolved proof candidates before rewriting legacy YAML.
		if d.hasLocalState() {
			if err := s.saveState(d); err != nil {
				return nil, err
			}
		}
		if err := s.saveAndPruneVersions(d); err != nil {
			return nil, err
		}
	}
	s.applyConfig = &d.raw
	defer func() { s.applyConfig = nil }()
	result := &Result{Results: []Outcome{}}
	failures := []error{}
	failed := map[string]bool{}
	restores := map[string]piRestoreReceipt{}
	for _, c := range p.Changes {
		key := c.Name + "\x00" + c.Target
		if failed[key] {
			continue
		}
		outcome := Outcome{Name: c.Name, Target: c.Target, Guidance: c.Guidance}
		if c.Action == "skip" {
			outcome.Status = "skipped"
			if c.batchError {
				outcome.Status, failed[key] = "failed", true
				failures = append(failures, errors.New(c.Message))
			}
			outcome.Message, outcome.MessageKey, outcome.MessageArgs = c.Message, c.MessageKey, c.MessageArgs
			result.Results = append(result.Results, outcome)
			continue
		}
		if c.Action == "selection" || c.Target == "" || c.ID == "" && c.Action == "forget" {
			outcome.Status = "saved"

			if c.Action == "forget" {
				outcome.Status = "removed"
				outcome.Message = "Removed from Skillshare."
				for target, local := range d.state.Bindings[c.Name] {
					if c.Target == "" || c.Target == target {
						local.Legacy = nil
						d.putLocal(c.Name, target, local)
					}
				}
			}
			result.Results = append(result.Results, outcome)
			continue
		}
		local := d.state.Bindings[c.Name][c.Target]
		decl, declared := d.intent[c.Name].Bindings[c.Target]

		if c.Action == "noop" {
			if r.Action == "add" && !c.Binding.Imported && local.Applied != nil && c.Binding.ID == local.Applied.Binding.ID {
				local.Applied.Intent, local.Applied.Binding.Imported = decl.fingerprint(), c.Binding.Imported
				local.Legacy = nil
				d.putLocal(c.Name, c.Target, local)
			}
			if local.Pending != nil && local.Applied == nil && c.Binding.Selected() && !c.Binding.Cleanup && local.Pending.Intent == decl.fingerprint() {
				h := s.host(ctx, c.Target)
				if h.Error != "" || !slices.ContainsFunc(h.Installed, func(i Installed) bool { return i.ID == c.ID }) || c.Binding.Source != "" && !s.ownedSnapshot(c.Binding, c.Target) {
					return result, fmt.Errorf("pending installation could not be verified; preview again")
				}
				local.Applied = local.Pending
				local.Applied.Action = ""
			}
			if local.Pending != nil {
				local.Pending = nil
				d.putLocal(c.Name, c.Target, local)
			}
			outcome.Status = "unchanged"
			outcome.Message, outcome.MessageKey, outcome.MessageArgs = c.Message, c.MessageKey, c.MessageArgs
			result.Results = append(result.Results, outcome)
			continue
		}
		if err := d.checkIntent(s.ConfigPath); err != nil {
			return result, errors.Join(append(failures, err)...)
		}
		if err := s.savePiRegistration(c); err != nil {
			return result, err
		}
		intent := decl.fingerprint()
		if !declared && local.Applied != nil {
			intent = local.Applied.Intent
		}
		b := c.Binding
		b.Cleanup, b.ContextChanged, b.RefChanged, b.ImportRetained = false, false, false, false
		local.Pending = &receipt{Intent: intent, Store: s.storeIdentity(c.Target), Binding: b, Action: c.Action}
		d.putLocal(c.Name, c.Target, local)
		if err := s.saveState(d); err != nil {
			return result, errors.Join(append(failures, err)...)
		}
		if err := d.checkIntent(s.ConfigPath); err != nil {
			return result, errors.Join(append(failures, err)...)
		}
		c.piRestores = restores
		applyErr := s.applyChange(ctx, c, &b)
		if applyErr != nil {
			outcome.Status, outcome.Message = "failed", applyErr.Error()
			outcome.MessageKey, outcome.MessageArgs = ErrorKey(applyErr)
			failures = append(failures, applyErr)
			failed[key] = true
		} else {
			local.Pending = nil
			local.Legacy = nil
			b.Pending = ""
			outcome.Status, outcome.Message = "installed", "Native installation recorded. Reload the Agent and complete any required login or hook trust."
			if c.Action == "remove" || c.Action == "forget" {
				local.Applied = nil
				outcome.Status = "removed"
				outcome.Message = "Native plugin removed; shared marketplaces are retained."
			} else {
				if c.piRecordAfter {
					if err := s.recordPiEntry(ctx, c.Target, &b); err != nil {
						outcome.Message = "Installed, but its extension settings could not be recorded to restore on a later reinstall: " + err.Error()
					}
				}
				local.Applied = &receipt{Intent: intent, Store: s.storeIdentity(c.Target), Binding: b}
				if c.Action == "uninstall" {
					outcome.Status = "excluded"
					outcome.Message = "Removed from this target; plugin definition retained."
				}
				if c.Action == "import" {
					outcome.Status = "imported"
					outcome.Message = "Existing installation adopted without changing its enabled state."
				}
			}
			if s.agentOf(c.Target) == "omp" && (c.Action == "remove" || c.Action == "forget" || c.Action == "uninstall") {
				outcome.Message, outcome.MessageKey = "Removed from this scope; shared cache, marketplaces and plugin settings were retained.", "plugins.outcome.ompRemoved"
			}
			d.putLocal(c.Name, c.Target, local)
			if err := s.saveState(d); err != nil {
				outcome.Status = "failed"
				outcome.Message = "Native operation completed, but recording its result failed; inspect status before retrying."
				result.Results = append(result.Results, outcome)
				return result, errors.Join(append(failures, err)...)
			}
		}
		result.Results = append(result.Results, outcome)
	}
	// This also persists verified migration receipts on an otherwise unchanged sync.
	if d.hasLocalState() || d.stateRaw != nil {
		if err := s.saveState(d); err != nil {
			return result, errors.Join(append(failures, err)...)
		}
	}
	return result, errors.Join(failures...)
}
