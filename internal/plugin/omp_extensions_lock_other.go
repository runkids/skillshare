//go:build !linux && !darwin && !windows

package plugin

func ompLockSupported() bool                      { return false }
func ompAcquireLock(string) (func() error, error) { return nil, ErrOMPExtensionsReadOnly }
