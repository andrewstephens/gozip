package zip

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"time"
	"unicode/utf8"
)

type ZipWriter struct {
	w      io.Writer
	files  []fileRecord
	offset int64
}

type fileRecord struct {
	name              string
	compressedSize    uint32
	uncompressedSize  uint32
	crc32             uint32
	compressionMethod uint16
	modTime           uint16
	modDate           uint16
	localHeaderOffset int64
	versionMadeBy     uint16
	versionNeeded     uint16
	flags             uint16
	extraField        []byte
	comment           string
	diskNumberStart   uint16
	internalAttrs     uint16
	externalAttrs     uint32 // file permissions
}

func NewZipWriter(w io.Writer) *ZipWriter {
	return &ZipWriter{
		w:      w,
		files:  make([]fileRecord, 0),
		offset: 0,
	}
}

func isValidUTF8(s string) bool {
	return utf8.ValidString(s)
}

func timeToMSDos(t time.Time) (uint16, uint16) {
	year := t.Year() - 1980
	if year < 0 {
		year = 0
	}
	dosDate := uint16(year<<9 | int(t.Month())<<5 | t.Day())
	dosTime := uint16(t.Hour()<<11 | t.Minute()<<5 | t.Second()/2)
	return dosTime, dosDate
}

func (zw *ZipWriter) AddFile(name string, data []byte) error {
	// Do some validation
	if name == "" {
		return errors.New("zip filename is empty")
	}
	if len(name) > 65535 {
		return errors.New("zip filename is too long (max 65535 bytes)")
	}
	if data == nil {
		return errors.New("data cannot be nil")
	}

	// 1. Calculate CRC32
	crc := crc32.ChecksumIEEE(data)

	// 2. Record where we're writing this file
	headerOffset := zw.offset

	// 3. Create the local file header
	// Signature
	if err := binary.Write(zw.w, binary.LittleEndian, uint32(LocalFileHeaderSignature)); err != nil {
		return err
	}

	// Fixed header fields
	if err := binary.Write(zw.w, binary.LittleEndian, uint16(20)); err != nil { // version needed
		return err
	}

	// TODO: Handle more flags eventually
	flags := uint16(0)
	if isValidUTF8(name) {
		flags |= 0x0800 // UTF-8 flag
	}
	if err := binary.Write(zw.w, binary.LittleEndian, flags); err != nil { // flags (UTF-8)
		return err
	}
	if err := binary.Write(zw.w, binary.LittleEndian, uint16(0)); err != nil { // compression method
		return err
	}

	// Time and date
	modTime, modDate := timeToMSDos(time.Now())
	if err := binary.Write(zw.w, binary.LittleEndian, modTime); err != nil {
		return err
	}
	if err := binary.Write(zw.w, binary.LittleEndian, modDate); err != nil {
		return err
	}

	// CRC and sizes
	if err := binary.Write(zw.w, binary.LittleEndian, crc); err != nil {
		return err
	}
	if err := binary.Write(zw.w, binary.LittleEndian, uint32(len(data))); err != nil { // compressed size
		return err
	}
	if err := binary.Write(zw.w, binary.LittleEndian, uint32(len(data))); err != nil { // uncompressed size
		return err
	}

	// Name length and extra field length
	if err := binary.Write(zw.w, binary.LittleEndian, uint16(len(name))); err != nil {
		return err
	}
	if err := binary.Write(zw.w, binary.LittleEndian, uint16(0)); err != nil { // no extra field
		return err
	}

	// 4. Write filename
	if _, err := zw.w.Write([]byte(name)); err != nil {
		return err
	}

	// 5. Write file data
	if _, err := zw.w.Write(data); err != nil {
		return err
	}

	// 6. Save file record for central directory
	record := fileRecord{
		name:              name,
		versionMadeBy:     0x0314, // Unix, version 2.0
		versionNeeded:     20,
		flags:             0x0800,
		compressionMethod: 0,
		modTime:           modTime,
		modDate:           modDate,
		crc32:             crc,
		compressedSize:    uint32(len(data)),
		uncompressedSize:  uint32(len(data)),
		diskNumberStart:   0,
		internalAttrs:     0,
		externalAttrs:     0x81A40000, // Unix regular file, 644 permissions
		localHeaderOffset: headerOffset,
	}
	zw.files = append(zw.files, record)

	// 7. Update offset
	zw.offset += 4 + 26 + int64(len(name)) + int64(len(data)) // signature + header + name + data

	return nil
}

func (zw *ZipWriter) Close() error {
	// Remember where central directory starts
	centralDirOffset := zw.offset

	// 1. Write all central directory entries
	for _, file := range zw.files {
		// Write central directory signature
		if err := binary.Write(zw.w, binary.LittleEndian, uint32(CentralDirectorySignature)); err != nil {
			return err
		}

		// Write all the fixed fields
		if err := binary.Write(zw.w, binary.LittleEndian, file.versionMadeBy); err != nil {
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, file.versionNeeded); err != nil {
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, file.flags); err != nil {
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, file.compressionMethod); err != nil {
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, file.modTime); err != nil {
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, file.modDate); err != nil {
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, file.crc32); err != nil {
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, file.compressedSize); err != nil {
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, file.uncompressedSize); err != nil {
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, uint16(len(file.name))); err != nil {
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, uint16(0)); err != nil { // extra field length
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, uint16(0)); err != nil { // comment length
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, file.diskNumberStart); err != nil {
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, file.internalAttrs); err != nil {
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, file.externalAttrs); err != nil {
			return err
		}
		if err := binary.Write(zw.w, binary.LittleEndian, uint32(file.localHeaderOffset)); err != nil {
			return err
		}

		// Write the filename
		if _, err := zw.w.Write([]byte(file.name)); err != nil {
			return err
		}

		// Update offset
		zw.offset += 4 + 42 + int64(len(file.name)) // signature + fixed header + filename
	}

	// 2. Calculate central directory size
	centralDirSize := zw.offset - centralDirOffset

	// 3. Write End of Central Directory
	if err := binary.Write(zw.w, binary.LittleEndian, uint32(EndOfCentralDirectorySignature)); err != nil {
		return err
	}
	if err := binary.Write(zw.w, binary.LittleEndian, uint16(0)); err != nil { // disk number
		return err
	}
	if err := binary.Write(zw.w, binary.LittleEndian, uint16(0)); err != nil { // disk with central dir
		return err
	}
	if err := binary.Write(zw.w, binary.LittleEndian, uint16(len(zw.files))); err != nil { // entries on disk
		return err
	}
	if err := binary.Write(zw.w, binary.LittleEndian, uint16(len(zw.files))); err != nil { // total entries
		return err
	}
	if err := binary.Write(zw.w, binary.LittleEndian, uint32(centralDirSize)); err != nil {
		return err
	}
	if err := binary.Write(zw.w, binary.LittleEndian, uint32(centralDirOffset)); err != nil {
		return err
	}
	if err := binary.Write(zw.w, binary.LittleEndian, uint16(0)); err != nil { // comment length
		return err
	}

	return nil
}
