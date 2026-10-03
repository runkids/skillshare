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
	if r.SourceRef != "" && r.Action != "add" && r.Action != "update" {
		return fmt.Errorf("source ref is only valid for add or update")
	}
	if r.Entry != "" && r.Action != "add" {
		return fmt.Errorf("entry is only valid for add")
	}
	if r.From != "" && r.Action != "import" {
		return fmt.Errorf("from is only valid for import")
	}
	if r.Plugin != "" && r.Action != "add" && r.Action != "import" {
		return fmt.Errorf("plugin selector is only valid for add or import")
	}
	seen := map[string]bool{}
	targets := s.targets()
	for _, t := range r.Targets {
		if !slices.Contains(targets, t) {
			return fmt.Errorf("unsupported plugin target %q", t)
		}
		if seen[t] {
			return fmt.Errorf("duplicate plugin target %q", t)
		}
		seen[t] = true
	}
	if r.Action == "add" && r.Source == "" {
		return fmt.Errorf("add requires a source")
	}
	if r.Action == "add" && isNpmSource(r.Source) {
		if !validNpmSource(r.Source) {
			return fmt.Errorf("invalid npm source; use npm:<package> or npm:<package>@<version>")
		}
		if r.SourceRef != "" || r.Entry != "" || r.Plugin != "" {
			return fmt.Errorf("an npm source takes no source ref, entry or plugin selector")
		}
		if len(r.Targets) == 0 {
			return fmt.Errorf("npm packages are installed by Pi; choose a Pi target")
		}
	}
	if r.Action == "import" && (!slices.Contains(targets, r.From) || !validTargetID(s.agentOf(r.From), r.Plugin)) {
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
	// ownMarket reports whether the marketplace Skillshare registered for b is still in place.
	ownMarket := func(target string, b Binding) bool {
		agent := s.agentOf(target)
		_, market, _ := strings.Cut(b.ID, "@")
		return (agent == "claude" || agent == "codex") && b.Source != "" && slices.Contains(host(target).ManagedMarketplaces, market)
	}
	// mayOwnMarket also covers an Agent whose marketplace list could not be read, so a removal
	// stays pending instead of dropping the binding before its cleanup.
	mayOwnMarket := func(target string, b Binding) bool {
		agent := s.agentOf(target)
		return ownMarket(target, b) || (agent == "claude" || agent == "codex") && b.Source != "" && host(target).Marketplaces == nil
	}
	appendChange := func(c Change) {
		h := host(c.Target)
		agent := s.agentOf(c.Target)
		if agent == "pi" && c.Binding.PiRegistration != "" && len(c.piRecord) == 0 && (c.Action == "install" || c.Action == "uninstall" || c.Action == "remove" || c.Action == "update") {
			if err := s.preparePiRegistration(ctx, &c); err != nil {
				c.Action, c.Message, c.MessageKey, c.MessageArgs = "blocked", err.Error(), "", nil
			}
		}
		// A change already blocked keeps its own reason; the key always matches the message.
		if h.Error != "" && c.Action != "blocked" {
			c.Action = "blocked"
			c.Message, c.MessageKey, c.MessageArgs = h.Error, h.ErrorKey, nil
		}
		if c.Action != "blocked" && c.Action != "noop" && c.Action != "skip" && c.Action != "import" && c.Action != "update-available" && c.Action != "native-check" {
			if err := s.verifyCommand(ctx, c.Target, c.Action, c.ID); err != nil {
				c.Action = "blocked"
				c.Message = err.Error()
			}
		}
		// An update that cannot reach a target skips it, so the plugin's other Agents still update.
		skip := func(key, message string, args map[string]string) {
			c.Action = "skip"
			c.Message, c.MessageKey, c.MessageArgs = message, key, args
		}
		// An import reinstalls and updates from its native marketplace. Once that is gone, only the
		// native client, or adding the plugin again from a reviewed source, can bring it back.
		if c.Binding.Source == "" && (agent == "claude" || agent == "codex") && (c.Action == "install" || c.Action == "update") && h.Marketplaces != nil {
			_, market, _ := strings.Cut(c.ID, "@")
			if _, ok := h.Marketplaces[market]; !ok {
				skip("plugins.skip.marketplaceGone", fmt.Sprintf("The native marketplace %s is gone. Restore it in the native client, or remove this Agent and add the plugin again from its source.", market), map[string]string{"market": market})
			}
		}
		// Claude reads a skill folder that has a plugin manifest as <name>@skills-dir, and loads
		// only one plugin per name.
		if c.Action == "install" && agent == "claude" && c.Message == "" {
			name, _, _ := strings.Cut(c.ID, "@")
			for _, item := range h.Installed {
				if item.ID == name+"@skills-dir" {
					c.Message = fmt.Sprintf("Claude's skills folder also has a %s plugin; Claude will load this plugin and skip that copy until one of them is renamed or removed.", name)
					c.MessageKey, c.MessageArgs = "plugins.note.skillsDirClash", map[string]string{"name": name}
				}
			}
		}
		if c.Action == "update" && agent == "antigravity-cli" {
			skip("plugins.skip.keepEnablement", "Update in Antigravity CLI to preserve native enablement; automatic reinstall updates are not supported.", map[string]string{"agent": "Antigravity CLI"})
		}
		// pi update keeps an npm package pinned to an exact version.
		if version := pinnedNpm(c.ID); c.Action == "update" && agent == "pi" && version != "" {
			skip("plugins.skip.npmPinned", fmt.Sprintf("Pinned to %s; to use another version, add the package again with that version.", version), map[string]string{"version": version})
		}
		// pi update has no project scope, and OpenCode v1 has no update command.
		if c.Action == "update" && c.Binding.Source == "" && s.ProjectRoot != "" && agent == "pi" || c.Action == "update" && c.Binding.Source == "" && agent == "opencode" && (s.ProjectRoot != "" || !strings.HasPrefix(strings.TrimPrefix(h.Version, "v"), "2.")) {
			skip("plugins.skip.imported", "Update imported packages in the native client; Skillshare updates reviewed source snapshots only.", nil)
		}
		if c.Action == "update" && agent == "opencode" && c.Binding.Source == "" {
			help, err := s.run(ctx, "opencode", "plugin", "update", "--help")
			if err != nil || !strings.Contains(string(help), "update") {
				skip("plugins.skip.opencodeNoUpdate", "Installed OpenCode does not expose a verified plugin update command.", nil)
			}
		}
		if c.Binding.Source == "" && commandTarget(agent) && c.Action == "install" {
			c.Action = "blocked"
			c.Message = "This imported plugin has no reviewed reinstall source; install or update in the native client."
		}
		if c.Binding.Source == "" && commandTarget(agent) && c.Action == "update" {
			skip("plugins.skip.imported", "This imported plugin has no reviewed reinstall source; update it in the native client.", nil)
		}
		if c.Action == "update" && agent == "copilot" {
			for _, item := range h.Installed {
				if item.ID == c.ID && (!item.EnabledKnown || !item.Enabled) {
					skip("plugins.skip.keepEnablement", "Update in Copilot to preserve native enablement.", map[string]string{"agent": "GitHub Copilot CLI"})
				}
			}
		}
		if c.Action == "update" && agent == "codex" && c.Binding.Source == "" {
			// An imported plugin comes from a Codex marketplace, which Codex upgrades together
			// with every plugin installed from it, as it also does when it starts.
			_, market, _ := strings.Cut(c.ID, "@")
			help, err := s.run(ctx, c.Target, "plugin", "marketplace", "--help")
			if err != nil || !strings.Contains(string(help), "upgrade") {
				skip("plugins.skip.codexNoUpgrade", "Installed Codex cannot upgrade marketplaces; update the plugin in Codex.", nil)
			} else {
				c.Message = fmt.Sprintf("Upgrades the Codex marketplace %s, which reinstalls every plugin installed from it.", market)
				c.MessageKey, c.MessageArgs = "plugins.note.codexUpgrade", map[string]string{"market": market}
			}
		}
		if c.Action == "update" && agent == "codex" && c.Binding.Source != "" {
			for _, item := range h.Installed {
				// Codex's add, which updates the plugin, always enables it.
				if item.ID == c.ID && !item.Enabled {
					skip("plugins.skip.codexDisabled", "Enable the plugin in Codex before updating it; updating would turn it back on.", nil)
				}
			}
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
	if r.Action == "add" && isNpmSource(r.Source) {
		name := r.Name
		if name == "" {
			if name = logicalName(r.Source); name == "" {
				return nil, fmt.Errorf("use --name to choose a package name for this npm source")
			}
		}
		others := []string{}
		for other := range d.packages {
			if other != name {
				others = append(others, other)
			}
		}
		slices.Sort(others)
		for _, target := range slices.Compact(slices.Sorted(slices.Values(r.Targets))) {
			owner := ""
			for _, other := range others {
				if b, ok := d.packages[other].Bindings[target]; ok && sameNpmPackage(b.ID, r.Source) {
					owner = other
					break
				}
			}
			old, bound := d.packages[name].Bindings[target]
			c := s.npmChange(name, target, r.Source, owner, old, bound, host(target).Installed)
			// An adopted entry with resource filters keeps them, as an import does. Installing
			// another version over one keeps them too, so its new entry is recorded after the install.
			filtered := func(id string) bool {
				return slices.ContainsFunc(host(target).Installed, func(i Installed) bool { return i.Filtered && (i.ID == id || sameNpmPackage(i.ID, id)) })
			}
			if c.Action == "import" && filtered(c.ID) {
				if err := s.preparePiRegistration(ctx, &c); err != nil {
					c.Action, c.Message = "blocked", err.Error()
				}
			}
			c.piRecordAfter = c.Action == "install" && filtered(c.ID)
			appendChange(c)
		}
	} else if r.Action == "add" {
		discovered, err := DiscoverOptions(ctx, r.Source, r.SourceRef, r.Entry)
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
		pack, exists := d.packages[name]
		// The recorded source is where Agents are bound from later. It does not stop one Agent
		// from using another distribution under the same name.
		otherSource := pack.Source != "" && (pack.Source != discovered.Source || pack.Plugin != c.Name || pack.SourceRef != discovered.SourceRef)
		if len(r.Targets) == 0 {
			// Only record the package; Agents are bound later from the same source.
			change := Change{Name: name, Action: "record", Components: c.Components, Message: "Add to Skillshare only; choose Agents later.", Binding: Binding{Source: discovered.Source, SourceRef: discovered.SourceRef, Plugin: c.Name, Entry: r.Entry, Version: c.Version}}
			if slices.Contains(c.Targets, "codex") {
				change.Logo = c.TargetInfo["codex"].Logo
			}
			if c.Problem != "" || len(c.Targets) == 0 {
				change.Action = "blocked"
				change.Message = c.Problem
				if change.Message == "" {
					change.Message = "No Agent can use this plugin."
				}
			} else if otherSource {
				change.Action = "blocked"
				change.Message = "Package already uses a different source; choose another name or remove it first."
			} else if exists {
				change.Action = "noop"
			}
			p.Blocked = change.Action == "blocked"
			p.Changes = append(p.Changes, change)
		}
		for _, target := range slices.Compact(slices.Sorted(slices.Values(r.Targets))) {
			agent := s.agentOf(target)
			// Name the plugin so native marketplace lists show what Skillshare added.
			market := "skillshare-" + marketUnsafe.ReplaceAllString(c.Name, "-") + "-" + hash([]byte(s.ConfigPath + "\x00" + discovered.Source + "\x00" + c.Name + "\x00" + target))[:16]
			b := Binding{ID: c.Name + "@" + market, Source: discovered.Source, SourceRef: discovered.SourceRef, Commit: discovered.Commit, Plugin: c.Name, Digest: discovered.Digest, Version: c.TargetInfo[agent].Version, Components: c.TargetInfo[agent].Components}
			switch agent {
			case "cursor", "antigravity", "antigravity-cli", "copilot", "grok", "kimi", "hermes", "devin":
				b.ID = c.Name
			case "pi":
				b.ID = filepath.Join(s.snapshotPath(b, target), "content", filepath.FromSlash(c.pathFor(agent)))
			case "opencode":
				b.Entry = c.Entry
				b.ID = fileURL(filepath.Join(s.snapshotPath(b, target), "content", filepath.FromSlash(c.pathFor(agent)), c.Entry))
			}
			change := Change{Name: name, Target: target, ID: b.ID, Binding: b, Action: "install", Components: b.Components}
			if c.Problem != "" || !slices.Contains(c.Targets, agent) {
				change.Action = "blocked"
				change.Message = c.Problem
				if info, ok := c.TargetInfo[agent]; ok && info.Problem != "" {
					change.Message = info.Problem
				}
				if change.Message == "" {
					change.Message = "No native manifest for this target; plugin components will not be silently converted or omitted."
				}
			}
			if old, ok := d.packages[name].Bindings[target]; ok {
				if old.Source == b.Source && old.Plugin == b.Plugin && old.SourceRef == b.SourceRef {
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
		if agent := s.agentOf(r.From); agent == "cursor" || agent == "antigravity" {
			return nil, fmt.Errorf("Local directory plugins can be added from source; importing existing or marketplace installations is not supported")
		}
		if installed.Filtered && s.agentOf(r.From) != "pi" {
			return nil, fmt.Errorf("this native entry has resource filters or options; manage it in the native client to preserve those settings")
		}
		b := Binding{ID: installed.ID, Version: installed.Version}
		if old, ok := d.packages[name].Bindings[r.From]; ok {
			if old.ID != b.ID {
				return nil, fmt.Errorf("package already manages a different plugin")
			}
			b = old
		}
		change := Change{Name: name, Target: r.From, ID: b.ID, Binding: b, Action: "import", Message: "Record the existing native installation; leave its content and enabled state unchanged."}
		if installed.Filtered {
			if err := s.preparePiRegistration(ctx, &change); err != nil {
				return nil, err
			}
		}
		appendChange(change)
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
			for _, target := range s.targets() {
				b, ok := d.packages[name].Bindings[target]
				if !ok || (len(r.Targets) > 0 && !slices.Contains(r.Targets, target)) {
					continue
				}
				agent := s.agentOf(target)
				c := Change{Name: name, Target: target, ID: b.ID, Binding: b, Action: r.Action}
				_, exists := find(target, b.ID)
				switch r.Action {
				case "sync":
					if !b.Selected() {
						c.Action = "uninstall"
						if !exists && !ownMarket(target, b) {
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
					if c.Action == "install" && exists && b.PiRegistration == "" {
						c.Action = "noop"
					}
					if c.Action == "remove" && !exists && host(target).Error == "" && !mayOwnMarket(target, b) {
						c.Action = "forget"
					}
					if c.Action == "install" && b.Source == "" {
						c.Message = "Reinstall using its existing native marketplace."
					}
				case "remove":
					if !exists && host(target).Error == "" && !mayOwnMarket(target, b) {
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
					ref := b.SourceRef
					if r.SourceRef != "" {
						ref = r.SourceRef
					}
					discoveryKey := b.Source + "\x00" + ref + "\x00" + b.Entry
					disc := discoveries[discoveryKey]
					if disc == nil {
						disc, err = DiscoverOptions(ctx, b.Source, ref, b.Entry)
						if err != nil {
							return nil, err
						}
						discoveries[discoveryKey] = disc
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
					if candidate.Problem != "" || !slices.Contains(candidate.Targets, agent) {
						c.Action = "blocked"
						c.Message = "Updated source no longer provides a compatible native plugin"
						break
					}
					c.Components = candidate.TargetInfo[agent].Components
					if disc.Digest == b.Digest && b.Pending == "" && ref == b.SourceRef {
						c.Action = "noop"
					} else if r.Action == "check" {
						c.Action = "update-available"
						c.Message = "Source content changed; review an update before applying."
						// Check is read-only; the binding carries the source's version so the change can show old → new.
						c.Binding.Version = candidate.TargetInfo[agent].Version
					} else {
						c.Binding.Digest = disc.Digest
						c.Binding.Commit = disc.Commit
						c.Binding.SourceRef = disc.SourceRef
						c.Binding.Version = candidate.TargetInfo[agent].Version
						c.Binding.Components = candidate.TargetInfo[agent].Components
					}
				}
				if (c.Action == "uninstall" || c.Action == "remove") && !exists && ownMarket(target, b) {
					c.Message, c.MessageKey = "The plugin is already gone; remove the marketplace Skillshare registered for it.", "plugins.note.marketplaceCleanup"
				}
				if c.Action == "install" && b.Source == "" && (agent == "antigravity" || agent == "cursor") {
					c.Action = "blocked"
					c.Message = "Imported installation has no reinstall source; install it in the native client, then sync again."
				}
				if c.Action == "forget" || c.Action == "selection" {
					p.Changes = append(p.Changes, c)
				} else {
					appendChange(c)
				}
			}
			if r.Action == "remove" && len(r.Targets) == 0 && len(d.packages[name].Bindings) == 0 {
				p.Changes = append(p.Changes, Change{Name: name, Action: "forget", Message: "Remove from Skillshare; no Agent has it."})
			}
		}
	}

	// A native installation can have only one owner in this configuration.
	for i := range p.Changes {
		c := &p.Changes[i]
		if c.Target == "" {
			continue
		}
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
