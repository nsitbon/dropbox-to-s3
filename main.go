package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cheggaaa/pb/v3"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files"
)

func main() {
	var directory, token, bucket string
	flag.StringVar(&directory, "directory", "", "dropbox directory")
	flag.StringVar(&token, "token", "", "dropbox access token")
	flag.StringVar(&bucket, "bucket", "", "S3 bucket")
	flag.Parse()

	ctx := context.Background()
	sess := session.Must(session.NewSession())
	uploader := s3manager.NewUploader(sess)
	config := dropbox.Config{Token: token}
	dbx := files.New(config)

	for _, v := range listFilesRecursively(dbx, directory) {
		moveFromDropboxToS3(ctx, v.PathDisplay, dbx, uploader, bucket)
	}
}

func moveFromDropboxToS3(ctx context.Context, file string, dbx files.Client, uploader *s3manager.Uploader, bucket string) {
	fmt.Printf("downloading '%s'\n", file)
	meta, content := downloadFromDropbox(dbx, file)
	defer func() { _ = content.Close() }()
	progressBar := pb.Full.Start64(int64(meta.Size))
	defer progressBar.Finish()
	proxyReader := progressBar.NewProxyReader(content)
	uploadToS3(ctx, uploader, bucket, file, proxyReader)
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

	result := appendResult(nil, lfr)

	for ;lfr.HasMore; {
		if lfr, err = dbx.ListFolderContinue(files.NewListFolderContinueArg(lfr.Cursor)); err != nil {
			log.Fatalf("fail to list more directory '%s': %+v", directory, err)
		} else {
			result = appendResult(result, lfr)
		}
	}

	return result
}

func appendResult(result []*files.FileMetadata, lfr *files.ListFolderResult) []*files.FileMetadata {
	for _, e := range lfr.Entries {
		if v, ok := e.(*files.FileMetadata); ok {
			result = append(result, v)
		}
	}

	return result
}

func uploadToS3(ctx context.Context, uploader *s3manager.Uploader, bucket string, file string, content io.Reader) {
	uploadInput := &s3manager.UploadInput{Bucket: aws.String(bucket), Key: aws.String(file), Body: content}

	if _, err := uploader.UploadWithContext(ctx, uploadInput); err != nil {
		log.Fatalf("fail to upload file '%s': %+v", file, err)
	} else {
		log.Printf("successfully upload file '%s'\n", file)
	}
}
