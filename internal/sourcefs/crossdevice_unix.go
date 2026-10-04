//go:build !windows

package sourcefs

import (
	"errors"
	"syscall"
)

// IsCrossDevice reports whether err is a rename that failed because the two
// paths live on different filesystems. It is the only MoveIn failure that
// CopyIn may recover from.
func IsCrossDevice(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}
