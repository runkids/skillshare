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
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
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
	RuleID    string          `json:"ruleId"`
	RuleIndex int             `json:"ruleIndex"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLoc `json:"physicalLocation"`
}

type sarifPhysicalLoc struct {
	ArtifactLocation sarifArtifactLoc `json:"artifactLocation"`
	Region           sarifRegion      `json:"region"`
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

	for _, r := range results {
		for _, f := range r.Findings {
			// Deduplicate rules by pattern
			idx, exists := ruleIndex[f.Pattern]
			if !exists {
				idx = len(rules)
				ruleIndex[f.Pattern] = idx

				mapping := severityToSARIF[f.Severity]
				rules = append(rules, sarifReportingDesc{
					ID:               f.Pattern,
					ShortDescription: sarifMessage{Text: f.Message},
					DefaultConfig:    sarifDefaultConf{Level: mapping.level},
					Properties:       sarifRuleProps{SecuritySeverity: mapping.secSev},
				})
			}

			mapping := severityToSARIF[f.Severity]

			sr := sarifResult{
				RuleID:    f.Pattern,
				RuleIndex: idx,
				Level:     mapping.level,
				Message:   sarifMessage{Text: f.Message},
				Locations: []sarifLocation{}, // non-nil for clean JSON
			}

			if f.File != "" {
				sr.Locations = []sarifLocation{
					{
						PhysicalLocation: sarifPhysicalLoc{
							ArtifactLocation: sarifArtifactLoc{URI: f.File},
							Region:           sarifRegion{StartLine: f.Line},
						},
					},
				}
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
				Results: sarifResults,
			},
		},
	}
}
