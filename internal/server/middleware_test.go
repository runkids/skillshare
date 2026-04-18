package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorIncludesStableCodeAndLegacyMessage(t *testing.T) {
	rr := httptest.NewRecorder()

	writeError(rr, http.StatusNotFound, "target not found: claude")

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}

	var body struct {
		Error     string `json:"error"`
		ErrorCode string `json:"error_code"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error != "target not found: claude" {
		t.Fatalf("legacy error mismatch: %q", body.Error)
	}
	if body.ErrorCode != "not_found" {
		t.Fatalf("error code mismatch: %q", body.ErrorCode)
	}
}

func TestWriteCodedErrorIncludesParams(t *testing.T) {
	rr := httptest.NewRecorder()

	writeCodedError(rr, http.StatusNotFound, "target.not_found", "target not found: claude", map[string]string{"target": "claude"})

	var body struct {
		Error       string            `json:"error"`
		ErrorCode   string            `json:"error_code"`
		ErrorParams map[string]string `json:"error_params"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ErrorCode != "target.not_found" {
		t.Fatalf("error code mismatch: %q", body.ErrorCode)
	}
	if body.ErrorParams["target"] != "claude" {
		t.Fatalf("target param mismatch: %#v", body.ErrorParams)
	}
}
