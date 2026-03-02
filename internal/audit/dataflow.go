package audit

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ── Internal types ──

// Taint source kinds.
const (
	taintKindCredentialRead = "credential-read"
	taintKindEnvVar         = "env-var"
)

// Pattern name for dataflow taint findings.
const patternDataflowTaint = "dataflow-taint"

// taintSource records where a taint originated.
type taintSource struct {
	Line    int
	Snippet string
	Kind    string // taintKindCredentialRead or taintKindEnvVar
}

// ── Source detection patterns ──

// credentialPathRe matches paths to sensitive credential files.
// Aligned with rules.yaml credential-access patterns.
var credentialPathRe = regexp.MustCompile(
	`(?i)(~/?\.ssh/(id_|known_hosts|authorized_keys|config)|` +
		`\.env\b|~/?\.aws/(credentials|config)|` +
		`~/?\.gnupg/|~/?\.kube/config|` +
		`/etc/(passwd|shadow|sudoers))`)

// sensitiveEnvRe matches sensitive environment variable references.
// Aligned with data-exfiltration rule patterns.
var sensitiveEnvRe = regexp.MustCompile(
	`(?i)\$\{?(SECRET|TOKEN|API_KEY|PASSWORD|PRIVATE_KEY|` +
		`OPENAI_API_KEY|ANTHROPIC_API_KEY|AWS_SECRET|` +
		`GITHUB_TOKEN|GH_TOKEN|SSH_KEY|GPG_KEY|` +
		`DATABASE_URL|DB_PASSWORD)[_A-Za-z0-9]*\}?`)

// ── Variable parsing patterns ──

var (
	// VAR=$(cmd) or VAR=$(cmd args)
	reAssignCmdSubst = regexp.MustCompile(`^(\w+)=\$\((.+)\)$`)
	// VAR=`cmd`
	reAssignBacktick = regexp.MustCompile("^(\\w+)=`(.+)`$")
	// VAR=$OTHER or VAR=${OTHER} or VAR="...$OTHER..."
	reAssignVarRef = regexp.MustCompile(`^(\w+)=.*\$\{?(\w+)\}?`)
	// Plain assignment: VAR=anything (for taint clearing)
	rePlainAssign = regexp.MustCompile(`^(\w+)=`)
	// export VAR=...
	reExportPrefix = regexp.MustCompile(`^export\s+`)
	// read VAR < file
	reReadCmd = regexp.MustCompile(`^read\s+(?:-\w+\s+)*(\w+)`)
	// Input redirection from file: < /path
	reInputRedirect = regexp.MustCompile(`<\s*(\S+)`)
)

// ── Output redirection / file read ──

var (
	// > /path or >> /path
	reRedirectOut = regexp.MustCompile(`>>?\s*(/\S+)`)
	// @/path or < /path (file read in commands like curl)
	reFileRef = regexp.MustCompile(`[@<]\s*(/\S+)`)
	// $VAR or ${VAR} reference
	reVarRef = regexp.MustCompile(`\$\{?(\w+)\}?`)
)

// networkCommands is the set of commands classified as TierNetwork sinks.
// Derived from commandTiers in tiers.go to stay in sync.
var networkCommands = func() map[string]bool {
	m := make(map[string]bool)
	for cmd, tier := range commandTiers {
		if tier == TierNetwork {
			m[cmd] = true
		}
	}
	return m
}()

// ── Public API ──

// ScanShellDataflow analyses shell script content for cross-line taint flows.
// Returns findings for tainted data reaching network sinks.
func ScanShellDataflow(content []byte, filename string) []Finding {
	lines := strings.Split(string(content), "\n")
	return analyzeShellBlock(lines, 0, filename)
}

// ScanMarkdownDataflow extracts shell code blocks from markdown and analyses
// each one for taint flows. Code blocks are isolated — taint does not propagate
// across blocks.
func ScanMarkdownDataflow(content []byte, filename string) []Finding {
	var findings []Finding
	text := string(content)
	lines := strings.Split(text, "\n")

	inCodeFence := false
	fenceMarker := ""
	var blockLines []string
	blockStart := 0
	isShell := false

	for i, line := range lines {
		if marker, ok := detectFenceMarker(line); ok {
			if !inCodeFence {
				inCodeFence = true
				fenceMarker = marker
				isShell = isShellFenceLang(line)
				blockLines = nil
				blockStart = i + 1 // next line is the first content line
			} else if marker == fenceMarker {
				// End of code block — analyse if shell
				if isShell && len(blockLines) > 0 {
					findings = append(findings,
						analyzeShellBlock(blockLines, blockStart, filename)...)
				}
				inCodeFence = false
				fenceMarker = ""
				isShell = false
				blockLines = nil
			}
			continue
		}
		if inCodeFence && isShell {
			blockLines = append(blockLines, line)
		}
	}

	return findings
}

// exfilPatterns is the set of per-line rule patterns that overlap with
// dataflow taint findings. Used by DeduplicateDataflow to avoid double-reporting.
// Must stay in sync with pattern names in rules.yaml.
var exfilPatterns = map[string]bool{
	"data-exfiltration": true,
	"credential-access": true,
	"suspicious-fetch":  true,
	"fetch-with-pipe":   true,
}

// DeduplicateDataflow removes dataflow findings whose sink line is already
// covered by an existing per-line finding with a data-exfiltration or
// credential-access pattern.
func DeduplicateDataflow(dfFindings, existing []Finding) []Finding {
	if len(dfFindings) == 0 {
		return nil
	}

	// Build set of (file, line) pairs from existing exfiltration/credential findings.
	type key struct {
		file string
		line int
	}
	covered := make(map[key]bool)
	for _, f := range existing {
		if exfilPatterns[f.Pattern] {
			covered[key{f.File, f.Line}] = true
		}
	}

	var result []Finding
	for _, f := range dfFindings {
		if !covered[key{f.File, f.Line}] {
			result = append(result, f)
		}
	}
	return result
}

// ── Shell fence language detection ──

var shellLangs = map[string]bool{
	"bash": true, "sh": true, "zsh": true, "shell": true,
}

// isShellFenceLang checks if a fence opening line indicates a shell code block.
// Unlabelled code blocks (``` with no language) are treated as shell.
func isShellFenceLang(fenceLine string) bool {
	lang := extractFenceLang(fenceLine)
	return lang == "" || shellLangs[lang]
}

// extractFenceLang extracts the language hint from a fence marker line.
// e.g. "```bash" → "bash", "```" → "", "~~~ python" → "python"
func extractFenceLang(line string) string {
	trimmed := strings.TrimSpace(line)
	// Remove fence markers
	for _, prefix := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, prefix) {
			rest := strings.TrimSpace(trimmed[len(prefix):])
			// Take first word as language
			if rest == "" {
				return ""
			}
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				return strings.ToLower(fields[0])
			}
			return ""
		}
	}
	return ""
}

// isShellFile returns true if the filename has a shell script extension.
func isShellFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".sh", ".bash", ".zsh":
		return true
	}
	return false
}

// ── Core analysis ──

// analyzeShellBlock performs forward taint propagation on a block of shell lines.
// lineOffset is the 1-based line number offset in the original file.
func analyzeShellBlock(lines []string, lineOffset int, filename string) []Finding {
	taintMap := make(map[string]taintSource) // var name → source
	fileMap := make(map[string]taintSource)  // temp file path → source
	var findings []Finding

	for i, rawLine := range lines {
		lineNum := lineOffset + i + 1
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Strip export prefix for assignment parsing.
		stripped := line
		if strings.HasPrefix(line, "export ") || strings.HasPrefix(line, "export\t") {
			stripped = reExportPrefix.ReplaceAllString(line, "")
		}

		// 1. Handle `read VAR < file` — check if reading from sensitive file.
		if m := reReadCmd.FindStringSubmatch(stripped); m != nil {
			varName := m[1]
			if redir := reInputRedirect.FindStringSubmatch(line); redir != nil {
				path := redir[1]
				if credentialPathRe.MatchString(path) {
					taintMap[varName] = taintSource{
						Line:    lineNum,
						Snippet: line,
						Kind:    taintKindCredentialRead,
					}
					continue
				}
				// Check if redirected from a tainted temp file.
				if src, ok := fileMap[path]; ok {
					taintMap[varName] = src
					continue
				}
				// Reading from a safe file clears pre-existing taint.
				delete(taintMap, varName)
				continue
			}
		}

		// 2. Parse variable assignments: VAR=$(cmd) or VAR=`cmd`
		if varName, cmdBody, ok := matchCmdSubst(stripped); ok {
			if src := classifyAssignmentSource(cmdBody, line, lineNum, taintMap); src != nil {
				taintMap[varName] = *src
			} else {
				delete(taintMap, varName) // reassigned to safe value
			}
			continue
		}

		// 3. Parse variable-to-variable assignment: VAR=$OTHER or VAR="${OTHER}"
		// Scan ALL $VAR references on the RHS to avoid greedy-match false negatives.
		if m := reAssignVarRef.FindStringSubmatch(stripped); m != nil {
			varName := m[1]
			if isAssignment(stripped) {
				rhs := stripped[strings.IndexByte(stripped, '=')+1:]
				if src := findTaintInRHS(rhs, lineNum, line, taintMap); src != nil {
					taintMap[varName] = *src
				} else {
					delete(taintMap, varName) // reassigned to safe value
				}
				continue
			}
		}

		// 3b. Plain assignment without $ reference (e.g. VAR="literal"):
		// clears taint on the variable.
		if m := rePlainAssign.FindStringSubmatch(stripped); m != nil {
			if isAssignment(stripped) {
				delete(taintMap, m[1])
				continue
			}
		}

		// 4. Temp file tracking: tainted content redirected to a file.
		if m := reRedirectOut.FindStringSubmatch(line); m != nil {
			path := m[1]
			if hasTaintedVarRef(line, taintMap) {
				// Pick the first tainted source for provenance.
				for _, ref := range reVarRef.FindAllStringSubmatch(line, -1) {
					if src, ok := taintMap[ref[1]]; ok {
						fileMap[path] = src
						break
					}
				}
			}
		}

		// 5. Pipe chain detection: source | ... | sink
		if strings.Contains(line, "|") {
			if f := checkPipeChain(line, lineNum, filename, taintMap); f != nil {
				findings = append(findings, *f)
				continue
			}
		}

		// 6. Sink detection: network commands with tainted variable references.
		findings = append(findings, detectSinks(line, lineNum, filename, taintMap, fileMap)...)
	}

	return findings
}

// classifyAssignmentSource determines if a command substitution produces
// tainted data. Returns nil if the source is considered safe.
func classifyAssignmentSource(cmdBody, fullLine string, lineNum int, taintMap map[string]taintSource) *taintSource {
	// Check for credential file reads.
	if credentialPathRe.MatchString(cmdBody) {
		return &taintSource{
			Line:    lineNum,
			Snippet: fullLine,
			Kind:    taintKindCredentialRead,
		}
	}

	// Check for sensitive env var references in the command.
	if sensitiveEnvRe.MatchString(cmdBody) {
		return &taintSource{
			Line:    lineNum,
			Snippet: fullLine,
			Kind:    taintKindEnvVar,
		}
	}

	// Check if the command body references any tainted variable.
	for _, ref := range reVarRef.FindAllStringSubmatch(cmdBody, -1) {
		if src, ok := taintMap[ref[1]]; ok {
			return &taintSource{
				Line:    src.Line,
				Snippet: src.Snippet,
				Kind:    src.Kind,
			}
		}
	}

	return nil
}

// findTaintInRHS scans all $VAR references in the assignment RHS and returns
// the first tainted source found, or a sensitive env var match. Returns nil if safe.
func findTaintInRHS(rhs string, lineNum int, fullLine string, taintMap map[string]taintSource) *taintSource {
	for _, ref := range reVarRef.FindAllStringSubmatch(rhs, -1) {
		varName := ref[1]
		if src, ok := taintMap[varName]; ok {
			return &src
		}
		if sensitiveEnvRe.MatchString("$" + varName) {
			return &taintSource{
				Line:    lineNum,
				Snippet: fullLine,
				Kind:    taintKindEnvVar,
			}
		}
	}
	return nil
}

// matchCmdSubst tries to match VAR=$(cmd) or VAR=`cmd` and returns
// the variable name, command body, and whether it matched.
func matchCmdSubst(line string) (varName, cmdBody string, ok bool) {
	if m := reAssignCmdSubst.FindStringSubmatch(line); m != nil {
		return m[1], m[2], true
	}
	if m := reAssignBacktick.FindStringSubmatch(line); m != nil {
		return m[1], m[2], true
	}
	return "", "", false
}

// isAssignment returns true if the line looks like a variable assignment
// (not a command with = in its arguments).
func isAssignment(line string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	// First token must contain = and start with a word char.
	first := fields[0]
	eqIdx := strings.IndexByte(first, '=')
	if eqIdx <= 0 {
		return false
	}
	// Everything before = must be a valid variable name.
	name := first[:eqIdx]
	for i, ch := range name {
		if ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			continue
		}
		if i > 0 && ch >= '0' && ch <= '9' {
			continue
		}
		return false
	}
	return true
}

// hasTaintedVarRef returns true if the line references any tainted variable.
func hasTaintedVarRef(line string, taintMap map[string]taintSource) bool {
	for _, ref := range reVarRef.FindAllStringSubmatch(line, -1) {
		if _, ok := taintMap[ref[1]]; ok {
			return true
		}
	}
	return false
}

// checkPipeChain detects source | ... | sink patterns.
// Returns a finding if a credential-reading command pipes into a network command.
func checkPipeChain(line string, lineNum int, filename string, taintMap map[string]taintSource) *Finding {
	segments := strings.Split(line, "|")
	if len(segments) < 2 {
		return nil
	}

	// Check if the first segment reads credentials.
	firstSeg := strings.TrimSpace(segments[0])
	hasCredSource := credentialPathRe.MatchString(firstSeg) || hasTaintedVarRef(firstSeg, taintMap)

	if !hasCredSource {
		// Also check if first segment references a sensitive env var.
		hasCredSource = sensitiveEnvRe.MatchString(firstSeg)
	}

	if !hasCredSource {
		return nil
	}

	// Check if the last segment (or any segment) is a network command.
	for i := 1; i < len(segments); i++ {
		seg := strings.TrimSpace(segments[i])
		for _, cmd := range ExtractCommands(seg) {
			if networkCommands[filepath.Base(cmd)] {
				return &Finding{
					Severity: SeverityHigh,
					Pattern:  patternDataflowTaint,
					Message:  fmt.Sprintf("tainted data piped to network command %q", cmd),
					File:     filename,
					Line:     lineNum,
					Snippet:  strings.TrimSpace(line),
				}
			}
		}
	}

	return nil
}

// detectSinks checks if any network command on this line uses tainted data.
func detectSinks(line string, lineNum int, filename string, taintMap map[string]taintSource, fileMap map[string]taintSource) []Finding {
	cmds := ExtractCommands(line)
	hasNetworkCmd := false
	for _, cmd := range cmds {
		if networkCommands[filepath.Base(cmd)] {
			hasNetworkCmd = true
			break
		}
	}
	if !hasNetworkCmd {
		return nil
	}

	var findings []Finding
	seen := make(map[string]bool) // deduplicate by source line

	// Check for tainted variable references on this line.
	for _, ref := range reVarRef.FindAllStringSubmatch(line, -1) {
		varName := ref[1]
		src, ok := taintMap[varName]
		if !ok {
			continue
		}
		key := fmt.Sprintf("%d→%d", src.Line, lineNum)
		if seen[key] {
			continue
		}
		seen[key] = true

		findings = append(findings, Finding{
			Severity: SeverityHigh,
			Pattern:  patternDataflowTaint,
			Message:  buildTaintMessage(src, varName, lineNum),
			File:     filename,
			Line:     lineNum,
			Snippet:  strings.TrimSpace(line),
		})
	}

	// Check for tainted temp file references.
	for _, ref := range reFileRef.FindAllStringSubmatch(line, -1) {
		path := ref[1]
		src, ok := fileMap[path]
		if !ok {
			continue
		}
		key := fmt.Sprintf("file:%d→%d", src.Line, lineNum)
		if seen[key] {
			continue
		}
		seen[key] = true

		findings = append(findings, Finding{
			Severity: SeverityHigh,
			Pattern:  patternDataflowTaint,
			Message:  fmt.Sprintf("tainted data flows from %s (line %d) to network send (line %d) via temp file %s", src.Kind, src.Line, lineNum, path),
			File:     filename,
			Line:     lineNum,
			Snippet:  strings.TrimSpace(line),
		})
	}

	return findings
}

// buildTaintMessage creates a human-readable message describing the taint chain.
func buildTaintMessage(src taintSource, sinkVar string, sinkLine int) string {
	return fmt.Sprintf("tainted data flows from %s (line %d) to network send (line %d) via $%s",
		src.Kind, src.Line, sinkLine, sinkVar)
}
