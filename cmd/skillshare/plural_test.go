package main

import "testing"

func TestPlural_ConsonantYBecomesIes(t *testing.T) {
	if got := plural(10, "category"); got != "10 categories" {
		t.Fatalf("plural(10, category) = %q", got)
	}
}

func TestPlural_VowelYStaysRegular(t *testing.T) {
	if got := plural(3, "day"); got != "3 days" {
		t.Fatalf("plural(3, day) = %q", got)
	}
}
