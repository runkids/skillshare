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
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
	"skillshare/internal/uidist"
	versionpkg "skillshare/internal/version"
)

func cmdUpgrade(args []string) error {
	start := time.Now()

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
	var newCLIVersion string

	// Upgrade CLI
	if upgradeCLI {
		newCLIVersion, cliErr = upgradeCLIBinary(dryRun, force)
	}

	// Upgrade skill
	if upgradeSkill {
		if upgradeCLI {
			fmt.Println()
		}
		skillErr = upgradeSkillshareSkill(dryRun, skillForce)
	}

	// Determine first error for return and logging
	var cmdErr error
	if cliErr != nil {
		cmdErr = cliErr
	} else if skillErr != nil {
		cmdErr = skillErr
	}

	logUpgradeOp(config.ConfigPath(), upgradeCLI && cliErr == nil, upgradeSkill && skillErr == nil, version, newCLIVersion, start, cmdErr)

	if cmdErr != nil {
		return cmdErr
	}

	if !dryRun && (upgradeCLI || upgradeSkill) {
		fmt.Println()
		ui.Info("If skillshare saved you time, please give us a star on GitHub: https://github.com/runkids/skillshare")
	}

	return nil
}

func logUpgradeOp(cfgPath string, cliUpgraded bool, skillUpgraded bool, fromVersion, toVersion string, start time.Time, cmdErr error) {
	e := oplog.NewEntry("upgrade", statusFromErr(cmdErr), time.Since(start))
	a := map[string]any{}
	if cliUpgraded {
		a["cli"] = true
	}
	if skillUpgraded {
		a["skill"] = true
	}
	if fromVersion != "" {
		a["from_version"] = fromVersion
	}
	if toVersion != "" {
		a["to_version"] = toVersion
	}
	e.Args = a
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

func upgradeCLIBinary(dryRun, force bool) (string, error) {
	// Step 1: Show current version
	ui.StepStart("CLI", fmt.Sprintf("v%s", version))

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlink: %w", err)
	}

	// Check if installed via Homebrew
	if isHomebrewInstall(execPath) {
		ui.StepContinue("Install", "Homebrew")
		if dryRun {
			ui.StepEnd("Action", "Would run: brew upgrade skillshare")
			return "", nil
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
			return "", nil
		}
	} else {
		latestVersion = release.Version
		treeSpinner.Success(fmt.Sprintf("Latest: v%s", latestVersion))
	}

	if version == latestVersion && !force {
		ui.StepEnd("Status", "Already up to date ✓")
		return "", nil
	}

	if dryRun {
		ui.StepEnd("Action", fmt.Sprintf("Would download v%s", latestVersion))
		return "", nil
	}

	// Confirm if not forced
	if !force {
		fmt.Printf("%s\n", ui.TreeLine())
		fmt.Printf("%s  Upgrade to v%s? [Y/n]: ", ui.TreeLine(), latestVersion)
		var input string
		fmt.Scanln(&input)
		input = strings.ToLower(strings.TrimSpace(input))
		if input == "n" || input == "no" {
			ui.StepEnd("Status", "Cancelled")
			return "", nil
		}
	}

	// Get download URL for current platform
	downloadURL, err := versionpkg.BuildDownloadURL(latestVersion)
	if err != nil {
		return "", fmt.Errorf("failed to get download URL: %w", err)
	}

	// Download
	hasUIDownload := latestVersion != ""
	downloadSpinner := ui.StartTreeSpinner(fmt.Sprintf("Downloading v%s...", latestVersion), !hasUIDownload)
	if err := downloadAndReplace(downloadURL, execPath); err != nil {
		downloadSpinner.Fail("Failed to download")
		return "", fmt.Errorf("failed to upgrade: %w", err)
	}
	downloadSpinner.Success(fmt.Sprintf("Upgraded  v%s → v%s", version, latestVersion))

	// Clear version cache so next check fetches fresh data
	versionpkg.ClearCache()

	// Pre-download UI assets for the new version (best-effort)
	if hasUIDownload {
		uiSpinner := ui.StartTreeSpinner("Downloading UI assets...", true)
		if err := uidist.Download(latestVersion); err != nil {
			uiSpinner.Warn("UI download skipped (run 'skillshare ui' to retry)")
		} else {
			uiSpinner.Success("UI assets cached")
		}
	}

	return latestVersion, nil
}

func upgradeSkillshareSkill(dryRun, force bool) error {
	// Step 1: Show skill info
	ui.StepStart("Skill", "skillshare")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config not found: run 'skillshare init' first")
	}

	skillshareSkillDir := filepath.Join(cfg.Source, "skillshare")
	localVersion := versionpkg.ReadLocalSkillVersion(cfg.Source)

	// Skill not installed
	if localVersion == "" {
		ui.StepContinue("Status", "Not installed")

		if force {
			if dryRun {
				ui.StepEnd("Action", "Would download")
				return nil
			}
			return doSkillDownload(skillshareSkillDir, cfg.Source, "")
		}

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

		return doSkillDownload(skillshareSkillDir, cfg.Source, "")
	}

	// Skill installed — compare versions
	ui.StepContinue("Current", fmt.Sprintf("v%s", localVersion))

	if force {
		if dryRun {
			ui.StepEnd("Action", "Would re-download (forced)")
			return nil
		}
		return doSkillDownload(skillshareSkillDir, cfg.Source, localVersion)
	}

	treeSpinner := ui.StartTreeSpinner("Checking latest version...", false)
	remoteVersion := versionpkg.FetchRemoteSkillVersion()
	if remoteVersion == "" {
		treeSpinner.Success("Skipped (network unavailable)")
		return nil
	}
	treeSpinner.Success(fmt.Sprintf("Latest: v%s", remoteVersion))

	if localVersion == remoteVersion {
		ui.StepEnd("Status", "Already up to date ✓")
		return nil
	}

	if dryRun {
		ui.StepEnd("Action", fmt.Sprintf("Would upgrade to v%s", remoteVersion))
		return nil
	}

	return doSkillDownload(skillshareSkillDir, cfg.Source, localVersion)
}

func doSkillDownload(skillshareSkillDir, sourceDir, fromVersion string) error {
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

	newVersion := versionpkg.ReadLocalSkillVersion(sourceDir)
	switch {
	case newVersion != "" && fromVersion != "" && fromVersion != newVersion:
		treeSpinner.Success(fmt.Sprintf("Upgraded  v%s → v%s", fromVersion, newVersion))
	case newVersion != "" && fromVersion == "":
		treeSpinner.Success(fmt.Sprintf("Installed v%s", newVersion))
	default:
		treeSpinner.Success("Upgraded")
	}

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

func runBrewUpgrade() (string, error) {
	// Phase 1: brew update (tap refresh)
	tapSpinner := ui.StartTreeSpinner("Updating tap...", false)
	updateCmd := exec.Command("brew", "update", "--quiet")
	var updateBuf bytes.Buffer
	updateCmd.Stdout = &updateBuf
	updateCmd.Stderr = &updateBuf
	if err := updateCmd.Run(); err != nil {
		tapSpinner.Fail("Tap update failed (continuing)")
	} else {
		tapSpinner.Success("Tap updated")
	}

	// Phase 2: brew upgrade
	upgradeSpinner := ui.StartTreeSpinner("Upgrading via Homebrew...", true)
	cmd := exec.Command("brew", "upgrade", "skillshare")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		upgradeSpinner.Fail("Upgrade failed")
		// Show captured output for debugging
		if out := strings.TrimSpace(buf.String()); out != "" {
			fmt.Println()
			fmt.Println(out)
		}
		return "", err
	}

	newVersion := getBrewVersion()
	switch {
	case newVersion != "" && newVersion != version:
		upgradeSpinner.Success(fmt.Sprintf("Upgraded  v%s → v%s", version, newVersion))
	case newVersion != "" && newVersion == version:
		upgradeSpinner.Success("Already up to date ✓")
	default:
		upgradeSpinner.Success("Upgraded")
	}

	versionpkg.ClearCache()
	return newVersion, nil
}

// getBrewVersion runs "brew list --versions skillshare" and parses the version.
func getBrewVersion() string {
	out, err := exec.Command("brew", "list", "--versions", "skillshare").Output()
	if err != nil {
		return ""
	}
	// Output format: "skillshare 0.16.5"
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return ""
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
