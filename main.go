/*
main.go file will establish connection to the cloud and keep the program running.
It will read the command line to see what the user is looking for, log into AWS SDK,
open a new virtual file, and send the user requests there.
*/

package main

// Import the necessary libraries
import (
	"context" // for handling timeouts and cancelations (AWS & FUSE)
	"flag"    // for reading command-line args
	"log"

	"bazil.org/fuse" // needed to create virtual file system
	"bazil.org/fuse/fs"
	"github.com/aws/aws-sdk-go-v2/config" // modern AWS library to replace lib3
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// User type settings
type Config struct {
	Bucket      string
	Incremental string
	MaxBytes    string
	MountPoint  string
	TargetObj   string
}

// Terminal commands
func main() {
	// return pointers to the following strings:
	bucket := flag.String("bucket", "", "bucket for all objects")
	incremental := flag.String("incremental", "", "incremental backup object")
	max := flag.String("max", "", "stop writing after SIZE (K/M/G)")

	flag.Parse()        // parse through/read the terminal commands
	args := flag.Args() // grabs leftover words in terminal without "--"

	if len(args) < 2 {
		log.Fatalf("Usage: s3-backup-go --bucket BUCKET [options] OBJECT /mountpoint")
	}

	cfg := Config{
		Bucket:     *bucket,
		TargetObj:  args[0],
		MountPoint: args[1],
	}

	// Initialize modern AWS S3 Client
	awsCfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}
	s3Client := s3.NewFromConfig(awsCfg)
	log.Println("S3 Client initialized successfully.")

	// Mount the FUSE directory
	log.Printf("Attempting to mount %s/%s to %s\n", cfg.Bucket, cfg.TargetObj, cfg.MountPoint)
	c, err := fuse.Mount(
		cfg.MountPoint,
		fuse.FSName("s3-backup"),
		fuse.Subtype("s3fs"),
		fuse.ReadOnly(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Start serving the file system using our custom S3FS struct (defined in fuse.go)
	err = fs.Serve(c, &S3FS{Client: s3Client, Bucket: cfg.Bucket, ObjectKey: cfg.TargetObj})
	if err != nil {
		log.Fatal(err)
	}
}
