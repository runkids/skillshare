package install

import (
	"fmt"
	"os"
	"path/filepath"
)

type extraCandidate struct {
	staged   string
	fetch    gitFetch
	audit    *AuditGateResult
	treeHash string
	warnings []string
}

func removeExtraCandidateTemp(dir string) { _ = os.RemoveAll(dir) }

// prepareExtraCandidate fetches and audits privately; it never publishes to root.
func prepareExtraCandidate(source *Source, root, dest string, opts InstallOptions, expectedTree ...string) (*extraCandidate, func(), error) {
	if source == nil || !source.IsGit() {
		return nil, nil, fmt.Errorf("remote extra requires a Git source")
	}
	if err := RejectSourceCredentials(source.Raw); err != nil {
		return nil, nil, err
	}
	if err := RejectSourceCredentials(source.CloneURL); err != nil {
		return nil, nil, err
	}
	tmp, err := os.MkdirTemp("", "skillshare-extra-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { removeExtraCandidateTemp(tmp) }
	working := *source
	result := &InstallResult{}
	fetch, treeHash, err := fetchExtraCandidate(&working, filepath.Join(tmp, "checkout"), result, opts, expectedTree...)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	staged := filepath.Join(tmp, "staged")
	if err := stageFetchedTree(fetch.RepoPath, fetch.Subdir, staged); err != nil {
		cleanup()
		return nil, nil, err
	}
	gate, err := gateExtraCandidate(staged, root, dest, fetch.Commit, opts)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	result.Warnings = extraAuditWarnings(result.Warnings, gate)
	return &extraCandidate{staged: staged, fetch: fetch, audit: gate, treeHash: treeHash, warnings: result.Warnings}, cleanup, nil
}

// extraAuditWarnings reports the completed gate without recording acceptance;
// publication owns those guarded writes.
func extraAuditWarnings(warnings []string, gate *AuditGateResult) []string {
	if gate.Audit == nil {
		return warnings
	}
	if gate.Audit.SkippedFiles > 0 {
		warnings = append(warnings, fmt.Sprintf("Extra audit coverage is limited: %d file(s) skipped ordinary content checks; hidden-parent files receive header checks only", gate.Audit.SkippedFiles))
	}
	for _, finding := range gate.Audit.Findings {
		warnings = append(warnings, fmt.Sprintf("%s [%s]", auditFindingWarning(finding), finding.RuleID))
	}
	if gate.Overridden {
		warnings = append(warnings, fmt.Sprintf("audit findings at/above block threshold (%s); audit overridden with --force", gate.Threshold))
	}
	return warnings
}
