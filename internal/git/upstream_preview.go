package git

import (
	"strconv"
	"strings"

	"skillshare/internal/gitenv"
)

// UpstreamCommit is a commit HEAD lacks that the upstream has.
type UpstreamCommit struct {
	Hash    string `json:"hash"` // short
	Subject string `json:"subject"`
	Author  string `json:"author"`
	Date    string `json:"date"` // RFC3339
}

// UpstreamFile is a file that differs between HEAD and its upstream.
type UpstreamFile struct {
	Path    string `json:"path"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Binary  bool   `json:"binary"`
}

// UpstreamPreview is what pulling dir's branch would bring in, as of the last
// fetch.
type UpstreamPreview struct {
	From         string // HEAD
	To           string // @{u}
	TotalCommits int
	Commits      []UpstreamCommit // newest first, at most the requested limit
	Files        []UpstreamFile
}

// PreviewUpstream compares HEAD with its upstream without fetching. It
// returns an error when the branch has no upstream.
func PreviewUpstream(dir string, limit int) (*UpstreamPreview, error) {
	revParse := func(rev string) (string, error) {
		cmd := gitenv.Command("rev-parse", rev)
		cmd.Dir = dir
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}
	run := func(args ...string) (string, error) {
		cmd := gitenv.Command(append([]string{"-c", "color.ui=false"}, args...)...)
		cmd.Dir = dir
		out, err := cmd.Output()
		return string(out), err
	}

	p := &UpstreamPreview{Commits: []UpstreamCommit{}, Files: []UpstreamFile{}}
	var err error
	if p.From, err = revParse("HEAD"); err != nil {
		return nil, err
	}
	if p.To, err = revParse("@{u}"); err != nil {
		return nil, err
	}

	count, err := run("rev-list", "--count", "HEAD..@{u}")
	if err != nil {
		return nil, err
	}
	p.TotalCommits, _ = strconv.Atoi(strings.TrimSpace(count))

	log, err := run("log", "-n", strconv.Itoa(limit), "--format=%h%x00%s%x00%an%x00%aI%x1e", "HEAD..@{u}")
	if err != nil {
		return nil, err
	}
	for _, rec := range strings.Split(log, "\x1e") {
		f := strings.Split(strings.TrimSpace(rec), "\x00")
		if len(f) == 4 {
			p.Commits = append(p.Commits, UpstreamCommit{Hash: f[0], Subject: f[1], Author: f[2], Date: f[3]})
		}
	}

	// -z keeps unusual file names unquoted; --no-renames keeps one path per record.
	stat, err := run("diff", "--numstat", "-z", "--no-renames", "HEAD...@{u}")
	if err != nil {
		return nil, err
	}
	for _, rec := range strings.Split(stat, "\x00") {
		f := strings.SplitN(rec, "\t", 3)
		if len(f) != 3 {
			continue
		}
		file := UpstreamFile{Path: f[2], Binary: f[0] == "-"}
		file.Added, _ = strconv.Atoi(f[0])
		file.Deleted, _ = strconv.Atoi(f[1])
		p.Files = append(p.Files, file)
	}
	return p, nil
}
