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
	"unicode"

	"mvdan.cc/sh/v3/syntax"

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
		parsed := true
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
				networkExecutable, remotePOST, parsed = bundledNetworkArgv(argv, 0)
				if !parsed {
					gap(path + ".command: shell parsing incomplete")
				}
			}
		}
		if !argsValid {
			networkExecutable, remotePOST, parsed = bundledShellNetwork(command, 0)
			if argsProvided {
				remotePOST = false
			}
			if !parsed {
				gap(path + ".command: shell parsing incomplete")
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
		if bundledRemoteURL([]string{destination}) {
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

// Nested shell scripts (`bash -c '...'`) are parsed again; this bounds that
// recursion. Substitutions and compound commands need no limit of their own.
const maxBundledShellDepth = 32

// bundledShellNetwork parses a hook command as bash and reports the first
// network executable it runs, whether a network command POSTs to a remote
// host, and whether the whole command parsed. Statements before a syntax error
// are still inspected, as a shell runs them before it reaches the error.
// Nothing is executed; quoting, comments, substitutions, loops and line
// continuations follow the parser.
func bundledShellNetwork(command string, depth int) (network string, post, complete bool) {
	if depth > maxBundledShellDepth {
		return "", false, false
	}
	complete = true
	merge := func(name string, p, nestedComplete bool) {
		complete = complete && nestedComplete
		if network == "" {
			network = name
		}
		post = post || p
	}
	visit := func(node syntax.Node) bool {
		if stmt, ok := node.(*syntax.Stmt); ok {
			// A script fed to a shell on stdin runs like `bash -c`.
			if script, ok := bundledShellStdin(command, stmt); ok {
				merge(bundledShellNetwork(script, depth+1))
			}
			return true
		}
		call, ok := node.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		words := make([]string, len(call.Args))
		for i, word := range call.Args {
			words[i] = bundledWordText(command, word)
		}
		merge(bundledNetworkArgv(words, depth))
		return true
	}
	failed := false
	// Drain the iterator after an error: it may yield again, and breaking out
	// of the loop then panics.
	for stmt, err := range syntax.NewParser().StmtsSeq(strings.NewReader(command)) {
		if failed {
			continue
		}
		if err != nil {
			failed = true
			// bash still runs some forms the parser rejects, such as an array
			// element assigned before a command, so treat every word as a
			// possible executable rather than trusting the partial parse.
			name, p := bundledLexicalNetwork(command)
			merge(name, p, false)
			continue
		}
		syntax.Walk(stmt, visit)
	}
	return network, post, complete
}

// bundledShellStdin returns the script a heredoc or here-string feeds to a
// shell, as in `bash <<'EOF'`.
func bundledShellStdin(src string, stmt *syntax.Stmt) (string, bool) {
	call, ok := stmt.Cmd.(*syntax.CallExpr)
	if !ok || len(call.Args) == 0 {
		return "", false
	}
	switch strings.ToLower(filepath.Base(bundledWordText(src, call.Args[0]))) {
	case "sh", "bash", "dash", "zsh", "ksh":
	default:
		return "", false
	}
	for _, r := range stmt.Redirs {
		if r.N != nil && r.N.Value != "0" {
			continue
		}
		switch {
		case (r.Op == syntax.Hdoc || r.Op == syntax.DashHdoc) && r.Hdoc != nil && len(r.Hdoc.Parts) > 0:
			// The raw body keeps line continuations for the nested parse.
			return src[r.Hdoc.Pos().Offset():r.Hdoc.End().Offset()], true
		case r.Op == syntax.WordHdoc && r.Word != nil:
			return bundledWordText(src, r.Word), true
		}
	}
	return "", false
}

func bundledLexicalNetwork(command string) (string, bool) {
	words := strings.FieldsFunc(command, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(";|&()<>`\"'\\$", r)
	})
	for i, word := range words {
		if name, ok := bundledNetworkName(word); ok {
			return name, bundledPOSTArgs(words[i+1:])
		}
	}
	return "", false
}

// bundledNetworkName reports whether word names a network executable.
func bundledNetworkName(word string) (string, bool) {
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(word)), ".exe")
	tier, known := commandTiers[name]
	return name, (known && tier == TierNetwork) || name == "http" || name == "https"
}

// bundledWordText returns a word as the shell would pass it, with quotes and
// escapes removed. Expansions keep their source text, so `$URL` or `$(cmd)`
// never reads as a literal executable or destination.
func bundledWordText(src string, word *syntax.Word) string {
	var b strings.Builder
	var parts func([]syntax.WordPart, bool)
	parts = func(list []syntax.WordPart, quoted bool) {
		for _, part := range list {
			switch part := part.(type) {
			case *syntax.Lit:
				b.WriteString(bundledUnescape(part.Value, quoted))
			case *syntax.SglQuoted:
				b.WriteString(part.Value)
			case *syntax.DblQuoted:
				parts(part.Parts, true)
			default:
				b.WriteString(src[part.Pos().Offset():part.End().Offset()])
			}
		}
	}
	parts(word.Parts, false)
	return b.String()
}

// bundledUnescape removes backslash escapes. Inside double quotes a backslash
// escapes only $, `, ", \ and newline, and is kept before anything else.
func bundledUnescape(s string, quoted bool) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && (!quoted || strings.IndexByte("$`\"\\\n", s[i+1]) >= 0) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// Inspect one argv, unwrapping supported command prefixes. A nested shell
// script (`bash -c '...'`) is parsed as a script, not read as arguments.
func bundledNetworkArgv(words []string, depth int) (string, bool, bool) {
	for len(words) > 0 {
		word := words[0]
		words = words[1:]
		if bundledAssignmentWord(word) {
			continue
		}
		name := strings.TrimSuffix(strings.ToLower(filepath.Base(word)), ".exe")
		switch name {
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
				option := words[0]
				words = words[1:]
				if strings.HasPrefix(option, "-") && !strings.HasPrefix(option, "--") && strings.Contains(option[1:], "c") && len(words) > 0 {
					return bundledShellNetwork(words[0], depth+1)
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
					return "git", bundledPOSTArgs(words), true
				}
				break
			}
		default:
			if network, ok := bundledNetworkName(word); ok {
				return network, bundledPOSTArgs(words), true
			}
		}
		break
	}
	return "", false, true
}

// bundledPOSTArgs reports a POST or data flag together with a non-loopback
// HTTP(S) destination among one command's arguments.
func bundledPOSTArgs(args []string) bool {
	return bundledPOST.MatchString(" "+strings.Join(args, " ")) && bundledRemoteURL(args)
}

func bundledAssignmentWord(word string) bool {
	equal := strings.IndexByte(word, '=')
	return equal > 0 && bundledShellIdentifier(strings.TrimSuffix(word[:equal], "+"))
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

func bundledRemoteURL(args []string) bool {
	// Skip known option values (payloads, headers, proxy URLs, output paths)
	// before reading positional URLs as destinations.
	for i := 0; i < len(args); i++ {
		raw := strings.TrimPrefix(args[i], "--url=")
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
	network, _, _ := bundledShellNetwork(command, 0)
	return bundledEvidenceWithNetwork(command, network)
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
