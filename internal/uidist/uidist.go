package uidist

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/version"
)

// AssetName is the filename of the UI dist tarball in GitHub releases
const AssetName = "skillshare-ui-dist.tar.gz"

// CacheDir returns the cache directory for a specific version's UI dist.
// Path: ~/.cache/skillshare/ui/{ver}/
func CacheDir(ver string) string {
	return filepath.Join(config.CacheDir(), "ui", ver)
}

// IsCached checks whether a cached UI dist exists for the given version.
// Returns the directory path and true if index.html exists there.
func IsCached(ver string) (string, bool) {
	dir := CacheDir(ver)
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
		return dir, true
	}
	return dir, false
}

// Download fetches the UI dist tarball for the given version, verifies its
// checksum, extracts it to the cache directory, and cleans up old versions.
func Download(ver string) error {
	dir := CacheDir(ver)

	// Fetch expected checksum
	expectedHash, err := fetchChecksum(ver)
	if err != nil {
		return fmt.Errorf("failed to fetch checksum: %w", err)
	}

	// Download tarball to temp file
	tarballURL := version.BuildUIDistURL(ver)
	tmpFile, err := downloadToTemp(tarballURL)
	if err != nil {
		return fmt.Errorf("failed to download UI assets: %w", err)
	}
	defer os.Remove(tmpFile)

	// Verify checksum
	actualHash, err := fileSHA256(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	// Extract to cache dir
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cache dir: %w", err)
	}
	if err := extractTarGz(tmpFile, dir); err != nil {
		// Clean up partial extraction
		os.RemoveAll(dir)
		return fmt.Errorf("failed to extract UI assets: %w", err)
	}

	// Clean old versions (best-effort)
	_ = cleanOldVersions(ver)

	return nil
}

// ClearCache removes the entire UI cache directory (~/.cache/skillshare/ui/).
func ClearCache() error {
	dir := filepath.Join(config.CacheDir(), "ui")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(dir)
}

// ClearVersion removes a specific version's cached UI dist.
func ClearVersion(ver string) error {
	dir := CacheDir(ver)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(dir)
}

// cleanOldVersions removes cached UI dist versions other than currentVer.
func cleanOldVersions(currentVer string) error {
	uiDir := filepath.Join(config.CacheDir(), "ui")
	entries, err := os.ReadDir(uiDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() && e.Name() != currentVer {
			os.RemoveAll(filepath.Join(uiDir, e.Name()))
		}
	}
	return nil
}

// fetchChecksum downloads checksums.txt and parses the SHA256 for AssetName.
func fetchChecksum(ver string) (string, error) {
	url := version.BuildChecksumsURL(ver)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksums.txt returned HTTP %d", resp.StatusCode)
	}

	return parseChecksum(resp.Body, AssetName)
}

// parseChecksum reads a checksums.txt (format: "hash  filename\n") and returns
// the hash for the given target filename.
func parseChecksum(r io.Reader, target string) (string, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		// GoReleaser format: "<sha256>  <filename>"
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == target {
			return parts[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksum for %s not found in checksums.txt", target)
}

// maxDownloadSize limits the UI dist download to prevent unexpected disk usage.
const maxDownloadSize = 100 * 1024 * 1024 // 100 MB

// downloadToTemp downloads a URL to a temporary file and returns its path.
func downloadToTemp(url string) (string, error) {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "skillshare-ui-dist-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	limited := io.LimitReader(resp.Body, maxDownloadSize+1)
	n, err := io.Copy(tmp, limited)
	if err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	if n > maxDownloadSize {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("download exceeds maximum size (%d MB)", maxDownloadSize/(1024*1024))
	}

	return tmp.Name(), nil
}

// fileSHA256 computes the SHA-256 hex digest of a file.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// maxFileSize limits individual file extraction to prevent decompression bombs.
const maxFileSize = 50 * 1024 * 1024 // 50 MB per file

// extractTarGz extracts a .tar.gz file into destDir.
// The tarball is expected to contain a flat structure (no top-level directory).
func extractTarGz(tarballPath, destDir string) error {
	f, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Sanitize path to prevent zip-slip
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("invalid path in archive: %s", hdr.Name)
		}
		target := filepath.Join(destDir, clean)
		// Canonical check: joined path must stay within destDir
		if !strings.HasPrefix(target, cleanDest) && target != filepath.Clean(destDir) {
			return fmt.Errorf("invalid path in archive: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0755|0644)
			if err != nil {
				return err
			}
			limited := io.LimitReader(tr, maxFileSize+1)
			n, copyErr := io.Copy(out, limited)
			out.Close()
			if copyErr != nil {
				return copyErr
			}
			if n > maxFileSize {
				return fmt.Errorf("file %s exceeds maximum size", hdr.Name)
			}
		}
	}
	return nil
}
