//go:build darwin

package plugin

import "github.com/gofrs/flock"

func ompLockSupported() bool { return true }

func ompAcquireLock(path string) (func() error, error) {
	path, err := ompResolvedLockPath(path)
	if err != nil {
		return nil, err
	}
	if err := ompSafePath(path + ".lock"); err != nil {
		return nil, err
	}
	lock := flock.New(path+".lock", flock.SetPermissions(0600))
	ok, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrOMPExtensionsBusy
	}
	return lock.Close, nil
}
