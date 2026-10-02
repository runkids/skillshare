package version

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchChecksum downloads the release checksums file at url and returns the
// hash recorded for asset. url normally comes from BuildChecksumsURL.
func FetchChecksum(url, asset string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksums.txt returned HTTP %d", resp.StatusCode)
	}

	return ParseChecksum(resp.Body, asset)
}

// ParseChecksum reads a checksums file ("<sha256>  <filename>", the format
// GoReleaser writes) and returns the hash for the given target filename.
func ParseChecksum(r io.Reader, target string) (string, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) == 2 && parts[1] == target {
			return parts[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksum for %s not found in checksums.txt", target)
}
