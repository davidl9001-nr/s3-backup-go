/*
fuse.go file will hold the cache map that holds every file's index.
It communicates where each part of each file is in AWS to the user's OS.
*/

package main

// Imports
import (
	"context"
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3FS, master file system structure
type S3FS struct {
	Client    *s3.Client
	Bucket    string
	ObjectKey string
}

// FUSE function Root()
func (f *S3FS) Root() (fs.Node, error) {
	// When operating system mounts drive and asks for top level folder, hand the directory node
	return &DirNode{
		Client: f.Client,
		Bucket: f.Bucket,
		Cache:  make(map[string]*S3DirEnt), // initialize an empty map in go (improvement upon labavl from C)
	}, nil
}

// Directory Node structure (it implements the directory behaviors for FUSE)
type DirNode struct {
	Client *s3.Client
	Bucket string
	Cache  map[string]*S3DirEnt // replaces libavl
}

// Attr tells the OS that this is a read-only directory
func (d *DirNode) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0555
	return nil
}

// Lookup is called when the OS tries to access a specific file by name
func (d *DirNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	ent, ok := d.Cache[name]
	if !ok {
		return nil, fuse.ENOENT // File not found
	}

	// If the file's mode says it's a directory, return another DirNode
	if ent.Mode&uint16(os.ModeDir) != 0 {
		return &DirNode{Client: d.Client, Bucket: d.Bucket, Cache: make(map[string]*S3DirEnt)}, nil
	}

	// Otherwise, it's a standard file
	return &FileNode{Client: d.Client, Bucket: d.Bucket, Ent: ent}, nil
}

// ReadDirAll tells the OS what files are inside this directory by reading our map
func (d *DirNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var res []fuse.Dirent
	for name, ent := range d.Cache {
		var typ fuse.DirentType
		if ent.Mode&uint16(os.ModeDir) != 0 {
			typ = fuse.DT_Dir
		} else {
			typ = fuse.DT_File
		}
		res = append(res, fuse.Dirent{Name: name, Type: typ})
	}
	return res, nil
}

// FileNode implements file behaviors for FUSE
type FileNode struct {
	Client *s3.Client
	Bucket string
	Ent    *S3DirEnt
}

// Attr tells the OS the file permissions and the exact file size (from our 52-bit unpacked size)
func (f *FileNode) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.FileMode(f.Ent.Mode) | 0444
	a.Size = f.Ent.Bytes
	return nil
}

// Note: The actual Read() method to fetch file data from AWS will go here next!
