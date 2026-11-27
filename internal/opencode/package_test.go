package opencode

import (
	"io/fs"
	"testing"
)

func TestPackageLoaderInputs(t *testing.T) {
	for _, tt := range []struct {
		name, manifest, entry string
		valid                 bool
	}{
		{"kata", `{"name":"kata-native-agent-hooks","type":"module","exports":"./index.js","dependencies":{"@opencode/plugin":"^2.0.0"}}`, "index.js", true},
		{"server", `{"name":"demo","exports":{"./server":"./src/server.ts",".":"./other.js"}}`, "src/server.ts", true},
		{"root export", `{"name":"demo","exports":{".":"./src/plugin.js"}}`, "src/plugin.js", true},
		{"convention", `{"name":"demo","main":"missing.js"}`, "server.tsx", true},
		{"missing export", `{"name":"demo","exports":"./missing.js"}`, "index.js", false},
		{"explicit only", `{"name":"demo","main":"src/plugin.js"}`, "src/plugin.js", false},
		{"unnamed export", `{"exports":"./src/plugin.js"}`, "src/plugin.js", false},
		{"conditional", `{"name":"demo","exports":{".":{"import":"./index.js","require":null}}}`, "index.js", false},
		{"escape", `{"name":"demo","exports":"./../index.js"}`, "../index.js", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string]string{"package.json": tt.manifest, tt.entry: "export default {}"}
			err := Package(func(name string) ([]byte, error) {
				v, ok := files[name]
				if !ok {
					return nil, fs.ErrNotExist
				}
				return []byte(v), nil
			})
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v: %v", tt.valid, err)
			}
		})
	}
}
