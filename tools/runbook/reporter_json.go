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

// WriteJSONReports writes multiple reports as a JSON array.
func WriteJSONReports(w io.Writer, reports []Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(reports)
}
