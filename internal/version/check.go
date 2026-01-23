package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	githubRepo     = "runkids/skillshare"
	checkInterval  = 24 * time.Hour
	cacheFileName  = "version-check.json"
)

// Cache holds version check cache data
type Cache struct {
	LastChecked   time.Time `json:"last_checked"`
	LatestVersion string    `json:"latest_version"`
}

// CheckResult holds the result of a version check
type CheckResult struct {
	CurrentVersion string
	LatestVersion  string
	UpdateAvailable bool
}

// getCachePath returns the path to the cache file
func getCachePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".skillshare", cacheFileName), nil
}

// loadCache loads the version check cache from disk
func loadCache() (*Cache, error) {
	cachePath, err := getCachePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No cache yet
		}
		return nil, err
	}

	var cache Cache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return &cache, nil
}

// saveCache saves the version check cache to disk
func saveCache(cache *Cache) error {
	cachePath, err := getCachePath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return err
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

// fetchLatestVersion fetches the latest version from GitHub
func fetchLatestVersion() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}

// compareVersions returns true if v1 < v2 (proper semver comparison)
func compareVersions(v1, v2 string) bool {
	if v1 == "dev" || v1 == "" {
		return false // Don't prompt for dev builds
	}

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Compare each part numerically
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}

		if n1 < n2 {
			return true // v1 < v2
		}
		if n1 > n2 {
			return false // v1 > v2
		}
	}

	return false // v1 == v2
}

// Check checks if a new version is available
// Returns nil if no check is needed or if there's no update
func Check(currentVersion string) *CheckResult {
	// Don't check for dev builds
	if currentVersion == "dev" || currentVersion == "" {
		return nil
	}

	cache, _ := loadCache()

	// Check if we need to fetch
	needsFetch := cache == nil || time.Since(cache.LastChecked) > checkInterval

	if needsFetch {
		// Fetch in foreground but with short timeout
		latestVersion, err := fetchLatestVersion()
		if err != nil {
			// Silently fail - don't bother user with network errors
			return nil
		}

		// Update cache
		cache = &Cache{
			LastChecked:   time.Now(),
			LatestVersion: latestVersion,
		}
		_ = saveCache(cache) // Ignore save errors
	}

	if cache == nil || cache.LatestVersion == "" {
		return nil
	}

	// Compare versions
	if compareVersions(currentVersion, cache.LatestVersion) {
		return &CheckResult{
			CurrentVersion:  currentVersion,
			LatestVersion:   cache.LatestVersion,
			UpdateAvailable: true,
		}
	}

	return nil
}
