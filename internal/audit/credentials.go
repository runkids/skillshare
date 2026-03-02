package audit

import (
	"fmt"
	"regexp"
	"strings"
)

// Access method bitmask for credential entries.
const (
	mRead     uint8 = 1 << iota // cat, head, tail, less, more, bat, tac, strings, base64, xxd, od, hexdump
	mCopy                       // ln, cp, install
	mRedirect                   // < path
	mDD                         // dd if=path
	mExfil                      // scp, rsync

	mAll         = mRead | mCopy | mRedirect | mDD | mExfil
	mReadCopyEx  = mRead | mCopy | mExfil
	mReadCopy    = mRead | mCopy
	mReadOnly    = mRead
)

// credentialEntry defines a single sensitive path family.
type credentialEntry struct {
	ID        string // stable descriptive ID, e.g. "ssh-private-key"
	PathRe    string // regex fragment (homePrefix is prepended for home-relative paths)
	Severity  string
	Message   string
	Methods   uint8
	IsHome    bool   // true → prepend homePrefix to PathRe
	ExcludeRe string // optional per-entry exclude regex (suppresses false positives)
}

// homePrefix matches ~, $HOME, or ${HOME} followed by optional /.
const homePrefix = `(?:\$\{?HOME\}?|~)/?`

// readCmds are commands that read file content.
var readCmds = []string{"cat", "head", "tail", "less", "more", "bat", "tac", "strings", "base64", "xxd", "od", "hexdump"}

// copyCmds are commands that copy/link files.
var copyCmds = []string{"ln", "cp", "install"}

// exfilCmds are commands that transfer files externally.
var exfilCmds = []string{"scp", "rsync"}

// credentialPaths is the exhaustive table of sensitive credential paths.
var credentialPaths = []credentialEntry{
	// ── CRITICAL ──
	{ID: "ssh-private-key", PathRe: `\.ssh/(id_|known_hosts|authorized_keys|config)`, Severity: SeverityCritical, Message: "Accessing SSH keys or config", Methods: mAll, IsHome: true, ExcludeRe: `\.ssh/id_[^/\s]+\.pub\b`},
	{ID: "env-file", PathRe: `\.env(rc)?\b`, Severity: SeverityCritical, Message: "Accessing .env secrets file", Methods: mReadOnly},
	{ID: "aws-credentials", PathRe: `\.aws/(credentials|config)`, Severity: SeverityCritical, Message: "Accessing AWS credentials", Methods: mAll, IsHome: true},
	{ID: "etc-shadow", PathRe: `/etc/(shadow|gshadow|master\.passwd)\b`, Severity: SeverityCritical, Message: "Reading highly sensitive system credential file", Methods: mAll},
	{ID: "git-credentials", PathRe: `\.git-credentials`, Severity: SeverityCritical, Message: "Accessing git credential store", Methods: mReadCopyEx, IsHome: true},
	{ID: "netrc", PathRe: `\.netrc\b`, Severity: SeverityCritical, Message: "Accessing .netrc credentials", Methods: mReadCopyEx, IsHome: true},
	{ID: "gnupg", PathRe: `\.gnupg/`, Severity: SeverityCritical, Message: "Accessing GnuPG keyring", Methods: mReadCopyEx, IsHome: true},
	{ID: "kube-config", PathRe: `\.kube/`, Severity: SeverityCritical, Message: "Accessing Kubernetes config", Methods: mReadCopyEx, IsHome: true},
	{ID: "vault-token", PathRe: `\.vault-token`, Severity: SeverityCritical, Message: "Accessing HashiCorp Vault token", Methods: mReadCopyEx, IsHome: true},
	{ID: "terraform-creds", PathRe: `\.terraform\.d/credentials`, Severity: SeverityCritical, Message: "Accessing Terraform credentials", Methods: mReadCopy, IsHome: true},
	{ID: "gnome-keyring", PathRe: `\.local/share/keyrings/`, Severity: SeverityCritical, Message: "Accessing GNOME keyring", Methods: mReadCopy, IsHome: true},
	{ID: "npmrc", PathRe: `\.npmrc`, Severity: SeverityCritical, Message: "Accessing .npmrc (may contain auth tokens)", Methods: mReadOnly, IsHome: true},
	{ID: "pypirc", PathRe: `\.pypirc`, Severity: SeverityCritical, Message: "Accessing .pypirc credentials", Methods: mReadOnly, IsHome: true},
	{ID: "gem-credentials", PathRe: `\.gem/credentials`, Severity: SeverityCritical, Message: "Accessing RubyGems credentials", Methods: mReadOnly, IsHome: true},
	{ID: "ssl-private", PathRe: `/etc/ssl/private/`, Severity: SeverityCritical, Message: "Accessing SSL private keys", Methods: mAll},
	{ID: "ssh-host-key", PathRe: `/etc/ssh/ssh_host_.*_key\b`, Severity: SeverityCritical, Message: "Accessing SSH host private key", Methods: mReadCopy},
	{ID: "pgpass", PathRe: `\.pgpass\b`, Severity: SeverityCritical, Message: "Accessing PostgreSQL password file", Methods: mReadCopyEx, IsHome: true},
	{ID: "mysql-cnf", PathRe: `\.my\.cnf\b`, Severity: SeverityCritical, Message: "Accessing MySQL client credentials", Methods: mReadOnly, IsHome: true},

	// ── HIGH ──
	{ID: "etc-passwd", PathRe: `/etc/(passwd|sudoers)\b`, Severity: SeverityHigh, Message: "Reading sensitive system account file", Methods: mAll},
	{ID: "azure-creds", PathRe: `\.azure/`, Severity: SeverityHigh, Message: "Accessing Azure CLI credentials", Methods: mReadCopyEx, IsHome: true},
	{ID: "gcloud-creds", PathRe: `\.gcloud/`, Severity: SeverityHigh, Message: "Accessing gcloud credentials", Methods: mReadCopyEx, IsHome: true},
	{ID: "docker-config", PathRe: `\.docker/config\.json`, Severity: SeverityHigh, Message: "Accessing Docker config (may contain registry auth)", Methods: mReadCopy, IsHome: true},
	{ID: "gh-cli-token", PathRe: `\.config/gh/hosts\.yml`, Severity: SeverityHigh, Message: "Accessing GitHub CLI token", Methods: mReadOnly, IsHome: true},
	{ID: "password-store", PathRe: `\.password-store/`, Severity: SeverityHigh, Message: "Accessing pass password store", Methods: mReadCopy, IsHome: true},
	{ID: "macos-keychain-user", PathRe: `Library/Keychains/`, Severity: SeverityHigh, Message: "Accessing macOS user keychain", Methods: mReadOnly, IsHome: true},
	{ID: "macos-keychain-sys", PathRe: `/Library/Keychains/`, Severity: SeverityHigh, Message: "Accessing macOS system keychain", Methods: mReadOnly},
	{ID: "terraformrc", PathRe: `\.terraformrc`, Severity: SeverityHigh, Message: "Accessing Terraform CLI config", Methods: mReadOnly, IsHome: true},
	{ID: "cargo-credentials", PathRe: `\.cargo/credentials`, Severity: SeverityHigh, Message: "Accessing Cargo registry credentials", Methods: mReadOnly, IsHome: true},
	{ID: "op-cli", PathRe: `\.op/`, Severity: SeverityHigh, Message: "Accessing 1Password CLI data", Methods: mReadCopyEx, IsHome: true},
	{ID: "age-keys", PathRe: `\.config/age/`, Severity: SeverityHigh, Message: "Accessing age encryption keys", Methods: mReadCopyEx, IsHome: true},

	// ── MEDIUM ──
	{ID: "shell-history", PathRe: `\.(bash_history|zsh_history|python_history|node_repl_history)`, Severity: SeverityMedium, Message: "Accessing shell/REPL history (may contain secrets)", Methods: mReadOnly, IsHome: true},
	{ID: "openvpn", PathRe: `/etc/openvpn/`, Severity: SeverityMedium, Message: "Accessing OpenVPN configuration", Methods: mReadOnly},

	// ── LOW ──
	{ID: "auth-log", PathRe: `/var/log/(auth\.log|secure)\b`, Severity: SeverityLow, Message: "Accessing authentication log", Methods: mReadOnly},
}

// safeDotfiles is the whitelist for the heuristic catch-all rule.
// These are common, safe dotfiles that should not trigger the catch-all.
var safeDotfiles = []string{
	".bashrc", ".bash_profile", ".zshrc", ".zprofile", ".profile",
	".gitconfig", ".gitignore", ".gitattributes",
	".vimrc", ".tmux.conf", ".screenrc", ".inputrc", ".editorconfig",
	".curlrc", ".wgetrc", ".nanorc",
}

// methodSuffix maps method bits to rule ID suffixes.
var methodMeta = []struct {
	Bit    uint8
	Suffix string // appended to base ID; empty for read (default)
	Label  string // for message prefix
}{
	{mRead, "", "Reading"},
	{mCopy, "-copy", "Copying"},
	{mRedirect, "-redirect", "Input redirect from"},
	{mDD, "-dd", "dd reading from"},
	{mExfil, "-exfil", "Exfiltrating"},
}

// credentialYAMLRules generates yamlRule entries from the credential table.
func credentialYAMLRules() []yamlRule {
	var rules []yamlRule
	for _, e := range credentialPaths {
		for _, mm := range methodMeta {
			if e.Methods&mm.Bit == 0 {
				continue
			}
			// Build the full path regex.
			pathRe := e.PathRe
			if e.IsHome {
				pathRe = homePrefix + pathRe
			}

			id := "credential-access-" + e.ID + mm.Suffix
			msg := mm.Label + " " + strings.ToLower(e.Message[0:1]) + e.Message[1:]
			regex := buildMethodRegex(mm.Bit, pathRe)

			yr := yamlRule{
				ID:       id,
				Severity: e.Severity,
				Pattern:  "credential-access",
				Message:  msg,
				Regex:    regex,
			}
			if e.ExcludeRe != "" {
				yr.Exclude = e.ExcludeRe
			}
			rules = append(rules, yr)
		}
	}
	// Heuristic catch-all: read commands accessing unknown dotdirs
	rules = append(rules, catchAllRule())
	return rules
}

// buildMethodRegex returns the regex for a specific access method + path.
func buildMethodRegex(method uint8, pathRe string) string {
	switch method {
	case mRead:
		return fmt.Sprintf(`\b(%s)\b\s+%s`, strings.Join(readCmds, "|"), pathRe)
	case mCopy:
		return fmt.Sprintf(`\b(%s)\b\s+(-\w+\s+)*%s`, strings.Join(copyCmds, "|"), pathRe)
	case mRedirect:
		return fmt.Sprintf(`<\s*%s`, pathRe)
	case mDD:
		return fmt.Sprintf(`\bdd\b\s+if=%s`, pathRe)
	case mExfil:
		return fmt.Sprintf(`\b(%s)\b\s+.*%s`, strings.Join(exfilCmds, "|"), pathRe)
	}
	return ""
}

// catchAllRule generates the INFO-level heuristic catch-all rule.
// Matches read commands accessing home-relative dot-directories not
// covered by the explicit table or the safe dotfiles whitelist.
func catchAllRule() yamlRule {
	// Collect all exclude patterns: safe dotfiles + known credential paths.
	var excludeParts []string

	// Safe dotfiles.
	escaped := make([]string, len(safeDotfiles))
	for i, f := range safeDotfiles {
		escaped[i] = regexp.QuoteMeta(f)
	}
	excludeParts = append(excludeParts,
		fmt.Sprintf(`\b(%s)\b\s+%s(%s)\b`,
			strings.Join(readCmds, "|"),
			homePrefix,
			strings.Join(escaped, "|"),
		),
	)

	// Auto-exclude home-relative paths already in the credential table.
	// Include command prefix to prevent bare path at end-of-line from suppressing
	// the catch-all for an unrelated unknown dotdir on the same line.
	cmdPrefix := fmt.Sprintf(`\b(%s)\b\s+`, strings.Join(readCmds, "|"))
	for _, e := range credentialPaths {
		if e.IsHome {
			excludeParts = append(excludeParts, cmdPrefix+homePrefix+e.PathRe)
		}
	}

	return yamlRule{
		ID:       "credential-access-unknown-dotdir",
		Severity: SeverityInfo,
		Pattern:  "credential-access",
		Message:  "Accessing unknown home dotfile/directory — verify intent",
		Regex: fmt.Sprintf(`\b(%s)\b\s+%s\.[a-z][\w.-]*/`,
			strings.Join(readCmds, "|"),
			homePrefix,
		),
		Exclude: strings.Join(excludeParts, "|"),
	}
}

// CredentialPathRegex returns a compiled regex matching all known credential
// paths from the table. Used by dataflow.go for taint source detection.
func CredentialPathRegex() *regexp.Regexp {
	var parts []string
	for _, e := range credentialPaths {
		p := e.PathRe
		if e.IsHome {
			p = homePrefix + p
		}
		parts = append(parts, p)
	}
	return regexp.MustCompile(`(?i)(` + strings.Join(parts, "|") + `)`)
}
