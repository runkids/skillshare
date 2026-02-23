package install

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func gitSupportsSparseCheckout() bool {
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return false
	}
	return supportsSparseCheckoutVersion(strings.TrimSpace(string(out)))
}

func supportsSparseCheckoutVersion(versionOutput string) bool {
	// Examples:
	//   git version 2.44.0
	//   git version 2.39.3 (Apple Git-146)
	fields := strings.Fields(versionOutput)
	if len(fields) < 3 {
		return false
	}
	ver := fields[2]
	parts := strings.Split(ver, ".")
	if len(parts) < 2 {
		return false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}

	if major > 2 {
		return true
	}
	return major == 2 && minor >= 25
}

func sparseCloneSubdir(url, subdir, destPath string, extraEnv []string, onProgress ProgressCallback) error {
	subdir = strings.TrimSpace(subdir)
	if subdir == "" {
		return fmt.Errorf("sparse checkout requires non-empty subdir")
	}

	cloneArgs := []string{"clone", "--filter=blob:none", "--no-checkout", "--depth", "1", "--quiet", url, destPath}
	if onProgress != nil {
		cloneArgs = []string{"clone", "--filter=blob:none", "--no-checkout", "--depth", "1", "--progress", url, destPath}
	}
	if err := runGitCommandWithProgress(cloneArgs, "", extraEnv, onProgress); err != nil {
		return err
	}

	if err := runGitCommandWithProgress([]string{"sparse-checkout", "set", subdir}, destPath, extraEnv, nil); err != nil {
		return err
	}

	if err := runGitCommandWithProgress([]string{"checkout"}, destPath, extraEnv, nil); err != nil {
		return err
	}

	return nil
}
