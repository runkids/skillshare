package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (s *Service) Apply(ctx context.Context, r Request, revision string) (*Result, error) {
	if revision == "" {
		return nil, fmt.Errorf("preview the plugin changes before applying")
	}
	// Serialize CLI and UI mutations across processes, not just HTTP requests.
	if err := os.MkdirAll(filepath.Dir(s.ConfigPath), 0755); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(s.ConfigPath+".plugins.lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("another plugin operation is running (lock: %s.plugins.lock)", s.ConfigPath)
	}
	_ = lock.Close()
	defer os.Remove(s.ConfigPath + ".plugins.lock")
	p, err := s.Preview(ctx, r)
	if err != nil {
		return nil, err
	}
	if p.Revision != revision {
		return nil, fmt.Errorf("plugin state changed since preview; preview again")
	}
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
	result := &Result{Results: []Outcome{}}
	changed := false
	for _, c := range p.Changes {
		if c.Action == "noop" {
			result.Results = append(result.Results, Outcome{Name: c.Name, Target: c.Target, Status: "unchanged"})
			if r.Action == "sync" && d.packages[c.Name].Bindings[c.Target].Pending != "" {
				changed = true
				pack := d.packages[c.Name]
				b := pack.Bindings[c.Target]
				b.Pending = ""
				pack.Bindings[c.Target] = b
				d.packages[c.Name] = pack
			}
			continue
		}
		changed = true
		pack := d.packages[c.Name]
		if pack.Bindings == nil {
			pack.Bindings = map[string]Binding{}
		}
		b := c.Binding
		b.Pending = c.Action
		if c.Action == "import" || c.Action == "selection" {
			b.Pending = ""
		}
		pack.Bindings[c.Target] = b
		d.packages[c.Name] = pack
	}
	if !changed {
		return result, nil
	}
	if err := s.save(d); err != nil {
		return result, err
	}
	var failures []error
	for _, c := range p.Changes {
		if c.Action == "noop" {
			continue
		}
		b := d.packages[c.Name].Bindings[c.Target]
		applyErr := s.applyChange(ctx, c, b)
		outcome := Outcome{Name: c.Name, Target: c.Target, Status: "installed", Message: "Native installation recorded. Reload the Agent and complete any required login or hook trust."}
		if applyErr != nil {
			outcome.Status = "failed"
			outcome.Message = applyErr.Error()
			failures = append(failures, applyErr)
		} else {
			pack := d.packages[c.Name]
			if c.Action == "remove" || c.Action == "forget" {
				delete(pack.Bindings, c.Target)
				outcome.Status = "removed"
				outcome.Message = "Native plugin removed; shared marketplaces are retained."
			} else {
				b.Pending = ""
				pack.Bindings[c.Target] = b
				if c.Action == "selection" {
					outcome.Status = "saved"
					outcome.Message = "Sync selection saved. Run sync to apply installation changes."
				}
				if c.Action == "uninstall" {
					outcome.Status = "excluded"
					outcome.Message = "Removed from this target; plugin definition retained."
				}
				if c.Action == "import" {
					outcome.Status = "imported"
					outcome.Message = "Existing installation adopted without changing its enabled state."
				}
			}
			if len(pack.Bindings) == 0 {
				delete(d.packages, c.Name)
			} else {
				d.packages[c.Name] = pack
			}
			if err := s.save(d); err != nil {
				outcome.Status = "failed"
				outcome.Message = "Native operation completed, but recording its result failed; inspect status before retrying."
				failures = append(failures, err)
			}
		}
		result.Results = append(result.Results, outcome)
	}
	return result, errors.Join(failures...)
}
