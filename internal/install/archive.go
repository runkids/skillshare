package install

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/utils"
)

const (
	archiveDownloadLimit = 100 << 20
	archiveExpandedLimit = 250 << 20
	archiveEntryLimit    = 10000
)

func parseArchiveSource(u *url.URL, source *Source) (*Source, error) {
	if u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return nil, fmt.Errorf("archive sources require HTTPS without userinfo")
	}
	if _, fragment, ok := strings.Cut(source.Raw, "#"); ok {
		values, err := url.ParseQuery(fragment)
		if err != nil || len(values) != 1 || len(values["skill"]) != 1 {
			return nil, fmt.Errorf("archive selector must be #skill=<relative-path>")
		}
		source.Subdir = values.Get("skill")
		if err := archiveRelativePath(source.Subdir); err != nil && source.Subdir != "." {
			return nil, err
		}
	}
	source.Type = SourceTypeArchive
	source.Name = strings.TrimSuffix(path.Base(u.Path), path.Ext(u.Path))
	if source.Subdir != "" && source.Subdir != "." {
		source.Name = path.Base(source.Subdir)
	}
	if err := validateSourceName(source.Name); err != nil {
		return nil, err
	}
	return source, nil
}

func archiveRelativePath(name string) error {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
		return fmt.Errorf("unsafe archive path %q", name)
	}
	return checkSubdirSafety(name)
}

func archiveURL(raw string) string {
	base, _, _ := strings.Cut(raw, "#")
	return base
}

func archiveSkillSource(source *Source, member string) string {
	base := archiveURL(source.Raw)
	if member == "" {
		member = "."
	}
	return base + "#skill=" + url.QueryEscape(member)
}

// downloadArchive owns its temporary directory; callers remove it after all
// readers are closed. It never changes installed state or extracts content.
func downloadArchive(raw string, progress ProgressCallback) (file, digest string, err error) {
	source, err := ParseSource(raw)
	if err != nil {
		return "", "", err
	}
	if source.Type != SourceTypeArchive {
		return "", "", fmt.Errorf("not an HTTPS ZIP source")
	}
	dir, err := os.MkdirTemp("", "skillshare-archive-*")
	if err != nil {
		return "", "", err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
		}
	}()
	file = filepath.Join(dir, "download.zip")
	f, err := os.Create(file)
	if err != nil {
		return "", "", err
	}
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return fmt.Errorf("archive redirect requires HTTPS")
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return nil
	}}
	if progress != nil {
		progress("Downloading archive...")
	}
	err = utils.DownloadToFile(client, archiveURL(raw), f, archiveDownloadLimit, nil)
	closeErr := f.Close()
	if err != nil {
		// net/http errors include the request URL, whose query may be private.
		message := err.Error()
		if requestErr, ok := err.(*url.Error); ok {
			_, redact := RedactRepoURL(requestErr.URL)
			message = redact.Replace(message)
		}
		_, redact := RedactRepoURL(archiveURL(raw))
		return "", "", fmt.Errorf("archive download failed: %s", redact.Replace(message))
	}
	if closeErr != nil {
		return "", "", closeErr
	}
	digest, err = utils.FileHash(file)
	return file, digest, err
}

// ArchiveHash fingerprints downloaded bytes without extracting or installing.
func ArchiveHash(raw string) (string, error) {
	file, digest, err := downloadArchive(raw, nil)
	if err == nil {
		defer os.RemoveAll(filepath.Dir(file))
	}
	return digest, err
}

func extractArchive(file, root string, expandedLimit int64, entryLimit int) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if err := preflightArchive(f, info.Size(), entryLimit); err != nil {
		return fmt.Errorf("invalid ZIP: %w", err)
	}
	z, err := zip.NewReader(f, info.Size())
	if err != nil {
		return fmt.Errorf("invalid ZIP: %w", err)
	}
	if len(z.File) > entryLimit {
		return fmt.Errorf("ZIP exceeds %d entries", entryLimit)
	}
	var expanded int64
	for _, f := range z.File {
		name := strings.TrimSuffix(f.Name, "/")
		if err := archiveRelativePath(name); err != nil {
			return err
		}
		mode := f.Mode()
		if !mode.IsRegular() && !mode.IsDir() {
			return fmt.Errorf("ZIP contains special file %q", f.Name)
		}
		if f.UncompressedSize64 > uint64(expandedLimit-expanded) {
			return fmt.Errorf("ZIP exceeds expanded size limit")
		}
		dest := filepath.Join(root, filepath.FromSlash(name))
		if mode.IsDir() {
			if err := os.MkdirAll(dest, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600|mode.Perm()&0111)
		if err != nil {
			_ = r.Close()
			return err
		}
		n, copyErr := io.Copy(w, io.LimitReader(r, expandedLimit-expanded+1))
		readErr, writeErr := r.Close(), w.Close()
		if copyErr != nil {
			return copyErr
		}
		if readErr != nil {
			return readErr
		}
		if writeErr != nil {
			return writeErr
		}
		expanded += n
		if expanded > expandedLimit {
			return fmt.Errorf("ZIP exceeds expanded size limit")
		}
	}
	return nil
}

// DiscoverArchive prepares an owned tree, then uses local discovery's naming,
// ignore and nested-skill rules. ZIP sources expose skills only.
func DiscoverArchive(source *Source, progress ProgressCallback) (*DiscoveryResult, error) {
	if source.Branch != "" || source.Commit != "" {
		return nil, fmt.Errorf("ZIP sources do not support Git refs")
	}
	file, digest, err := downloadArchive(source.Raw, progress)
	if err != nil {
		return nil, err
	}
	discovery, err := discoverDownloadedArchive(source, file, digest)
	if err != nil {
		_ = os.RemoveAll(filepath.Dir(file))
	}
	return discovery, err
}

func discoverDownloadedArchive(source *Source, file, digest string) (*DiscoveryResult, error) {
	root := filepath.Join(filepath.Dir(file), "repo")
	if err := extractArchive(file, root, archiveExpandedLimit, archiveEntryLimit); err != nil {
		return nil, err
	}
	local := &Source{Type: SourceTypeLocalPath, Path: root, Name: source.Name}
	if source.Subdir != "" {
		local.Path = filepath.Join(root, filepath.FromSlash(source.Subdir))
	}
	if source.Subdir != "" {
		if _, err := os.Stat(filepath.Join(local.Path, "SKILL.md")); err != nil {
			return nil, fmt.Errorf("archive member %q has no SKILL.md", source.Subdir)
		}
	}
	discovery, err := DiscoverLocal(local)
	if err != nil {
		return nil, err
	}
	source.ArchiveSHA256 = digest
	discovery.RepoPath, discovery.Source, discovery.Agents = filepath.Dir(file), source, nil
	return discovery, nil
}

func installArchive(source *Source, destPath string, opts InstallOptions) (*InstallResult, error) {
	if opts.Track || opts.Branch != "" || source.Branch != "" || source.Commit != "" || opts.IsAgentMode() || opts.HasAgentFilter() {
		return nil, fmt.Errorf("ZIP sources support skills only, without Git tracking or refs")
	}
	file, digest, err := downloadArchive(source.Raw, opts.OnProgress)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(filepath.Dir(file))
	if archiveUnchanged(opts, destPath, source.Raw, digest) {
		return &InstallResult{SkillName: filepath.Base(destPath), SkillPath: destPath, Source: source.Raw, Action: "skipped"}, nil
	}
	discovery, err := discoverDownloadedArchive(source, file, digest)
	if err != nil {
		return nil, err
	}
	if source.Subdir != "" {
		if skill, ok := matchDiscoveredSkillBySubdir(discovery, "."); ok {
			return InstallFromDiscovery(discovery, skill, destPath, opts)
		}
	}
	if len(discovery.Skills) != 1 {
		return nil, fmt.Errorf("archive contains %d skills; select one with --skill", len(discovery.Skills))
	}
	return InstallFromDiscovery(discovery, discovery.Skills[0], destPath, opts)
}

// archiveUnchanged is shared by direct updates and discovery-based --update.
func archiveUnchanged(opts InstallOptions, dest, raw, digest string) bool {
	if !opts.Update || opts.Reinstall {
		return false
	}
	rel, err := filepath.Rel(opts.SourceDir, dest)
	if err != nil {
		return false
	}
	entry := LoadMetadataOrNew(opts.SourceDir).GetByPath(filepath.ToSlash(rel))
	if entry == nil || entry.Source != raw || entry.ArchiveSHA256 != digest {
		return false
	}
	_, err = os.Stat(filepath.Join(dest, "SKILL.md"))
	return err == nil
}
