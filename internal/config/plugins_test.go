package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPluginConfigPreservedByResourceConfig(t *testing.T) {
	raw := []byte("plugins:\n  packages:\n    demo:\n      bindings:\n        codex:\n          id: demo@team\n          sync: false\n")
	for _, cfg := range []any{&Config{}, &ProjectConfig{}} {
		if err := yaml.Unmarshal(raw, cfg); err != nil {
			t.Fatal(err)
		}
		data, err := yaml.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "id: demo@team") || !strings.Contains(string(data), "sync: false") {
			t.Fatalf("plugin config lost: %s", data)
		}
	}
}
