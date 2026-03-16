package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"time"

	"skillshare/internal/ui"
)

// writeJSON pretty-prints v as JSON to stdout.
// Nil slices are converted to empty arrays to ensure valid JSON ([] not null).
func writeJSON(v any) error {
	ensureEmptySlices(v)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// jsonSilentError is a sentinel error that signals main() to exit with
// code 1 without printing anything to stdout. The JSON error has already
// been written by writeJSONError, so main must not add plain-text output.
type jsonSilentError struct{ cause error }

func (e *jsonSilentError) Error() string { return e.cause.Error() }
func (e *jsonSilentError) Unwrap() error { return e.cause }

// writeJSONResult writes v as JSON, then wraps cmdErr (if non-nil) in
// jsonSilentError so that main() exits non-zero without extra output.
func writeJSONResult(v any, cmdErr error) error {
	if writeErr := writeJSON(v); writeErr != nil {
		return writeErr
	}
	if cmdErr != nil {
		return &jsonSilentError{cause: cmdErr}
	}
	return nil
}

// writeJSONError writes a JSON error object to stdout and returns a
// jsonSilentError so that main() exits non-zero without extra output.
func writeJSONError(err error) error {
	out, merr := json.MarshalIndent(map[string]string{"error": err.Error()}, "", "  ")
	if merr != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: %v\n", merr)
		fmt.Printf("{\"error\": %q}\n", err.Error())
	} else {
		os.Stdout.Write(out)          //nolint:errcheck
		os.Stdout.Write([]byte("\n")) //nolint:errcheck
	}
	return &jsonSilentError{cause: err}
}

// suppressUIToDevnull temporarily redirects os.Stdout and the progress
// writer to /dev/null so that handler functions using fmt.Printf / ui.*
// produce zero visible output.  This keeps --json output clean even when
// stdout and stderr share the same terminal (e.g. docker exec).
// Returns a restore function that MUST be called before writing JSON.
func suppressUIToDevnull() func() {
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		// Fallback: redirect to stderr (better than nothing)
		devnull = os.Stderr
	}
	origStdout := os.Stdout
	os.Stdout = devnull

	prevProgress := ui.ProgressWriter
	ui.SetProgressWriter(devnull)
	ui.SuppressProgress()

	return func() {
		os.Stdout = origStdout
		ui.SetProgressWriter(prevProgress)
		ui.RestoreProgress()
		if devnull != os.Stderr {
			devnull.Close()
		}
	}
}

// formatDuration returns a human-readable duration string truncated to milliseconds.
func formatDuration(start time.Time) string {
	return time.Since(start).Truncate(time.Millisecond).String()
}

// hasFlag checks if a flag is present in args.
func hasFlag(args []string, flag string) bool {
	return slices.Contains(args, flag)
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
