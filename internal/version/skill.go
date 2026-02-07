package version

import (
	"bufio"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/utils"
)

// SkillSourceURL is the raw URL to the official skillshare skill's SKILL.md.
const SkillSourceURL = "https://raw.githubusercontent.com/runkids/skillshare/main/skills/skillshare/SKILL.md"

// ReadLocalSkillVersion reads the "version" field from source/skillshare/SKILL.md.
func ReadLocalSkillVersion(sourceDir string) string {
	skillFile := filepath.Join(sourceDir, "skillshare", "SKILL.md")
	return utils.ParseFrontmatterField(skillFile, "version")
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

	for scanner.Scan() {
		line := scanner.Text()

		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break
		}

		if inFrontmatter && strings.HasPrefix(line, "version:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}
