package audit

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"skillshare/internal/utils"
)

var bundledPaths = []string{"hooks.json", "hooks/hooks.json", ".claude/settings.json", ".claude/settings.local.json"}

// This closed table is the bundled declaration policy. Presence means potential
// execution if activated, not that a client has loaded this skill's settings.
var bundledRules = []yamlRule{
	{ID: "bundled-hook-command", Severity: SeverityMedium, Message: "Bundled lifecycle command can run if activated"},
	{ID: "bundled-network-command", Severity: SeverityHigh, Message: "Bundled declaration has network capability; data transfer is not proven"},
	{ID: "bundled-remote-post", Severity: SeverityHigh, Message: "Bundled declaration can POST to a non-loopback destination; exfiltration is not proven"},
	{ID: "bundled-permission", Severity: SeverityHigh, Message: "Bundled settings request broad permissions if activated"},
	{ID: "bundled-helper-command", Severity: SeverityHigh, Message: "Bundled settings declare a command-valued helper if activated"},
	{ID: "bundled-config-unparsed", Severity: SeverityInfo, Message: "Bundled configuration was not fully inspected"},
}

func bundledYAMLRules() []yamlRule {
	rules := slices.Clone(bundledRules)
	for i := range rules {
		rules[i].Pattern = rules[i].ID
		// Listing/override metadata only: detections require parsed JSON.
		rules[i].Regex = `\b\B`
	}
	return rules
}

func isBundledConfig(path string) bool { return slices.Contains(bundledPaths, path) }

// OpenRoot confines symlinks as well as directory traversal to the resolved
// skill root. Check type before opening so FIFOs/devices cannot block the scan.
func collectBundled(root string, cache map[string][]byte) (gaps []Finding) {
	// scanSkillImpl resolves only roots allowed by its Follow policy. Keep the
	// walker's default refusal to enter a root link when no policy resolved it.
	if info, err := os.Lstat(root); err == nil && utils.IsLinkMode(root, info.Mode()) {
		return nil
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return []Finding{{File: "hooks.json", Message: "$: skill root unavailable"}}
	}
	defer dir.Close()
	for _, path := range bundledPaths {
		info, err := dir.Stat(path)
		if os.IsNotExist(err) {
			brokenLink := false
			for component := path; component != "."; component = filepath.Dir(component) {
				link, linkErr := dir.Lstat(component)
				if linkErr == nil && utils.IsLinkMode(filepath.Join(root, component), link.Mode()) {
					if _, targetErr := dir.Stat(component); targetErr != nil {
						brokenLink = true
						break
					}
				}
			}
			if !brokenLink {
				continue
			}
		}
		reason := ""
		switch {
		case err != nil:
			reason = "path unreadable or outside skill root"
		case !info.Mode().IsRegular():
			reason = "not a regular file"
		case info.Size() > maxScanFileSize:
			reason = "file exceeds 1000000-byte scan limit"
		default:
			data, readErr := readBundled(dir, path)
			if readErr != nil {
				reason = "bounded read failed or scan limit exceeded"
			} else {
				cache[path] = data
			}
		}
		if reason != "" {
			gaps = append(gaps, Finding{File: path, Message: "$: " + reason})
		}
	}
	return gaps
}

func readBundled(root *os.Root, path string) ([]byte, error) {
	f, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxScanFileSize+1))
	if len(data) > maxScanFileSize {
		return nil, fmt.Errorf("scan limit exceeded")
	}
	return data, err
}

type bundledAnalyzer struct{}

func (*bundledAnalyzer) ID() string           { return AnalyzerStructure }
func (*bundledAnalyzer) Scope() AnalyzerScope { return ScopeSkill }

func (*bundledAnalyzer) Analyze(ctx *AnalyzeContext) ([]Finding, error) {
	var findings []Finding
	emit := func(file, path, id, evidence string) {
		if ctx.DisabledIDs[id] {
			return
		}
		for _, policy := range bundledRules {
			if policy.ID != id {
				continue
			}
			sev := policy.Severity
			for _, r := range ctx.Rules {
				if r.ID == id {
					sev = r.Severity
					break
				}
			}
			findings = append(findings, Finding{File: file, RuleID: id, Pattern: id,
				Severity: sev, Analyzer: AnalyzerStructure, Category: CategoryRisk,
				Confidence: 1, Message: policy.Message + ": " + path, Snippet: evidence, acceptanceContext: path})
		}
	}
	for _, gap := range ctx.BundledDiagnostics {
		emit(gap.File, gap.Message, "bundled-config-unparsed", bundledDiagnosticEvidence(gap.File, gap.Message, sha256.Sum256(nil)))
	}
	for _, file := range bundledPaths {
		data, ok := ctx.FileCache[file]
		if !ok {
			continue
		}
		contentDigest := sha256.Sum256(data)
		gap := func(path string) {
			emit(file, path, "bundled-config-unparsed", bundledDiagnosticEvidence(file, path, contentDigest))
		}
		var doc map[string]any
		if json.Unmarshal(data, &doc) != nil || doc == nil {
			gap("$: malformed JSON or unsupported root")
			continue
		}
		hooks, hasHooks := doc["hooks"]
		settings := strings.HasPrefix(file, ".claude/")
		if !hasHooks && !settings {
			hooks = doc
			hasHooks = true
		}
		if hasHooks {
			events, ok := hooks.(map[string]any)
			if !ok {
				gap("$.hooks: unsupported event map")
			} else {
				// Sort event names for deterministic reports and fingerprints.
				var keys []string
				for key := range events {
					keys = append(keys, key)
				}
				slices.Sort(keys)
				for _, event := range keys {
					if !knownBundledEvent(event) {
						gap(fmt.Sprintf("$.hooks: unsupported event (name redacted; sha256:%x)", sha256.Sum256([]byte(event))))
						continue
					}
					path := fmt.Sprintf("$.hooks[%q]", event)
					if !hasField(doc, "hooks") {
						path = fmt.Sprintf("$[%q]", event)
					}
					groups, ok := events[event].([]any)
					if !ok {
						gap(path + ": expected array")
						continue
					}
					for i, group := range groups {
						p := fmt.Sprintf("%s[%d]", path, i)
						obj, ok := group.(map[string]any)
						if !ok {
							gap(p + ": expected declaration")
							continue
						}
						if children, grouped := obj["hooks"]; grouped {
							list, ok := children.([]any)
							if !ok {
								gap(p + ".hooks: expected array")
								continue
							}
							for j, child := range list {
								inspectBundledHook(child, fmt.Sprintf("%s.hooks[%d]", p, j), file, emit, gap)
							}
						} else {
							inspectBundledHook(obj, p, file, emit, gap)
						}
					}
				}
			}
		}
		if settings {
			inspectBundledSettings(doc, file, emit, gap)
		}
	}
	return findings, nil
}

func knownBundledEvent(name string) bool {
	return name != "" && !strings.Contains(name, "|") && strings.Contains(bundledEvents, "|"+name+"|")
}

// Keep this supported-schema table local: importing hooks creates a test cycle
// through config -> install -> audit. New/unknown events remain coverage gaps.
const bundledEvents = "|SessionStart||Setup||UserPromptSubmit||UserPromptExpansion||PreToolUse||PermissionRequest||PermissionDenied||PostToolUse||PostToolUseFailure||PostToolBatch||Notification||MessageDisplay||SubagentStart||SubagentStop||TaskCreated||TaskCompleted||Stop||StopFailure||TeammateIdle||InstructionsLoaded||ConfigChange||CwdChanged||DirectoryAdded||FileChanged||WorktreeCreate||WorktreeRemove||PreCompact||PostCompact||PreModelSwitch||PostModelSwitch||Elicitation||ElicitationResult||SessionEnd|"

func hasField(obj map[string]any, key string) bool { _, ok := obj[key]; return ok }

type bundledEmitter func(file, path, id, evidence string)

func inspectBundledHook(value any, path, file string, emit bundledEmitter, gap func(string)) {
	obj, ok := value.(map[string]any)
	if !ok {
		gap(path + ": expected hook object")
		return
	}
	switch obj["type"] {
	case "command":
		command, ok := obj["command"].(string)
		if !ok || strings.TrimSpace(command) == "" {
			gap(path + ".command: expected nonempty command")
			return
		}
		var segments []string
		substitutionsComplete := true
		networkExecutable := ""
		remotePOST := false
		argsProvided := false
		argsValid := false
		argsEvidence := ""
		var argv []string
		if value, exists := obj["args"]; exists {
			argsProvided = true
			args, ok := value.([]any)
			if !ok {
				gap(path + ".args: expected argument array")
			} else {
				argsValid = true
				argv = []string{command}
				encoded, _ := json.Marshal(args)
				argsEvidence = fmt.Sprintf("; args [content redacted; sha256:%x]", sha256.Sum256(encoded))
				for _, arg := range args {
					if s, ok := arg.(string); ok {
						argv = append(argv, s)
					} else {
						gap(path + ".args: expected string arguments")
					}
				}
				var parsed bool
				networkExecutable, remotePOST, parsed = bundledNetworkArgv(argv, strings.Join(argv, " "), 0)
				if !parsed {
					gap(path + ".command: shell expansion inspection incomplete")
				}
			}
		}
		if !argsValid {
			segments, substitutionsComplete = bundledCommandSegments(command)
			var wordsComplete bool
			networkExecutable, remotePOST, wordsComplete = bundledNetworkAnalysis(segments)
			substitutionsComplete = substitutionsComplete && wordsComplete
			if argsProvided {
				remotePOST = false
			}
			if !substitutionsComplete {
				gap(path + ".command: shell expansion inspection incomplete")
			}
		}
		network := networkExecutable != ""
		evidence := bundledEvidenceWithNetwork(command, networkExecutable) + argsEvidence
		id := "bundled-hook-command"
		if network {
			id = "bundled-network-command"
		}
		if id == "bundled-network-command" && remotePOST {
			id = "bundled-remote-post"
			evidence += "; POST/data flag with non-loopback HTTP(S) destination"
		}
		emit(file, path+".command", id, evidence)
	case "http":
		destination, ok := obj["url"].(string)
		if !ok || !bundledURL.MatchString(destination) {
			gap(path + ".url: unsupported HTTP destination")
			return
		}
		id := "bundled-network-command"
		if bundledRemoteURL(destination) {
			id = "bundled-remote-post"
		}
		emit(file, path+".url (HTTP hook POST)", id, bundledEvidence(destination))
	default:
		gap(path + ": unsupported hook type (including prompt/agent hooks)")
	}
}

var (
	bundledPOST = regexp.MustCompile(`(?i:--request[=\s]+POST\b|-X\s*POST\b|--data(?:-raw|-binary|-urlencode)?(?:[=\s]|$)|--json(?:[=\s]|$)|--form(?:-string)?(?:[=\s]|$)|--post-(?:data|file)(?:[=\s]|$)|\sPOST\s)|\s-[dF]`)
	bundledURL  = regexp.MustCompile(`https?://[^\s'"<>]+`)
	// Prefix/path wildcards constrain a grant; only unrestricted tool patterns
	// and MCP tool groups belong to the HIGH broad-permission policy.
	bundledBroad = regexp.MustCompile(`^(?:\*|Bash|Read|Write|Edit|WebFetch|(?:Bash|Read|Write|Edit|WebFetch)\(\*\)|Bash\(:\*\))$`)
	bundledWords = regexp.MustCompile(`"(?:\\.|[^"\\])*"|'[^']*'|[^\s]+`)
)

func bundledBroadPermission(permission string) bool {
	if rest, ok := strings.CutPrefix(permission, "mcp__"); ok {
		if server, ok := strings.CutSuffix(rest, "__*"); ok {
			return server != "" && !strings.Contains(server, "*")
		}
		return rest != "" && !strings.Contains(rest, "__") && !strings.Contains(rest, "*")
	}
	return bundledBroad.MatchString(permission)
}

// Split common shell operators without breaking quoted/escaped arguments or
// variable references, then inspect executable command substitutions too.
// This is lexical inspection, not shell evaluation.
const maxBundledShellExpansionDepth = 32

type bundledCommandBody struct {
	text              string
	legacyBackticks   bool
	substitutionsOnly bool
}

func bundledCommandSegments(command string) ([]string, bool) {
	type pendingCommand struct {
		text              string
		depth             int
		legacyBackticks   bool
		substitutionsOnly bool
	}
	queue := []pendingCommand{{text: command}}
	complete := true
	var all []string
	for i := 0; i < len(queue); i++ {
		pending := queue[i]
		bodies, parsed := bundledCommandSubstitutions(pending.text, pending.legacyBackticks)
		if !parsed {
			complete = false
		}
		for _, body := range bodies {
			if pending.depth >= maxBundledShellExpansionDepth {
				complete = false
				continue
			}
			queue = append(queue, pendingCommand{
				text:              body.text,
				depth:             pending.depth + 1,
				legacyBackticks:   body.legacyBackticks,
				substitutionsOnly: body.substitutionsOnly,
			})
		}
		if !pending.substitutionsOnly {
			all = append(all, splitBundledCommandSegments(pending.text)...)
		}
	}
	return all, complete
}

func splitBundledCommandSegments(command string) []string {
	var segments []string
	var quote byte
	escaped := false
	wordOpen := false
	start := 0
	for i := 0; i < len(command); i++ {
		c := command[i]
		if escaped {
			escaped = false
			if c != '\n' {
				wordOpen = true
			}
			continue
		}
		if c == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			wordOpen = true
			continue
		}
		if end, comment := bundledShellCommentEnd(command, i, wordOpen); comment {
			i = end - 1
			continue
		}
		if c == '$' && i+2 < len(command) && command[i+1] == '(' && command[i+2] == '(' {
			if _, end, ok := bundledArithmeticExpansion(command, i); ok {
				i = end - 1
				wordOpen = true
				continue
			}
			return append(segments, command[start:])
		}
		if c == '$' && i+1 < len(command) && command[i+1] == '(' {
			if _, end, ok := bundledCommandSubstitution(command, i); ok {
				i = end - 1
				wordOpen = true
				continue
			}
			return append(segments, command[start:])
		}
		if quote == 0 && c == '(' && i+1 < len(command) && command[i+1] == '(' {
			if _, end, ok := bundledArithmeticCommand(command, i); ok {
				i = end - 1
				wordOpen = false
				continue
			}
			return append(segments, command[start:])
		}
		if c == '`' {
			if _, end, ok := bundledBacktickSubstitution(command, i); ok {
				i = end - 1
				wordOpen = true
				continue
			}
		}
		switch c {
		case '\'', '"':
			quote = c
			wordOpen = true
		case ' ', '\t':
			wordOpen = false
		case '|', '&', ';', '(', ')', '\r', '\n':
			segments = append(segments, command[start:i])
			start = i + 1
			wordOpen = false
		default:
			wordOpen = true
		}
	}
	return append(segments, command[start:])
}

func bundledShellCommentEnd(command string, start int, wordOpen bool) (int, bool) {
	if start >= len(command) || command[start] != '#' {
		return 0, false
	}
	if wordOpen {
		return 0, false
	}
	for end := start + 1; end < len(command); end++ {
		if command[end] == '\n' {
			return end, true
		}
	}
	return len(command), true
}

func bundledCommandSubstitutions(command string, legacyBackticks bool) ([]bundledCommandBody, bool) {
	var substitutions []bundledCommandBody
	var quote byte
	escaped := false
	wordOpen := false
	for i := 0; i < len(command); i++ {
		c := command[i]
		if escaped {
			escaped = false
			if c != '\n' {
				wordOpen = true
			}
			continue
		}
		if quote == 0 {
			if end, comment := bundledShellCommentEnd(command, i, wordOpen); comment {
				i = end - 1
				continue
			}
		}
		if legacyBackticks && quote != '\'' && c == '\\' && i+1 < len(command) && command[i+1] == '`' {
			body, end, ok := bundledEscapedBacktickSubstitution(command, i)
			if !ok {
				return substitutions, false
			}
			substitutions = append(substitutions, bundledCommandBody{text: body, legacyBackticks: true})
			i = end - 1
			wordOpen = true
			continue
		}
		if c == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote == '\'' {
			if c == '\'' {
				quote = 0
			}
			wordOpen = true
			continue
		}
		if c == '$' && i+1 < len(command) && command[i+1] == '(' {
			if i+2 < len(command) && command[i+2] == '(' {
				body, end, ok := bundledArithmeticExpansion(command, i)
				if !ok {
					return substitutions, false
				}
				substitutions = append(substitutions, bundledCommandBody{
					text:              body,
					legacyBackticks:   legacyBackticks,
					substitutionsOnly: true,
				})
				i = end - 1
				wordOpen = true
				continue
			}
			if body, end, ok := bundledCommandSubstitution(command, i); ok {
				substitutions = append(substitutions, bundledCommandBody{text: body})
				i = end - 1
				wordOpen = true
				continue
			}
			return substitutions, false
		}
		if quote == 0 && c == '(' && i+1 < len(command) && command[i+1] == '(' {
			body, end, ok := bundledArithmeticCommand(command, i)
			if !ok {
				return substitutions, false
			}
			substitutions = append(substitutions, bundledCommandBody{
				text:              body,
				legacyBackticks:   legacyBackticks,
				substitutionsOnly: true,
			})
			i = end - 1
			wordOpen = false
			continue
		}
		if c == '`' {
			if body, end, ok := bundledBacktickSubstitution(command, i); ok {
				substitutions = append(substitutions, bundledCommandBody{text: body, legacyBackticks: true})
				i = end - 1
				wordOpen = true
				continue
			}
			return substitutions, false
		}
		if quote == '"' {
			if c == '"' {
				quote = 0
			}
			wordOpen = true
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			wordOpen = true
			continue
		}
		switch c {
		case ' ', '\t', '\r', '\n', '|', '&', ';', '(', ')', '<', '>':
			wordOpen = false
		default:
			wordOpen = true
		}
	}
	return substitutions, true
}

func bundledArithmeticExpansion(command string, start int) (string, int, bool) {
	return bundledArithmeticBody(command, start+3)
}

func bundledArithmeticCommand(command string, start int) (string, int, bool) {
	return bundledArithmeticBody(command, start+2)
}

func bundledArithmeticBody(command string, bodyStart int) (string, int, bool) {
	depth := 0
	var quote byte
	escaped := false
	for i := bodyStart; i < len(command); i++ {
		c := command[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '$' && i+1 < len(command) && command[i+1] == '(' && (i+2 >= len(command) || command[i+2] != '(') {
			if _, end, ok := bundledCommandSubstitution(command, i); ok {
				i = end - 1
				continue
			}
			return "", 0, false
		}
		if c == '`' {
			if _, end, ok := bundledBacktickSubstitution(command, i); ok {
				i = end - 1
				continue
			}
			return "", 0, false
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		switch c {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
				continue
			}
			if i+1 < len(command) && command[i+1] == ')' {
				return command[bodyStart:i], i + 2, true
			}
			return "", 0, false
		}
	}
	return "", 0, false
}

func bundledCommandSubstitution(command string, start int) (string, int, bool) {
	depth := 1
	var quote byte
	escaped := false
	wordOpen := false
	var expansionDepths []int
	for i := start + 2; i < len(command); i++ {
		c := command[i]
		if escaped {
			escaped = false
			if c != '\n' {
				wordOpen = true
			}
			continue
		}
		if c == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			wordOpen = true
			continue
		}
		if end, comment := bundledShellCommentEnd(command, i, wordOpen); comment {
			i = end - 1
			continue
		}
		if c == '$' && i+1 < len(command) && command[i+1] == '(' {
			expansionDepths = append(expansionDepths, depth)
			wordOpen = false
			continue
		}
		if c == '`' {
			if _, end, ok := bundledBacktickSubstitution(command, i); ok {
				i = end - 1
				wordOpen = true
				continue
			}
			return "", 0, false
		}
		if c == '\'' || c == '"' {
			quote = c
			wordOpen = true
			continue
		}
		switch c {
		case '(':
			depth++
			wordOpen = false
		case ')':
			if depth == 1 {
				return command[start+2 : i], i + 1, true
			}
			depth--
			expansionClose := len(expansionDepths) > 0 && depth == expansionDepths[len(expansionDepths)-1]
			if expansionClose {
				expansionDepths = expansionDepths[:len(expansionDepths)-1]
				wordOpen = true
			} else {
				wordOpen = false
			}
		case ' ', '\t', '\r', '\n', '|', '&', ';', '<', '>':
			wordOpen = false
		default:
			wordOpen = true
		}
	}
	return "", 0, false
}

func bundledBacktickSubstitution(command string, start int) (string, int, bool) {
	var quote byte
	escaped := false
	wordOpen := false
	parenDepth := 0
	var expansionDepths []int
	for i := start + 1; i < len(command); i++ {
		c := command[i]
		if escaped {
			escaped = false
			if c != '\n' {
				wordOpen = true
			}
			continue
		}
		if c == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote == 0 {
			if end, comment := bundledShellCommentEnd(command, i, wordOpen); comment {
				i = end - 1
				continue
			}
			if c == '`' {
				return command[start+1 : i], i + 1, true
			}
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			wordOpen = true
			continue
		}
		if c == '$' && i+1 < len(command) && command[i+1] == '(' {
			expansionDepths = append(expansionDepths, parenDepth)
			wordOpen = false
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			wordOpen = true
			continue
		}
		switch c {
		case '(':
			parenDepth++
			wordOpen = false
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
			if len(expansionDepths) > 0 && parenDepth == expansionDepths[len(expansionDepths)-1] {
				expansionDepths = expansionDepths[:len(expansionDepths)-1]
				wordOpen = true
			} else {
				wordOpen = false
			}
		case ' ', '\t', '\r', '\n', '|', '&', ';', '<', '>':
			wordOpen = false
		default:
			wordOpen = true
		}
	}
	return "", 0, false
}

func bundledEscapedBacktickSubstitution(command string, start int) (string, int, bool) {
	var quote byte
	escaped := false
	wordOpen := false
	parenDepth := 0
	var expansionDepths []int
	for i := start + 2; i+1 < len(command); i++ {
		c := command[i]
		if escaped {
			escaped = false
			if c != '\n' {
				wordOpen = true
			}
			continue
		}
		if quote == 0 {
			if end, comment := bundledShellCommentEnd(command, i, wordOpen); comment {
				i = end - 1
				continue
			}
		}
		if quote == '\'' {
			if c == '\'' {
				quote = 0
			}
			wordOpen = true
			continue
		}
		if c == '\\' {
			if command[i+1] == '`' {
				return command[start+2 : i], i + 2, true
			}
			escaped = true
			continue
		}
		if quote == '"' {
			if c == '"' {
				quote = 0
			}
			wordOpen = true
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			wordOpen = true
			continue
		}
		if c == '$' && i+1 < len(command) && command[i+1] == '(' {
			expansionDepths = append(expansionDepths, parenDepth)
			wordOpen = false
			continue
		}
		switch c {
		case '(':
			parenDepth++
			wordOpen = false
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
			if len(expansionDepths) > 0 && parenDepth == expansionDepths[len(expansionDepths)-1] {
				expansionDepths = expansionDepths[:len(expansionDepths)-1]
				wordOpen = true
			} else {
				wordOpen = false
			}
		case ' ', '\t', '\r', '\n', '|', '&', ';', '<', '>':
			wordOpen = false
		default:
			wordOpen = true
		}
	}
	return "", 0, false
}

func bundledNetworkExecutable(command string) string {
	segments, _ := bundledCommandSegments(command)
	network, _, _ := bundledNetworkAnalysis(segments)
	return network
}

func bundledNetworkAnalysis(segments []string) (string, bool, bool) {
	return bundledNetworkAnalysisDepth(segments, 0)
}

func bundledNetworkAnalysisDepth(segments []string, depth int) (string, bool, bool) {
	if depth > maxBundledShellExpansionDepth {
		return "", false, false
	}
	network, remotePOST, complete := "", false, true
	for _, segment := range segments {
		words, parsed := bundledShellWords(segment)
		name, post, nestedComplete := bundledNetworkArgv(words, segment, depth)
		complete = complete && parsed && nestedComplete
		if network == "" {
			network = name
		}
		remotePOST = remotePOST || post
	}
	return network, remotePOST, complete
}

// bundledShellWords keeps whitespace inside quotes and shell expansions within
// one word, so an assignment value cannot shift executable detection.
func bundledShellWords(command string) ([]string, bool) {
	var words []string
	start := -1
	var quote byte
	escaped := false
	appendWord := func(end int) {
		if start >= 0 {
			words = append(words, command[start:end])
			start = -1
		}
	}
	appendIncomplete := func(at int) []string {
		if start < 0 {
			start = at
		}
		appendWord(len(command))
		return words
	}
	for i := 0; i < len(command); i++ {
		c := command[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && quote != '\'' {
			if start < 0 {
				start = i
			}
			escaped = true
			continue
		}
		if quote == '\'' {
			if c == '\'' {
				quote = 0
			}
			continue
		}
		if c == '$' {
			if end, ok := bundledShellExpansionEnd(command, i); ok {
				if start < 0 {
					start = i
				}
				i = end - 1
				continue
			} else if bundledShellExpansionOpener(command, i) {
				return appendIncomplete(i), false
			}
		}
		if c == '`' {
			if _, end, ok := bundledBacktickSubstitution(command, i); ok {
				if start < 0 {
					start = i
				}
				i = end - 1
				continue
			} else {
				return appendIncomplete(i), false
			}
		}
		if quote == '"' {
			if c == '"' {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			if start < 0 {
				start = i
			}
			quote = c
			continue
		}
		switch c {
		case ' ', '\t', '\r', '\n':
			appendWord(i)
		default:
			if start < 0 {
				start = i
			}
		}
	}
	appendWord(len(command))
	return words, true
}

func bundledShellExpansionOpener(command string, start int) bool {
	return start+1 < len(command) && command[start] == '$' && (command[start+1] == '(' || command[start+1] == '{')
}

func bundledShellExpansionEnd(command string, start int) (int, bool) {
	if start+1 < len(command) && command[start] == '$' && command[start+1] == '{' {
		return bundledParameterExpansionEnd(command, start)
	}
	return bundledShellCommandExpansionEnd(command, start)
}

func bundledShellCommandExpansionEnd(command string, start int) (int, bool) {
	if start+2 < len(command) && command[start] == '$' && command[start+1] == '(' && command[start+2] == '(' {
		_, end, ok := bundledArithmeticExpansion(command, start)
		return end, ok
	}
	if start+1 < len(command) && command[start] == '$' && command[start+1] == '(' {
		_, end, ok := bundledCommandSubstitution(command, start)
		return end, ok
	}
	return 0, false
}

func bundledParameterExpansionEnd(command string, start int) (int, bool) {
	depth := 1
	var quote byte
	quoteDepth := 0
	escaped := false
	for i := start + 2; i < len(command); i++ {
		c := command[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && quote != '\'' {
			if quote != '"' || i+1 < len(command) && strings.ContainsRune("$`\"\\\n", rune(command[i+1])) {
				escaped = true
				continue
			}
		}
		if quote == '\'' {
			if c == '\'' {
				quote = 0
			}
			continue
		}
		if c == '$' && i+1 < len(command) && command[i+1] == '{' {
			depth++
			i++
			continue
		}
		if quote == '"' {
			if c == '"' {
				quote = 0
				continue
			}
			if c == '$' {
				if end, ok := bundledShellCommandExpansionEnd(command, i); ok {
					i = end - 1
				} else if bundledShellExpansionOpener(command, i) {
					return 0, false
				}
			} else if c == '`' {
				if _, end, ok := bundledBacktickSubstitution(command, i); ok {
					i = end - 1
				} else {
					return 0, false
				}
			} else if c == '}' && depth > quoteDepth {
				depth--
				if depth == 0 {
					return i + 1, true
				}
			}
			continue
		}
		if c == '$' {
			if end, ok := bundledShellCommandExpansionEnd(command, i); ok {
				i = end - 1
				continue
			} else if bundledShellExpansionOpener(command, i) {
				return 0, false
			}
		}
		if c == '`' {
			if _, end, ok := bundledBacktickSubstitution(command, i); ok {
				i = end - 1
				continue
			} else {
				return 0, false
			}
		}
		if c == '\'' || c == '"' {
			quote = c
			quoteDepth = depth
			continue
		}
		if c == '}' {
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}
	return 0, false
}

// Inspect one executable position, unwrapping supported command prefixes.
// Nested shell scripts are parsed as separate segments, not as echoed arguments.
func bundledNetworkArgv(words []string, segment string, depth int) (string, bool, bool) {
	for len(words) > 0 {
		word := strings.Trim(words[0], "\"'")
		words = words[1:]
		if bundledAssignmentWord(word) {
			continue
		}
		name := strings.TrimSuffix(strings.ToLower(filepath.Base(word)), ".exe")
		switch name {
		case "if", "then", "elif", "else", "!", "while", "until", "do", "{", "time":
			continue
		case "sudo", "env", "command", "exec", "nohup", "timeout", "xargs":
			for len(words) > 0 && strings.HasPrefix(words[0], "-") {
				option := words[0]
				words = words[1:]
				if option == "--" {
					break
				}
				if name == "command" {
					flags := strings.TrimLeft(option, "-")
					if flags != "" && strings.Trim(flags, "pvV") == "" && strings.ContainsAny(flags, "vV") {
						return "", false, true
					}
				}
				takesValue := slices.Contains([]string{"-u", "-g", "--user", "--group"}, option)
				if name == "timeout" {
					takesValue = slices.Contains([]string{"-k", "--kill-after", "-s", "--signal"}, option)
				}
				if name == "xargs" {
					takesValue = slices.Contains([]string{"-a", "--arg-file", "-d", "--delimiter", "-E", "-I", "-L", "-n", "-P", "-s"}, option)
				}
				if takesValue && len(words) > 0 {
					words = words[1:]
				}
			}
			if name == "timeout" && len(words) > 0 {
				words = words[1:]
			}
			continue
		case "sh", "bash", "dash", "zsh", "ksh":
			for len(words) > 0 {
				option := strings.Trim(words[0], "\"'")
				words = words[1:]
				if strings.HasPrefix(option, "-") && !strings.HasPrefix(option, "--") && strings.Contains(option[1:], "c") && len(words) > 0 {
					body := words[0]
					if len(body) >= 2 && (body[0] == '\'' || body[0] == '"') && body[len(body)-1] == body[0] {
						body = body[1 : len(body)-1]
					}
					segments, complete := bundledCommandSegments(body)
					network, post, parsed := bundledNetworkAnalysisDepth(segments, depth+1)
					return network, post, complete && parsed
				}
				if !strings.HasPrefix(option, "-") || option == "--" {
					break
				}
			}
			return "", false, true
		case "git":
			for len(words) > 0 {
				option := words[0]
				words = words[1:]
				if slices.Contains([]string{"-C", "-c", "--git-dir", "--work-tree", "--namespace", "--config-env"}, option) {
					if len(words) > 0 {
						words = words[1:]
					}
					continue
				}
				if strings.HasPrefix(option, "-") {
					continue
				}
				if slices.Contains([]string{"clone", "fetch", "pull", "push", "ls-remote"}, option) {
					return "git", bundledPOST.MatchString(segment) && bundledRemoteURL(segment), true
				}
				break
			}
		default:
			tier, known := commandTiers[name]
			if (known && tier == TierNetwork) || name == "http" || name == "https" {
				return name, bundledPOST.MatchString(segment) && bundledRemoteURL(segment), true
			}
		}
		break
	}
	return "", false, true
}

func bundledAssignmentWord(word string) bool {
	equal := bundledAssignmentEquals(word)
	if equal < 0 {
		return false
	}
	name := word[:equal]
	name = strings.TrimSuffix(name, "+")
	if bundledShellIdentifier(name) {
		return true
	}
	bracket := strings.IndexByte(name, '[')
	if bracket <= 0 || !strings.HasSuffix(name, "]") || !bundledShellIdentifier(name[:bracket]) {
		return false
	}
	return len(name[bracket+1:len(name)-1]) > 0
}

func bundledAssignmentEquals(word string) int {
	var quote byte
	escaped := false
	bracketDepth := 0
	for i := 0; i < len(word); i++ {
		c := word[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		switch c {
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '=':
			if bracketDepth == 0 {
				return i
			}
		}
	}
	return -1
}

func bundledShellIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		letter := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if i == 0 {
			if !letter && c != '_' {
				return false
			}
		} else if !letter && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

func bundledRemoteURL(command string) bool {
	// Exclude known option values (payloads, headers, proxy URLs, output paths)
	// before interpreting positional URLs as destinations. This is a lexical
	// heuristic, not shell evaluation; quoting remains in the source only.
	words := bundledWords.FindAllString(command, -1)
	for i := 0; i < len(words); i++ {
		raw := strings.Trim(words[i], "\"'")
		raw = strings.Trim(strings.TrimPrefix(raw, "--url="), "\"'")
		switch raw {
		case "-d", "--data", "--data-raw", "--data-binary", "--data-urlencode", "--json", "-F", "--form", "--form-string", "-H", "--header", "-u", "--user", "-o", "--output", "-A", "--user-agent", "-e", "--referer", "-x", "--proxy", "-U", "--proxy-user", "-X", "--request", "--post-data", "--post-file":
			i++
			continue
		}
		if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			continue
		}
		host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
		if host != "localhost" && !net.ParseIP(host).IsLoopback() {
			return true
		}
	}
	return false
}

// No arbitrary argument, URL, event name, or helper output may reach a report.
// Keep fixed executable vocabulary as evidence and a digest so changed commands
// receive different acceptance fingerprints even when arguments are redacted.
func bundledEvidence(command string) string {
	return bundledEvidenceWithNetwork(command, bundledNetworkExecutable(command))
}

func bundledEvidenceWithNetwork(command, network string) string {
	verb := "command"
	if network != "" {
		verb = network
	} else {
		for _, name := range ExtractCommands(command) {
			// Return only fixed command vocabulary, never a user-defined path.
			base := strings.TrimSuffix(strings.ToLower(filepath.Base(name)), ".exe")
			if _, known := commandTiers[base]; known {
				verb = base
				break
			}
		}
	}
	return bundledRedactedEvidence(verb, []byte(command))
}

func inspectBundledSettings(doc map[string]any, file string, emit bundledEmitter, gap func(string)) {
	if value, ok := doc["apiKeyHelper"]; ok {
		if command, ok := value.(string); ok && strings.TrimSpace(command) != "" {
			emit(file, "$.apiKeyHelper", "bundled-helper-command", bundledEvidence(command))
		} else {
			gap("$.apiKeyHelper: expected nonempty command")
		}
	}
	if value, ok := doc["statusLine"]; ok {
		obj, ok := value.(map[string]any)
		command, valid := obj["command"].(string)
		if ok && obj["type"] == "command" && valid && strings.TrimSpace(command) != "" {
			emit(file, "$.statusLine.command", "bundled-helper-command", bundledEvidence(command))
		} else {
			gap("$.statusLine: unsupported helper shape")
		}
	}
	if value, ok := doc["permissions"]; ok {
		obj, ok := value.(map[string]any)
		if !ok {
			gap("$.permissions: expected object")
			return
		}
		if mode, ok := obj["defaultMode"]; ok {
			modeValue, ok := mode.(string)
			if !ok {
				gap("$.permissions.defaultMode: expected string")
			} else if modeValue == "bypassPermissions" {
				emit(file, "$.permissions.defaultMode", "bundled-permission", "mode: "+modeValue)
			}
		}
		if value, ok := obj["allow"]; ok {
			list, ok := value.([]any)
			if !ok {
				gap("$.permissions.allow: expected array")
				return
			}
			for i, value := range list {
				path := fmt.Sprintf("$.permissions.allow[%d]", i)
				permission, ok := value.(string)
				if !ok {
					gap(path + ": expected permission string")
					continue
				}
				if bundledBroadPermission(permission) {
					emit(file, path, "bundled-permission", bundledEvidence(permission))
				}
			}
		}
	}
}

func bundledDiagnosticEvidence(file, path string, contentDigest [32]byte) string {
	identity := []byte(fmt.Sprintf("%s\x00%s\x00%x", file, path, contentDigest))
	return bundledRedactedEvidence("declaration", identity)
}

func bundledRedactedEvidence(label string, content []byte) string {
	return fmt.Sprintf("%s [content redacted; sha256:%x]", label, sha256.Sum256(content))
}
