package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

func hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func (s *Service) managedRoot() string {
	p, _ := filepath.Abs(s.ConfigPath)
	return filepath.Join(s.StateDir, "plugins", hash([]byte(p))[:16])
}

func (s *Service) validate(r Request) error {
	switch r.Action {
	case "add", "import", "sync", "remove", "update", "check", "enable", "disable":
	default:
		return fmt.Errorf("unknown plugin action %q", r.Action)
	}
	if r.Name != "" && !namePattern.MatchString(r.Name) {
		return fmt.Errorf("invalid package name")
	}
	if r.Source != "" && r.Action != "add" {
		return fmt.Errorf("source is only valid for add")
	}
	if r.From != "" && r.Action != "import" {
		return fmt.Errorf("from is only valid for import")
	}
	if r.Plugin != "" && r.Action != "add" && r.Action != "import" {
		return fmt.Errorf("plugin selector is only valid for add or import")
	}
	seen := map[string]bool{}
	for _, t := range r.Targets {
		if !slices.Contains(Targets, t) {
			return fmt.Errorf("unsupported plugin target %q", t)
		}
		if seen[t] {
			return fmt.Errorf("duplicate plugin target %q", t)
		}
		seen[t] = true
	}
	if r.Action == "add" && (r.Source == "" || len(r.Targets) == 0) {
		return fmt.Errorf("add requires a source and at least one target")
	}
	if r.Action == "import" && (!slices.Contains(Targets, r.From) || !validTargetID(r.From, r.Plugin)) {
		return fmt.Errorf("import requires --from <target> and a native plugin identifier")
	}
	if (r.Action == "remove" || r.Action == "update" || r.Action == "enable" || r.Action == "disable") && r.Name == "" {
		return fmt.Errorf("%s requires a package name", r.Action)
	}
	return nil
}

func (s *Service) Preview(ctx context.Context, r Request) (*Plan, error) {
	r.Targets = slices.Clone(r.Targets)
	for i, target := range r.Targets {
		r.Targets[i] = NormalizeTarget(target)
	}
	r.From = NormalizeTarget(r.From)
	if err := s.validate(r); err != nil {
		return nil, err
	}
	d, err := s.load()
	if err != nil {
		return nil, err
	}
	p := &Plan{Changes: []Change{}}
	hosts := map[string]Host{}
	host := func(target string) Host {
		if h, ok := hosts[target]; ok {
			return h
		}
		h := s.host(ctx, target)
		hosts[target] = h
		return h
	}
	appendChange := func(c Change) {
		h := host(c.Target)
		if h.Error != "" {
			c.Action = "blocked"
			c.Message = h.Error
		}
		if c.Action != "blocked" && c.Action != "noop" && c.Action != "import" && c.Action != "update-available" && c.Action != "native-check" {
			if err := s.verifyCommand(ctx, c.Target, c.Action, c.ID); err != nil {
				c.Action = "blocked"
				c.Message = err.Error()
			}
		}
		if c.Action == "update" && c.Binding.Source == "" && (c.Target == "pi" || c.Target == "opencode") {
			c.Action = "blocked"
			c.Message = "Update imported packages in the native client; Skillshare updates reviewed source snapshots only."
		}
		if c.Action == "blocked" {
			p.Blocked = true
		}
		p.Changes = append(p.Changes, c)
	}
	find := func(target, id string) (Installed, bool) {
		for _, i := range host(target).Installed {
			if i.ID == id {
				return i, true
			}
		}
		return Installed{}, false
	}
	if r.Action == "add" {
		discovered, err := Discover(ctx, r.Source)
		if err != nil {
			return nil, err
		}
		var candidate *Candidate
		for _, c := range discovered.Candidates {
			if c.Name == r.Plugin || (r.Plugin == "" && len(discovered.Candidates) == 1) {
				cc := c
				candidate = &cc
				break
			}
		}
		if candidate == nil {
			return nil, fmt.Errorf("choose a plugin from this source using --plugin")
		}
		c := *candidate
		name := r.Name
		if name == "" {
			name = c.Name
		}
		for _, target := range slices.Compact(slices.Sorted(slices.Values(r.Targets))) {
			market := "skillshare-" + hash([]byte(s.ConfigPath + "\x00" + discovered.Source + "\x00" + c.Name + "\x00" + target))[:16]
			b := Binding{ID: c.Name + "@" + market, Source: discovered.Source, Plugin: c.Name, Digest: discovered.Digest, Version: c.Version, Components: c.Components}
			switch target {
			case "cursor", "antigravity":
				b.ID = c.Name
			case "pi":
				b.ID = filepath.Join(s.snapshotPath(b, target), "content", filepath.FromSlash(c.Path))
			case "opencode":
				b.ID = fileURL(filepath.Join(s.snapshotPath(b, target), "content", filepath.FromSlash(c.Path), c.Entry))
			}
			change := Change{Name: name, Target: target, ID: b.ID, Binding: b, Action: "install", Components: c.Components}
			if c.Problem != "" || !slices.Contains(c.Targets, target) {
				change.Action = "blocked"
				change.Message = c.Problem
				if change.Message == "" {
					change.Message = "No native manifest for this target; plugin components will not be silently converted or omitted."
				}
			}
			if old, ok := d.packages[name].Bindings[target]; ok {
				if old.Source == b.Source && old.Plugin == b.Plugin {
					change.Binding = old
					change.ID = old.ID
					if _, ok := find(target, old.ID); ok {
						change.Action = "noop"
					}
				} else {
					change.Action = "blocked"
					change.Message = "Package already has a different binding for this target; choose another name or remove it first."
				}
			}
			if c.Marketplace != "" && change.Action == "install" {
				if native, ok := find(target, c.Name+"@"+c.Marketplace); ok {
					change.Action = "import"
					change.ID = native.ID
					change.Binding = Binding{ID: native.ID, Version: native.Version}
					change.Message = "Already installed natively; adopt without reinstalling."
				}
			}
			if _, ok := find(target, change.ID); ok && change.Action == "install" {
				change.Action = "blocked"
				change.Message = "A native installation already uses this identity; import it explicitly instead of replacing it."
			}
			appendChange(change)
		}
	} else if r.Action == "import" {
		installed, ok := find(r.From, r.Plugin)
		if !ok {
			return nil, fmt.Errorf("plugin is not installed in this scope, or native inventory is unavailable")
		}
		name := r.Name
		if name == "" {
			name = installed.Name
			if name == "" {
				name, _, _ = strings.Cut(installed.ID, "@")
			}
			if !namePattern.MatchString(name) {
				return nil, fmt.Errorf("use --name to choose a logical package name for this native identifier")
			}
		}
		if r.From == "cursor" || r.From == "antigravity" {
			return nil, fmt.Errorf("Local directory plugins can be added from source; importing existing or marketplace installations is not supported")
		}
		if installed.Filtered {
			return nil, fmt.Errorf("this native entry has resource filters or options; manage it in the native client to preserve those settings")
		}
		b := Binding{ID: installed.ID, Version: installed.Version}
		if old, ok := d.packages[name].Bindings[r.From]; ok {
			if old.ID != b.ID {
				return nil, fmt.Errorf("package already manages a different plugin")
			}
			b = old
		}
		appendChange(Change{Name: name, Target: r.From, ID: b.ID, Binding: b, Action: "import", Message: "Record the existing native installation; leave its content and enabled state unchanged."})
	} else {
		if r.Name != "" {
			if _, ok := d.packages[r.Name]; !ok {
				return nil, fmt.Errorf("plugin package %q not found", r.Name)
			}
		}
		names := make([]string, 0, len(d.packages))
		for name := range d.packages {
			names = append(names, name)
		}
		slices.Sort(names)
		discoveries := map[string]*Discovery{}
		for _, name := range names {
			if r.Name != "" && r.Name != name {
				continue
			}
			for _, target := range Targets {
				b, ok := d.packages[name].Bindings[target]
				if !ok || (len(r.Targets) > 0 && !slices.Contains(r.Targets, target)) {
					continue
				}
				c := Change{Name: name, Target: target, ID: b.ID, Binding: b, Action: r.Action}
				_, exists := find(target, b.ID)
				switch r.Action {
				case "sync":
					if !b.Selected() {
						c.Action = "uninstall"
						if !exists {
							c.Action = "noop"
						}
						break
					}
					c.Action = b.Pending
					if c.Action == "" {
						c.Action = "install"
						if exists {
							c.Action = "noop"
						}
					}
					if c.Action == "install" && exists {
						c.Action = "noop"
					}
					if c.Action == "remove" && !exists && host(target).Error == "" {
						c.Action = "forget"
					}
					if c.Action == "install" && b.Source == "" {
						c.Message = "Reinstall using its existing native marketplace."
					}
				case "remove":
					if !exists && host(target).Error == "" {
						c.Action = "forget"
					}
				case "enable", "disable":
					selected := r.Action == "enable"
					c.Binding.Sync = &selected
					c.Binding.Pending = ""
					c.Action = "selection"
					c.Message = "Save sync selection only. Run sync to install or remove this managed target."
				case "check", "update":
					if !b.Selected() {
						c.Action = "noop"
						break
					}
					if b.Source == "" {
						if r.Action == "check" {
							c.Action = "native-check"
							c.Message = "Imported installation: use the native marketplace to check releases."
						}
						break
					}
					disc := discoveries[b.Source]
					if disc == nil {
						disc, err = Discover(ctx, b.Source)
						if err != nil {
							return nil, err
						}
						discoveries[b.Source] = disc
					}
					var candidate *Candidate
					for _, x := range disc.Candidates {
						if x.Name == b.Plugin {
							xx := x
							candidate = &xx
						}
					}
					if candidate == nil {
						return nil, fmt.Errorf("plugin %s no longer exists in source", b.Plugin)
					}
					if candidate.Problem != "" || !slices.Contains(candidate.Targets, target) {
						c.Action = "blocked"
						c.Message = "Updated source no longer provides a compatible native plugin"
						break
					}
					c.Components = candidate.Components
					if disc.Digest == b.Digest && b.Pending == "" {
						c.Action = "noop"
					} else if r.Action == "check" {
						c.Action = "update-available"
						c.Message = "Source content changed; review an update before applying."
					} else {
						c.Binding.Digest = disc.Digest
						c.Binding.Version = candidate.Version
						c.Binding.Components = candidate.Components
					}
				}
				if c.Action == "install" && b.Source == "" && (target == "antigravity" || target == "cursor") {
					c.Action = "blocked"
					c.Message = "Imported installation has no reinstall source; install it in the native client, then sync again."
				}
				if c.Action == "forget" || c.Action == "selection" {
					p.Changes = append(p.Changes, c)
				} else {
					appendChange(c)
				}
			}
		}
	}

	// A native installation can have only one owner in this configuration.
	for i := range p.Changes {
		c := &p.Changes[i]
		for name, pack := range d.packages {
			if name != c.Name && pack.Bindings[c.Target].ID == c.ID {
				c.Action = "blocked"
				c.Message = "This native plugin is already managed by package " + name
				p.Blocked = true
			}
		}
	}
	// Bind previews to source bytes, selected operations, native versions/inventory,
	// and config bytes. A changed selection or external edit invalidates approval.
	fingerprint, _ := json.Marshal(struct {
		Request Request
		Raw     string
		Changes []Change
		Hosts   map[string]Host
	}{r, string(d.raw), p.Changes, hosts})
	p.Revision = hash(fingerprint)
	return p, nil
}
