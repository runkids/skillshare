package audit

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// CommandTier classifies shell commands into behavioral safety tiers.
// Tiers are orthogonal to pattern-based severity — they describe the
// *kind* of action rather than a specific dangerous invocation.
type CommandTier int

const (
	TierReadOnly    CommandTier = iota // T0: cat, ls, grep, echo
	TierMutating                       // T1: mkdir, cp, mv, touch
	TierDestructive                    // T2: rm, dd, mkfs, kill
	TierNetwork                        // T3: curl, wget, ssh, nc
	TierPrivilege                      // T4: sudo, su, chown, mount
	TierStealth                        // T5: history -c, shred, unset HISTFILE
	TierInterpreter                    // T6: python, node, ruby, perl, lua, php
	tierCount                          // sentinel — must be last
)

// TierLabel returns a human-readable label for the tier.
func (t CommandTier) TierLabel() string {
	switch t {
	case TierReadOnly:
		return "read-only"
	case TierMutating:
		return "mutating"
	case TierDestructive:
		return "destructive"
	case TierNetwork:
		return "network"
	case TierPrivilege:
		return "privilege"
	case TierStealth:
		return "stealth"
	case TierInterpreter:
		return "interpreter"
	default:
		return "unknown"
	}
}

// TierProfile accumulates command-tier counts for a skill.
// Compile-time assertion: Counts array size must match tierCount.
var _ [tierCount]int = [7]int{}

// TierProfile accumulates command-tier counts for a skill.
type TierProfile struct {
	Counts [tierCount]int `json:"counts"` // indexed by CommandTier (T0–T6)
	Total  int            `json:"total"`
}

// Add increments the count for the given tier.
func (p *TierProfile) Add(t CommandTier) {
	if t >= 0 && int(t) < len(p.Counts) {
		p.Counts[t]++
		p.Total++
	}
}

// Merge adds another profile's counts into this one.
func (p *TierProfile) Merge(other TierProfile) {
	for i := range p.Counts {
		p.Counts[i] += other.Counts[i]
	}
	p.Total += other.Total
}

// HasTier returns true if the tier has a non-zero count.
func (p *TierProfile) HasTier(t CommandTier) bool {
	if t >= 0 && int(t) < len(p.Counts) {
		return p.Counts[t] > 0
	}
	return false
}

// IsEmpty returns true if no commands were classified.
func (p *TierProfile) IsEmpty() bool { return p.Total == 0 }

// NonZeroTiers returns the tier labels with non-zero counts, ordered T0→T6.
func (p *TierProfile) NonZeroTiers() []string {
	var tiers []string
	for i, c := range p.Counts {
		if c > 0 {
			tiers = append(tiers, CommandTier(i).TierLabel())
		}
	}
	return tiers
}

// String returns a compact summary like "destructive:3 network:2".
func (p *TierProfile) String() string {
	parts := make([]string, 0, int(tierCount))
	for i, c := range p.Counts {
		if c > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", CommandTier(i).TierLabel(), c))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " ")
}

// commandTiers maps command basenames to their tier classification.
var commandTiers = map[string]CommandTier{
	// T0: read-only
	"cat": TierReadOnly, "head": TierReadOnly, "tail": TierReadOnly,
	"less": TierReadOnly, "more": TierReadOnly, "bat": TierReadOnly,
	"ls": TierReadOnly, "dir": TierReadOnly, "tree": TierReadOnly,
	"find": TierReadOnly, "locate": TierReadOnly, "which": TierReadOnly,
	"whereis": TierReadOnly, "file": TierReadOnly, "stat": TierReadOnly,
	"wc": TierReadOnly, "du": TierReadOnly, "df": TierReadOnly,
	"grep": TierReadOnly, "rg": TierReadOnly, "ag": TierReadOnly,
	"awk": TierReadOnly, "sort": TierReadOnly, "uniq": TierReadOnly,
	"cut": TierReadOnly, "tr": TierReadOnly, "diff": TierReadOnly,
	"echo": TierReadOnly, "printf": TierReadOnly, "env": TierReadOnly,
	"printenv": TierReadOnly, "id": TierReadOnly, "whoami": TierReadOnly,
	"uname": TierReadOnly, "hostname": TierReadOnly, "date": TierReadOnly,
	"pwd": TierReadOnly, "realpath": TierReadOnly, "basename": TierReadOnly,
	"dirname": TierReadOnly, "readlink": TierReadOnly, "test": TierReadOnly,
	"true": TierReadOnly, "false": TierReadOnly, "type": TierReadOnly,
	"sha256sum": TierReadOnly, "md5sum": TierReadOnly, "sha1sum": TierReadOnly,
	"jq": TierReadOnly, "yq": TierReadOnly, "xargs": TierReadOnly,

	// T1: mutating
	"mkdir": TierMutating, "cp": TierMutating, "mv": TierMutating,
	"touch": TierMutating, "ln": TierMutating, "install": TierMutating,
	"sed": TierMutating, "tee": TierMutating, "patch": TierMutating,
	"chmod": TierMutating, "tar": TierMutating, "zip": TierMutating,
	"unzip": TierMutating, "gzip": TierMutating, "gunzip": TierMutating,
	"git": TierMutating, "npm": TierMutating, "yarn": TierMutating,
	"pnpm": TierMutating, "pip": TierMutating, "pip3": TierMutating,
	"go": TierMutating, "cargo": TierMutating, "make": TierMutating,
	"cmake": TierMutating,
	"rustc": TierMutating, "gcc": TierMutating, "clang": TierMutating,
	"javac": TierMutating, "mvn": TierMutating, "gradle": TierMutating,

	// T2: destructive
	"rm": TierDestructive, "rmdir": TierDestructive,
	"dd": TierDestructive, "mkfs": TierDestructive,
	"kill": TierDestructive, "killall": TierDestructive, "pkill": TierDestructive,
	"truncate": TierDestructive, "wipefs": TierDestructive,
	"fdisk": TierDestructive, "parted": TierDestructive,
	"reboot": TierDestructive, "shutdown": TierDestructive, "halt": TierDestructive,
	"init": TierDestructive,

	// T3: network
	"curl": TierNetwork, "wget": TierNetwork, "fetch": TierNetwork,
	"ssh": TierNetwork, "scp": TierNetwork, "sftp": TierNetwork, "rsync": TierNetwork,
	"nc": TierNetwork, "ncat": TierNetwork, "netcat": TierNetwork, "socat": TierNetwork,
	"nmap": TierNetwork, "ping": TierNetwork, "traceroute": TierNetwork,
	"dig": TierNetwork, "nslookup": TierNetwork, "host": TierNetwork,
	"telnet": TierNetwork, "ftp": TierNetwork,
	"aria2c": TierNetwork, "axel": TierNetwork,
	"netstat": TierNetwork, "ss": TierNetwork, "iptables": TierNetwork,

	// T4: privilege
	"sudo": TierPrivilege, "su": TierPrivilege, "doas": TierPrivilege,
	"chown": TierPrivilege, "chgrp": TierPrivilege,
	"mount": TierPrivilege, "umount": TierPrivilege,
	"systemctl": TierPrivilege, "service": TierPrivilege,
	"useradd": TierPrivilege, "usermod": TierPrivilege, "userdel": TierPrivilege,
	"groupadd": TierPrivilege, "groupmod": TierPrivilege, "groupdel": TierPrivilege,
	"visudo": TierPrivilege, "passwd": TierPrivilege,
	"crontab": TierPrivilege, "at": TierPrivilege,
	"modprobe": TierPrivilege, "insmod": TierPrivilege, "rmmod": TierPrivilege,

	// T5: stealth
	"shred": TierStealth,

	// T6: interpreter (Turing-complete — can execute arbitrary operations)
	"python": TierInterpreter, "python3": TierInterpreter,
	"node": TierInterpreter, "ruby": TierInterpreter,
	"perl": TierInterpreter, "lua": TierInterpreter,
	"php": TierInterpreter, "bun": TierInterpreter,
	"deno": TierInterpreter, "npx": TierInterpreter,
	"tsx": TierInterpreter, "pwsh": TierInterpreter,
	"powershell": TierInterpreter,
}

// stealthPatterns detects stealth commands that need context beyond basename.
var stealthPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bhistory\s+-[cd]`),
	regexp.MustCompile(`\bunset\s+HISTFILE\b`),
	regexp.MustCompile(`\bunset\s+HISTSIZE\b`),
	regexp.MustCompile(`\bexport\s+HISTFILE\s*=\s*/dev/null\b`),
	regexp.MustCompile(`\bexport\s+HISTSIZE\s*=\s*0\b`),
}

// ClassifyCommand returns the tier for a command basename and whether it was found.
func ClassifyCommand(name string) (CommandTier, bool) {
	// Strip path prefix (e.g. /usr/bin/curl → curl).
	base := filepath.Base(name)
	if t, ok := commandTiers[base]; ok {
		return t, ok
	}
	// Try stripping version suffix (e.g. python3.11 → python3).
	if idx := strings.LastIndex(base, "."); idx > 0 {
		if t, ok := commandTiers[base[:idx]]; ok {
			return t, ok
		}
	}
	return TierReadOnly, false
}

// cmdSplitRe splits a line on shell operators: pipe, chain, semicolon, subshell.
var cmdSplitRe = regexp.MustCompile(`[|&;$()]`)

// commentRe detects shell-style comments.
var commentRe = regexp.MustCompile(`(?:^|\s)#`)

// ExtractCommands extracts command names from a shell line.
// It splits on pipe, chain, and semicolon operators, then takes the first
// token from each segment as the command name.
func ExtractCommands(line string) []string {
	// Strip comments.
	if loc := commentRe.FindStringIndex(line); loc != nil {
		line = line[:loc[0]]
	}

	segments := cmdSplitRe.Split(line, -1)
	var cmds []string
	seen := make(map[string]bool)

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		// Skip variable assignments (VAR=value).
		// Skip environment prefixes before a command (KEY=val cmd).
		fields := strings.Fields(seg)
		cmdIdx := 0
		for cmdIdx < len(fields) && strings.Contains(fields[cmdIdx], "=") {
			cmdIdx++
		}
		if cmdIdx >= len(fields) {
			continue
		}
		cmd := filepath.Base(fields[cmdIdx])
		// Skip `env` prefix — classify the inner command instead.
		if cmd == "env" && cmdIdx+1 < len(fields) {
			cmdIdx++
			cmd = filepath.Base(fields[cmdIdx])
		}
		if cmd == "" || cmd == "." || cmd == "-" {
			continue
		}
		if !seen[cmd] {
			seen[cmd] = true
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

// classifyLineCommands checks a single line for stealth patterns and
// extracts/classifies commands, accumulating into profile.
func classifyLineCommands(line string, profile *TierProfile) {
	for _, re := range stealthPatterns {
		if re.MatchString(line) {
			profile.Add(TierStealth)
			break
		}
	}
	for _, cmd := range ExtractCommands(line) {
		if tier, ok := ClassifyCommand(cmd); ok {
			profile.Add(tier)
		}
	}
}

// DetectCommandTiers scans file content (script/config) and returns a TierProfile.
// Every line is analysed for shell commands.
func DetectCommandTiers(content []byte) TierProfile {
	var profile TierProfile
	for line := range strings.SplitSeq(string(content), "\n") {
		classifyLineCommands(line, &profile)
	}
	return profile
}

// DetectCommandTiersInMarkdown scans only fenced code blocks within Markdown content.
func DetectCommandTiersInMarkdown(content []byte) TierProfile {
	var profile TierProfile
	text := string(content)
	inCodeFence := false
	fenceMarker := ""

	for line := range strings.SplitSeq(text, "\n") {
		if marker, ok := detectFenceMarker(line); ok {
			if !inCodeFence {
				inCodeFence = true
				fenceMarker = marker
			} else if marker == fenceMarker {
				inCodeFence = false
				fenceMarker = ""
			}
			continue
		}
		if !inCodeFence {
			continue
		}
		classifyLineCommands(line, &profile)
	}
	return profile
}

// TierCombinationFindings generates findings from a TierProfile when
// dangerous tier combinations are detected. These are complementary to
// pattern-based rules — patterns catch specific dangerous invocations,
// tier findings catch profile-level risk combinations.
func TierCombinationFindings(p TierProfile) []Finding {
	var findings []Finding

	// T5 stealth → always CRITICAL.
	if p.HasTier(TierStealth) {
		findings = append(findings, syntheticFinding(SeverityCritical, "tier-stealth",
			fmt.Sprintf("detection evasion commands found (%d occurrence(s))", p.Counts[TierStealth])))
	}

	// T2 destructive + T3 network → HIGH (data exfiltration risk).
	if p.HasTier(TierDestructive) && p.HasTier(TierNetwork) {
		findings = append(findings, syntheticFinding(SeverityHigh, "tier-destructive-network",
			fmt.Sprintf("destructive + network commands found (%d destructive, %d network) — data exfiltration risk", p.Counts[TierDestructive], p.Counts[TierNetwork])))
	}

	// T3 network count > 5 → MEDIUM (abnormally high network usage).
	if p.Counts[TierNetwork] > 5 {
		findings = append(findings, syntheticFinding(SeverityMedium, "tier-network-heavy",
			fmt.Sprintf("abnormally high density of network commands (%d)", p.Counts[TierNetwork])))
	}

	// T6 interpreter present → INFO (advisory: Turing-complete runtime).
	if p.HasTier(TierInterpreter) {
		findings = append(findings, syntheticFinding(SeverityInfo, "tier-interpreter",
			fmt.Sprintf("interpreter commands found (%d occurrence(s)) — Turing-complete runtime can execute arbitrary operations", p.Counts[TierInterpreter])))
	}

	// T6 interpreter + T3 network → MEDIUM (interpreter can generate arbitrary requests).
	if p.HasTier(TierInterpreter) && p.HasTier(TierNetwork) {
		findings = append(findings, syntheticFinding(SeverityMedium, "tier-interpreter-network",
			fmt.Sprintf("interpreter + network commands found (%d interpreter, %d network) — interpreter can generate arbitrary network requests", p.Counts[TierInterpreter], p.Counts[TierNetwork])))
	}

	return findings
}
