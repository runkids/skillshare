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
func prepareExtraCandidate(source *Source, root, dest string, opts InstallOptions) (*extraCandidate, func(), error) {
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
	fetch, err := fetchGitSubdir(&working, filepath.Join(tmp, "checkout"), result, opts, true)
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
	if gate.Audit != nil && gate.Audit.SkippedFiles > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Extra audit coverage is limited: %d file(s) skipped ordinary content checks; hidden-parent files receive header checks only", gate.Audit.SkippedFiles))
	}
	return &extraCandidate{staged: staged, fetch: fetch, audit: gate, treeHash: getSubdirTreeHash(fetch.RepoPath, fetch.Subdir), warnings: result.Warnings}, cleanup, nil
}
