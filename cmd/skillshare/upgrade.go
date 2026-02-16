package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
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
	"skillshare/internal/uidist"
	versionpkg "skillshare/internal/version"
)

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

	// Show logo
	ui.Logo(version)

	// Default: upgrade both
	upgradeCLI := !skillOnly
	upgradeSkill := !cliOnly
	skillForce := force

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
		skillErr = upgradeSkillshareSkill(dryRun, skillForce)
	}

	// Return first error
	if cliErr != nil {
		return cliErr
	}
	if skillErr != nil {
		return skillErr
	}

	if !dryRun && (upgradeCLI || upgradeSkill) {
		fmt.Println()
		ui.Info("If skillshare saved you time, please give us a star on GitHub: https://github.com/runkids/skillshare")
	}

	return nil
}

func upgradeCLIBinary(dryRun, force bool) error {
	// Step 1: Show current version
	ui.StepStart("CLI", fmt.Sprintf("v%s", version))

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
		ui.StepContinue("Install", "Homebrew")
		if dryRun {
			ui.StepEnd("Action", "Would run: brew upgrade runkids/tap/skillshare")
			return nil
		}
		return runBrewUpgrade()
	}

	// Get latest version from GitHub
	treeSpinner := ui.StartTreeSpinner("Checking latest version...", false)
	release, err := versionpkg.FetchLatestRelease()

	var latestVersion string
	if err != nil {
		// API failed - try to use cached version
		cachedVersion := versionpkg.GetCachedVersion()
		if cachedVersion != "" && cachedVersion != version {
			latestVersion = cachedVersion
			treeSpinner.Success(fmt.Sprintf("Latest: v%s (cached)", latestVersion))
		} else {
			// No useful cache - skip silently
			treeSpinner.Success("Skipped (rate limited)")
			return nil
		}
	} else {
		latestVersion = release.Version
		treeSpinner.Success(fmt.Sprintf("Latest: v%s", latestVersion))
	}

	if version == latestVersion && !force {
		ui.StepEnd("Status", "Already up to date ✓")
		return nil
	}

	if dryRun {
		ui.StepEnd("Action", fmt.Sprintf("Would download v%s", latestVersion))
		return nil
	}

	// Confirm if not forced
	if !force {
		fmt.Println()
		fmt.Printf("  Upgrade to v%s? [Y/n]: ", latestVersion)
		var input string
		fmt.Scanln(&input)
		input = strings.ToLower(strings.TrimSpace(input))
		if input == "n" || input == "no" {
			ui.Info("Cancelled")
			return nil
		}
	}

	// Get download URL for current platform
	downloadURL, err := versionpkg.BuildDownloadURL(latestVersion)
	if err != nil {
		return fmt.Errorf("failed to get download URL: %w", err)
	}

	// Download
	fmt.Println()
	downloadSpinner := ui.StartSpinner(fmt.Sprintf("Downloading v%s...", latestVersion))
	if err := downloadAndReplace(downloadURL, execPath); err != nil {
		downloadSpinner.Fail("Failed to download")
		return fmt.Errorf("failed to upgrade: %w", err)
	}
	downloadSpinner.Success(fmt.Sprintf("Upgraded to v%s", latestVersion))

	// Clear version cache so next check fetches fresh data
	versionpkg.ClearCache()

	// Pre-download UI assets for the new version (best-effort)
	if latestVersion != "" {
		uiSpinner := ui.StartSpinner("Downloading UI assets...")
		if err := uidist.Download(latestVersion); err != nil {
			uiSpinner.Warn("UI download skipped (run 'skillshare ui' to retry)")
		} else {
			uiSpinner.Success("UI assets cached")
		}
	}

	return nil
}

func upgradeSkillshareSkill(dryRun, force bool) error {
	// Step 1: Show skill info
	ui.StepStart("Skill", "skillshare")

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

	// Exists and not forced → already up to date
	if exists && !force {
		ui.StepEnd("Status", "Already up to date ✓")
		return nil
	}

	// Exists and forced → re-download
	if exists {
		ui.StepContinue("Status", "Will upgrade")

		if dryRun {
			ui.StepEnd("Action", "Would upgrade")
			return nil
		}

		return doSkillDownload(skillshareSkillDir)
	}

	// Not installed + force → install without asking
	if force {
		ui.StepContinue("Status", "Not installed")

		if dryRun {
			ui.StepEnd("Action", "Would download")
			return nil
		}

		return doSkillDownload(skillshareSkillDir)
	}

	// Not installed + no force → prompt
	ui.StepContinue("Status", "Not installed")

	if dryRun {
		ui.StepEnd("Action", "Would prompt to install")
		return nil
	}

	fmt.Println()
	fmt.Print("  Install built-in skillshare skill? [y/N]: ")
	var input string
	fmt.Scanln(&input)
	input = strings.ToLower(strings.TrimSpace(input))

	if input != "y" && input != "yes" {
		ui.StepEnd("Status", "Not installed (skipped)")
		return nil
	}

	return doSkillDownload(skillshareSkillDir)
}

func doSkillDownload(skillshareSkillDir string) error {
	treeSpinner := ui.StartTreeSpinner("Downloading from GitHub...", true)

	source, err := install.ParseSource(skillshareSkillSource)
	if err != nil {
		treeSpinner.Fail("Failed to parse source")
		return err
	}
	source.Name = "skillshare"

	_, err = install.Install(source, skillshareSkillDir, install.InstallOptions{
		Force:  true,
		DryRun: false,
	})
	if err != nil {
		treeSpinner.Fail("Failed to download")
		return err
	}

	treeSpinner.Success("Upgraded")

	fmt.Println()
	ui.Info("Run 'skillshare sync' to distribute to all targets")

	return nil
}

func downloadAndReplace(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Windows uses zip, others use tar.gz
	if runtime.GOOS == "windows" {
		return extractFromZip(resp.Body, destPath)
	}
	return extractFromTarGz(resp.Body, destPath)
}

func extractFromTarGz(r io.Reader, destPath string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("skillshare binary not found in archive")
		}
		if err != nil {
			return err
		}
		if header.Name == "skillshare" || header.Name == "./skillshare" {
			return writeBinary(tr, destPath)
		}
	}
}

func extractFromZip(r io.Reader, destPath string) error {
	// zip.Reader needs ReaderAt, so read all into memory
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		if f.Name == "skillshare.exe" || f.Name == "./skillshare.exe" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			return writeBinary(rc, destPath)
		}
	}
	return fmt.Errorf("skillshare.exe not found in archive")
}

func writeBinary(r io.Reader, destPath string) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(destPath), "skillshare-upgrade-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, r); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// On Windows, we can't directly replace a running executable.
	// However, we CAN rename it. So we:
	// 1. Rename current exe to .old
	// 2. Rename new exe to the correct name
	// 3. Try to delete .old (may fail if still running, but that's OK)
	if runtime.GOOS == "windows" {
		oldPath := destPath + ".old"
		// Remove any previous .old file
		os.Remove(oldPath)
		// Rename running exe to .old
		if err := os.Rename(destPath, oldPath); err != nil {
			os.Remove(tmpPath)
			if errors.Is(err, os.ErrPermission) {
				return fmt.Errorf("binary is locked by another process (is 'skillshare ui' running?)\n         Close other skillshare processes and try again")
			}
			return fmt.Errorf("failed to rename current binary: %w", err)
		}
		// Rename new exe to correct name
		if err := os.Rename(tmpPath, destPath); err != nil {
			// Try to restore
			os.Rename(oldPath, destPath)
			os.Remove(tmpPath)
			return err
		}
		// Try to clean up old file (may fail, that's OK)
		os.Remove(oldPath)
		return nil
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
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
	// First update the tap to get latest formula
	ui.Info("Updating tap...")
	updateCmd := exec.Command("brew", "update", "--quiet")
	updateCmd.Stdout = os.Stdout
	updateCmd.Stderr = os.Stderr
	if err := updateCmd.Run(); err != nil {
		ui.Warning("brew update failed, trying upgrade anyway...")
	}

	// Then upgrade
	ui.Info("Upgrading...")
	cmd := exec.Command("brew", "upgrade", "runkids/tap/skillshare")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		// Clear version cache so next check fetches fresh data
		versionpkg.ClearCache()
	}
	return err
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
