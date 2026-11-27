package server

import (
	"fmt"
	"io/fs"
	"net/http/httptest"
	"testing"

	"skillshare/internal/filepreview"
)

// Every file viewer (plugins, extensions, MCP) maps read errors the same way, so the dialog
// can show its own text for a missing or oversized file.
func TestWritePreviewErrorStatus(t *testing.T) {
	for err, want := range map[error]int{
		fmt.Errorf("open a: %w", fs.ErrNotExist):       404,
		fmt.Errorf("a is %w", filepreview.ErrTooLarge): 413,
		fmt.Errorf("path escapes from parent"):         400,
	} {
		rec := httptest.NewRecorder()
		writePreviewError(rec, err, "a")
		if rec.Code != want {
			t.Errorf("%v: %d, want %d", err, rec.Code, want)
		}
	}
}
