package zip

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func findEOCD(file *os.File) (int64, error) {
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, err
	}

	// Get the size of the file
	fileSize := fileInfo.Size()

	if fileSize < EOCDMinSize {
		return 0, errors.New("file too small")
	}

	maxCommentSize := int64(65535) // 64K - 1
	searchStart := fileSize - EOCDMinSize - maxCommentSize
	if searchStart < 0 {
		searchStart = 0
	}

	// Read from searchStart to the end of the file
	bufSize := fileSize - searchStart
	buf := make([]byte, bufSize)

	_, err = file.Seek(searchStart, 0)
	if err != nil {
		return 0, err
	}

	_, err = file.Read(buf)
	if err != nil {
		return 0, err
	}

	// Search the buffer for the EOCD signature
	signature := []byte{0x50, 0x4b, 0x05, 0x06}
	sigPos := bytes.LastIndex(buf, signature)
	if sigPos > 0 {
		// Calculate the position of the EOCD signature in the file
		eocdPos := searchStart + int64(sigPos)
		return eocdPos, nil
	} else {
		return 0, errors.New("EOCD signature not found")
	}
}

func parseEOCD(file *os.File, offset int64) (*EndOfCentralDirectory, error) {
	_, err := file.Seek(offset, 0)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, EOCDMinSize)
	_, err = file.Read(buf)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(buf[4:])

	eocd := &EndOfCentralDirectory{}

	// Read each of the fields in the EOCD structure
	if err := binary.Read(reader, binary.LittleEndian, &eocd.DiskNumber); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &eocd.DiskWithCDStart); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &eocd.EntriesOnDisk); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &eocd.TotalEntries); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &eocd.CentralDirSize); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &eocd.CentralDirOffset); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &eocd.CommentLength); err != nil {
		return nil, err
	}

	if eocd.CommentLength > 0 {
		commentBuf := make([]byte, eocd.CommentLength)
		_, err := file.Read(commentBuf)
		if err != nil {
			return nil, err
		}
		eocd.Comment = string(commentBuf)
	}

	return eocd, nil
}

func readCentralDirectoryEntry(file *os.File, offset int64) (*CentralDirectoryHeader, int64, error) {
	_, err := file.Seek(offset, 0)
	if err != nil {
		return nil, 0, err
	}

	// Read and check signature
	var signature uint32
	err = binary.Read(file, binary.LittleEndian, &signature)
	if err != nil {
		return nil, 0, err
	}
	if signature != CentralDirectorySignature {
		return nil, 0, fmt.Errorf("invalid signature: %x", signature)
	}

	// Read the fixed part of the central directory header
	type fixedPart struct {
		VersionMadeBy      uint16
		VersionNeeded      uint16
		Flags              uint16
		CompressionMethod  uint16
		LastModTime        uint16
		LastModDate        uint16
		CRC32              uint32
		CompressedSize     uint32
		UncompressedSize   uint32
		FilenameLength     uint16
		ExtraFieldLength   uint16
		CommentLength      uint16
		DiskNumberStart    uint16
		InternalAttributes uint16
		ExternalAttributes uint32
		LocalHeaderOffset  uint32
	}
	var fixed fixedPart
	err = binary.Read(file, binary.LittleEndian, &fixed)
	if err != nil {
		return nil, 0, err
	}

	cd := &CentralDirectoryHeader{
		VersionMadeBy:      fixed.VersionMadeBy,
		VersionNeeded:      fixed.VersionNeeded,
		Flags:              fixed.Flags,
		CompressionMethod:  fixed.CompressionMethod,
		LastModTime:        fixed.LastModTime,
		LastModDate:        fixed.LastModDate,
		CRC32:              fixed.CRC32,
		CompressedSize:     fixed.CompressedSize,
		UncompressedSize:   fixed.UncompressedSize,
		FilenameLength:     fixed.FilenameLength,
		ExtraFieldLength:   fixed.ExtraFieldLength,
		CommentLength:      fixed.CommentLength,
		DiskNumberStart:    fixed.DiskNumberStart,
		InternalAttributes: fixed.InternalAttributes,
		ExternalAttributes: fixed.ExternalAttributes,
		LocalHeaderOffset:  fixed.LocalHeaderOffset,
	}

	// Read the filename
	filenameBuf := make([]byte, cd.FilenameLength)
	_, err = file.Read(filenameBuf)
	if err != nil {
		return nil, 0, err
	}
	cd.Filename = string(filenameBuf)

	// Read the extra field
	extraFieldBuf := make([]byte, cd.ExtraFieldLength)
	_, err = file.Read(extraFieldBuf)
	if err != nil {
		return nil, 0, err
	}
	cd.ExtraField = extraFieldBuf

	// Read the comment
	commentBuf := make([]byte, cd.CommentLength)
	_, err = file.Read(commentBuf)
	if err != nil {
		return nil, 0, err
	}
	cd.Comment = string(commentBuf)

	// return the entry and the next offset
	nextOffset := offset + 4 + 42 + int64(cd.FilenameLength+cd.ExtraFieldLength+cd.CommentLength)
	return cd, nextOffset, nil
}

func readLocalFileHeader(file *os.File, offset int64) (*LocalFileHeader, int64, error) {
	file.Seek(offset, 0)

	// Check the signature
	var signature uint32
	binary.Read(file, binary.LittleEndian, &signature)
	if signature != LocalFileHeaderSignature {
		return nil, 0, fmt.Errorf("invalid local file header signature: %x", signature)
	}

	// Read the fixed part of the local file header
	type fixedPart struct {
		VersionNeeded     uint16
		Flags             uint16
		CompressionMethod uint16
		LastModTime       uint16
		LastModDate       uint16
		CRC32             uint32
		CompressedSize    uint32
		UncompressedSize  uint32
		FilenameLength    uint16
		ExtraFieldLength  uint16
	}

	var fixed fixedPart
	err := binary.Read(file, binary.LittleEndian, &fixed)
	if err != nil {
		return nil, 0, err
	}

	lh := &LocalFileHeader{
		VersionNeeded:     fixed.VersionNeeded,
		Flags:             fixed.Flags,
		CompressionMethod: fixed.CompressionMethod,
		LastModTime:       fixed.LastModTime,
		LastModDate:       fixed.LastModDate,
		CRC32:             fixed.CRC32,
		CompressedSize:    fixed.CompressedSize,
		UncompressedSize:  fixed.UncompressedSize,
		FilenameLength:    fixed.FilenameLength,
		ExtraFieldLength:  fixed.ExtraFieldLength,
	}

	// Read the filename
	filenameBuf := make([]byte, lh.FilenameLength)
	_, err = file.Read(filenameBuf)
	if err != nil {
		return nil, 0, err
	}
	lh.Filename = string(filenameBuf)

	// Read the extra field
	extraFieldBuf := make([]byte, lh.ExtraFieldLength)
	_, err = file.Read(extraFieldBuf)
	if err != nil {
		return nil, 0, err
	}
	lh.ExtraField = extraFieldBuf

	return lh, offset + 4 + 26 + int64(lh.FilenameLength+lh.ExtraFieldLength), nil
}

func extractFile(file *os.File, centralDir *CentralDirectoryHeader) ([]byte, error) {
	// Read local header
	localHeader, dataOffset, err := readLocalFileHeader(file, int64(centralDir.LocalHeaderOffset))
	if err != nil {
		return nil, err
	}

	// Print the local header struct for debugging
	fmt.Printf("Local File Header for %s:\n", centralDir.Filename)
	fmt.Printf("  Version Needed: %d\n", localHeader.VersionNeeded)
	fmt.Printf("  Flags: %04x\n", localHeader.Flags)
	fmt.Printf("  Compression Method: %d\n", localHeader.CompressionMethod)
	fmt.Printf("  Last Mod Time: %d\n", localHeader.LastModTime)
	fmt.Printf("  Last Mod Date: %d\n", localHeader.LastModDate)
	fmt.Printf("  CRC32: %08x\n", localHeader.CRC32)
	fmt.Printf("  Compressed Size: %d\n", localHeader.CompressedSize)
	fmt.Printf("  Uncompressed Size: %d\n", localHeader.UncompressedSize)
	fmt.Printf("  Filename Length: %d\n", localHeader.FilenameLength)
	fmt.Printf("  Extra Field Length: %d\n", localHeader.ExtraFieldLength)
	fmt.Printf("  Filename: %s\n", localHeader.Filename)
	fmt.Printf("  Extra Field: %x\n", localHeader.ExtraField)

	// Check if the local header's compressed size is zero and if the data descriptor flag is set
	if localHeader.CompressedSize == 0 && (localHeader.Flags&0x0008) != 0 {
		// Use sizes from central directory instead
		localHeader.CompressedSize = centralDir.CompressedSize
		localHeader.UncompressedSize = centralDir.UncompressedSize
	}

	// Seek to compressed data
	file.Seek(dataOffset, 0)

	// Read compressed data
	compressedData := make([]byte, localHeader.CompressedSize)
	_, err = file.Read(compressedData)
	if err != nil {
		return nil, err
	}

	// Decompress
	if localHeader.CompressionMethod == 8 { // Deflate
		reader := flate.NewReader(bytes.NewReader(compressedData))
		defer reader.Close()

		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}

		// Verify size
		if len(decompressed) != int(localHeader.UncompressedSize) {
			return nil, fmt.Errorf("size mismatch: got %d, expected %d",
				len(decompressed), localHeader.UncompressedSize)
		}

		return decompressed, nil
	}

	// If stored (method 0), just return as-is
	return compressedData, nil
}

func ReadZip(file *os.File) {
	eocdPos, err := findEOCD(file)
	if err != nil {
		panic(err)
	}

	eocd, err := parseEOCD(file, eocdPos)
	if err != nil {
		panic(err)
	}

	// Print the parsed EOCD struct
	println("End of Central Directory:")
	println("Disk Number:", eocd.DiskNumber)
	println("Disk with CD Start:", eocd.DiskWithCDStart)
	println("Entries on Disk:", eocd.EntriesOnDisk)
	println("Total Entries:", eocd.TotalEntries)
	println("Central Directory Size:", eocd.CentralDirSize)
	println("Central Directory Offset:", eocd.CentralDirOffset)
	println("Comment Length:", eocd.CommentLength)
	println("")

	// Read all Central Directory entries
	offset := int64(eocd.CentralDirOffset)
	for i := 0; i < int(eocd.TotalEntries); i++ {
		centralDirectoryEntry, nextOffset, err := readCentralDirectoryEntry(file, offset)
		if err != nil {
			panic(err)
		}

		if !strings.HasPrefix(centralDirectoryEntry.Filename, "__MACOSX/") {
			data, err := extractFile(file, centralDirectoryEntry)
			if err != nil {
				fmt.Printf("Error extracting file %s: %v\n", centralDirectoryEntry.Filename, err)
			} else {
				fmt.Printf("Extracted %s (%d bytes)\n", centralDirectoryEntry.Filename, len(data))
			}

			// Print the bytes of the data file
			//fmt.Printf("Data: %s\n", string(data))
		} else {
			fmt.Printf("Skipping file %s\n", centralDirectoryEntry.Filename)
		}

		offset = nextOffset
	}
}
