//go:build !windows

package contextrouter

import "syscall"

// nonblock keeps opening a FIFO from waiting for a writer.
const nonblock = syscall.O_NONBLOCK
