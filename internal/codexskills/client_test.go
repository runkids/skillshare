package codexskills

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

type writeBuffer struct{ bytes.Buffer }

func (*writeBuffer) Close() error { return nil }

func TestClientCatalogHandlesNotificationsAndRejectsLoadErrors(t *testing.T) {
	for _, bad := range []bool{false, true} {
		errorsJSON := "[]"
		if bad {
			errorsJSON = `[{"message":"unreadable skill"}]`
		}
		input := `{"method":"skills/changed"}` + "\n" + `{"id":1,"result":{"data":[{"skills":[{"name":"gem","path":"/skills/gem/SKILL.md","scope":"user","enabled":true,"pluginId":null}],"errors":` + errorsJSON + `}]}}`
		out := &writeBuffer{}
		c := &Client{in: out, out: json.NewDecoder(strings.NewReader(input))}
		skills, err := c.List("/workspace")
		if bad {
			if err == nil {
				t.Fatal("accepted incomplete catalog")
			}
		} else if err != nil || len(skills) != 1 || skills[0].PluginID != "" {
			t.Fatalf("catalog: %+v %v", skills, err)
		}
		var request struct {
			Method string
			Params struct {
				Cwds        []string
				ForceReload bool
			}
		}
		if err := json.Unmarshal(out.Bytes(), &request); err != nil {
			t.Fatal(err)
		}
		if request.Method != "skills/list" || len(request.Params.Cwds) != 1 || request.Params.Cwds[0] != "/workspace" || !request.Params.ForceReload {
			t.Fatalf("wrong request: %s", out.String())
		}
	}
}

func TestClientDisableUsesPathAndPropagatesError(t *testing.T) {
	for _, response := range []string{`{"id":1,"result":{"effectiveEnabled":false}}`, `{"id":1,"error":{"message":"write denied"}}`} {
		out := &writeBuffer{}
		c := &Client{in: out, out: json.NewDecoder(strings.NewReader(response))}
		err := c.Disable("/skills/gem/SKILL.md")
		if strings.Contains(response, "write denied") && (err == nil || !strings.Contains(err.Error(), "write denied")) {
			t.Fatalf("lost RPC failure: %v", err)
		}
		var request map[string]any
		if err := json.Unmarshal(out.Bytes(), &request); err != nil {
			t.Fatal(err)
		}
		params := request["params"].(map[string]any)
		if request["method"] != "skills/config/write" || params["path"] != "/skills/gem/SKILL.md" || params["enabled"] != false || params["name"] != nil {
			t.Fatalf("wrong edit: %s", out.String())
		}
	}
}

func TestClientEOFDoesNotHang(t *testing.T) {
	c := &Client{in: &writeBuffer{}, out: json.NewDecoder(strings.NewReader(""))}
	if _, err := c.List("/workspace"); err == nil || !strings.Contains(err.Error(), io.EOF.Error()) {
		t.Fatalf("expected EOF: %v", err)
	}
}
