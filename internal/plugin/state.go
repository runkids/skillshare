package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Declarations deliberately cannot encode native IDs or installation progress.
type declaration struct {
	Source    string `yaml:"source,omitempty" json:"source,omitempty"`
	SourceRef string `yaml:"source_ref,omitempty" json:"sourceRef,omitempty"`
	Plugin    string `yaml:"plugin,omitempty" json:"plugin,omitempty"`
	Entry     string `yaml:"entry,omitempty" json:"entry,omitempty"`
	Imported  bool   `yaml:"imported,omitempty" json:"imported,omitempty"`
	Sync      *bool  `yaml:"sync,omitempty" json:"-"`
}
type packageDeclaration struct {
	Source    string                 `yaml:"source,omitempty"`
	SourceRef string                 `yaml:"source_ref,omitempty"`
	Plugin    string                 `yaml:"plugin,omitempty"`
	Entry     string                 `yaml:"entry,omitempty"`
	Bindings  map[string]declaration `yaml:"bindings"`
}
type receipt struct {
	Intent  string  `json:"intent"`
	Store   string  `json:"store"`
	Binding Binding `json:"binding"`
	Action  string  `json:"action,omitempty"`
}
type localBinding struct {
	Applied *receipt `json:"applied,omitempty"`
	Pending *receipt `json:"pending,omitempty"`
	// Legacy is a passive proof candidate, never executable installation state.
	Legacy *receipt `json:"legacy,omitempty"`
	// LegacySeen survives cleanup so unchanged v1 YAML cannot recreate ownership.
	LegacySeen bool `json:"legacySeen,omitempty"`
}
type localState struct {
	Version  int                                `json:"version"`
	Bindings map[string]map[string]localBinding `json:"bindings"`
	Versions map[string]packageVersion          `json:"versions,omitempty"`
}

func (b declaration) fingerprint() string { b.Sync = nil; raw, _ := json.Marshal(b); return hash(raw) }
func (b declaration) runtime() Binding {
	r := Binding{Source: b.Source, SourceRef: b.SourceRef, Plugin: b.Plugin, Entry: b.Entry, Sync: b.Sync, Imported: b.Imported}
	if isNpmSource(b.Source) {
		r.Source = ""
	} // The adapter's npm contract has no snapshot.
	return r
}
func declareBinding(b Binding) declaration {
	return declaration{Source: b.Source, SourceRef: b.SourceRef, Plugin: b.Plugin, Entry: b.Entry, Imported: b.Source == "", Sync: b.Sync}
}
func (s *Service) statePath() string { return filepath.Join(s.managedRoot(), "state.json") }
func (s *Service) operationLockPath() string {
	return filepath.Join(s.managedRoot(), ".operation.lock")
}

// storeIdentity prevents an old receipt from removing a plugin in a new account.
func (s *Service) storeIdentity(target string) string {
	a, account := s.account(target)
	home, _ := os.UserHomeDir()
	dir := a.Dir
	agent := s.agentOf(target)
	if !account {
		switch agent {
		case "pi":
			dir, _ = s.piSettingsPath(target)
		case "opencode":
			dir, _ = s.openCodeConfigPath(target)
		case "claude":
			dir = os.Getenv(accountEnv[agent])
			if dir == "" {
				dir = filepath.Join(home, ".claude")
			}
		case "codex":
			dir = os.Getenv(accountEnv[agent])
			if dir == "" {
				dir = filepath.Join(home, ".codex")
			}
		case "omp":
			dir = ompPluginRoot(home, "omp") + "\x00" + ompDefaultCacheRoot()
		case "cursor", "antigravity":
			dir, _, _ = s.localRoot(target)
		default:
			dir = home
		}
	}
	raw, _ := json.Marshal([]any{agent, dir, a.CLI, s.ProjectRoot})
	return string(raw)
}
func (s *Service) loadState(d *document) error {
	path := s.statePath()
	if err := noSymlink(s.StateDir, path); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	d.stateRaw = raw
	d.state = localState{Version: 1, Bindings: map[string]map[string]localBinding{}}
	if err == nil {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&d.state); err != nil {
			return fmt.Errorf("invalid plugin local state: %w", err)
		}
		if d.state.Version != 1 || d.state.Bindings == nil {
			return fmt.Errorf("unsupported plugin local state version or bindings")
		}
		if dec.Decode(new(any)) != io.EOF {
			return fmt.Errorf("invalid plugin local state document")
		}
		for name, targets := range d.state.Bindings {
			if !namePattern.MatchString(name) {
				return fmt.Errorf("invalid plugin local package")
			}
			for _, b := range targets {
				if b.Legacy != nil && (b.Legacy.Action != "" || b.Legacy.Binding.Pending != "") {
					return fmt.Errorf("invalid passive plugin legacy hint for %s", name)
				}
				for _, r := range []*receipt{b.Applied, b.Pending, b.Legacy} {
					if r == nil {
						continue
					}
					var store []string
					if json.Unmarshal([]byte(r.Store), &store) != nil || len(store) != 4 || !validTargetID(store[0], r.Binding.ID) || r.Intent == "" || r.Action != "" && !slices.Contains([]string{"install", "update", "remove", "uninstall", "import", "forget"}, r.Action) {
						return fmt.Errorf("invalid plugin local receipt for %s", name)
					}

				}
			}
		}
	}
	// A disappeared shared file is not a declaration to remove every local plugin.
	if d.raw == nil && d.hasReceipts() {
		return fmt.Errorf("plugin configuration is missing; restore it before reconciling local installations")
	}
	s.compose(d)
	return nil
}
func (s *Service) compose(d *document) {
	d.packages = map[string]Package{}
	for name, p := range d.intent {
		pack := d.packageWithVersion(name, p)
		for target, decl := range p.Bindings {
			b := decl.runtime()
			local := d.state.Bindings[name][target]
			r := local.Applied
			if r == nil && local.Pending != nil {
				r = local.Pending
			}
			if r != nil {
				b = r.Binding
				b.Sync = decl.Sync
				// Older native-only receipts predate the explicit Imported flag.
				// Managed npm receipts also omit Source, but retain an npm recipe fingerprint.
				imported := r.Binding.Imported || r.Binding.Source == "" && r.Intent == (declaration{Imported: true}).fingerprint()
				b.Imported = decl.Imported || imported
				b.ImportRetained = imported && !decl.Imported && r.Intent != decl.fingerprint()
				b.Cleanup = !imported && r.Intent != decl.fingerprint() && (r.Binding.Source != decl.Source || r.Binding.Plugin != decl.Plugin || r.Binding.Entry != decl.Entry || decl.Imported)
				b.ContextChanged = r.Store != s.storeIdentity(target)
				b.RefChanged = b.Selected() && sameDistribution(r.Binding, decl) && r.Binding.SourceRef != decl.SourceRef
				if local.Pending != nil && local.Pending.Intent == decl.fingerprint() {
					b.Pending = local.Pending.Action
				}
			}
			pack.Bindings[target] = b
		}
		d.packages[name] = pack
	}
	for name, targets := range d.state.Bindings {
		pack := d.packages[name]
		if pack.Bindings == nil {
			pack.Bindings = map[string]Binding{}
		}
		for target, local := range targets {
			if _, ok := pack.Bindings[target]; ok {
				continue
			}
			r := local.Applied
			if r == nil {
				r = local.Pending
			}
			if r == nil {
				continue
			}
			b := r.Binding
			b.Cleanup = true
			b.ContextChanged = r.Store != s.storeIdentity(target)
			pack.Bindings[target] = b
		}
		if len(pack.Bindings) > 0 {
			d.packages[name] = pack
		}
	}
}

// UnimportTargets lists local imports backed by this target's shared source recipe.
func (s *Service) UnimportTargets(name string) ([]string, error) {
	d, err := s.load()
	if err != nil {
		return nil, err
	}
	var targets []string
	for target := range d.packages[name].Bindings {
		if d.canUnimport(name, target) {
			targets = append(targets, target)
		}
	}
	slices.Sort(targets)
	return targets, nil
}

func (d *document) canUnimport(name, target string) bool {
	decl, declared := d.intent[name].Bindings[target]
	b := d.packages[name].Bindings[target]
	return declared && !decl.Imported && decl.Source != "" && b.Imported && b.ID != ""
}

func (s *Service) saveState(d *document) error {
	if !filepath.IsAbs(s.StateDir) {
		return fmt.Errorf("plugin state directory must be absolute")
	}
	path := s.statePath()
	if err := noSymlink(s.StateDir, path); err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !bytes.Equal(current, d.stateRaw) {
		return fmt.Errorf("plugin local state changed; preview again")
	}
	data, err := json.MarshalIndent(d.state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if bytes.Equal(current, data) {
		return nil
	}
	if err := os.MkdirAll(s.managedRoot(), 0700); err != nil {
		return err
	}
	if err := atomicNativeWrite(s.StateDir, path, data, 0600); err != nil {
		return err
	}
	d.stateRaw = data
	return nil
}
func (d *document) putLocal(name, target string, b localBinding) {
	if b.Applied == nil && b.Pending == nil && b.Legacy == nil && !b.LegacySeen {
		delete(d.state.Bindings[name], target)
		if len(d.state.Bindings[name]) == 0 {
			delete(d.state.Bindings, name)
		}
		return
	}
	if d.state.Bindings[name] == nil {
		d.state.Bindings[name] = map[string]localBinding{}
	}
	d.state.Bindings[name][target] = b
}
func (d *document) checkIntent(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !bytes.Equal(raw, d.raw) {
		return fmt.Errorf("configuration changed; preview again")
	}
	return nil
}

// checkApplyIntent is also called at adapter write boundaries after materialization.
func (s *Service) checkApplyIntent() error {
	if s.applyConfig == nil {
		return nil
	}
	return (&document{raw: *s.applyConfig}).checkIntent(s.ConfigPath)
}

// captureLegacy runs before intent mutation: a later proof must retain the
// original declaration and store even if the shared target is removed or replaced.
func (s *Service) captureLegacy(d *document) {
	for name, p := range d.legacy {
		for target, b := range p.Bindings {
			local := d.state.Bindings[name][target]
			if local.LegacySeen {
				continue
			}
			local.LegacySeen = true
			if local.Applied == nil && local.Legacy == nil {
				b.Pending = ""
				local.Legacy = &receipt{Intent: declareBinding(b).fingerprint(), Store: s.storeIdentity(target), Binding: b}
			}
			d.putLocal(name, target, local)
		}
	}
}

// Legacy candidates become receipts only after the same scoped native, snapshot
// and private-registration proofs, independently of today's shared declarations.
// Names optionally limit a read-only projection to its requested packages.
func (s *Service) adoptLegacy(d *document, host func(string) Host, names ...string) {
	prove := func(name, target string, hint *receipt) {
		if len(names) > 0 && !slices.Contains(names, name) {
			return
		}
		local := d.state.Bindings[name][target]
		if local.Applied != nil || hint.Store != s.storeIdentity(target) {
			return
		}
		b := hint.Binding
		h := host(target)
		if h.Error != "" || !slices.ContainsFunc(h.Installed, func(i Installed) bool { return i.ID == b.ID }) {
			return
		}
		if b.Source != "" {
			if !s.ownedSnapshot(b, target) {
				return
			}
			if marketplaceAgent(s.agentOf(target)) {
				_, market, _ := strings.Cut(b.ID, "@")
				if filepath.Clean(h.Marketplaces[market]) != s.snapshotPath(b, target) {
					return
				}
			}
		}
		if b.PiRegistration != "" {
			if _, err := s.readPiRegistration(b.PiRegistration, target, b.ID); err != nil {
				return
			}
		}
		b.Pending = ""
		local.Applied = &receipt{Intent: hint.Intent, Store: hint.Store, Binding: b}
		local.Legacy = nil
		d.putLocal(name, target, local)
	}
	for name, targets := range d.state.Bindings {
		for target, local := range targets {
			if local.Legacy != nil {
				prove(name, target, local.Legacy)
			}
		}
	}
	for name, p := range d.legacy {
		for target, b := range p.Bindings {
			// A captured candidate's context and fingerprint take precedence over current intent.
			if local := d.state.Bindings[name][target]; local.Legacy != nil || local.LegacySeen {
				continue
			}
			prove(name, target, &receipt{Intent: declareBinding(b).fingerprint(), Store: s.storeIdentity(target), Binding: b})
		}
	}
	s.compose(d)
}
func (s *Service) ownedSnapshot(b Binding, target string) bool {
	if b.Source == "" || b.ID == "" || b.Digest == "" {
		return false
	}
	root := s.snapshotPath(b, target)
	if err := noSymlink(s.managedRoot(), root); err != nil {
		return false
	}
	owner, err := os.ReadFile(filepath.Join(root, ".skillshare-owner"))
	if err != nil || string(owner) != b.Source+"\n"+b.Plugin {
		return false
	}
	recorded, err := os.ReadFile(filepath.Join(root, ".skillshare-digest"))
	if err != nil || string(recorded) != b.Digest {
		return false
	}
	actual, err := treeDigest(filepath.Join(root, "content"))
	if err != nil || actual != b.Digest {
		return false
	}
	agent := s.agentOf(target)
	if agent == "pi" || agent == "opencode" {
		disc, err := discoverRoot(filepath.Join(root, "content"), b.Source, b.Entry)
		if err != nil {
			return false
		}
		for _, c := range disc.Candidates {
			if c.Name == b.Plugin {
				path := filepath.Join(root, "content", filepath.FromSlash(c.pathFor(agent)))
				if agent == "opencode" {
					return b.ID == fileURL(filepath.Join(path, c.Entry))
				}
				return b.ID == path
			}
		}
		return false
	}
	return true
}

func sameDistribution(b Binding, decl declaration) bool {
	return !decl.Imported && b.Source == decl.Source && b.Plugin == decl.Plugin && b.Entry == decl.Entry
}

// Native IDs for reviewed source installations are still generated by add's
// existing discovery and adapters, including target-specific nested paths.
func (s *Service) sourceChange(ctx context.Context, name, target string, decl declaration, all bool) (Change, error) {
	p, err := s.Preview(ctx, Request{Action: "add", Name: name, Targets: []string{target}, Source: decl.Source, SourceRef: decl.SourceRef, Plugin: decl.Plugin, Entry: decl.Entry})
	if err != nil {
		return sourceChangeFailure(name, target, all, err)
	}
	if len(p.Changes) != 1 {
		return Change{}, fmt.Errorf("source preview did not resolve one target")
	}
	c := p.Changes[0]
	c.Binding.Sync = decl.Sync
	return c, nil
}

func localImportMessage() string {
	return "Import required on this machine; use plugin import --from TARGET --name NAME with its local native identifier."
}

// declarationForAdd retains the recipe even when the adapter adopts a native ID.
func declarationForAdd(r Request, c Change) declaration {
	if c.recipe != nil {
		return *c.recipe
	}
	if isNpmSource(r.Source) {
		return declaration{Source: r.Source, Sync: c.Binding.Sync}
	}
	b := declareBinding(c.Binding)
	if c.Binding.Source == "" {
		b = declaration{Source: canonicalSource(r.Source), SourceRef: r.SourceRef, Plugin: r.Plugin, Entry: r.Entry, Sync: c.Binding.Sync}
		if b.Plugin == "" {
			b.Plugin = c.Name
		}
	}
	return b
}

// Source errors use the same per-binding failure contract as update --all.
func sourceChangeFailure(name, target string, all bool, err error) (Change, error) {
	if !all {
		return Change{}, err
	}
	key, args := ErrorKey(err)
	return Change{Name: name, Target: target, Action: "skip", batchError: true, Message: err.Error(), MessageKey: key, MessageArgs: args}, nil
}

// Prepare the replacement before scheduling cleanup, so a failed batch target
// keeps its existing local installation and cleanup receipt.
func (s *Service) sourceReplacementChanges(ctx context.Context, r Request, name, target string, decl declaration, declared bool, old Binding, action string) ([]Change, error) {
	old.Cleanup = false
	changes := []Change{{Name: name, Target: target, ID: old.ID, Binding: old, Action: action, Message: "Remove the previous local installation before reconciling current intent.", MessageKey: "plugins.note.replaceLocal"}}
	if r.Action != "remove" && declared && (decl.Sync == nil || *decl.Sync) && !decl.Imported {
		c, err := s.sourceChange(ctx, name, target, decl, r.All)
		if err != nil {
			return nil, err
		}
		if r.All && c.Action == "blocked" {
			c.Action, c.batchError = "skip", true
		}
		if c.batchError {
			return []Change{c}, nil
		}
		changes = append(changes, c)
	}
	return changes, nil
}

// importMatchesRecipe compares package identities, not a dashboard's logical alias.
// Native paths need their manifest identity; their basename may just be index.js.
func (s *Service) importMatchesRecipe(ctx context.Context, target string, installed Installed, decl declaration) (bool, error) {
	if isNpmSource(decl.Source) {
		return sameNpmPackage(installed.ID, decl.Source), nil
	}
	nativeName := installed.Name
	agent := s.agentOf(target)
	switch {
	case marketplaceAgent(agent) || agent == "omp" && strings.Contains(installed.ID, "@"):
		nativeName, _, _ = strings.Cut(installed.ID, "@")
	case isNpmSource(installed.ID):
		nativeName = logicalName(installed.ID)
	case agent == "opencode":
		root, ok := openCodeImportRoot(installed.ID, decl.Entry)
		if !ok {
			return false, nil
		}
		if root != "" {
			nativeName = importManifestName(root, decl, "")
		}
	case agent == "pi":
		settings, err := s.piSettingsPath(target)
		if err != nil {
			return false, err
		}
		scope := "user"
		if s.ProjectRoot != "" {
			scope = "project"
		}
		path := resolvePiSource(installed.ID, filepath.Dir(settings), filepath.Dir(settings), scope).install
		if filepath.IsAbs(path) {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				path = filepath.Dir(path)
				for parent := path; ; parent = filepath.Dir(parent) {
					if info, err := os.Stat(filepath.Join(parent, "package.json")); err == nil && info.Mode().IsRegular() {
						path = parent
						break
					}
					if filepath.Dir(parent) == parent {
						break
					}
				}
			}
			nativeName = importManifestName(path, decl, nativeName)
		}
	}
	if decl.Plugin != "" {
		return nativeName == decl.Plugin, nil
	}
	// Import observes local installations. Do not fetch a remote recipe to infer
	// a missing identity: accepting an alias would also admit unrelated plugins.
	source := canonicalSource(decl.Source)
	info, err := os.Stat(source)
	if err != nil || !info.IsDir() {
		return false, agentError{key: "plugins.error.importRecipeIdentityRequired", message: "This remote recipe has no plugin identity. Set plugin: to the native package name in the shared declaration before importing; import does not download source."}
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return false, err
	}
	disc, err := s.discover(ctx, source, decl.SourceRef, decl.Entry)
	if err != nil {
		return false, err
	}
	for _, candidate := range disc.Candidates {
		if candidate.Name == nativeName && slices.Contains(candidate.Targets, agent) && candidate.Problem == "" {
			return true, nil
		}
	}
	return false, nil
}

// importManifestName reads native package identity without acquiring source.
func importManifestName(path string, decl declaration, fallback string) string {
	if native, err := discoverRoot(path, path, decl.Entry); err == nil {
		for _, candidate := range native.Candidates {
			if candidate.Name == decl.Plugin || decl.Plugin == "" && len(native.Candidates) == 1 {
				return candidate.Name
			}
		}
	}
	return fallback
}
