package audit

// Registry holds all registered analyzers and provides scope-filtered access.
type Registry struct {
	analyzers []Analyzer
}

// defaultRegistry is a package-level singleton. All built-in analyzers are
// stateless (zero-field structs), so sharing a single instance is safe.
var defaultRegistry = &Registry{
	analyzers: []Analyzer{
		&staticAnalyzer{},
		&dataflowAnalyzer{},
		&markdownLinkAnalyzer{},
		&structureAnalyzer{},
		&integrityAnalyzer{},
		&tierAnalyzer{},
		&crossSkillAnalyzer{},
		&metadataAnalyzer{},
	},
}

// DefaultRegistry returns a registry with all built-in analyzers.
func DefaultRegistry() *Registry { return defaultRegistry }

// ForPolicy returns a filtered registry respecting policy.EnabledAnalyzers.
// If EnabledAnalyzers is nil/empty, all analyzers are enabled (default behavior).
func (r *Registry) ForPolicy(p Policy) *Registry {
	if len(p.EnabledAnalyzers) == 0 {
		return r
	}
	allowed := make(map[string]bool, len(p.EnabledAnalyzers))
	for _, id := range p.EnabledAnalyzers {
		allowed[id] = true
	}
	var filtered []Analyzer
	for _, a := range r.analyzers {
		if allowed[a.ID()] {
			filtered = append(filtered, a)
		}
	}
	return &Registry{analyzers: filtered}
}

// FileAnalyzers returns analyzers that run per-file.
func (r *Registry) FileAnalyzers() []Analyzer {
	return r.byScope(ScopeFile)
}

// SkillAnalyzers returns analyzers that run per-skill.
func (r *Registry) SkillAnalyzers() []Analyzer {
	return r.byScope(ScopeSkill)
}

// BundleAnalyzers returns analyzers that run per-bundle.
func (r *Registry) BundleAnalyzers() []Analyzer {
	return r.byScope(ScopeBundle)
}

// IDs returns the unique IDs of all analyzers in the registry.
func (r *Registry) IDs() []string {
	seen := make(map[string]bool, len(r.analyzers))
	var ids []string
	for _, a := range r.analyzers {
		if !seen[a.ID()] {
			seen[a.ID()] = true
			ids = append(ids, a.ID())
		}
	}
	return ids
}

// Has returns true if an analyzer with the given ID is in the registry.
func (r *Registry) Has(id string) bool {
	for _, a := range r.analyzers {
		if a.ID() == id {
			return true
		}
	}
	return false
}

func (r *Registry) byScope(scope AnalyzerScope) []Analyzer {
	var out []Analyzer
	for _, a := range r.analyzers {
		if a.Scope() == scope {
			out = append(out, a)
		}
	}
	return out
}
