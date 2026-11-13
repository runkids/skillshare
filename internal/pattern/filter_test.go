package pattern

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	include, exclude, err := Normalize([]string{" team__* ", "", " "}, []string{" *-old "})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(include, []string{"team__*"}) || !reflect.DeepEqual(exclude, []string{"*-old"}) {
		t.Fatalf("normalized patterns = %v, %v", include, exclude)
	}
	for _, tc := range []struct {
		include, exclude []string
		want             string
	}{
		{[]string{"["}, nil, `invalid include pattern: "["`},
		{nil, []string{"["}, `invalid exclude pattern: "["`},
	} {
		_, _, err := Normalize(tc.include, tc.exclude)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Normalize(%v, %v) error = %v, want %q", tc.include, tc.exclude, err, tc.want)
		}
	}
}

func TestMatch(t *testing.T) {
	for _, tc := range []struct {
		name             string
		include, exclude []string
		want             bool
	}{
		{"team__one", nil, nil, true},
		{"team__one", []string{"team__*"}, []string{"*-old"}, true},
		{"team__old", []string{"team__*"}, []string{"*-old"}, false},
		{"other", []string{"team__*"}, nil, false},
		{"team" + string(os.PathSeparator) + "one", []string{"team*"}, nil, false},
	} {
		if got := Match(tc.name, tc.include, tc.exclude); got != tc.want {
			t.Errorf("Match(%q, %v, %v) = %v, want %v", tc.name, tc.include, tc.exclude, got, tc.want)
		}
	}
	if got, ok := FirstMatch("team__one", []string{"other*", "team__*", "*"}); !ok || got != "team__*" {
		t.Errorf("FirstMatch = %q, %v; want first matching pattern", got, ok)
	}
}
