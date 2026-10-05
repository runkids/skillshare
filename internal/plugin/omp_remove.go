package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"skillshare/internal/utils"
)

// OMP's native uninstall deletes shared cache. This adapter removes only the
// selected scope's registration, runtime selection and verified link. Cache,
// marketplaces and plugin settings stay intact, including on partial retries.
type ompRemoval struct {
	root, id, version, cacheRoot, link, target, revision string
	files                                                []*ompRemovalFile
	linkPresent                                          bool
}
type ompRemovalFile struct {
	path       string
	raw, after []byte
	mode       os.FileMode
	body       map[string]json.RawMessage
}

var ompRuntimeName = regexp.MustCompile(`^(?:@[a-zA-Z0-9][a-zA-Z0-9._~-]*/)?[a-zA-Z0-9][a-zA-Z0-9._~-]*$`)

func ompRemovalJSON(path string) (*ompRemovalFile, error) {
	if err := ompSafePath(path); err != nil {
		return nil, err
	}
	f := &ompRemovalFile{path: path, body: map[string]json.RawMessage{}}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return f, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("OMP metadata is not a readable regular file")
	}
	f.mode = info.Mode().Perm()
	f.raw, err = os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !json.Valid(f.raw) || !piLossless(f.raw) || jsonDuplicateKeys(f.raw) || json.Unmarshal(f.raw, &f.body) != nil || f.body == nil {
		return nil, fmt.Errorf("OMP metadata must be an unambiguous JSON object")
	}
	return f, nil
}
func ompRemovalObject(f *ompRemovalFile, key string) (map[string]json.RawMessage, error) {
	m := map[string]json.RawMessage{}
	if raw, ok := f.body[key]; ok && (json.Unmarshal(raw, &m) != nil || m == nil) {
		return nil, fmt.Errorf("invalid OMP %s map", key)
	}
	return m, nil
}
func ompRemovalPrune(f *ompRemovalFile, key, name string) error {
	m, err := ompRemovalObject(f, key)
	if err != nil {
		return err
	}
	if _, exists := m[name]; !exists {
		return nil
	}
	delete(m, name)
	f.body[key], err = json.Marshal(m)
	if err != nil {
		return err
	}
	f.after, err = json.MarshalIndent(f.body, "", "  ")
	f.after = append(f.after, '\n')
	return err
}
func ompRemovalPackage(cache, fallback string) (string, *ompRemovalFile, error) {
	pkg, err := ompRemovalJSON(filepath.Join(cache, "package.json"))
	if err != nil {
		return "", nil, err
	}
	name := fallback
	if raw, ok := pkg.body["name"]; ok && (json.Unmarshal(raw, &name) != nil || name == "") {
		return "", nil, fmt.Errorf("invalid OMP runtime package name")
	}
	if len(name) > 214 || !ompRuntimeName.MatchString(name) {
		return "", nil, fmt.Errorf("unsafe OMP runtime package name")
	}
	return name, pkg, nil
}

func (s *Service) ompRemovalPlan(b Binding, cacheRoot string) (*ompRemoval, error) {
	if b.Version == "" {
		b.Version = "0.0.0"
	}
	name, market, ok := strings.Cut(b.ID, "@")
	if !ok || !validOMPName(name) || !validOMPName(market) || b.Version == "" || filepath.Base(b.Version) != b.Version || strings.ContainsAny(b.Version, "\\/\r\n\x00") || cacheRoot == "" {
		return nil, fmt.Errorf("OMP installation identity cannot be verified")
	}
	root := filepath.Dir(filepath.Dir(cacheRoot))
	scope := "user"
	if s.ProjectRoot != "" {
		root = filepath.Join(s.ProjectRoot, ".omp", "plugins")
		scope = "project"
	}
	p := &ompRemoval{root: root, id: b.ID, version: b.Version, cacheRoot: cacheRoot}
	reg, err := ompRemovalJSON(filepath.Join(root, "installed_plugins.json"))
	if err != nil {
		return nil, err
	}
	if len(reg.raw) != 0 && string(reg.body["version"]) != "2" {
		return nil, fmt.Errorf("unsupported OMP installed registry version")
	}
	plugins, err := ompRemovalObject(reg, "plugins")
	if err != nil {
		return nil, err
	}
	cache := ompCachePath(cacheRoot, b.ID, b.Version)
	// Even when native registration is already absent, the retained cache proves
	// which runtime key/link a partially completed removal still owns.
	module, pkg, err := ompRemovalPackage(cache, name)
	if err != nil {
		return nil, err
	}
	p.files = append(p.files, reg, pkg)
	cfg, err := ompRemovalJSON(filepath.Join(root, "omp-plugins.lock.json"))
	if err != nil {
		return nil, err
	}
	runtimePlugins, err := ompRemovalObject(cfg, "plugins")
	if err != nil {
		return nil, err
	}
	// A missing manifest cannot prove the runtime name. Recover it only from
	// one lock key whose scope-local link targets this exact retained cache.
	matched := false
	runtimeNames := map[string]string{}
	for key := range runtimePlugins {
		if len(key) > 214 || !ompRuntimeName.MatchString(key) {
			return nil, fmt.Errorf("unsafe OMP runtime package name")
		}
		link := filepath.Join(root, "node_modules", filepath.FromSlash(key))
		if err := ompSafePath(filepath.Dir(link)); err != nil {
			return nil, err
		}
		target, err := os.Readlink(link)
		if err != nil {
			continue
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(link), target)
		}
		target = filepath.Clean(target)
		if _, exists := runtimeNames[target]; exists {
			runtimeNames[target] = ""
		} else {
			runtimeNames[target] = key
		}
		if target != cache {
			continue
		}
		if matched || pkg.body["name"] != nil && module != key {
			return nil, fmt.Errorf("OMP runtime ownership is ambiguous")
		}
		module, matched = key, true
	}
	if pkg.body["name"] == nil && !matched {
		_, registered := plugins[b.ID]
		_, selected := runtimePlugins[module]
		_, cacheErr := os.Lstat(cache)
		if !registered && !selected && os.IsNotExist(cacheErr) {
			// Native uninstall already removed this installation. Forget only the
			// Skillshare binding; another plugin's node_modules is not our state.
			p.files = append(p.files, cfg)
			return p, nil
		}
		_, err := os.Lstat(filepath.Join(root, "node_modules"))
		if registered || !os.IsNotExist(err) {
			return nil, fmt.Errorf("OMP runtime name cannot be verified without its manifest or link")
		}
	}
	for id, raw := range plugins {
		var entries []struct{ Scope, InstallPath, Version string }
		if json.Unmarshal(raw, &entries) != nil || len(entries) != 1 {
			return nil, fmt.Errorf("ambiguous OMP installed entries")
		}
		e := entries[0]
		if id == b.ID {
			if e.Scope != scope || filepath.Clean(e.InstallPath) != cache || e.Version != b.Version {
				return nil, fmt.Errorf("OMP installation changed; import its current version before removing")
			}
			continue
		}
		other, _, valid := strings.Cut(id, "@")
		if !valid || !filepath.IsAbs(e.InstallPath) || ompSafePath(e.InstallPath) != nil {
			return nil, fmt.Errorf("another OMP runtime owner cannot be verified")
		}
		otherName, otherPkg, err := ompRemovalPackage(e.InstallPath, other)
		if err != nil {
			return nil, err
		}
		p.files = append(p.files, otherPkg)
		if otherPkg.body["name"] == nil {
			otherName = runtimeNames[filepath.Clean(e.InstallPath)]
			if otherName == "" {
				return nil, fmt.Errorf("another OMP runtime owner cannot be verified")
			}
		}
		if strings.EqualFold(module, otherName) {
			return nil, fmt.Errorf("OMP runtime package has another owner")
		}
	}
	manifest, err := ompRemovalJSON(filepath.Join(root, "package.json"))
	if err != nil {
		return nil, err
	}
	deps, err := ompRemovalObject(manifest, "dependencies")
	if err != nil {
		return nil, err
	}
	for dep := range deps {
		if strings.EqualFold(dep, module) {
			return nil, fmt.Errorf("OMP runtime package is also an npm dependency")
		}
	}
	p.files = append(p.files, cfg, manifest)
	if err := ompRemovalPrune(reg, "plugins", b.ID); err != nil {
		return nil, err
	}
	if err := ompRemovalPrune(cfg, "plugins", module); err != nil {
		return nil, err
	}
	p.link = filepath.Join(root, "node_modules", filepath.FromSlash(module))
	if err := ompSafePath(filepath.Dir(p.link)); err != nil {
		return nil, err
	}
	info, err := os.Lstat(p.link)
	if err == nil {
		if !utils.IsLinkMode(p.link, info.Mode()) {
			return nil, fmt.Errorf("OMP runtime path is not a link; preserve its files before removing")
		}
		p.target, err = os.Readlink(p.link)
		if err != nil {
			return nil, err
		}
		resolved := p.target
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(filepath.Dir(p.link), resolved)
		}
		if filepath.Clean(resolved) != cache {
			return nil, fmt.Errorf("OMP runtime link belongs to another installation")
		}
		p.linkPresent = true
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	var fingerprint bytes.Buffer
	for _, f := range p.files {
		fmt.Fprintf(&fingerprint, "%s\x00%o", f.path, f.mode)
		fingerprint.WriteByte(0)
		fingerprint.Write(f.raw)
		fingerprint.WriteByte(0)
	}
	fmt.Fprintf(&fingerprint, "%s\x00%s\x00%t", p.link, p.target, p.linkPresent)
	p.revision = hash(fingerprint.Bytes())
	return p, nil
}

func (s *Service) prepareOMPRemoval(h Host, c *Change) {
	if strings.TrimPrefix(h.Version, "omp/") != "18.6.1" || h.ompCacheRoot == "" {
		c.Action = "blocked"
		c.Message = "OMP removal requires version 18.6.1 and a verified native root."
	} else {
		p, err := s.ompRemovalPlan(c.Binding, h.ompCacheRoot)
		if err != nil {
			c.Action = "blocked"
			c.Message = "Cannot safely remove this OMP installation: " + err.Error()
		} else if len(p.files[0].after) == 0 && slices.ContainsFunc(h.Installed, func(i Installed) bool { return i.ID == c.ID }) {
			c.Action, c.Message = "blocked", "OMP inventory and native registration disagree; preview again."
		} else {
			for _, f := range p.files {
				if len(f.after) == 0 {
					continue
				}
				// The existing private-state ACL check prevents temporary copies of
				// native settings becoming public on Windows. Never repair ACLs here.
				if err := checkPiRegistrationPrivate(filepath.Dir(f.path)); err != nil {
					c.Action, c.Message, c.MessageKey = "blocked", err.Error(), "plugins.error.ompRemovalSafety"
					return
				}
				if err := checkPiRegistrationPrivate(f.path); err != nil {
					c.Action, c.Message, c.MessageKey = "blocked", err.Error(), "plugins.error.ompRemovalSafety"
					return
				}
			}
			changed := p.linkPresent || len(p.files[0].after) != 0 || len(p.files[len(p.files)-2].after) != 0
			if changed {
				c.ompRemoval = p
				if c.Action == "forget" {
					c.Action = "remove"
				}
			} else if c.Action == "uninstall" {
				c.Action = "noop"
			}
			c.Message, c.MessageKey = "Removes this scope's registration and runtime link. Shared cache, marketplaces and plugin settings are retained for other projects and later reinstalls.", "plugins.note.ompRemoval"
			return
		}
	}
	c.MessageKey, c.MessageArgs = "plugins.error.ompRemovalSafety", nil
}

func (s *Service) applyOMPRemoval(p *ompRemoval) error {
	// ponytail: native plugin writers do not take this lock; revisions catch observed
	// edits, not an atomic native CAS. Concurrent native management needs an upstream transaction.
	release, err := ompAcquireLock(filepath.Join(p.root, "installed_plugins.json"))
	if err != nil {
		return err
	}
	defer release()
	current, err := s.ompRemovalPlan(Binding{ID: p.id, Version: p.version}, p.cacheRoot)
	if err != nil {
		return err
	}
	if current.revision != p.revision {
		return fmt.Errorf("OMP native files changed; preview removal again")
	}
	root, err := os.OpenRoot(p.root)
	if err != nil {
		return err
	}
	defer root.Close()
	// Write only pruned maps, then unlink last. Retained cache makes interruption
	// recoverable from the binding's identity without copying private native settings.
	for _, f := range p.files {
		if len(f.after) == 0 {
			continue
		}
		name, err := filepath.Rel(p.root, f.path)
		if err != nil {
			return err
		}
		if err := checkPiRegistrationPrivate(filepath.Dir(f.path)); err != nil {
			return err
		}
		if err := checkPiRegistrationPrivate(f.path); err != nil {
			return err
		}
		info, err := root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("OMP metadata path changed during removal")
		}
		before, err := root.ReadFile(name)
		if err != nil || !bytes.Equal(before, f.raw) {
			return fmt.Errorf("OMP native file changed during removal; preview again")
		}
		if err := rootAtomicWrite(root, name, f.after, f.mode, false); err != nil {
			return err
		}
	}
	if p.linkPresent {
		name, err := filepath.Rel(p.root, p.link)
		if err != nil {
			return err
		}
		target, err := root.Readlink(name)
		if err != nil || target != p.target {
			return fmt.Errorf("OMP runtime link changed during removal; inspect it before retrying")
		}
		// Remove the link itself, never recurse into its target (including junctions).
		if err := root.Remove(name); err != nil {
			return err
		}
	}
	return nil
}
