package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// metadataAnalyzer cross-references SKILL.md metadata (name, description)
// against the actual git source URL from the centralized metadata store.
// Detects social-engineering patterns: publisher mismatch and authority claims.
// Runs at skill scope after all files are walked.
type metadataAnalyzer struct{}

func (a *metadataAnalyzer) ID() string           { return AnalyzerMetadata }
func (a *metadataAnalyzer) Scope() AnalyzerScope { return ScopeSkill }

// metaJSON is a minimal subset of install.SkillMeta to avoid import cycles.
type metaJSON struct {
	RepoURL    string            `json:"repo_url"`
	FileHashes map[string]string `json:"file_hashes"`
}

// metadataStoreJSON is a minimal subset of install.MetadataStore for reading
// the centralized .metadata.json without importing the install package.
type metadataStoreJSON struct {
	Entries map[string]metaJSON `json:"entries"`
}

// metadataFileName mirrors install.MetadataFileName to avoid a circular import.
const metadataFileName = ".metadata.json"

// Rule IDs for disable support via audit-rules.yaml.
const (
	rulePublisherMismatch = "publisher-mismatch"
	ruleAuthorityLanguage = "authority-language"
)

// reOrgClaim matches explicit organization claims like "from Acme Corp",
// "published by Widget Labs", or "@acme". Keep the captured organization
// case-sensitive so ordinary trigger text such as "from any .docx" or
// "from CSV data" is not treated as a publisher claim.
var reOrgClaim = regexp.MustCompile(`(?i:(?:from|by|made by|created by|published by|maintained by))\s+([A-Z][a-z][\w-]*(?:\s+(?:[A-Z][\w-]*|Corp|Inc|Ltd|Team|Labs|AI|HQ|Co|Group))*)|@([A-Za-z][\w-]+)`)

// authorityWords are terms that imply official endorsement.
var authorityWords = []string{
	"official",
	"verified",
	"trusted",
	"authorized",
	"endorsed",
	"certified",
}

func (a *metadataAnalyzer) Analyze(ctx *AnalyzeContext) ([]Finding, error) {
	if ctx.SkillPath == "" {
		return nil, nil
	}

	repoURL := readMetaRepoURL(ctx.SkillPath)

	// Read SKILL.md frontmatter for name and description.
	skillMDPath := filepath.Join(ctx.SkillPath, "SKILL.md")
	name, description := readSkillFrontmatter(skillMDPath, ctx.FileCache)

	var findings []Finding

	// Rule A: Publisher mismatch — description claims an org but repo owner differs.
	if !ctx.DisabledIDs[rulePublisherMismatch] && repoURL != "" {
		if f := checkPublisherMismatch(name, description, repoURL); f != nil {
			findings = append(findings, *f)
		}
	}

	// Rule B: Authority language from unrecognized source.
	if !ctx.DisabledIDs[ruleAuthorityLanguage] {
		if f := checkAuthorityLanguage(description, repoURL); f != nil {
			findings = append(findings, *f)
		}
	}

	return findings, nil
}

// findMetaEntry walks up parent directories of skillPath looking for the
// centralized .metadata.json store and returns the raw entry for this skill.
func findMetaEntry(skillPath string) *metaJSON {
	skillName := filepath.Base(skillPath)
	dir := filepath.Dir(skillPath)

	for i := 0; i < 10 && dir != filepath.Dir(dir); i++ {
		data, err := os.ReadFile(filepath.Join(dir, metadataFileName))
		if err == nil {
			var store metadataStoreJSON
			if json.Unmarshal(data, &store) == nil {
				if rel, relErr := filepath.Rel(dir, skillPath); relErr == nil {
					key := filepath.ToSlash(rel)
					if e, ok := store.Entries[key]; ok {
						return &e
					}
				}
				if e, ok := store.Entries[skillName]; ok {
					return &e
				}
			}
		}
		dir = filepath.Dir(dir)
	}
	return nil
}

func readMetaRepoURL(skillPath string) string {
	if e := findMetaEntry(skillPath); e != nil {
		return e.RepoURL
	}
	return ""
}

func readMetaFileHashes(skillPath string) map[string]string {
	if e := findMetaEntry(skillPath); e != nil {
		return e.FileHashes
	}
	return nil
}

// readSkillFrontmatter extracts name and description from SKILL.md.
// Uses fileCache if available, otherwise reads from disk.
func readSkillFrontmatter(skillMDPath string, fileCache map[string][]byte) (name, description string) {
	var data []byte
	if cached, ok := fileCache["SKILL.md"]; ok {
		data = cached
	} else {
		var err error
		data, err = os.ReadFile(skillMDPath)
		if err != nil {
			return "", ""
		}
	}
	return parseFrontmatterNameDesc(data)
}

// parseFrontmatterNameDesc extracts name and description from YAML frontmatter bytes.
func parseFrontmatterNameDesc(data []byte) (name, description string) {
	lines := strings.Split(string(data), "\n")
	inFrontmatter := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if inFrontmatter {
				break
			}
			inFrontmatter = true
			continue
		}
		if !inFrontmatter {
			continue
		}
		if strings.HasPrefix(trimmed, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			name = strings.Trim(name, "\"'")
		}
		if strings.HasPrefix(trimmed, "description:") {
			description = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			description = strings.Trim(description, "\"'")
		}
	}
	return name, description
}

// extractRepoOwner returns the owner/org segment from a git URL.
// Supports HTTPS (https://github.com/owner/repo.git) and SSH (git@github.com:owner/repo.git).
func extractRepoOwner(repoURL string) string {
	s := strings.TrimSpace(repoURL)
	if s == "" {
		return ""
	}

	// SSH: git@host:owner/repo.git
	if idx := strings.Index(s, ":"); idx > 0 && strings.Contains(s[:idx], "@") && !strings.Contains(s, "://") {
		path := s[idx+1:]
		parts := strings.SplitN(path, "/", 2)
		if len(parts) >= 1 {
			return strings.ToLower(parts[0])
		}
	}

	// HTTPS: https://host/owner/repo.git
	// Skip file:// URLs — they're local.
	if strings.HasPrefix(s, "file://") {
		return ""
	}
	// Strip scheme and host, get first path segment.
	if strings.Contains(s, "://") {
		afterScheme := strings.SplitN(s, "://", 2)
		if len(afterScheme) == 2 {
			path := afterScheme[1]
			// Remove host
			if slashIdx := strings.Index(path, "/"); slashIdx >= 0 {
				path = path[slashIdx+1:]
			}
			parts := strings.SplitN(path, "/", 2)
			if len(parts) >= 1 {
				return strings.ToLower(parts[0])
			}
		}
	}
	return ""
}

// checkPublisherMismatch detects when SKILL.md claims a publisher
// that doesn't match the actual repo owner.
func checkPublisherMismatch(name, description, repoURL string) *Finding {
	owner := extractRepoOwner(repoURL)
	if owner == "" {
		return nil
	}

	// Combine name and description for claim extraction.
	text := name + " " + description
	matches := reOrgClaim.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	for _, m := range matches {
		claimed := ""
		if m[1] != "" {
			claimed = m[1]
		} else if m[2] != "" {
			claimed = m[2]
		}
		if claimed == "" {
			continue
		}
		claimed = strings.TrimSpace(claimed)
		claimedLower := strings.ToLower(claimed)

		// Allow if claimed name is a substring of owner or vice versa.
		if strings.Contains(owner, claimedLower) || strings.Contains(claimedLower, owner) {
			continue
		}

		return &Finding{
			Severity:   SeverityHigh,
			Pattern:    rulePublisherMismatch,
			Message:    fmt.Sprintf("skill claims origin %q but sourced from %q", claimed, owner),
			File:       "SKILL.md",
			Line:       0,
			RuleID:     rulePublisherMismatch,
			Analyzer:   AnalyzerMetadata,
			Category:   CategoryTrust,
			Confidence: 0.7,
		}
	}
	return nil
}

// checkAuthorityLanguage detects authority claims ("official", "verified")
// from sources that aren't well-known or verifiable.
func checkAuthorityLanguage(description, repoURL string) *Finding {
	if description == "" {
		return nil
	}
	lower := strings.ToLower(description)

	var found []string
	for _, w := range authorityWords {
		if strings.Contains(lower, w) {
			found = append(found, w)
		}
	}
	if len(found) == 0 {
		return nil
	}

	// If no repo URL (local skill), skip — user controls the source.
	if repoURL == "" {
		return nil
	}

	// Well-known orgs are allowed authority claims.
	owner := extractRepoOwner(repoURL)
	if isWellKnownOrg(owner) {
		return nil
	}

	return &Finding{
		Severity:   SeverityMedium,
		Pattern:    ruleAuthorityLanguage,
		Message:    fmt.Sprintf("skill uses authority language (%s) but source is unverified", strings.Join(found, ", ")),
		File:       "SKILL.md",
		Line:       0,
		RuleID:     ruleAuthorityLanguage,
		Analyzer:   AnalyzerMetadata,
		Category:   CategoryTrust,
		Confidence: 0.5,
	}
}

// isWellKnownOrg returns true for organizations whose authority claims
// are expected and should not trigger warnings.
func isWellKnownOrg(owner string) bool {
	known := map[string]bool{
		"anthropics":    true,
		"openai":        true,
		"google":        true,
		"microsoft":     true,
		"github":        true,
		"gitlab":        true,
		"vercel":        true,
		"vercel-labs":   true,
		"meta":          true,
		"facebook":      true,
		"aws":           true,
		"amazon":        true,
		"apple":         true,
		"hashicorp":     true,
		"docker":        true,
		"kubernetes":    true,
		"rust-lang":     true,
		"golang":        true,
		"python":        true,
		"nodejs":        true,
		"composiohq":    true,
		"google-gemini": true,
	}
	return known[strings.ToLower(owner)]
}
