package audit

import "testing"

func TestDefaultRegistry_HasAllAnalyzers(t *testing.T) {
	r := DefaultRegistry()
	for _, id := range []string{
		AnalyzerStatic, AnalyzerDataflow, AnalyzerTier,
		AnalyzerIntegrity, AnalyzerStructure, AnalyzerCrossSkill,
	} {
		if !r.Has(id) {
			t.Errorf("default registry missing analyzer %q", id)
		}
	}
}

func TestDefaultRegistry_ScopePartition(t *testing.T) {
	r := DefaultRegistry()
	if len(r.FileAnalyzers()) != 2 {
		t.Errorf("file analyzers = %d, want 2 (static + dataflow)", len(r.FileAnalyzers()))
	}
	if len(r.SkillAnalyzers()) != 4 {
		t.Errorf("skill analyzers = %d, want 4 (markdownLink + structure + integrity + tier)", len(r.SkillAnalyzers()))
	}
	if len(r.BundleAnalyzers()) != 1 {
		t.Errorf("bundle analyzers = %d, want 1 (cross-skill)", len(r.BundleAnalyzers()))
	}
}

func TestRegistry_ForPolicy_NilEnablesAll(t *testing.T) {
	r := DefaultRegistry()
	filtered := r.ForPolicy(Policy{})
	if filtered != r {
		t.Error("nil EnabledAnalyzers should return same registry")
	}
}

func TestRegistry_ForPolicy_FiltersByID(t *testing.T) {
	r := DefaultRegistry()
	filtered := r.ForPolicy(Policy{EnabledAnalyzers: []string{AnalyzerStatic}})
	if !filtered.Has(AnalyzerStatic) {
		t.Error("filtered should have static")
	}
	if filtered.Has(AnalyzerDataflow) {
		t.Error("filtered should not have dataflow")
	}
	if filtered.Has(AnalyzerCrossSkill) {
		t.Error("filtered should not have cross-skill")
	}
}

func TestRegistry_ForPolicy_EmptySliceEnablesAll(t *testing.T) {
	r := DefaultRegistry()
	filtered := r.ForPolicy(Policy{EnabledAnalyzers: []string{}})
	if filtered != r {
		t.Error("empty EnabledAnalyzers should return same registry")
	}
}
