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
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
	"skillshare/internal/uidist"
	"skillshare/internal/utils"
	versionpkg "skillshare/internal/version"
)

// replaceBinaryFlag is internal: the sudo child receives only this flag, so
// root replaces the binary and never touches the user's skill, caches, or logs.
const replaceBinaryFlag = "--replace-binary"

func cmdUpgrade(args []string) error {
	start := time.Now()

	dryRun := false
	force := false
	skillOnly := false
	cliOnly := false
	replaceVersion := ""

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
		default:
			if v, ok := strings.CutPrefix(arg, replaceBinaryFlag+"="); ok {
				replaceVersion = v
			}
		}
	}

	if replaceVersion != "" {
		execPath, err := resolveExecPath()
		if err != nil {
			return err
		}
		return downloadBinary(execPath, replaceVersion)
	}

	// Default: upgrade both
	upgradeCLI := !skillOnly
	upgradeSkill := !cliOnly
	skillForce := force

	var cliErr, skillErr error
	var newCLIVersion string

	// Upgrade CLI
	if upgradeCLI {
		newCLIVersion, cliErr = upgradeCLIBinary(dryRun, force)
	}

	// Upgrade skill
	skillChanged := false
	if upgradeSkill {
		skillChanged, skillErr = upgradeSkillshareSkill(dryRun, skillForce)
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

	if dryRun {
		fmt.Println()
		ui.DryRun()
		return nil
	}
	if skillChanged {
		ui.Next("skillshare sync", "link the new skill into your targets")
	}
	fmt.Println()
	fmt.Println(ui.DimText("If skillshare saved you time, please give us a star on GitHub: https://github.com/runkids/skillshare"))

	return nil
}

// upgradeRowWidth lines up the CLI and Skill rows.
var upgradeRowWidth = ui.RowWidth("CLI", "Skill")

// printUpgradeRow prints the CLI or Skill row, with the time it took when
// start is set.
func printUpgradeRow(mark, label, value string, start time.Time) {
	if !start.IsZero() {
		value += ui.Took(time.Since(start))
	}
	ui.Row(mark, label, value, upgradeRowWidth)
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
	start := time.Now()
	current := ui.VersionLabel(version)

	execPath, err := resolveExecPath()
	if err != nil {
		return "", err
	}

	// Check if installed via Homebrew
	if versionpkg.DetectInstallMethod(execPath) == versionpkg.InstallBrew {
		if dryRun {
			printUpgradeRow(ui.MarkNone, "CLI", current+" · would run brew upgrade skillshare", time.Time{})
			return "", nil
		}
		return runBrewUpgrade(start)
	}

	// Get latest version from GitHub
	spinner := ui.StartSpinner("Checking latest version...")
	release, err := versionpkg.FetchLatestRelease()
	spinner.Stop()

	var latestVersion string
	if err != nil {
		// API failed - try to use cached version
		cachedVersion := versionpkg.GetCachedVersion()
		if cachedVersion == "" || cachedVersion == version {
			printUpgradeRow(ui.MarkNone, "CLI", current+ui.DimText(" · couldn't check for a newer version (rate limited)"), time.Time{})
			return "", nil
		}
		latestVersion = cachedVersion
	} else {
		latestVersion = release.Version
	}

	if version == latestVersion && !force {
		printUpgradeRow(ui.MarkOK, "CLI", current+", already up to date", time.Time{})
		return "", nil
	}

	if dryRun {
		printUpgradeRow(ui.MarkNone, "CLI", fmt.Sprintf("%s → v%s · would download", current, latestVersion), time.Time{})
		return "", nil
	}

	// Confirm if not forced
	if !force {
		ok, err := ui.ConfirmAction(fmt.Sprintf("Upgrade to v%s?", latestVersion), true)
		if err != nil {
			return "", err
		}
		if !ok {
			printUpgradeRow(ui.MarkNone, "CLI", "cancelled, kept "+current, time.Time{})
			return "", nil
		}
	}

	// Only the binary replacement runs as root; everything after it writes
	// into the user's home and must stay owned by the user.
	if runtime.GOOS != "windows" && needsSudo(execPath) {
		ui.Note(fmt.Sprintf("Need elevated permissions to write to %s", filepath.Dir(execPath)))
		err = upgradeBinaryWithSudo(execPath, latestVersion)
	} else {
		err = downloadBinary(execPath, latestVersion)
	}
	if err != nil {
		printUpgradeRow(ui.MarkFail, "CLI", "upgrade to v"+latestVersion+" failed", time.Time{})
		return "", err
	}
	printUpgradeRow(ui.MarkOK, "CLI", fmt.Sprintf("%s → v%s", current, latestVersion), start)

	// Clear version cache so next check fetches fresh data
	versionpkg.ClearCache()

	// Pre-download UI assets for the new version (best-effort)
	uiSpinner := ui.StartSpinner("Downloading UI assets...")
	uiErr := uidist.Download(latestVersion, downloadProgress("Downloading UI assets...", uiSpinner.Update))
	uiSpinner.Stop()
	if uiErr != nil {
		ui.Note("UI download skipped — run 'skillshare ui' to retry")
	}

	return latestVersion, nil
}

func resolveExecPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlink: %w", err)
	}
	return execPath, nil
}

func downloadBinary(execPath, targetVersion string) error {
	downloadURL, err := versionpkg.BuildDownloadURL(targetVersion)
	if err != nil {
		return fmt.Errorf("failed to get download URL: %w", err)
	}

	downloadLabel := fmt.Sprintf("Downloading v%s...", targetVersion)
	downloadSpinner := ui.StartSpinner(downloadLabel)
	err = downloadAndReplace(downloadURL, versionpkg.BuildChecksumsURL(targetVersion), execPath,
		downloadProgress(downloadLabel, downloadSpinner.Update))
	downloadSpinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to upgrade: %w", err)
	}
	return nil
}

// upgradeSkillshareSkill installs or upgrades the built-in skill and
// reports whether it changed the skill.
func upgradeSkillshareSkill(dryRun, force bool) (bool, error) {
	cfg, err := config.Load()
	if err != nil {
		return false, fmt.Errorf("config not found: run 'skillshare init' first")
	}

	sourceDir := cfg.EffectiveSkillsSource()
	skillshareSkillDir := filepath.Join(sourceDir, "skillshare")
	localVersion := versionpkg.ReadLocalSkillVersion(sourceDir)

	// Skill not installed
	if localVersion == "" {
		if force {
			if dryRun {
				printUpgradeRow(ui.MarkNone, "Skill", "not installed · would download", time.Time{})
				return false, nil
			}
			return true, doSkillDownload(skillshareSkillDir, sourceDir, "")
		}

		if dryRun {
			printUpgradeRow(ui.MarkNone, "Skill", "not installed · would ask to install", time.Time{})
			return false, nil
		}

		ok, err := ui.ConfirmAction("Install built-in skillshare skill?", false)
		if err != nil {
			return false, err
		}
		if !ok {
			printUpgradeRow(ui.MarkNone, "Skill", "not installed", time.Time{})
			return false, nil
		}

		return true, doSkillDownload(skillshareSkillDir, sourceDir, "")
	}

	// Skill installed — compare versions
	current := "v" + localVersion
	if force {
		if dryRun {
			printUpgradeRow(ui.MarkNone, "Skill", current+" · would download again", time.Time{})
			return false, nil
		}
		return true, doSkillDownload(skillshareSkillDir, sourceDir, localVersion)
	}

	spinner := ui.StartSpinner("Checking latest skill version...")
	remoteVersion := versionpkg.FetchRemoteSkillVersion()
	spinner.Stop()
	if remoteVersion != "" {
		versionpkg.SaveRemoteSkillVersion(remoteVersion)
	}
	if remoteVersion == "" {
		printUpgradeRow(ui.MarkNone, "Skill", current+ui.DimText(" · couldn't check for a newer version (network unavailable)"), time.Time{})
		return false, nil
	}

	if localVersion == remoteVersion {
		printUpgradeRow(ui.MarkOK, "Skill", current+", already up to date", time.Time{})
		return false, nil
	}

	if dryRun {
		printUpgradeRow(ui.MarkNone, "Skill", fmt.Sprintf("%s → v%s · would download", current, remoteVersion), time.Time{})
		return false, nil
	}

	return true, doSkillDownload(skillshareSkillDir, sourceDir, localVersion)
}

func doSkillDownload(skillshareSkillDir, sourceDir, fromVersion string) error {
	start := time.Now()
	spinner := ui.StartSpinner("Downloading the skillshare skill...")

	source, err := install.ParseSource(skillshareSkillSource)
	if err != nil {
		spinner.Stop()
		printUpgradeRow(ui.MarkFail, "Skill", "failed to parse source", time.Time{})
		return err
	}
	source.Name = "skillshare"

	_, err = install.Install(source, skillshareSkillDir, install.InstallOptions{
		Force:  true,
		DryRun: false,
	})
	spinner.Stop()
	if err != nil {
		printUpgradeRow(ui.MarkFail, "Skill", "download failed", time.Time{})
		return skillPermissionHint(err, sourceDir)
	}

	newVersion := versionpkg.ReadLocalSkillVersion(sourceDir)
	switch {
	case newVersion != "" && fromVersion != "" && fromVersion != newVersion:
		printUpgradeRow(ui.MarkOK, "Skill", fmt.Sprintf("v%s → v%s", fromVersion, newVersion), start)
	case newVersion != "" && fromVersion == "":
		printUpgradeRow(ui.MarkOK, "Skill", "installed v"+newVersion, start)
	case newVersion != "":
		printUpgradeRow(ui.MarkOK, "Skill", "v"+newVersion+", downloaded again", start)
	default:
		printUpgradeRow(ui.MarkOK, "Skill", "upgraded", start)
	}
	return nil
}

// skillPermissionHint adds the command that gives the skills source back to the user when
// its files could not be replaced. Before v0.21.10 an upgrade that needed sudo wrote the
// skill as root, and every later upgrade then fails with only "permission denied".
func skillPermissionHint(err error, sourceDir string) error {
	if !errors.Is(err, os.ErrPermission) || runtime.GOOS == "windows" {
		return err
	}
	quoted := "'" + strings.ReplaceAll(sourceDir, "'", `'\''`) + "'"
	return fmt.Errorf("%w\n  Files in the skills source are not yours, often left by an earlier upgrade run with sudo.\n  Take them back, then run 'skillshare upgrade --skill' again:\n  sudo chown -R \"$(id -un)\" %s", err, quoted)
}

// downloadProgress returns a callback that appends "read / total" to label.
// Only on a TTY: non-TTY spinners print a new line per update.
func downloadProgress(label string, update func(string)) utils.ProgressFunc {
	if !ui.IsTTY() {
		return nil
	}
	return func(read, total int64) {
		if total > 0 {
			update(fmt.Sprintf("%s  %s / %s", label, formatBytes(read), formatBytes(total)))
		} else {
			update(fmt.Sprintf("%s  %s", label, formatBytes(read)))
		}
	}
}

// maxArchiveSize bounds the staged release archive; the archives are ~10MB.
const maxArchiveSize = 100 * 1024 * 1024

// downloadAndReplace verifies the release archive against the release
// checksums file before replacing the binary. The archive is staged to a temp
// file first because checksums.txt covers the compressed asset, not the binary
// extracted from it.
func downloadAndReplace(downloadURL, checksumsURL, destPath string, onProgress utils.ProgressFunc) error {
	expectedHash, err := versionpkg.FetchChecksum(checksumsURL, path.Base(downloadURL))
	if err != nil {
		return fmt.Errorf("failed to fetch checksum: %w", err)
	}

	archivePath, err := downloadArchive(downloadURL, onProgress)
	if err != nil {
		return err
	}
	defer os.Remove(archivePath)

	actualHash, err := utils.FileHash(archivePath)
	if err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	// Windows uses zip, others use tar.gz
	if runtime.GOOS == "windows" {
		return extractFromZip(archive, destPath)
	}
	return extractFromTarGz(archive, destPath)
}

func downloadArchive(url string, onProgress utils.ProgressFunc) (string, error) {
	tmp, err := os.CreateTemp("", "skillshare-upgrade-archive-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	client := &http.Client{Timeout: 5 * time.Minute}
	if err := utils.DownloadToFile(client, url, tmp, maxArchiveSize, onProgress); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
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

func runBrewUpgrade(start time.Time) (string, error) {
	// Phase 1: brew update (tap refresh); a failure here is not fatal
	spinner := ui.StartSpinner("Updating tap...")
	updateCmd := exec.Command("brew", "update", "--quiet")
	var updateBuf bytes.Buffer
	updateCmd.Stdout = &updateBuf
	updateCmd.Stderr = &updateBuf
	tapErr := updateCmd.Run()

	// Phase 2: brew upgrade
	spinner.Update("Upgrading via Homebrew...")
	cmd := exec.Command("brew", "upgrade", "skillshare")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	spinner.Stop()
	if tapErr != nil {
		ui.Note("brew update failed, upgraded from the tap as it was")
	}
	if err != nil {
		printUpgradeRow(ui.MarkFail, "CLI", "brew upgrade skillshare failed", time.Time{})
		// Show captured output for debugging
		if out := strings.TrimSpace(buf.String()); out != "" {
			fmt.Println(out)
		}
		return "", err
	}

	newVersion := getBrewVersion()
	switch {
	case newVersion != "" && newVersion != version:
		printUpgradeRow(ui.MarkOK, "CLI", ui.VersionLabel(version)+" → "+ui.VersionLabel(newVersion), start)
	case newVersion != "" && newVersion == version:
		printUpgradeRow(ui.MarkOK, "CLI", ui.VersionLabel(version)+", already up to date", time.Time{})
	default:
		printUpgradeRow(ui.MarkOK, "CLI", "upgraded with Homebrew", start)
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
	printHelp("skillshare upgrade [options]", "Upgrade the CLI binary and/or built-in skillshare skill.",
		helpGroup{title: "Options", rows: []helpRow{
			{"--skill", "Upgrade skill only"},
			{"--cli", "Upgrade CLI only"},
			{"-f, --force", "Skip confirmation prompts"},
			{"-n, --dry-run", "Preview without making changes"},
		}},
		helpExamples(
			helpRow{"skillshare upgrade", "Upgrade both CLI and skill"},
			helpRow{"skillshare upgrade --cli", "Upgrade CLI only"},
			helpRow{"skillshare upgrade --skill", "Upgrade skill only"},
			helpRow{"skillshare upgrade --dry-run", "Preview upgrades"},
		),
	)
}
