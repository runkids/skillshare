package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pelletier/go-toml/v2"
)

var ternID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

func inspectTern(root string) (string, string, TargetPackage, bool) {
	info := TargetPackage{Manifest: "plugin.toml", Components: []string{}}
	data, err := os.ReadFile(filepath.Join(root, info.Manifest))
	if os.IsNotExist(err) {
		return "", "", info, false
	}
	var m struct {
		ID          string `toml:"id"`
		Version     string `toml:"version"`
		Description string `toml:"description"`
	}
	if err == nil {
		err = toml.Unmarshal(data, &m)
	}
	info.Version = m.Version
	reason := ""
	switch {
	case err != nil:
		reason = "Invalid plugin.toml"
	case !ternID.MatchString(m.ID):
		reason = "plugin.toml requires a valid Tern id"
	}
	if reason != "" {
		info.block("plugins.problem.invalidManifest", reason, map[string]string{"manifest": info.Manifest})
	}
	return m.ID, m.Description, info, true
}

func parseTernInventory(data []byte) ([]Installed, error) {
	var envelope struct {
		Plugins []struct {
			ID      string `json:"id"`
			Version string `json:"version"`
		} `json:"plugins"`
	}
	if json.Unmarshal(data, &envelope) != nil || envelope.Plugins == nil {
		return nil, fmt.Errorf("unrecognized tern plugin inventory")
	}
	result := []Installed{}
	for _, row := range envelope.Plugins {
		if row.ID == "tern" || !ternID.MatchString(row.ID) {
			continue
		}
		result = append(result, Installed{ID: row.ID, Name: row.ID, Version: row.Version, Installed: true, Scope: "user"})
	}
	return result, nil
}
