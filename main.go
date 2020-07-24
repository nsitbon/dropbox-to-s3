package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cheggaaa/pb/v3"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files"
)

func main() {
	var directory, outputDirectory, token, bucket string
	var disableProgressbar bool
	flag.StringVar(&directory, "input-directory", "", "dropbox directory")
	flag.StringVar(&outputDirectory, "output-directory", "", "optional output directory")
	flag.StringVar(&token, "dropbox-token", "", "dropbox access token")
	flag.StringVar(&bucket, "output-bucket", "", "optional S3 bucket")
	flag.BoolVar(&disableProgressbar, "disable-progressbar", false, "disable progressbar")
	flag.Parse()

	ctx := context.Background()
	config := dropbox.Config{Token: token}
	dbx := files.New(config)
	uploader := createUploader(outputDirectory, bucket)

	for _, v := range listFilesRecursively(dbx, directory) {
		if uploader == nil {
			fmt.Println(v.PathDisplay)
		} else {
			uploadFromDropbox(ctx, v.PathDisplay, dbx, uploader, disableProgressbar)
		}
	}
}

func createUploader(outputDirectory string, bucket string) Uploader {
	if outputDirectory != "" {
		return NewDirectoryUploader(outputDirectory)
	} else if bucket != "" {
		return NewS3Uploader(bucket)
	}

	return nil
}

func uploadFromDropbox(ctx context.Context, file string, dbx files.Client, uploader Uploader, disableProgressbar bool) {
	fmt.Printf("downloading '%s'\n", file)
	meta, content := downloadFromDropbox(dbx, file)
	defer func() { _ = content.Close() }()

	if !disableProgressbar {
		progressBar := pb.Full.Start64(int64(meta.Size))
		defer progressBar.Finish()
		content = progressBar.NewProxyReader(content)
	}

	uploader.Upload(ctx, file, content)
}

func downloadFromDropbox(dbx files.Client, file string) (*files.FileMetadata, io.ReadCloser) {
	 meta, content, err := dbx.Download(files.NewDownloadArg(file))

	 if err != nil {
		log.Fatalf("fail to download file '%s': %+v", file, err)
	}

	return meta, content
}

func listFilesRecursively(dbx files.Client, directory string) []*files.FileMetadata {
	lfa := files.NewListFolderArg(directory)
	lfa.Recursive = true
	lfr, err := dbx.ListFolder(lfa)

	if  err != nil {
		log.Fatalf("fail to list directory '%s': %+v", directory, err)
	}

	result := appendResult(nil, lfr.Entries)

	for ;lfr.HasMore; {
		if lfr, err = dbx.ListFolderContinue(files.NewListFolderContinueArg(lfr.Cursor)); err != nil {
			log.Fatalf("fail to list more directory '%s': %+v", directory, err)
		} else {
			result = appendResult(result, lfr.Entries)
		}
	}

	return result
}

func appendResult(result []*files.FileMetadata, entries []files.IsMetadata) []*files.FileMetadata {
	for _, e := range entries {
		if v, ok := e.(*files.FileMetadata); ok {
			result = append(result, v)
		}
	}

	return result
}

type Uploader interface {
	Upload(ctx context.Context, file string, reader io.Reader)
}

type S3Uploader struct {
	uploader *s3manager.Uploader
	bucket *string
}

func (s *S3Uploader) Upload(ctx context.Context, file string, reader io.Reader) {
	uploadInput := &s3manager.UploadInput{Bucket: s.bucket, Key: aws.String(file), Body: reader}

	if _, err := s.uploader.UploadWithContext(ctx, uploadInput); err != nil {
		log.Fatalf("fail to upload file '%s': %+v", file, err)
	} else {
		log.Printf("successfully upload file '%s'\n", file)
	}
}

func NewS3Uploader(bucket string) Uploader {
	return &S3Uploader{
		uploader: s3manager.NewUploader(session.Must(session.NewSession()), func(u *s3manager.Uploader) {
			u.MaxUploadParts = math.MaxInt32
		}),
		bucket: aws.String(bucket),
	}
}

type DirectoryUploader struct {
	dir string
}

func (d *DirectoryUploader) Upload(_ context.Context, file string, reader io.Reader) {
	path := filepath.Join(d.dir, file)
	containingDir := filepath.Dir(path)

	if err := os.MkdirAll(containingDir, 0666); err != nil {
		log.Fatalf("fail to create directory '%s': %+v", containingDir, err)
	} else if outputFile, err := os.Create(path); err != nil {
		log.Fatalf("fail to create file '%s': %+v", path, err)
	} else {
		defer func() { _ = outputFile.Close() }()

		if _, err = io.Copy(outputFile, reader); err != nil {
			log.Fatalf("fail to copy file '%s': %+v", file, err)
		}
	}
}

func NewDirectoryUploader(dir string) Uploader {
	return &DirectoryUploader{dir: dir}
}
