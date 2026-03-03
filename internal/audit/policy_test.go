package audit

import "testing"

func TestResolvePolicy_Defaults(t *testing.T) {
	p := ResolvePolicy(PolicyInputs{})
	if p.Profile != ProfileDefault {
		t.Errorf("profile = %q, want %q", p.Profile, ProfileDefault)
	}
	if p.Threshold != SeverityCritical {
		t.Errorf("threshold = %q, want %q", p.Threshold, SeverityCritical)
	}
	if p.DedupeMode != DedupeGlobal {
		t.Errorf("dedupe = %q, want %q", p.DedupeMode, DedupeGlobal)
	}
}

func TestResolvePolicy_StrictProfile(t *testing.T) {
	p := ResolvePolicy(PolicyInputs{Profile: "strict"})
	if p.Threshold != SeverityHigh {
		t.Errorf("strict threshold = %q, want %q", p.Threshold, SeverityHigh)
	}
	if p.DedupeMode != DedupeGlobal {
		t.Errorf("strict dedupe = %q, want %q", p.DedupeMode, DedupeGlobal)
	}
}

func TestResolvePolicy_PermissiveProfile(t *testing.T) {
	p := ResolvePolicy(PolicyInputs{Profile: "permissive"})
	if p.Threshold != SeverityCritical {
		t.Errorf("permissive threshold = %q, want %q", p.Threshold, SeverityCritical)
	}
	if p.DedupeMode != DedupeLegacy {
		t.Errorf("permissive dedupe = %q, want %q", p.DedupeMode, DedupeLegacy)
	}
}

func TestResolvePolicy_ExplicitOverridesProfile(t *testing.T) {
	p := ResolvePolicy(PolicyInputs{
		Profile:   "strict",
		Threshold: SeverityCritical,
	})
	if p.Threshold != SeverityCritical {
		t.Errorf("explicit threshold = %q, want %q", p.Threshold, SeverityCritical)
	}
}

func TestResolvePolicy_ExplicitDedupeOverridesProfile(t *testing.T) {
	p := ResolvePolicy(PolicyInputs{
		Profile: "strict",
		Dedupe:  "legacy",
	})
	if p.DedupeMode != DedupeLegacy {
		t.Errorf("explicit dedupe = %q, want %q", p.DedupeMode, DedupeLegacy)
	}
}

func TestResolvePolicy_ConfigLayering(t *testing.T) {
	p := ResolvePolicy(PolicyInputs{
		ConfigThreshold: SeverityHigh,
	})
	if p.Threshold != SeverityHigh {
		t.Errorf("config threshold = %q, want %q", p.Threshold, SeverityHigh)
	}
}

func TestResolvePolicy_CLIOverridesConfig(t *testing.T) {
	p := ResolvePolicy(PolicyInputs{
		ConfigThreshold: SeverityHigh,
		Threshold:       SeverityMedium,
	})
	if p.Threshold != SeverityMedium {
		t.Errorf("CLI override threshold = %q, want %q", p.Threshold, SeverityMedium)
	}
}

func TestResolvePolicy_InvalidProfileFallsBack(t *testing.T) {
	p := ResolvePolicy(PolicyInputs{Profile: "bogus"})
	if p.Profile != ProfileDefault {
		t.Errorf("invalid profile = %q, want %q", p.Profile, ProfileDefault)
	}
}

func TestResolvePolicy_InvalidDedupeFallsBack(t *testing.T) {
	p := ResolvePolicy(PolicyInputs{Dedupe: "bogus"})
	if p.DedupeMode != DedupeLegacy {
		t.Errorf("invalid dedupe = %q, want %q", p.DedupeMode, DedupeLegacy)
	}
}
