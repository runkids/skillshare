//go:build !unix

package inspect

import "errors"

func createTestFIFO(path string, mode uint32) error {
	return errors.New("fifo creation is not supported on this platform")
}
