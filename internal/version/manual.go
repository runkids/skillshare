package version

// CheckNow bypasses the automatic-check policy and cache for an explicit request.
func CheckNow(current string, method InstallMethod) (*CheckResult, error) {
	var latest string
	var err error
	if method == InstallBrew {
		latest, err = FetchBrewLatestVersion()
	} else {
		latest, err = FetchLatestVersionOnly()
	}
	if err != nil {
		return nil, err
	}
	available, err := compareVersions(current, latest)
	if err != nil {
		return nil, err
	}
	return &CheckResult{CurrentVersion: current, LatestVersion: latest, UpdateAvailable: available, InstallMethod: method}, nil
}
