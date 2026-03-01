package version

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"skillshare/internal/config"
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
	InstallMethod   InstallMethod
}

// getCachePath returns the path to the cache file
func getCachePath() string {
	return filepath.Join(config.CacheDir(), cacheFileName)
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
	os.Remove(getCachePath())
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
	data, err := os.ReadFile(getCachePath())
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
	cachePath := getCachePath()

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

// Check checks if a new version is available.
// method determines where to look: InstallBrew queries Homebrew,
// InstallDirect queries the GitHub Release API.
// Returns nil if no check is needed or if there's no update.
func Check(currentVersion string, method InstallMethod) *CheckResult {
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
		var latestVersion string
		var err error

		switch method {
		case InstallBrew:
			latestVersion, err = FetchBrewLatestVersion()
		default:
			latestVersion, err = FetchLatestVersionOnly()
		}

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
			InstallMethod:   method,
		}
	}

	return nil
}
