//go:build windows

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ioReparseTagMountPoint is the reparse tag of a junction (IO_REPARSE_TAG_MOUNT_POINT).
const ioReparseTagMountPoint = 0xA0000003

// platformIsJunction reports whether path is a junction. Other reparse points,
// such as OneDrive placeholders or deduplicated files, are not links.
func platformIsJunction(path string) bool {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	var data syscall.Win32finddata
	h, err := syscall.FindFirstFile(ptr, &data)
	if err != nil {
		return false
	}
	syscall.FindClose(h)
	// For a reparse point, FindFirstFile reports its tag in Reserved0.
	return data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 && data.Reserved0 == ioReparseTagMountPoint
}

// fsctlSetReparsePoint is FSCTL_SET_REPARSE_POINT.
const fsctlSetReparsePoint = 0x000900A4

// CreateJunction creates a directory junction (no admin required) through the
// reparse-point API, so no shell ever parses the paths. It never falls back to
// a symlink, so callers can restore a junction exactly. Like mklink /J it fails
// when linkPath already exists.
func CreateJunction(linkPath, sourcePath string) error {
	target, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("create junction %s: %w", linkPath, err)
	}
	// Like mklink /J, refuse a network path: a junction cannot point at one.
	if strings.HasPrefix(target, `\\`) && !strings.HasPrefix(target, `\\?\`) {
		return fmt.Errorf("create junction %s: a junction cannot target the network path %s", linkPath, target)
	}
	if err := os.Mkdir(linkPath, 0777); err != nil {
		return err
	}
	if err := setJunctionTarget(linkPath, target); err != nil {
		os.Remove(linkPath)
		return fmt.Errorf("create junction %s: %w", linkPath, err)
	}
	return nil
}

func setJunctionTarget(linkPath, target string) error {
	name, err := syscall.UTF16PtrFromString(linkPath)
	if err != nil {
		return err
	}
	h, err := syscall.CreateFile(name, syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(h)

	// The substitute name is an NT path: \??\C:\x. A Win32 extended path such
	// as \\?\C:\x or \\?\Volume{GUID}\x only needs its prefix swapped.
	display := strings.TrimPrefix(target, `\\?\`)
	sub, err := syscall.UTF16FromString(`\??\` + display)
	if err != nil {
		return err
	}
	print, err := syscall.UTF16FromString(display)
	if err != nil {
		return err
	}
	// sub and print include their NUL terminators; the name lengths exclude them.
	subBytes, printBytes := (len(sub)-1)*2, (len(print)-1)*2
	pathBytes := len(sub)*2 + len(print)*2
	buf := make([]byte, 16+pathBytes)
	put16 := func(off int, v int) { buf[off], buf[off+1] = byte(v), byte(v>>8) }
	buf[0], buf[1], buf[2], buf[3] = 0x03, 0x00, 0x00, 0xA0 // IO_REPARSE_TAG_MOUNT_POINT
	put16(4, 8+pathBytes)                                   // ReparseDataLength
	put16(8, 0)                                             // SubstituteNameOffset
	put16(10, subBytes)
	put16(12, len(sub)*2) // PrintNameOffset
	put16(14, printBytes)
	for i, c := range append(append([]uint16{}, sub...), print...) {
		put16(16+i*2, int(c))
	}
	var returned uint32
	return syscall.DeviceIoControl(h, fsctlSetReparsePoint, &buf[0], uint32(len(buf)), nil, 0, &returned, nil)
}

// canonicalSpelling expands DOS short names after resolving links. A junction
// target returned by Readlink can use long names even when its caller used an
// 8.3 alias; both sides must use one spelling before filepath.Rel or comparison.
func canonicalSpelling(path string) string {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return path
	}
	buf := make([]uint16, 260)
	for {
		n, err := syscall.GetLongPathName(name, &buf[0], uint32(len(buf)))
		if err != nil {
			return path
		}
		if n < uint32(len(buf)) {
			return syscall.UTF16ToString(buf[:n])
		}
		buf = make([]uint16, n+1)
	}
}
