package hooks

import (
	"encoding/json"
	"os"
	"testing"
)

func TestHookSchemaEntryDoesNotAdvertiseUnknownFields(t *testing.T) {
	data, err := os.ReadFile("../../schemas/hooks.schema.json")
	must(t, err)
	var schema struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	must(t, json.Unmarshal(data, &schema))
	var definition struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Additional *bool                      `json:"additionalProperties"`
	}
	must(t, json.Unmarshal(schema.Defs["entry"], &definition))
	for field := range definition.Properties {
		var value any
		// Give the parser a valid value for known fields. Schema keys must be
		// accepted source fields rather than editor-only phantom properties.
		switch field {
		case "description":
			value = "description"
		case "enabled":
			value = true
		default:
			value = map[string]any{}
		}
		doc, _ := json.Marshal(map[string]any{"bindings": map[string]any{}, field: value})
		if _, err := ParseEntry(doc); err != nil {
			t.Errorf("schema advertises rejected entry field %s: %v", field, err)
		}
	}
	if definition.Additional == nil || *definition.Additional {
		t.Fatal("schema must explicitly reject unknown entry fields with additionalProperties: false")
	}
}
