package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"
)

// writeJSON pretty-prints v as JSON to stdout.
// Nil slices are converted to empty arrays to ensure valid JSON ([] not null).
func writeJSON(v any) error {
	ensureEmptySlices(v)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// writeJSONError writes a JSON error object to stdout.
// Used when a command fails but we still need parseable JSON output.
func writeJSONError(err error) {
	out, _ := json.MarshalIndent(map[string]string{"error": err.Error()}, "", "  ")
	fmt.Println(string(out))
}

// formatDuration returns a human-readable duration string truncated to milliseconds.
func formatDuration(start time.Time) string {
	return time.Since(start).Truncate(time.Millisecond).String()
}

// hasFlag checks if a flag is present in args.
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// ensureEmptySlices recursively walks exported struct fields and replaces nil
// slices with empty slices so json.Marshal produces [] instead of null.
// It handles nested structs and slices of structs.
func ensureEmptySlices(v any) {
	rv := reflect.ValueOf(v)
	ensureEmptySlicesValue(rv)
}

func ensureEmptySlicesValue(rv reflect.Value) {
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		if !f.CanSet() {
			continue
		}
		switch f.Kind() {
		case reflect.Slice:
			if f.IsNil() {
				f.Set(reflect.MakeSlice(f.Type(), 0, 0))
			} else if f.Type().Elem().Kind() == reflect.Struct {
				// Recurse into each element of a slice of structs
				for j := 0; j < f.Len(); j++ {
					ensureEmptySlicesValue(f.Index(j))
				}
			}
		case reflect.Struct:
			ensureEmptySlicesValue(f)
		}
	}
}
