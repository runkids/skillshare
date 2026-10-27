package plugin

// packageVersion is private reviewed metadata for a package's default source.
type packageVersion struct {
	Intent  string `json:"intent"`
	Version string `json:"version"`
}

func packageFingerprint(p packageDeclaration) string {
	return (declaration{Source: p.Source, SourceRef: p.SourceRef, Plugin: p.Plugin, Entry: p.Entry}).fingerprint()
}

func (d *document) packageWithVersion(name string, p packageDeclaration) Package {
	pack := Package{Source: p.Source, SourceRef: p.SourceRef, Plugin: p.Plugin, Entry: p.Entry, Bindings: map[string]Binding{}}
	if v, ok := d.state.Versions[name]; ok && v.Intent == packageFingerprint(p) {
		pack.Version = v.Version
	} else if legacy, ok := d.legacy[name]; ok && packageFingerprint(packageDeclaration{Source: legacy.Source, SourceRef: legacy.SourceRef, Plugin: legacy.Plugin, Entry: legacy.Entry}) == packageFingerprint(p) {
		pack.Version = legacy.Version
	}
	return pack
}

func (d *document) hasLocalState() bool {
	return len(d.state.Bindings) > 0 || len(d.state.Versions) > 0
}

func (s *Service) captureLegacyAndVersions(d *document) {
	s.captureLegacy(d)
	for name, p := range d.legacy {
		if p.Version != "" {
			if v, exists := d.state.Versions[name]; !exists || v.Intent != packageFingerprint(d.intent[name]) {
				d.putPackageVersion(name, d.intent[name], p.Version)
			}
		}
	}
}

func (d *document) putPackageVersion(name string, p packageDeclaration, version string) {
	if d.state.Versions == nil {
		d.state.Versions = map[string]packageVersion{}
	}
	d.state.Versions[name] = packageVersion{Intent: packageFingerprint(p), Version: version}
}

// recordPackageVersions records only a source observation for the actual default recipe.
func recordPackageVersions(d *document, r Request, plan *Plan) map[string]packageDeclaration {
	if r.Action != "add" && r.Action != "update" && r.Action != "sync" {
		return d.intent
	}
	for _, c := range plan.Changes {
		if c.Action == "skip" || c.Action == "blocked" {
			continue
		}
		version := c.sourceVersion
		if c.Target == "" && (c.Action == "record" || c.Action == "noop") {
			version = &c.Binding.Version
		}
		if version == nil {
			continue
		}
		b := declarationForAdd(r, c)
		recipe := packageDeclaration{Source: b.Source, SourceRef: b.SourceRef, Plugin: b.Plugin, Entry: b.Entry}
		if packageFingerprint(recipe) == packageFingerprint(d.intent[c.Name]) {
			d.putPackageVersion(c.Name, recipe, *version)
		}
	}
	return d.intent
}

// Include private observations in the approval revision without exposing them in the plan.
func reviewedChanges(changes []Change) any {
	type reviewedChange struct {
		Change
		SourceVersion *string
	}
	out := make([]reviewedChange, 0, len(changes))
	for _, c := range changes {
		out = append(out, reviewedChange{c, c.sourceVersion})
	}
	return out
}

func (s *Service) saveAndPruneVersions(d *document) error {
	if err := s.save(d); err != nil {
		return err
	}
	changed := false
	for name := range d.state.Versions {
		if _, exists := d.intent[name]; !exists {
			delete(d.state.Versions, name)
			changed = true
		}
	}
	if changed {
		return s.saveState(d)
	}
	return nil
}

func (d *document) hasReceipts() bool {
	for _, targets := range d.state.Bindings {
		for _, b := range targets {
			if b.Applied != nil || b.Pending != nil || b.Legacy != nil {
				return true
			}
		}
	}
	return false
}
