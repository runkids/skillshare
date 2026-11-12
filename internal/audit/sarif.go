package audit

const (
	sarifVersion = "2.1.0"
	sarifSchema  = "https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/schemas/sarif-schema-2.1.0.json"
	toolName     = "skillshare"
	toolInfoURI  = "https://skillshare.runkids.cc/"
)

// --- SARIF 2.1.0 structs (unexported except SARIFOptions) ---

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool       sarifTool      `json:"tool"`
	Results    []sarifResult  `json:"results"`
	Properties map[string]any `json:"properties,omitempty"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string               `json:"name"`
	Version        string               `json:"version,omitempty"`
	InformationURI string               `json:"informationUri"`
	Rules          []sarifReportingDesc `json:"rules"`
}

type sarifReportingDesc struct {
	ID               string           `json:"id"`
	ShortDescription sarifMessage     `json:"shortDescription"`
	DefaultConfig    sarifDefaultConf `json:"defaultConfiguration"`
	Properties       sarifRuleProps   `json:"properties"`
}

type sarifDefaultConf struct {
	Level string `json:"level"`
}

type sarifRuleProps struct {
	SecuritySeverity float64 `json:"security-severity"`
}

type sarifResult struct {
	RuleID       string             `json:"ruleId"`
	RuleIndex    int                `json:"ruleIndex"`
	Level        string             `json:"level"`
	Message      sarifMessage       `json:"message"`
	Locations    []sarifLocation    `json:"locations"`
	Fingerprints map[string]string  `json:"fingerprints,omitempty"`
	Suppressions []sarifSuppression `json:"suppressions,omitempty"`
	Properties   map[string]any     `json:"properties,omitempty"`
}

// sarifSuppression records a finding accepted earlier with --force.
type sarifSuppression struct {
	Kind          string `json:"kind"`
	Justification string `json:"justification,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLoc `json:"physicalLocation"`
}

type sarifPhysicalLoc struct {
	ArtifactLocation sarifArtifactLoc `json:"artifactLocation"`
	Region           *sarifRegion     `json:"region,omitempty"`
}

type sarifArtifactLoc struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

// --- Severity mapping ---

var severityToSARIF = map[string]struct {
	level  string
	secSev float64
}{
	SeverityCritical: {"error", 9.0},
	SeverityHigh:     {"error", 7.0},
	SeverityMedium:   {"warning", 4.0},
	SeverityLow:      {"note", 2.0},
	SeverityInfo:     {"note", 0.5},
}

// SARIFOptions controls optional fields in the SARIF output.
type SARIFOptions struct {
	ToolVersion string
	BaseURI     string
}

// ToSARIF converts audit results into a SARIF 2.1.0 log.
// It deduplicates rules by pattern (first occurrence wins) and maps each
// Finding to a sarifResult with the correct ruleIndex.
func ToSARIF(results []*Result, opts SARIFOptions) *sarifLog {
	ruleIndex := map[string]int{} // pattern → index in rules slice
	var rules []sarifReportingDesc
	var sarifResults []sarifResult
	var coverage []map[string]any

	for _, r := range results {
		coverage = append(coverage, map[string]any{"resource": r.SkillName, "binaryFiles": r.BinaryFiles, "skippedFiles": r.SkippedFiles, "oversizedFiles": r.OversizedFiles})
		for _, f := range r.Findings {
			// Use RuleID when available, fall back to Pattern.
			ruleID := f.RuleID
			if ruleID == "" {
				ruleID = f.Pattern
			}

			// Deduplicate rules by ruleID
			idx, exists := ruleIndex[ruleID]
			if !exists {
				idx = len(rules)
				ruleIndex[ruleID] = idx

				mapping := severityToSARIF[f.Severity]
				description := f.Message
				switch ruleID {
				case "executable-artifact":
					description = "Executable artifact detected; runtime behavior has not been analyzed"
				case "disguised-executable":
					description = "Executable artifact detected in a disguised location; runtime behavior has not been analyzed"
				case "python-bytecode":
					description = "Python bytecode detected; runtime behavior has not been analyzed"
				case "archive-artifact":
					description = "Archive signature detected; contents have not been inspected"
				case "uncertain-header":
					description = "Uncertain executable or bytecode header detected"
				}
				rules = append(rules, sarifReportingDesc{
					ID:               ruleID,
					ShortDescription: sarifMessage{Text: description},
					DefaultConfig:    sarifDefaultConf{Level: mapping.level},
					Properties:       sarifRuleProps{SecuritySeverity: mapping.secSev},
				})
			}

			mapping := severityToSARIF[f.Severity]

			sr := sarifResult{
				RuleID:    ruleID,
				RuleIndex: idx,
				Level:     mapping.level,
				Message:   sarifMessage{Text: f.Message},
				Locations: []sarifLocation{}, // non-nil for clean JSON
			}

			if f.Fingerprint != "" {
				sr.Fingerprints = map[string]string{"skillshare/v1": f.Fingerprint}
			}
			if f.Acknowledged {
				sr.Suppressions = []sarifSuppression{{Kind: "external", Justification: "accepted earlier with --force"}}
			}

			props := map[string]any{}
			if f.Analyzer != "" {
				props["analyzer"] = f.Analyzer
			}
			if f.Category != "" {
				props["category"] = f.Category
			}
			if f.Confidence > 0 {
				props["confidence"] = f.Confidence
			}
			if f.SeverityContext != "" {
				props["severityContext"] = f.SeverityContext
			}
			if len(props) > 0 {
				sr.Properties = props
			}

			if f.File != "" {
				loc := sarifPhysicalLoc{
					ArtifactLocation: sarifArtifactLoc{URI: f.File},
				}
				if f.Line > 0 {
					loc.Region = &sarifRegion{StartLine: f.Line}
				}
				sr.Locations = []sarifLocation{{PhysicalLocation: loc}}
			}

			sarifResults = append(sarifResults, sr)
		}
	}

	// Ensure non-nil slices for clean JSON output
	if sarifResults == nil {
		sarifResults = []sarifResult{}
	}
	if rules == nil {
		rules = []sarifReportingDesc{}
	}

	return &sarifLog{
		Schema:  sarifSchema,
		Version: sarifVersion,
		Runs: []sarifRun{
			{
				Tool: sarifTool{
					Driver: sarifDriver{
						Name:           toolName,
						Version:        opts.ToolVersion,
						InformationURI: toolInfoURI,
						Rules:          rules,
					},
				},
				Results:    sarifResults,
				Properties: map[string]any{"skillshareCoverage": coverage},
			},
		},
	}
}
