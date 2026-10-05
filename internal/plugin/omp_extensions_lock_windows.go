//go:build windows

package plugin

import (
	"errors"
	"golang.org/x/sys/windows"
)

func ompLockSupported() bool { return true }

func ompAcquireLock(path string) (func() error, error) {
	path, err := ompResolvedLockPath(path)
	if err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(`Global\` + ompLockName(path+".lock"))
	if err != nil {
		return nil, err
	}
	// Native uses named-object existence, not thread-affine mutex ownership.
	handle, err := windows.CreateMutex(nil, false, name)
	if handle != 0 && errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		windows.CloseHandle(handle)
		return nil, ErrOMPExtensionsBusy
	}
	if err != nil {
		return nil, err
	}
	return func() error { return windows.CloseHandle(handle) }, nil
}
