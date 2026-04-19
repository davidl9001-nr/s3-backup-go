package main

import (
	"encoding/binary"
	"io"
)

const (
	S3BUMagic     uint32 = 0x55423353 // "S3BU" magic validation number
	FlagUseInum   uint32 = 2
	FlagDirLoc    uint32 = 4
	FlagDirData   uint32 = 8
	FlagDirPacked uint32 = 16
	FlagHardLinks uint32 = 32
)

// S3Super is the backup file's header
type S3Super struct {
	Magic   uint32
	Version uint32
	Flags   uint32
	Len     uint32
	NVers   uint32
}

func ParseSuperblock(r io.Reader) (*S3Super, error) {
	var super S3Super
	// Read C-style little-endian bytes into Go struct
	err := binary.Read(r, binary.LittleEndian, &super)
	if err != nil {
		return nil, err
	}
	return &super, nil
}

// S3Offset cleanly separates Sector and Object ID
type S3Offset struct {
	Sector uint64
	Object uint16
}

func ParseS3Offset(raw uint64) S3Offset {
	return S3Offset{
		Sector: raw & 0xFFFFFFFFFFFF, // Mask: Keep bottom 48 bits
		Object: uint16(raw >> 48),    // Shift: Keep top 16 bits
	}
}

// S3DirEnt holds unpacked file metadata
type S3DirEnt struct {
	Mode    uint16
	UID     uint16
	GID     uint16
	CTime   uint32
	Off     S3Offset
	Bytes   uint64 // Unpacked 52-bit size
	Xattr   uint16 // Unpacked 12-bit attributes
	NameLen uint8
	Name    string
}

func ParseDirEnt(r io.Reader) (*S3DirEnt, error) {
	var ent S3DirEnt

	// Read standard fixed-size fields
	if err := binary.Read(r, binary.LittleEndian, &ent.Mode); err != nil {
		return nil, err
	}
	binary.Read(r, binary.LittleEndian, &ent.UID)
	binary.Read(r, binary.LittleEndian, &ent.GID)
	binary.Read(r, binary.LittleEndian, &ent.CTime)

	// Read and split 64-bit Offset chunk
	var rawOffset uint64
	binary.Read(r, binary.LittleEndian, &rawOffset)
	ent.Off = ParseS3Offset(rawOffset)

	// Read and split 64-bit Size/Attribute chunk
	var rawBytesXattr uint64
	binary.Read(r, binary.LittleEndian, &rawBytesXattr)
	ent.Bytes = rawBytesXattr & 0xFFFFFFFFFFFFF // Mask: Keep bottom 52 bits
	ent.Xattr = uint16(rawBytesXattr >> 52)     // Shift: Keep top 12 bits

	// Read variable-length filename
	binary.Read(r, binary.LittleEndian, &ent.NameLen)
	nameBytes := make([]byte, ent.NameLen) // Allocate exact space

	if _, err := io.ReadFull(r, nameBytes); err != nil {
		return nil, err
	}
	ent.Name = string(nameBytes) // Convert bytes to string

	return &ent, nil
}
