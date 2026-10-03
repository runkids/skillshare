package plugin

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func openPiLockAnchor(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.DELETE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

func removePiLockDirectory(_ string, anchor *os.File) error {
	// POSIX disposition removes this anchored empty directory's name when our
	// delete handle closes, even while another share-delete anchor remains open.
	// Unsupported filesystems fail closed; never fall back to recursive removal.
	flags := uint32(0x1 | 0x2) // FILE_DISPOSITION_DELETE | FILE_DISPOSITION_POSIX_SEMANTICS
	return windows.SetFileInformationByHandle(windows.Handle(anchor.Fd()), windows.FileDispositionInfoEx, (*byte)(unsafe.Pointer(&flags)), uint32(unsafe.Sizeof(flags)))
}
