package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3FS is the top-level file system
type S3FS struct {
	Client    *s3.Client
	Bucket    string
	ObjectKey string
	RootCache map[string]*S3DirEnt // Master inventory list
}

// Root is called by OS on mount
func (f *S3FS) Root() (fs.Node, error) {
	return &DirNode{
		Client:    f.Client,
		Bucket:    f.Bucket,
		ObjectKey: f.ObjectKey,
		Cache:     f.RootCache, // Pass inventory to root folder
	}, nil
}

// DirNode represents a folder
type DirNode struct {
	Client    *s3.Client
	Bucket    string
	ObjectKey string
	Cache     map[string]*S3DirEnt
}

// Attr sets folder permissions
func (d *DirNode) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0555 // Read/Execute only
	return nil
}

// Lookup is called when user opens a specific file
func (d *DirNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	ent, ok := d.Cache[name] // Check map
	if !ok {
		return nil, fuse.ENOENT // File not found
	}

	// Check standard UNIX directory bit (040000)
	if (ent.Mode & 040000) != 0 {
		return &DirNode{Client: d.Client, Bucket: d.Bucket, ObjectKey: d.ObjectKey, Cache: make(map[string]*S3DirEnt)}, nil
	}

	// Otherwise, it's a file
	return &FileNode{Client: d.Client, Bucket: d.Bucket, ObjectKey: d.ObjectKey, Ent: ent}, nil
}

// ReadDirAll is called when user types 'ls' or opens Finder
func (d *DirNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var res []fuse.Dirent

	// Format map entries for OS display
	for name, ent := range d.Cache {
		var typ fuse.DirentType

		// Check standard UNIX directory bit (040000)
		if (ent.Mode & 040000) != 0 {
			typ = fuse.DT_Dir
		} else {
			typ = fuse.DT_File
		}
		res = append(res, fuse.Dirent{Name: name, Type: typ})
	}
	return res, nil
}

// FileNode represents a single file
type FileNode struct {
	Client    *s3.Client
	Bucket    string
	ObjectKey string
	Ent       *S3DirEnt
}

// Attr sets file size and permissions
func (f *FileNode) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.FileMode(f.Ent.Mode) | 0444 // Read only
	a.Size = f.Ent.Bytes                    // Tell OS exact file size
	return nil
}

// Read is called when user opens file contents
func (f *FileNode) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	// Convert sector to absolute byte position
	baseOffset := int64(f.Ent.Off.Sector * 512)
	startByte := baseOffset + req.Offset
	endByte := startByte + int64(req.Size) - 1

	// Prevent reading past EOF
	if req.Offset >= int64(f.Ent.Bytes) {
		return nil
	}

	// Build HTTP range request
	rangeHeader := fmt.Sprintf("bytes=%d-%d", startByte, endByte)
	getObjectInput := &s3.GetObjectInput{
		Bucket: &f.Bucket,
		Key:    &f.ObjectKey,
		Range:  &rangeHeader,
	}

	// Download exact bytes from S3
	s3Out, err := f.Client.GetObject(ctx, getObjectInput)
	if err != nil {
		return err
	}
	defer s3Out.Body.Close()

	// Copy S3 data directly to OS buffer
	buf := make([]byte, req.Size)
	n, err := io.ReadFull(s3Out.Body, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return err
	}

	resp.Data = buf[:n] // Return bytes
	return nil
}
