package zip

const (
	EOCDMinSize               = 22
	LocalFileHeaderSignature  = 0x04034b50
	CentralDirectorySignature = 0x02014b50
)

type LocalFileHeader struct {
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
	Filename          string
	ExtraField        []byte
}

type EndOfCentralDirectory struct {
	DiskNumber       uint16
	DiskWithCDStart  uint16
	EntriesOnDisk    uint16
	TotalEntries     uint16
	CentralDirSize   uint32
	CentralDirOffset uint32
	CommentLength    uint16
	Comment          string
}

type CentralDirectoryHeader struct {
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
	Filename           string
	ExtraField         []byte
	Comment            string
}
