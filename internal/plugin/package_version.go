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

func (s *Service) captureLegacyAndVersions(d *document, r Request, plan *Plan) {
	s.captureLegacy(d)
	for name, p := range d.legacy {
		if p.Version != "" {
			if v, exists := d.state.Versions[name]; !exists || v.Intent != packageFingerprint(d.intent[name]) {
				d.putPackageVersion(name, d.intent[name], p.Version)
			}
		}
	}
	if r.Action != "add" {
		return
	}
	for _, c := range plan.Changes {
		if c.Target == "" && (c.Action == "record" || c.Action == "noop") {
			b := declarationForAdd(r, c)
			d.putPackageVersion(c.Name, packageDeclaration{Source: b.Source, SourceRef: b.SourceRef, Plugin: b.Plugin, Entry: b.Entry}, c.Binding.Version)
		}
	}
}

func (d *document) putPackageVersion(name string, p packageDeclaration, version string) {
	if d.state.Versions == nil {
		d.state.Versions = map[string]packageVersion{}
	}
	d.state.Versions[name] = packageVersion{Intent: packageFingerprint(p), Version: version}
}
