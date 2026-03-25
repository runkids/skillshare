package version

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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
