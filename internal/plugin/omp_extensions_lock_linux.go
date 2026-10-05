//go:build linux

package plugin

import (
	"errors"
	"net"
	"syscall"
)

func ompLockSupported() bool { return true }

func ompAcquireLock(path string) (func() error, error) {
	path, err := ompResolvedLockPath(path)
	if err != nil {
		return nil, err
	}
	// Do not substitute flock: native Linux writers use an abstract socket.
	c, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: "\x00" + ompLockName(path+".lock"), Net: "unixgram"})
	if errors.Is(err, syscall.EADDRINUSE) {
		return nil, ErrOMPExtensionsBusy
	}
	if err != nil {
		return nil, err
	}
	return c.Close, nil
}
