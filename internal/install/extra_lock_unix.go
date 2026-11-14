//go:build unix

package install

import (
	"os"
	"syscall"
)

const extraLockOpenFlags = os.O_CREATE | os.O_RDWR | syscall.O_NOFOLLOW
