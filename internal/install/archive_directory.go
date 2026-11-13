package install

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
)

// preflightArchive bounds directory allocations before archive/zip builds its
// File slice. It only locates/counts headers; zip still decodes and verifies them.
// Multi-disk and inconsistent directory offsets are deliberately unsupported.
func preflightArchive(r io.ReaderAt, size int64, limit int) error {
	if size < 22 {
		return zip.ErrFormat
	}
	tail := make([]byte, min(size, 65*1024))
	if _, err := r.ReadAt(tail, size-int64(len(tail))); err != nil {
		return err
	}
	u16, u32, u64 := binary.LittleEndian.Uint16, binary.LittleEndian.Uint32, binary.LittleEndian.Uint64
	end := -1
	// Match archive/zip's footer search, including signatures inside comments.
	for i := len(tail) - 22; i >= 0; i-- {
		if u32(tail[i:]) == 0x06054b50 && int(u16(tail[i+20:])) <= len(tail)-i-22 {
			end = i
			break
		}
	}
	if end < 0 {
		return zip.ErrFormat
	}
	footer := tail[end:]
	if u16(footer[4:]) != 0 || u16(footer[6:]) != 0 || u16(footer[8:]) != u16(footer[10:]) {
		return zip.ErrFormat
	}
	count, bytes, offset := uint64(u16(footer[10:])), uint64(u32(footer[12:])), uint64(u32(footer[16:]))
	directoryEnd := size - int64(len(tail)) + int64(end)
	// ZIP64's locator immediately precedes the conventional end record.
	var locator [20]byte
	has64 := false
	if directoryEnd >= 20 {
		if _, err := r.ReadAt(locator[:], directoryEnd-20); err != nil {
			return err
		}
		has64 = u32(locator[:]) == 0x07064b50
	}
	if has64 {
		if u32(locator[4:]) != 0 || u32(locator[16:]) != 1 {
			return zip.ErrFormat
		}
		pos := u64(locator[8:])
		if directoryEnd < 76 || pos > uint64(directoryEnd-76) {
			return zip.ErrFormat
		}
		var record [56]byte
		if _, err := r.ReadAt(record[:], int64(pos)); err != nil {
			return err
		}
		if u32(record[:]) != 0x06064b50 || u64(record[4:]) != uint64(directoryEnd-20)-pos-12 || u32(record[16:]) != 0 || u32(record[20:]) != 0 || u64(record[24:]) != u64(record[32:]) {
			return zip.ErrFormat
		}
		c, b, o := u64(record[32:]), u64(record[40:]), u64(record[48:])
		if (count != 0xffff && count != c) || (bytes != 0xffffffff && bytes != b) || (offset != 0xffffffff && offset != o) {
			return zip.ErrFormat
		}
		count, bytes, offset, directoryEnd = c, b, o, int64(pos)
	} else if count == 0xffff || bytes == 0xffffffff || offset == 0xffffffff {
		return zip.ErrFormat
	}
	if limit < 0 || count > uint64(limit) {
		return fmt.Errorf("ZIP exceeds %d entries", limit)
	}
	if offset > uint64(directoryEnd) || bytes != uint64(directoryEnd)-offset {
		return zip.ErrFormat
	}
	// Count actual records too: archive/zip only compares the low 16 count bits.
	// Never allocate names, comments or extra fields during this pass.
	var header [46]byte
	actual := uint64(0)
	for pos := int64(offset); pos < directoryEnd; {
		if actual >= uint64(limit) {
			return fmt.Errorf("ZIP exceeds %d entries", limit)
		}
		if directoryEnd-pos < int64(len(header)) {
			return zip.ErrFormat
		}
		if _, err := r.ReadAt(header[:], pos); err != nil {
			return err
		}
		if u32(header[:]) != 0x02014b50 {
			return zip.ErrFormat
		}
		n := int64(len(header)) + int64(u16(header[28:])) + int64(u16(header[30:])) + int64(u16(header[32:]))
		if n > directoryEnd-pos {
			return zip.ErrFormat
		}
		pos += n
		actual++
	}
	if actual != count {
		return zip.ErrFormat
	}
	return nil
}
