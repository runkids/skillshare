//go:build !unix

package install

import "os"

// Non-Unix opens retain the shared Lstat/reparse-path guards around flock.
const extraLockOpenFlags = os.O_CREATE | os.O_RDWR
