package version

import (
	"bufio"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/skillignore"
	"skillshare/internal/sourcewalk"
	ssync "skillshare/internal/sync"
)

// SkillSourceURL is the raw URL to the official skillshare skill's SKILL.md.
const SkillSourceURL = "https://raw.githubusercontent.com/runkids/skillshare/main/skills/skillshare/SKILL.md"

// builtinSkillName is the built-in skill's folder name.
const builtinSkillName = "skillshare"

// LocalSkill is the built-in skill as found in a skills source.
type LocalSkill struct {
	Dir      string // its folder
	RelPath  string // slash path from the source, such as "skillshare" or "tools/skillshare"
	Version  string // metadata.version without a "v" prefix; "" when SKILL.md has none
	Disabled bool   // .skillignore disables it, so sync leaves it out of every target
}

// FindLocalSkill finds the built-in skill in sourceDir. init and upgrade put it at
// <source>/skillshare, which wins. The dashboard can move it into a folder, so otherwise
// the first skill named skillshare outside tracked repos is used, discovered with
// walk as the dashboard lists skills. ok is false when there is none.
func FindLocalSkill(sourceDir string, walk ...sourcewalk.Options) (skill LocalSkill, ok bool) {
	dir := filepath.Join(sourceDir, builtinSkillName)
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
		disabled := skillignore.ReadMatcher(sourceDir).Match(builtinSkillName, true)
		return newLocalSkill(dir, builtinSkillName, disabled), true
	}
	skills, err := ssync.DiscoverSourceSkillsAll(sourceDir, walk...)
	if err != nil {
		return LocalSkill{}, false
	}
	for _, s := range skills {
		if !s.IsInRepo && path.Base(s.RelPath) == builtinSkillName {
			return newLocalSkill(s.SourcePath, s.RelPath, s.Disabled), true
		}
	}
	return LocalSkill{}, false
}

func newLocalSkill(dir, relPath string, disabled bool) LocalSkill {
	version := strings.TrimPrefix(ParseMetadataVersion(filepath.Join(dir, "SKILL.md")), "v")
	return LocalSkill{Dir: dir, RelPath: relPath, Version: version, Disabled: disabled}
}

// ReadLocalSkillVersion reads metadata.version from the built-in skill in sourceDir,
// wherever FindLocalSkill finds it. The returned value never has a "v" prefix.
func ReadLocalSkillVersion(sourceDir string) string {
	skill, _ := FindLocalSkill(sourceDir)
	return skill.Version
}

// ParseMetadataVersion reads the version from a metadata block in YAML frontmatter,
// line by line, so it also works when the rest of the frontmatter is not valid YAML.
func ParseMetadataVersion(filePath string) string {
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
	// The CLI and skill ship in the same release, so a newer cached CLI release means this cache predates it.
	if cache, _ := loadCacheAt(skillCachePath()); cache != nil && time.Since(cache.LastChecked) < checkInterval && !SkillOutdated(cache.LatestVersion, GetCachedVersion()) {
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
