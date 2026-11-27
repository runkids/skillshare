package server

import (
	"errors"
	"io/fs"
	"net/http"

	"skillshare/internal/filepreview"
)

// writePreviewError reports why a file could not be shown: missing, too large, or unreadable.
func writePreviewError(w http.ResponseWriter, err error, name string) {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		writeError(w, http.StatusNotFound, "file not found: "+name)
	case errors.Is(err, filepreview.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
