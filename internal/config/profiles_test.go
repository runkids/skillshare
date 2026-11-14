package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProfilesRoundTrip(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			input := "targets: {}\nprofiles:\n  work: # retained\n    future_option: true\n"
			if project {
				input = "targets: []\nprofiles:\n  work: # retained\n    future_option: true\n"
			}
			var data []byte
			var err error
			if project {
				var cfg ProjectConfig
				if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
					t.Fatal(err)
				}
				data, err = marshalConfig(&cfg, nil, []byte(input))
			} else {
				var cfg Config
				if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
					t.Fatal(err)
				}
				data, err = marshalConfig(&cfg, nil, []byte(input))
			}
			if err != nil || !strings.Contains(string(data), "future_option: true") || !strings.Contains(string(data), "# retained") {
				t.Fatalf("profile lost on save: %s (%v)", data, err)
			}
		})
	}
}

func TestLoadProjectReadOnlyDoesNotMigrate(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".skillshare")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	before := []byte("targets:\n  - name: custom\n    path: ./skills\n")
	if err := os.WriteFile(path, before, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProjectReadOnly(root); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("read-only load changed config: %q (%v)", after, err)
	}
}
