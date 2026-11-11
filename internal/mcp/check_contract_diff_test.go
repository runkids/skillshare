package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func schemaContract(t *testing.T, schema string) ToolContract {
	t.Helper()
	_, c, err := captureTool(json.RawMessage(`{"name":"fixture","inputSchema":`+schema+`}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCompareContractsConservativeDirections(t *testing.T) {
	cases := []struct{ old, now, grade string }{
		{`{"type":"object"}`, `{"type":"object","properties":{"x":{"type":"string"}}}`, "UNKNOWN"},
		{`{"type":"object","additionalProperties":false}`, `{"type":"object","additionalProperties":false,"properties":{"x":{"type":"string"}}}`, "EXPANDED"},
		{`{"properties":{"x":{"type":"number"}}}`, `{"properties":{"x":{"type":"integer"}}}`, "BREAKING"},
		{`{"properties":{"x":{"type":"integer"}}}`, `{"properties":{"x":{"type":"number"}}}`, "EXPANDED"},
		{`{"properties":{"x":{"minimum":9007199254740992}}}`, `{"properties":{"x":{"minimum":9007199254740993}}}`, "BREAKING"},
		{`{"properties":{"x":{"maximum":1e900}}}`, `{"properties":{"x":{"maximum":9e899}}}`, "BREAKING"},
		{`{"required":["x"]}`, `{"required":[]}`, "LOOSENED"},
		{`{"required":["a","b"]}`, `{"required":["b","a"]}`, "UNKNOWN"},
		{`{"properties":{"x":{"enum":[1,2]}}}`, `{"properties":{"x":{"enum":[2,3]}}}`, "UNKNOWN"},
		{`{"allOf":[{"required":["x"]}]}`, `{"allOf":[{"required":[]}]}`, "UNKNOWN"},
		{`{"properties":{"x":{"description":"one"}}}`, `{"properties":{"x":{"description":"two"}}}`, "NOTATION"},
		{`{"x-extension":1}`, `{"x-extension":2}`, "UNKNOWN"},
		{`{"additionalProperties":false}`, `{"additionalProperties":true}`, "LOOSENED"},
		{`{"properties":{"x":{"minimum":1}},"not":{}}`, `{"properties":{"x":{"minimum":2}},"not":{}}`, "UNKNOWN"},
		{`{"properties":{"x":{"minimum":1}}}`, `{"properties":{"x":{"minimum":2,"maximum":9}}}`, "UNKNOWN"},
	}
	for _, tt := range cases {
		t.Run(tt.old+tt.now, func(t *testing.T) {
			changes, omitted := compareContracts(map[string]ToolContract{"fixture": schemaContract(t, tt.old)}, map[string]ToolContract{"fixture": schemaContract(t, tt.now)})
			if omitted != 0 || len(changes) == 0 {
				t.Fatal(changes, omitted)
			}
			found := false
			for _, c := range changes {
				if c.Area == "input" && c.Grade == tt.grade {
					found = true
				}
				if strings.Contains(c.Message, "900719") {
					t.Fatal("payload in message")
				}
			}
			if !found {
				t.Fatal("missing grade", tt.grade, changes)
			}
		})
	}
}

func TestCompareContractsMixedDimensionsAndOutputUnknown(t *testing.T) {
	a := capturedContract(t, `,"description":"one","outputSchema":{"type":"object"},"title":"one","annotations":{"readOnlyHint":true}`)
	_, b, err := captureTool(json.RawMessage(`{"name":"fixture","inputSchema":{"type":"object","required":["x"]},"description":"two","outputSchema":{"type":"object","required":["x"]},"title":"two","annotations":{"readOnlyHint":false},"unknown":true}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	changes, _ := compareContracts(map[string]ToolContract{"fixture": a}, map[string]ToolContract{"fixture": b})
	areas := map[string]bool{}
	for _, c := range changes {
		areas[c.Area] = true
		if c.Area == "output" && c.Grade != "UNKNOWN" {
			t.Fatal("output incorrectly treated as input", c)
		}
	}
	for _, area := range []string{"input", "prose", "output", "metadata"} {
		if !areas[area] {
			t.Fatal("dimension lost", area, changes)
		}
	}
}

func TestCompareContractsRemovedNullMetadataAlongsideProse(t *testing.T) {
	a := capturedContract(t, `,"description":"one","extension":null`)
	b := capturedContract(t, `,"description":"two"`)
	changes, _ := compareContracts(map[string]ToolContract{"fixture": a}, map[string]ToolContract{"fixture": b})
	found := false
	for _, c := range changes {
		if c.Area == "metadata" && c.Path == "/extension" {
			found = true
		}
	}
	if !found {
		t.Fatal("null metadata removal lost", changes)
	}
}

func TestCompareContractsBoundsDiagnosticLocations(t *testing.T) {
	key := strings.Repeat("z", 9000)
	a := capturedContract(t, `,"extension":{"`+key+`":1}`)
	b := capturedContract(t, `,"extension":{"`+key+`":2}`)
	changes, _ := compareContracts(map[string]ToolContract{"fixture": a}, map[string]ToolContract{"fixture": b})
	if len(changes) != 1 || len(changes[0].Path) > 4096 || !strings.Contains(changes[0].Message, "truncated") {
		t.Fatal("unbounded/hidden location truncation", len(changes))
	}
}

func TestCompareContractsMixedOmissionAccounting(t *testing.T) {
	fieldsA, fieldsB := []string{}, []string{}
	for i := 0; i < 225; i++ {
		fieldsA = append(fieldsA, fmt.Sprintf(`"k%03d":0`, i))
		fieldsB = append(fieldsB, fmt.Sprintf(`"k%03d":1`, i))
	}
	a := capturedContract(t, `,"description":"one","extension":{`+strings.Join(fieldsA, ",")+`}`)
	b := capturedContract(t, `,"description":"two","extension":{`+strings.Join(fieldsB, ",")+`}`)
	changes, omitted := compareContracts(map[string]ToolContract{"fixture": a}, map[string]ToolContract{"fixture": b})
	if len(changes) != 200 || omitted != 26 {
		t.Fatal("mixed details lost", len(changes), omitted)
	}
	t.Run("CompareContractsCountsEveryOmittedDetail", func(t *testing.T) {
		old, now := map[string]ToolContract{}, map[string]ToolContract{}
		for i := 0; i < 230; i++ {
			now[fmt.Sprintf("tool-%03d", i)] = capturedContract(t, "")
		}
		changes, omitted := compareContracts(old, now)
		if len(changes) != 200 || omitted != 30 {
			t.Fatal(len(changes), omitted)
		}
		changes, omitted = compareContracts(now, old)
		if len(changes) != 200 || omitted != 30 {
			t.Fatal(len(changes), omitted)
		}
		for _, c := range changes {
			if c.Grade != "BREAKING" {
				t.Fatal(c)
			}
		}
	})
}

func TestCompareContractsExactNumericExtremes(t *testing.T) {
	for _, tt := range []struct{ a, b, grade string }{
		{"-9007199254740993", "-9007199254740992", "BREAKING"},
		{"1e999999999999999999999999", "9e999999999999999999999998", "LOOSENED"},
		{"1e-999999999999999999999999", "9e-999999999999999999999999", "BREAKING"},
		{"-0", "0.0e99", "UNKNOWN"},
		{"1e2", "100.0", "UNKNOWN"},
	} {
		a := schemaContract(t, `{"properties":{"x":{"minimum":`+tt.a+`}}}`)
		b := schemaContract(t, `{"properties":{"x":{"minimum":`+tt.b+`}}}`)
		changes, _ := compareContracts(map[string]ToolContract{"fixture": a}, map[string]ToolContract{"fixture": b})
		if len(changes) != 1 || changes[0].Grade != tt.grade {
			t.Fatal("inexact decimal direction", tt, changes)
		}
	}
}

func TestCompareContractsNumericEnumsNeverInferLexicalDirection(t *testing.T) {
	for _, tt := range []struct{ before, after string }{
		{`[1,1.0]`, `[1]`},
		{`[1]`, `[1,1.0]`},
		{`[1e0,1.00]`, `[1.00]`},
		{`[0,-0]`, `[0]`},
		{`[{"x":[1]},{"x":[1.0]}]`, `[{"x":[1]}]`},
		{`[[{"x":1}]]`, `[[{"x":1}],[{"x":1.0}]]`},
		{`["one"]`, `["one",{"nested":{"value":1}}]`},
		{`[1]`, `[1,2]`},
	} {
		a := schemaContract(t, `{"properties":{"x":{"enum":`+tt.before+`}}}`)
		b := schemaContract(t, `{"properties":{"x":{"enum":`+tt.after+`}}}`)
		changes, _ := compareContracts(map[string]ToolContract{"fixture": a}, map[string]ToolContract{"fixture": b})
		if len(changes) != 1 || changes[0].Grade != "UNKNOWN" || changes[0].Path != "/inputSchema/properties/x/enum" {
			t.Fatal("numeric enum given lexical direction", tt, changes)
		}
		if a.Hash == b.Hash || a.InputHash == b.InputHash {
			t.Fatal("numeric enum fingerprint lexemes lost", tt)
		}
	}
	for _, tt := range []struct{ before, after, grade string }{
		{`["one"]`, `["one","two"]`, "EXPANDED"},
		{`["one","two"]`, `["one"]`, "BREAKING"},
		{`[{"x":["one"]}]`, `[{"x":["one"]},false,null]`, "EXPANDED"},
	} {
		a := schemaContract(t, `{"properties":{"x":{"enum":`+tt.before+`}}}`)
		b := schemaContract(t, `{"properties":{"x":{"enum":`+tt.after+`}}}`)
		changes, _ := compareContracts(map[string]ToolContract{"fixture": a}, map[string]ToolContract{"fixture": b})
		if len(changes) != 1 || changes[0].Grade != tt.grade {
			t.Fatal("nonnumeric subset direction lost", tt, changes)
		}
	}
}
