package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// directTools and the adapter's includeTools/excludeTools were pi-mcp-adapter settings. Loading
// converts them to what Pi's built-in MCP reads, and the next save writes the result.
func TestDirectToolsConvertedOnLoad(t *testing.T) {
	s, tmp := projectsService(t, `mcp:
  targets: [pi]
  directTools: true
  servers:
    inherits:
      command: a
    elsewhere:
      command: b
      targets: [claude]
    names:
      command: c
      directTools: [search, fetch]
    held:
      command: d
      directTools: search
    lists:
      command: e
      directTools: true
      piOptions:
        excludeTools: ["delete_*"]
        includeTools: ["get_*"]
  projects:
    $TMP/quiet:
      directTools: false
      servers:
        local:
          command: f
`)
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	pi := func(name string) string {
		data, _ := json.Marshal(source.Servers[name].PiOptions)
		return string(data)
	}
	if got := pi("inherits"); got != `{"exposure":"direct"}` {
		t.Errorf("mcp.directTools default: %s", got)
	}
	if got := source.Servers["elsewhere"]; got.PiOptions != nil || !got.Tools.IsZero() {
		t.Errorf("the default reached a server without Pi: %+v", got)
	}
	if got := pi("names"); got != `{"toolExposure":{"search":"direct","fetch":"direct"}}` {
		t.Errorf("a list keeps the other tools at Pi's default: %s", got)
	}
	if got := pi("held"); got != `{"exposure":"deferred"}` {
		t.Errorf("search: %s", got)
	}
	if got, want := source.Servers["lists"].Tools, (ToolPolicy{Allow: []string{"get_*"}, Deny: []string{"delete_*"}}); !reflect.DeepEqual(got, want) || pi("lists") != `{"exposure":"direct"}` {
		t.Errorf("tool lists: %+v %v", got, source.Servers["lists"].PiOptions)
	}
	if got := source.Projects[filepath.Join(tmp, "quiet")].Servers["local"]; got.PiOptions != nil {
		t.Errorf("a project's false overrides the global default: %+v", got)
	}
	if len(source.Notices) != 1 || !strings.HasSuffix(source.Notices[0], "the next sync converts them: held, inherits, lists, names") {
		t.Fatalf("notices: %q", source.Notices)
	}
	if _, err := s.Mutate(Mutation{Name: "other", Server: &Server{Command: "other"}}, "", false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(s.ConfigPath)
	for _, gone := range []string{"directTools", "excludeTools", "includeTools"} {
		if strings.Contains(string(data), gone) {
			t.Fatalf("saved config keeps %s: %s", gone, data)
		}
	}
	if source, err = LoadSource(s.ConfigPath); err != nil || len(source.Notices) != 0 || pi("inherits") != `{"exposure":"direct"}` {
		t.Fatalf("after save: %v %v", source.Notices, err)
	}
}

// A setting that would overwrite exposure the server already sets is dropped, and said so.
func TestDirectToolsDroppedWhenExposureIsSet(t *testing.T) {
	s, _ := projectsService(t, `mcp:
  targets: [pi]
  servers:
    docs:
      command: a
      directTools: true
      piOptions:
        exposure: hidden
`)
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := source.Servers["docs"].PiOptions["exposure"]; got != "hidden" {
		t.Fatalf("exposure: %v", got)
	}
	if len(source.Notices) != 2 || !strings.HasSuffix(source.Notices[1], "are dropped; the next sync removes them: docs") {
		t.Fatalf("notices: %q", source.Notices)
	}
}

func TestDirectToolsDefaultRejectsBadValue(t *testing.T) {
	s, _ := projectsService(t, "mcp:\n  directTools: all\n")
	if _, err := s.Preview(); err == nil || !strings.Contains(err.Error(), "directTools") {
		t.Fatalf("got %v", err)
	}
}

// A dashboard that still sends directTools gets the same conversion.
func TestServerJSONConvertsDirectTools(t *testing.T) {
	var server Server
	if err := json.Unmarshal([]byte(`{"command":"docs","directTools":true}`), &server); err != nil || server.PiOptions["exposure"] != "direct" {
		t.Fatalf("%+v %v", server, err)
	}
	var settings Mutation
	if err := json.Unmarshal([]byte(`{"settings":{"targets":["pi"],"directTools":true}}`), &settings); err != nil {
		t.Fatalf("settings.directTools must still be accepted: %v", err)
	}
}

func TestAdapterToolsImportedFromPi(t *testing.T) {
	candidates, err := Import("pi", []byte(`{"mcpServers":{"docs":{"command":"docs","directTools":true,"excludeTools":["delete_*"],"lifecycle":"eager"}}}`), "")
	if err != nil || len(candidates) != 1 || len(candidates[0].Problems) != 0 {
		t.Fatalf("%+v %v", candidates, err)
	}
	if got, want := candidates[0].Server.Tools, (ToolPolicy{Deny: []string{"delete_*"}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("tools: %+v", got)
	}
	if got := candidates[0].Server.PiOptions; len(got) != 1 || got["exposure"] != "direct" {
		t.Fatalf("directTools becomes piOptions.exposure, and nothing else reaches piOptions: %v", got)
	}
	if warnings := strings.Join(candidates[0].Warnings, ";"); !strings.Contains(warnings, "converted") || !strings.Contains(warnings, "not imported, because Pi's built-in MCP does not read it: lifecycle") {
		t.Fatalf("warnings: %v", candidates[0].Warnings)
	}
}
