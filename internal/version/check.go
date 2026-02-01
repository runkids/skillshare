package version

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Version is set by main.go at startup
var Version = "dev"

const (
	githubRepo    = "runkids/skillshare"
	checkInterval = 24 * time.Hour
	cacheFileName = "version-check.json"
)

// Cache holds version check cache data
type Cache struct {
	LastChecked   time.Time `json:"last_checked"`
	LatestVersion string    `json:"latest_version"`
}

// CheckResult holds the result of a version check
type CheckResult struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
}

// getCacheDir returns the cache directory, respecting XDG_CACHE_HOME
func getCacheDir() (string, error) {
	// XDG Base Directory Specification: use XDG_CACHE_HOME if set
	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		return filepath.Join(cacheHome, "skillshare"), nil
	}
	// Default: ~/.cache/skillshare
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".cache", "skillshare"), nil
}

// getCachePath returns the path to the cache file
func getCachePath() (string, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, cacheFileName), nil
}

// legacyCachePath returns the old cache path for migration cleanup
func legacyCachePath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".skillshare", cacheFileName)
}

// cleanupLegacyCache removes cache from old location
func cleanupLegacyCache() {
	legacyPath := legacyCachePath()
	if _, err := os.Stat(legacyPath); err == nil {
		os.Remove(legacyPath)
		// Try to remove parent dir if empty
		parentDir := filepath.Dir(legacyPath)
		os.Remove(parentDir) // Fails silently if not empty
	}
}

// ClearCache removes the version check cache file
func ClearCache() {
	cachePath, err := getCachePath()
	if err != nil {
		return
	}
	os.Remove(cachePath)
}

// GetCachedVersion returns the cached latest version if available
func GetCachedVersion() string {
	cache, err := loadCache()
	if err != nil || cache == nil {
		return ""
	}
	return cache.LatestVersion
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
// Uses the shared FetchLatestVersionOnly which supports GITHUB_TOKEN
func fetchLatestVersion() (string, error) {
	return FetchLatestVersionOnly()
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

	// Clean up legacy cache location (~/.skillshare/)
	cleanupLegacyCache()

	cache, _ := loadCache()

	// If current version >= cached latest, clear stale cache (user may have manually updated)
	if cache != nil && cache.LatestVersion != "" && !compareVersions(currentVersion, cache.LatestVersion) {
		ClearCache()
		cache = nil
	}

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
