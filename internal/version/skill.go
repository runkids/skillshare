package version

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
)

// SkillSourceURL is the raw URL to the official skillshare skill's SKILL.md.
const SkillSourceURL = "https://raw.githubusercontent.com/runkids/skillshare/main/skills/skillshare/SKILL.md"

// ReadLocalSkillVersion reads metadata.version from source/skillshare/SKILL.md.
// The returned value never has a "v" prefix.
func ReadLocalSkillVersion(sourceDir string) string {
	skillFile := filepath.Join(sourceDir, "skillshare", "SKILL.md")
	return strings.TrimPrefix(parseMetadataVersion(skillFile), "v")
}

// parseMetadataVersion reads the version from a metadata block in YAML frontmatter.
func parseMetadataVersion(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inFrontmatter := false
	inMetadata := false

	for scanner.Scan() {
		line := scanner.Text()
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

		// Detect "metadata:" at root level (no leading whitespace)
		if line == "metadata:" || strings.TrimRight(line, " \t") == "metadata:" {
			inMetadata = true
			continue
		}

		// Inside metadata block: indented lines
		if inMetadata {
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
				break // left metadata block
			}
			if strings.HasPrefix(trimmed, "version:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) == 2 {
					return strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				}
			}
		}
	}

	return ""
}

// SkillOutdated reports whether remote is a newer skill version than local.
// An unknown or unparsable version on either side is never outdated.
func SkillOutdated(local, remote string) bool {
	if local == "" || remote == "" {
		return false
	}
	older, err := compareVersions(local, remote)
	return err == nil && older
}

const skillCacheFileName = "skill-version-check.json"

// CachedRemoteSkillVersion returns the latest published skill version,
// asking GitHub at most once per checkInterval like Check does for the
// CLI. Returns "" when offline with no fresh cache.
func CachedRemoteSkillVersion() string {
	if cache, _ := loadCacheAt(skillCachePath()); cache != nil && time.Since(cache.LastChecked) < checkInterval {
		return cache.LatestVersion
	}
	latest := FetchRemoteSkillVersion()
	if latest != "" {
		SaveRemoteSkillVersion(latest)
	}
	return latest
}

// SaveRemoteSkillVersion records a freshly fetched skill version, so a
// command that already asked GitHub spares the next one the request.
func SaveRemoteSkillVersion(latest string) {
	_ = saveCacheAt(skillCachePath(), &Cache{LastChecked: time.Now(), LatestVersion: latest})
}

func skillCachePath() string {
	return filepath.Join(config.CacheDir(), skillCacheFileName)
}

// FetchRemoteSkillVersion fetches the latest skill version from GitHub (3s timeout).
func FetchRemoteSkillVersion() string {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(SkillSourceURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	scanner := bufio.NewScanner(resp.Body)
	inFrontmatter := false
	inMetadata := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break
		}

		if !inFrontmatter {
			continue
		}

		// "metadata:" block
		if line == "metadata:" || strings.TrimRight(line, " \t") == "metadata:" {
			inMetadata = true
			continue
		}

		if inMetadata {
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
				inMetadata = false
				continue
			}
			if strings.HasPrefix(trimmed, "version:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) == 2 {
					return strings.TrimPrefix(strings.TrimSpace(parts[1]), "v")
				}
			}
		}
	}

	return ""
}
