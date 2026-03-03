package audit

import (
	"regexp"
	"testing"
)

func TestCredentialTable_NoDuplicateIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, e := range credentialPaths {
		if seen[e.ID] {
			t.Errorf("duplicate credential entry ID: %s", e.ID)
		}
		seen[e.ID] = true
	}
}

func TestCredentialTable_ValidSeverities(t *testing.T) {
	for _, e := range credentialPaths {
		if !validSeverities[e.Severity] {
			t.Errorf("entry %s has invalid severity %q", e.ID, e.Severity)
		}
	}
}

func TestCredentialTable_TargetedSeverityProfile(t *testing.T) {
	want := map[string]string{
		"etc-passwd":    SeverityMedium,
		"shell-history": SeverityLow,
		"openvpn":       SeverityLow,
		"auth-log":      SeverityInfo,
	}

	got := make(map[string]string)
	for _, e := range credentialPaths {
		if _, ok := want[e.ID]; ok {
			got[e.ID] = e.Severity
		}
	}

	for id, wantSev := range want {
		gotSev, ok := got[id]
		if !ok {
			t.Errorf("entry %s not found in credential table", id)
			continue
		}
		if gotSev != wantSev {
			t.Errorf("entry %s severity = %s, want %s", id, gotSev, wantSev)
		}
	}
}

func TestCredentialTable_RegexCompile(t *testing.T) {
	for _, e := range credentialPaths {
		pathRe := e.PathRe
		if e.IsHome {
			pathRe = homePrefix + pathRe
		}
		if _, err := regexp.Compile(pathRe); err != nil {
			t.Errorf("entry %s: regex does not compile: %v", e.ID, err)
		}
	}
}

func TestCredentialTable_MethodsNonZero(t *testing.T) {
	for _, e := range credentialPaths {
		if e.Methods == 0 {
			t.Errorf("entry %s has zero methods", e.ID)
		}
	}
}

func TestCredentialYAMLRules_Count(t *testing.T) {
	rules := credentialYAMLRules()
	if len(rules) == 0 {
		t.Fatal("expected non-zero credential rules")
	}

	// Count expected rules: for each entry, count set bits in Methods.
	// Plus 1 for the catch-all.
	expected := 0
	for _, e := range credentialPaths {
		for _, mm := range methodMeta {
			if e.Methods&mm.Bit != 0 {
				expected++
			}
		}
	}
	expected++ // catch-all
	if len(rules) != expected {
		t.Errorf("expected %d rules, got %d", expected, len(rules))
	}
}

func TestCredentialYAMLRules_NoDuplicateIDs(t *testing.T) {
	rules := credentialYAMLRules()
	seen := make(map[string]bool)
	for _, r := range rules {
		if seen[r.ID] {
			t.Errorf("duplicate generated rule ID: %s", r.ID)
		}
		seen[r.ID] = true
	}
}

func TestCredentialYAMLRules_AllCompile(t *testing.T) {
	rules := credentialYAMLRules()
	for _, r := range rules {
		if _, err := regexp.Compile(r.Regex); err != nil {
			t.Errorf("rule %s: regex does not compile: %v", r.ID, err)
		}
		if r.Exclude != "" {
			if _, err := regexp.Compile(r.Exclude); err != nil {
				t.Errorf("rule %s: exclude regex does not compile: %v", r.ID, err)
			}
		}
	}
}

func TestCatchAll_FiresForUnknownDotdir(t *testing.T) {
	rule := catchAllRule()
	re := regexp.MustCompile(rule.Regex)
	excl := regexp.MustCompile(rule.Exclude)

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"unknown dotdir", "cat ~/.secret-stuff/key", true},
		{"unknown dotdir $HOME", "cat $HOME/.secret-stuff/key", true},
		{"unknown dotdir ${HOME}", "cat ${HOME}/.secret-stuff/key", true},
		{"safe bashrc", "cat ~/.bashrc/", false}, // bashrc is not a dir
		{"head unknown", "head ~/.some-tool/config", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := re.MatchString(tt.input)
			excluded := excl.MatchString(tt.input)
			got := matched && !excluded
			if got != tt.want {
				t.Errorf("input %q: matched=%v excluded=%v got=%v want=%v", tt.input, matched, excluded, got, tt.want)
			}
		})
	}
}

func TestCatchAll_SafeDotfilesExcluded(t *testing.T) {
	rule := catchAllRule()
	re := regexp.MustCompile(rule.Regex)
	excl := regexp.MustCompile(rule.Exclude)

	for _, f := range safeDotfiles {
		// Safe dotfiles are files, not dirs — the catch-all regex requires trailing /
		// so they won't match the main regex anyway. But if someone writes "cat ~/.bashrc/"
		// the exclude should also catch it.
		input := "cat ~/" + f + "/"
		if re.MatchString(input) && !excl.MatchString(input) {
			t.Errorf("safe dotfile %q should be excluded", f)
		}
	}
}

func TestCredentialPathRegex_HomeVariants(t *testing.T) {
	re := CredentialPathRegex()
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"tilde ssh", "~/.ssh/id_rsa", true},
		{"$HOME ssh", "$HOME/.ssh/id_rsa", true},
		{"${HOME} ssh", "${HOME}/.ssh/id_rsa", true},
		{"tilde aws", "~/.aws/credentials", true},
		{"$HOME aws", "$HOME/.aws/credentials", true},
		{"etc shadow", "/etc/shadow", true},
		{"etc passwd", "/etc/passwd", true},
		{"tilde gnupg", "~/.gnupg/pubring.kbx", true},
		{"tilde kube", "~/.kube/config", true},
		{"tilde vault", "~/.vault-token", true},
		{"tilde git-creds", "~/.git-credentials", true},
		{"tilde netrc", "~/.netrc", true},
		{"tilde azure", "~/.azure/token", true},
		{"tilde gcloud", "~/.gcloud/credentials.db", true},
		{"tilde docker", "~/.docker/config.json", true},
		{"tilde gh cli", "~/.config/gh/hosts.yml", true},
		{"tilde bash_history", "~/.bash_history", true},
		{"ssl private", "/etc/ssl/private/server.key", true},
		{"ssh host key", "/etc/ssh/ssh_host_rsa_key", true},
		// New entries from security review
		{"tilde pgpass", "~/.pgpass", true},
		{"tilde my.cnf", "~/.my.cnf", true},
		{"tilde cargo", "~/.cargo/credentials.toml", true},
		{"tilde op", "~/.op/session", true},
		{"tilde age", "~/.config/age/keys.txt", true},
		{"envrc", ".envrc", true},
		{"safe path", "/tmp/myfile", false},
		{"safe dotfile", "~/.bashrc", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := re.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("input %q: got %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSSHPublicKey_NotFlagged(t *testing.T) {
	// SSH public keys (.pub) should NOT trigger credential-access (Finding 2).
	safe := []string{
		"cat ~/.ssh/id_rsa.pub",
		"cat ~/.ssh/id_ed25519.pub",
		"cat $HOME/.ssh/id_ecdsa.pub",
	}
	for _, input := range safe {
		t.Run(input, func(t *testing.T) {
			findings := ScanContent([]byte(input), "SKILL.md")
			for _, f := range findings {
				if f.Pattern == "credential-access" {
					t.Errorf("SSH public key should NOT trigger credential-access: %s", f.Message)
				}
			}
		})
	}

	// SSH private keys SHOULD still trigger.
	dangerous := []string{
		"cat ~/.ssh/id_rsa",
		"cat ~/.ssh/id_ed25519",
	}
	for _, input := range dangerous {
		t.Run("private/"+input, func(t *testing.T) {
			findings := ScanContent([]byte(input), "SKILL.md")
			found := false
			for _, f := range findings {
				if f.Pattern == "credential-access" && f.Severity == SeverityCritical {
					found = true
				}
			}
			if !found {
				t.Errorf("SSH private key should trigger CRITICAL credential-access: %+v", findings)
			}
		})
	}
}

func TestExcludeRe_Compiles(t *testing.T) {
	for _, e := range credentialPaths {
		if e.ExcludeRe != "" {
			if _, err := regexp.Compile(e.ExcludeRe); err != nil {
				t.Errorf("entry %s: ExcludeRe does not compile: %v", e.ID, err)
			}
		}
	}
}
