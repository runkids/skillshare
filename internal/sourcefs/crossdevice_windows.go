package sourcefs

import (
	"errors"

	"golang.org/x/sys/windows"
)

// IsCrossDevice reports whether err is a rename that failed because the two
// paths live on different volumes. It is the only MoveIn failure that CopyIn
// may recover from.
func IsCrossDevice(err error) bool {
	return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE)
}
