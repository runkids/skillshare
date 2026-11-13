package install

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// extraInstallSource settles only the private selection copy. The request's
// original parsed subdir and raw origin remain the admission identity.
func extraInstallSource(req RemoteExtraRequest, entry *MetadataEntry) (*Source, string, error) {
	pinnedBranch := req.PinBranch()
	if req.Branch == "" {
		req.Branch = pinnedBranch
	}
	source := *req.Source
	source.Branch, source.Commit = req.Branch, ""
	treeHash := ""
	if !req.Opts.Update && pinnedBranch == req.Branch {
		if req.ProjectRoot != "" {
			lock, err := LoadLock(req.LockDir)
			if err != nil {
				return nil, "", err
			}
			source.Commit = lock.ExtraCommitFor(req.Name, req.RawSource)
			if source.Commit != "" {
				treeHash = lock.Extras[req.Name].TreeHash
			}
		} else if entry != nil && entry.Source == req.RawSource && entry.Branch == req.Branch && len(entry.Commit) == 40 && IsCommitSHA(entry.Commit) {
			source.Commit, treeHash = entry.Commit, entry.TreeHash
		}
	}
	if source.Commit != "" {
		if entry != nil && entry.Kind == "extra" && entry.Source == req.RawSource && entry.Branch == req.Branch && entry.Commit == source.Commit {
			if err := settleExtraSubdir(&source, entry.Subdir); err != nil {
				return nil, "", err
			}
		} else if req.Branch != "" || !source.HasAmbiguousWebRef() {
			source.ApplyRecordedBranch(req.Branch)
		}
		if source.HasAmbiguousWebRef() {
			if len(treeHash) != 40 || !IsCommitSHA(treeHash) {
				return nil, "", extraSelectionError()
			}
			return &source, treeHash, nil
		}
	}
	if err := resolveWebRef(&source); err != nil {
		return nil, "", err
	}
	if req.Branch == "" {
		source.Branch = ""
	}
	return &source, treeHash, nil
}

func extraSelectionError() error {
	return fmt.Errorf("cannot determine pinned extra directory selection; preserve existing files and records, or use --update explicitly to select the current source again")
}

func extraWebSubdirs(w webRef) []string {
	parts := strings.Split(w.tail, "/")
	var paths []string
	for i := 1; i <= len(parts); i++ {
		subdir, _, ok := w.split(strings.Join(parts[:i], "/"))
		if ok && validateRepoSubdir(subdir) == nil && !slices.Contains(paths, subdir) {
			paths = append(paths, subdir)
		}
	}
	return paths
}

func settleExtraSubdir(source *Source, subdir string) error {
	if err := validateRepoSubdir(subdir); err != nil {
		return err
	}
	if source.webRef.tail != "" {
		if !slices.Contains(extraWebSubdirs(source.webRef), subdir) {
			return extraSelectionError()
		}
	} else if source.Subdir != subdir {
		return extraSelectionError()
	}
	source.Subdir, source.webRef = subdir, webRef{}
	return nil
}

func journalExtraSource(j extraJournal, opts ParseOptions) (*Source, error) {
	source, err := ParseSourceWithOptions(j.Remote, opts)
	if err != nil || source == nil || !source.IsGit() {
		return nil, fmt.Errorf("invalid journaled Git source for %q", j.Name)
	}
	if j.Subdir != nil {
		if err := settleExtraSubdir(source, *j.Subdir); err != nil {
			return nil, err
		}
	} else {
		if source.HasAmbiguousWebRef() && !source.webRef.covers(j.Branch) {
			return nil, extraSelectionError()
		}
		source.ApplyRecordedBranch(j.Branch)
	}
	source.Branch, source.Commit = j.Branch, j.Commit
	return source, nil
}

// Ambiguous pinned selection fetches one root checkout, then inspects only the
// URL's finite suffixes in that immutable object database before D16 staging.
func fetchExtraCandidate(source *Source, checkout string, result *InstallResult, opts InstallOptions, expectedTree ...string) (gitFetch, string, error) {
	working := *source
	selectTree := working.Commit != "" && working.HasAmbiguousWebRef()
	if selectTree {
		working.Subdir = ""
	}
	fetched, err := fetchGitSubdir(&working, checkout, result, opts, true)
	if err != nil {
		return gitFetch{}, "", err
	}
	if selectTree {
		if len(expectedTree) == 0 {
			return gitFetch{}, "", extraSelectionError()
		}
		subdir, err := pinnedExtraSubdir(fetched, source.webRef, expectedTree[0])
		if err != nil {
			return gitFetch{}, "", err
		}
		fetched.Subdir, fetched.SubdirPath = subdir, filepath.Join(fetched.RepoPath, filepath.FromSlash(subdir))
	}
	treeHash, err := extraGitTreeHash(fetched.RepoPath, fetched.Commit, fetched.Subdir)
	if err == nil && len(expectedTree) > 0 && expectedTree[0] != "" && treeHash != expectedTree[0] {
		err = extraSelectionError()
	}
	return fetched, treeHash, err
}

func pinnedExtraSubdir(fetched gitFetch, w webRef, expectedTree string) (string, error) {
	if len(expectedTree) != 40 || !IsCommitSHA(expectedTree) {
		return "", extraSelectionError()
	}
	var matches []string
	for _, subdir := range extraWebSubdirs(w) {
		if hash, err := extraGitTreeHash(fetched.RepoPath, fetched.Commit, subdir); err == nil && hash == expectedTree {
			matches = append(matches, subdir)
		}
	}
	if len(matches) != 1 {
		return "", extraSelectionError()
	}
	return matches[0], nil
}

// Unlike skills, every admitted extra link target lies inside the selected tree.
func extraGitTreeHash(repo, commit, subdir string) (string, error) {
	if len(commit) != 40 || !IsCommitSHA(commit) {
		return "", fmt.Errorf("extra Git tree needs its full fetched commit")
	}
	if err := validateRepoSubdir(subdir); err != nil {
		return "", err
	}
	expr := commit + "^{tree}"
	if subdir != "" {
		expr = commit + ":" + subdir
	}
	hash, err := gitOutput(repo, "rev-parse", "--verify", expr)
	if err != nil || len(hash) != 40 || !IsCommitSHA(hash) {
		return "", fmt.Errorf("cannot read extra Git tree for selected directory: %v", err)
	}
	kind, err := gitOutput(repo, "cat-file", "-t", hash)
	if err != nil || kind != "tree" {
		return "", fmt.Errorf("selected extra Git object is not a directory tree: %v", err)
	}
	return hash, nil
}
