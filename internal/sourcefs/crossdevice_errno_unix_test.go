//go:build !windows

package sourcefs

import "syscall"

var crossDeviceErrno = syscall.EXDEV
