/*
s3fs.go file will act as the translator between data (messy, compressed binary)
coming from AWS to the computer's operating system (using bitwise math operations).
*/

package main

// Imports
import (
	"encoding/binary"
	"io"
)

// Constants
const (
	S3BUMagic     uint32 = 0x55423353 // hexadecimal representation of "S3BU" needed
	FlagUseInum   uint32 = 2
	FlagDirLoc    uint32 = 4
	FlagDirData   uint32 = 8
	FlagDirPacked uint32 = 16
	FlagHardLinks uint32 = 32
)

// S3Super struct variables (Acts as the backup file's "header")
type S3Super struct {
	Magic   uint32
	Version uint32 // superblock version
	Flags   uint32
	Len     uint32 // how long the superblock is
	NVers   uint32
}

func ParseSuperblock(r io.Reader) (*S3Super, error) {
	var super S3Super
	// binary.Read will read the binary stream "r", flip their bytes, and map them to the S3Super struct variables
	err := binary.Read(r, binary.LittleEndian, &super)
	if err != nil {
		return nil, err
	}
	return &super, nil
}

type S3Offset struct {
	Sector uint64
	Object uint16
}

func ParseS3Offset(raw uint64) S3Offset {
	return S3Offset{
		// We use a bitmask (twelve 'F's = 48 binary 1s) as a filter.
		// It keeps the bottom 48 bits (the Sector) and wipes the top 16 bits clean.
		Sector: raw & 0xFFFFFFFFFFFF,
		// We slide all the bits 48 spaces to the right. This pushes the sector data off a cliff,
		// leaving only the 16-bit Object ID.
		Object: uint16(raw >> 48),
	}
}

// S3DirEnt represents a single file or folder entry in the backup
type S3DirEnt struct {
	Mode    uint16
	UID     uint16
	GID     uint16
	CTime   uint32
	Off     S3Offset
	Bytes   uint64 // The unpacked 52-bit file size
	Xattr   uint16 // The unpacked 12-bit extended attributes
	NameLen uint8
	Name    string
}

func ParseDirEnt(r io.Reader) (*S3DirEnt, error) {
	var ent S3DirEnt

	// Read the standard, fixed-size numbers one by one out of the binary stream.
	if err := binary.Read(r, binary.LittleEndian, &ent.Mode); err != nil {
		return nil, err
	}
	binary.Read(r, binary.LittleEndian, &ent.UID)
	binary.Read(r, binary.LittleEndian, &ent.GID)
	binary.Read(r, binary.LittleEndian, &ent.CTime)

	// Read the raw 64-bit chunk containing the offset, then hand it to our math function to split it up.
	var rawOffset uint64
	binary.Read(r, binary.LittleEndian, &rawOffset)
	ent.Off = ParseS3Offset(rawOffset)

	// Read the next 64-bit chunk containing the squished File Size and Attributes.
	var rawBytesXattr uint64
	binary.Read(r, binary.LittleEndian, &rawBytesXattr)

	// Thirteen 'F's equals fifty-two 1s. This mask isolates the exact 52-bit file size.
	ent.Bytes = rawBytesXattr & 0xFFFFFFFFFFFFF
	// Slide the data 52 spaces to the right, leaving only the 12-bit attribute data.
	ent.Xattr = uint16(rawBytesXattr >> 52)

	// Filenames change in length, so we have to read the length first.
	binary.Read(r, binary.LittleEndian, &ent.NameLen)

	// Create a blank byte array of exactly that size, read exactly that many letters
	// from the stream, and convert those raw bytes into a readable Go string.
	nameBytes := make([]byte, ent.NameLen)
	if _, err := io.ReadFull(r, nameBytes); err != nil {
		return nil, err
	}
	ent.Name = string(nameBytes)

	return &ent, nil
}
