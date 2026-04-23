Instructions:

To build the system, we use Go to create a Linux-compatible execuatble file.
(We run this line in terminal: " GOOS=linux GOARCH=amd64 go build -o s3-fuse-client ")

To run the system, we create an empty folder to act as mkdir mnt_point (the virtual drive) and then tell the program which file to read with directions to the AWS bucket name, the backup file name, and the empty folder.
(We run this line in terminal: " ./s3-fuse-client --bucket <BUCKET_NAME> <BACKUP_FILE> ./mnt_point ")

This project was built entirely in Golang, and for a Linux operating system environment, which was required because the FUSE library needs to use Linux kernel features. Amazon's AWS SDK (github.com/aws/aws-sdk-go-v2) library was implemented as it was needed to properly connect to Amazon's S3 cloud. The FUSE library (bazil.org/fuse) was used to bridge the Golang code with a Linux operating system so it could create virtual folders and allow storage in them.


Design Decisions

The goal of this project was to modernize a legacy S3 backup FUSE client by porting highly compressed, legacy C code into memory-safe, modern Go. To achieve this safely and efficiently, three main architectural decisions were made.

The original C code heavily relied on __attribute__((packed)) to compress data, saving space by cramming multiple pieces of information (like compressing a 48-bit sector offset and a 16-bit object ID into a single 64-bit chunk so it could fit into exact 512-byte memory alignments). If Go attempts to read this normally, memory padding issues will cause the stream to corrupt or crash. To prevent this, a bitwise translation layer was built in s3fs.go using bitwise math masks (&) and shifts (>>) to safely slice the raw 64-bit chunks from S3 apart into clean, usable Go structures.

The legacy C program used a complicated AVL Tree from libavl to keep track of directory caching. This was completely replaced with Go's built-in, highly optimized map[string]*S3DirEnt from fuse.go. During the startup loop, the client downloads the root directory, unpacks the binary stream, and populates the map. It acts like a simple dictionary so when the OS asks for a filename via the FUSE Lookup method, the map will instantly return the file's details. This provides a standard O(1) lookup time, making it significantly more efficient than the old tree.

Instead of downloading entire files into local memory which can be massive and very costly, the Read method was designed to fetch only the exact chunk of bytes the user wants to immediately access. It translates the decoded 48-bit sector offsets into HTTP Range headers, utilizing the AWS SDK to fetch and serve these exact byte chunks directly to Linux on demand.


Structure:

This program is comprised of 3 parts: a startup loop, a mount, and the FUSE server. First, the program will connect to the AWS S3 cloud, download the "superblock" / the master index of the backup, and then run that through the bitwise translator. It will then save the list of files into a Go map, which will be the cache.

The Mount will then give the cache to the FUSE library. This tells the operating system to create a virtual drive and display those files there.

Finally, when a user clicks on a file, the FUSE server will activate the program to look up where that file is in AWS, fetch its data, and hand it back to the user by displaying it on the user's screen.


Testing: 

A Go unit test was made for the bitwise translation layer and math. Golang code is fed into a compressed 64-bit number to check if 48-bit and 16-bit numbers its comprised of could be extracted and come out exactly as they were input.

I also checked manually the program could be manually mounted on a virtual Linux machine, that a virtual folder was created and could be opened, and used standard terminal commands (ls and cat) to test that the files appeared and could be read clearly. The terminal commands used were: (" ls -la mnt_point " and " cat mnt_point/some_text_file.txt ").


Results:

Testing showed the Go port was successful, and completed its goal of being able to store user data to Amazon's S3 cloud as intended. The program can connect to the cloud, decode binary struct data, and provide a stable read-only FUSE mount, so a user can browse and open cloud files as if they were on their local hard drive. Meanwhile, the client can now bypass legacy libs3 and libavl C libraries.
