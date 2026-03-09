package main

import (
	"encoding/json"
	"io"
)

// WriteJSONReport writes the report as indented JSON.
func WriteJSONReport(w io.Writer, report Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
