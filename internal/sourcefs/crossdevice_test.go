package sourcefs

import (
	"errors"
	"os"
	"testing"
)

func TestIsCrossDeviceMatchesOnlyRenameAcrossFilesystems(t *testing.T) {
	if IsCrossDevice(&os.LinkError{Op: "rename", Old: "a", New: "b", Err: crossDeviceErrno}) != true {
		t.Fatal("a wrapped cross-device errno must match")
	}
	for name, err := range map[string]error{
		"link refusal": &LinkError{Path: "/src/_f"},
		"exists":       errors.New("/src/a already exists"),
		"not exist":    os.ErrNotExist,
	} {
		if IsCrossDevice(err) {
			t.Fatalf("%s must not count as cross-device", name)
		}
	}
}
