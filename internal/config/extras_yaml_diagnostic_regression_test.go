package config

import (
	"errors"
	"strings"
	"testing"
)

func TestRawExtras_SyntaxLocationWithoutParserValues(t *testing.T) {
	// yaml.v3 itself pins this input's numeric location in decode_test.go.
	_, err := RawExtras([]byte("a: 1\nb: 2\nSYNTHETIC 2\nd: 3\n"))
	if err == nil || !strings.Contains(err.Error(), "line 3") || strings.Contains(err.Error(), "SYNTHETIC") {
		t.Fatalf("safe syntax location missing: %v", err)
	}
	for _, raw := range []string{"extras: *SYNTHETIC\n", "extras: [SYNTHETIC\n", "extras: &SYNTHETIC [*SYNTHETIC]\n", "SYNTHETIC: x\nSYNTHETIC: y\n", "extras: SYNTHETIC\n"} {
		_, err := RawExtras([]byte(raw))
		if err == nil || strings.Contains(err.Error(), "SYNTHETIC") {
			t.Errorf("parser data leaked: %v", err)
		}
	}
	if _, err := RawExtras([]byte("extras: SYNTHETIC\n")); !errors.Is(err, ErrRawExtrasUnreadable) {
		t.Fatalf("type-error recovery classification changed: %v", err)
	}
}
