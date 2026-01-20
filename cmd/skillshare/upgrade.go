package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/ui"
)

const githubRepo = "runkids/skillshare"

func cmdUpgrade(args []string) error {
	dryRun := false
	force := false
	skillOnly := false
	cliOnly := false

	// Parse args
	for _, arg := range args {
		switch arg {
		case "--dry-run", "-n":
			dryRun = true
		case "--force", "-f":
			force = true
		case "--skill":
			skillOnly = true
		case "--cli":
			cliOnly = true
		case "--help", "-h":
			printUpgradeHelp()
			return nil
		}
	}

	// Default: upgrade both
	upgradeCLI := !skillOnly
	upgradeSkill := !cliOnly

	if dryRun {
		ui.Warning("Dry run mode - no changes will be made")
		fmt.Println()
	}

	var cliErr, skillErr error

	// Upgrade CLI
	if upgradeCLI {
		cliErr = upgradeCLIBinary(dryRun, force)
	}

	// Upgrade skill
	if upgradeSkill {
		if upgradeCLI {
			fmt.Println()
		}
		skillErr = upgradeSkillshareSkill(dryRun, force)
	}

	// Return first error
	if cliErr != nil {
		return cliErr
	}
	return skillErr
}

func upgradeCLIBinary(dryRun, force bool) error {
	ui.Header("Upgrading CLI")

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlink: %w", err)
	}

	// Check if installed via Homebrew
	if isHomebrewInstall(execPath) {
		ui.Info("Detected Homebrew installation")
		if dryRun {
			ui.Info("Would run: brew upgrade runkids/tap/skillshare")
			return nil
		}
		ui.Info("Running: brew upgrade runkids/tap/skillshare")
		if err := runBrewUpgrade(); err != nil {
			return err
		}
		// Ensure symlink after brew upgrade
		ensureShorthandSymlink(execPath)
		return nil
	}

	// Get current version
	currentVersion := version

	// Get latest version from GitHub
	latestVersion, downloadURL, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check latest version: %w", err)
	}

	ui.Info("Current version: %s", currentVersion)
	ui.Info("Latest version:  %s", latestVersion)

	if currentVersion == latestVersion && !force {
		ui.Success("Already up to date")
		// Still ensure symlink exists
		ensureShorthandSymlink(execPath)
		return nil
	}

	if dryRun {
		ui.Info("Would download: %s", downloadURL)
		return nil
	}

	// Confirm if not forced
	if !force {
		fmt.Printf("  Upgrade to %s? [y/N]: ", latestVersion)
		var input string
		fmt.Scanln(&input)
		if input != "y" && input != "Y" && input != "yes" {
			ui.Info("Cancelled")
			return nil
		}
	}

	ui.Info("Downloading %s...", latestVersion)

	// Download and extract
	if err := downloadAndReplace(downloadURL, execPath); err != nil {
		return fmt.Errorf("failed to upgrade: %w", err)
	}

	// Create ss symlink
	ensureShorthandSymlink(execPath)

	ui.Success("Upgraded to %s", latestVersion)
	return nil
}

func upgradeSkillshareSkill(dryRun, force bool) error {
	ui.Header("Upgrading skillshare skill")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config not found: run 'skillshare init' first")
	}

	skillshareSkillDir := filepath.Join(cfg.Source, "skillshare")
	skillshareSkillFile := filepath.Join(skillshareSkillDir, "SKILL.md")

	// Check if skill exists
	exists := false
	if _, err := os.Stat(skillshareSkillFile); err == nil {
		exists = true
	}

	if dryRun {
		if exists {
			ui.Info("Would upgrade: %s", skillshareSkillDir)
		} else {
			ui.Info("Would download: %s", skillshareSkillDir)
		}
		ui.Info("Source: %s", skillshareSkillSource)
		return nil
	}

	// Confirm if exists and not forced
	if exists && !force {
		fmt.Print("  Overwrite existing skillshare skill? [y/N]: ")
		var input string
		fmt.Scanln(&input)
		if input != "y" && input != "Y" && input != "yes" {
			ui.Info("Cancelled")
			return nil
		}
	}

	// Install using install package (downloads entire directory including references/)
	ui.Info("Downloading from GitHub...")
	source, err := install.ParseSource(skillshareSkillSource)
	if err != nil {
		return fmt.Errorf("failed to parse source: %w", err)
	}
	source.Name = "skillshare"

	_, err = install.Install(source, skillshareSkillDir, install.InstallOptions{
		Force:  true,
		DryRun: false,
	})
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	ui.Success("Upgraded skillshare skill")
	ui.Info("Path: %s", skillshareSkillDir)
	ui.Info("")
	ui.Info("Run 'skillshare sync' to distribute to all targets")

	return nil
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func getLatestRelease() (version string, downloadURL string, err error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)

	resp, err := http.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", err
	}

	version = strings.TrimPrefix(release.TagName, "v")

	// Find matching asset
	osName := runtime.GOOS
	archName := runtime.GOARCH
	expectedName := fmt.Sprintf("skillshare_%s_%s_%s.tar.gz", version, osName, archName)

	for _, asset := range release.Assets {
		if asset.Name == expectedName {
			return version, asset.BrowserDownloadURL, nil
		}
	}

	return "", "", fmt.Errorf("no release found for %s/%s", osName, archName)
}

func downloadAndReplace(url, destPath string) error {
	// Download tarball
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Extract from tar.gz
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	// Find skillshare binary in archive
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("skillshare binary not found in archive")
		}
		if err != nil {
			return err
		}

		if header.Name == "skillshare" || header.Name == "./skillshare" {
			// Write to temp file first
			tmpFile, err := os.CreateTemp(filepath.Dir(destPath), "skillshare-upgrade-*")
			if err != nil {
				return err
			}
			tmpPath := tmpFile.Name()

			if _, err := io.Copy(tmpFile, tr); err != nil {
				tmpFile.Close()
				os.Remove(tmpPath)
				return err
			}
			tmpFile.Close()

			// Make executable
			if err := os.Chmod(tmpPath, 0755); err != nil {
				os.Remove(tmpPath)
				return err
			}

			// Replace original
			if err := os.Rename(tmpPath, destPath); err != nil {
				os.Remove(tmpPath)
				return err
			}

			return nil
		}
	}
}

func isHomebrewInstall(execPath string) bool {
	// Check common Homebrew paths
	homebrewPaths := []string{
		"/usr/local/Cellar/skillshare",
		"/opt/homebrew/Cellar/skillshare",
		"/home/linuxbrew/.linuxbrew/Cellar/skillshare",
	}
	for _, prefix := range homebrewPaths {
		if strings.HasPrefix(execPath, prefix) {
			return true
		}
	}
	return false
}

func runBrewUpgrade() error {
	cmd := exec.Command("brew", "upgrade", "runkids/tap/skillshare")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func printUpgradeHelp() {
	fmt.Println(`Usage: skillshare upgrade [options]

Upgrade the CLI binary and/or built-in skillshare skill.

Options:
  --skill       Upgrade skill only
  --cli         Upgrade CLI only
  --force, -f   Skip confirmation prompts
  --dry-run, -n Preview without making changes
  --help, -h    Show this help

Examples:
  skillshare upgrade              # Upgrade both CLI and skill
  skillshare upgrade --cli        # Upgrade CLI only
  skillshare upgrade --skill      # Upgrade skill only
  skillshare upgrade --dry-run    # Preview upgrades`)
}

// ensureShorthandSymlink creates ss -> skillshare symlink if not exists
func ensureShorthandSymlink(skillsharePath string) {
	dir := filepath.Dir(skillsharePath)
	ssPath := filepath.Join(dir, "ss")

	// Check if ss already exists and points to skillshare
	if target, err := os.Readlink(ssPath); err == nil {
		if filepath.Base(target) == "skillshare" {
			return // Already correct
		}
	}

	// Remove existing ss (file or wrong symlink)
	os.Remove(ssPath)

	// Create symlink
	if err := os.Symlink(skillsharePath, ssPath); err != nil {
		// Try with sudo hint
		ui.Warning("Could not create 'ss' shorthand: %v", err)
		ui.Info("  Run: sudo ln -sf %s %s", skillsharePath, ssPath)
	} else {
		ui.Success("Created 'ss' shorthand")
	}
}
