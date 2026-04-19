package main

import (
	"context"
	"flag"
	"log"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Config holds user terminal inputs
type Config struct {
	Bucket      string
	Incremental string
	MaxBytes    string
	MountPoint  string
	TargetObj   string
}

func main() {
	// 1. Define CLI flags
	bucket := flag.String("bucket", "", "bucket for all objects")
	// incremental := flag.String("incremental", "", "incremental backup object")
	// max := flag.String("max", "", "stop writing after SIZE (K/M/G)")

	// 2. Read terminal input
	flag.Parse()
	args := flag.Args()

	// Ensure required args (Object, MountPoint) exist
	if len(args) < 2 {
		log.Fatalf("Usage: s3-backup-go --bucket BUCKET [options] OBJECT /mountpoint")
	}

	// Store CLI inputs in struct
	cfg := Config{
		Bucket:     *bucket,
		TargetObj:  args[0],
		MountPoint: args[1],
	}

	// 3. Load AWS credentials from system environment
	awsCfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	// Create AWS API client
	s3Client := s3.NewFromConfig(awsCfg)
	log.Println("S3 Client initialized successfully.")

	// --- STARTUP LOOP: Download & Parse ---
	log.Println("Downloading root directory from S3...")

	// Prepare S3 download request
	getObjectInput := &s3.GetObjectInput{
		Bucket: &cfg.Bucket,
		Key:    &cfg.TargetObj,
	}

	// Execute download
	s3Out, err := s3Client.GetObject(context.TODO(), getObjectInput)
	if err != nil {
		log.Fatalf("Failed to download target object: %v", err)
	}
	defer s3Out.Body.Close() // Ensure stream closes on exit

	// Read and discard header
	_, err = ParseSuperblock(s3Out.Body)
	if err != nil {
		log.Fatalf("Failed to parse superblock: %v", err)
	}

	// Init empty map for directory cache
	masterCache := make(map[string]*S3DirEnt)

	// Loop through entire binary stream
	for {
		ent, err := ParseDirEnt(s3Out.Body)
		if err != nil {
			break // EOF reached
		}
		// Skip empty or corrupted names
		if ent.NameLen > 0 {
			masterCache[ent.Name] = ent // Save to map
		}
	}
	log.Printf("Successfully loaded %d files into the cache.\n", len(masterCache))

	// --- MOUNT FUSE ---
	log.Printf("Attempting to mount %s/%s to %s\n", cfg.Bucket, cfg.TargetObj, cfg.MountPoint)

	// Tell OS to create virtual folder (Read Only)
	c, err := fuse.Mount(
		cfg.MountPoint,
		fuse.FSName("s3-backup"),
		fuse.Subtype("s3fs"),
		fuse.ReadOnly(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close() // Unmount safely on exit

	// Start listening for OS requests, pass in loaded cache
	err = fs.Serve(c, &S3FS{
		Client:    s3Client,
		Bucket:    cfg.Bucket,
		ObjectKey: cfg.TargetObj,
		RootCache: masterCache,
	})
	if err != nil {
		log.Fatal(err)
	}
}
