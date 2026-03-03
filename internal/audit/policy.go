package audit

// Profile is a preset shorthand for audit policy settings.
type Profile string

const (
	ProfileDefault    Profile = "default"
	ProfileStrict     Profile = "strict"
	ProfilePermissive Profile = "permissive"
)

// DedupeMode controls how duplicate findings are handled.
type DedupeMode string

const (
	DedupeLegacy DedupeMode = "legacy"
	DedupeGlobal DedupeMode = "global"
)

// Policy holds resolved audit policy settings.
type Policy struct {
	Profile          Profile
	Threshold        string
	DedupeMode       DedupeMode
	EnabledAnalyzers []string // nil = all enabled (default)
}

// PolicyInputs carries raw inputs from CLI flags and config for resolution.
type PolicyInputs struct {
	Profile   string // CLI --profile
	Threshold string // CLI --threshold (already normalized)
	Dedupe    string // CLI --dedupe

	ConfigProfile   string
	ConfigThreshold string
	ConfigDedupe    string

	EnabledAnalyzers []string // CLI --analyzer (repeatable)
	ConfigAnalyzers  []string // config.yaml audit.enabled_analyzers
}

var profilePresets = map[Profile]struct {
	Threshold  string
	DedupeMode DedupeMode
}{
	ProfileDefault:    {SeverityCritical, DedupeGlobal},
	ProfileStrict:     {SeverityHigh, DedupeGlobal},
	ProfilePermissive: {SeverityCritical, DedupeLegacy},
}

func ResolvePolicy(in PolicyInputs) Policy {
	profile := resolveProfile(in.Profile, in.ConfigProfile)
	preset := profilePresets[profile]

	threshold := coalesce(in.Threshold, in.ConfigThreshold)
	if threshold == "" {
		threshold = preset.Threshold
	}

	dedupe := resolveDedupeMode(coalesce(in.Dedupe, in.ConfigDedupe))
	if dedupe == "" {
		dedupe = preset.DedupeMode
	}

	analyzers := in.EnabledAnalyzers
	if len(analyzers) == 0 {
		analyzers = in.ConfigAnalyzers
	}

	return Policy{
		Profile:          profile,
		Threshold:        threshold,
		DedupeMode:       dedupe,
		EnabledAnalyzers: analyzers,
	}
}

func resolveProfile(cli, cfg string) Profile {
	raw := coalesce(cli, cfg)
	switch Profile(raw) {
	case ProfileDefault, ProfileStrict, ProfilePermissive:
		return Profile(raw)
	default:
		return ProfileDefault
	}
}

func resolveDedupeMode(raw string) DedupeMode {
	switch DedupeMode(raw) {
	case DedupeLegacy, DedupeGlobal:
		return DedupeMode(raw)
	case "":
		return ""
	default:
		return DedupeLegacy
	}
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
