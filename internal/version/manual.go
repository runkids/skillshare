package version

// CheckNow bypasses the automatic-check policy and cache for an explicit request.
func CheckNow(current string, method InstallMethod) (*CheckResult, error) {
	latest, err := FetchLatestVersionFor(method)
	if err != nil {
		return nil, err
	}
	available, err := compareVersions(current, latest)
	if err != nil {
		return nil, err
	}
	return &CheckResult{CurrentVersion: current, LatestVersion: latest, UpdateAvailable: available, InstallMethod: method}, nil
}
