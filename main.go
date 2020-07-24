package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cheggaaa/pb/v3"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files"
)

type UploadStatus struct {
	Downloaded bool  `json:",omitempty"`
	Error     string `json:",omitempty"`
}

type UploadStatuses = map[string]*UploadStatus

func main() {
	var directory, outputDirectory, token, bucket string
	var disableProgressbar bool
	flag.StringVar(&directory, "input-directory", "", "dropbox directory")
	flag.StringVar(&outputDirectory, "output-directory", "", "optional output directory")
	flag.StringVar(&token, "dropbox-token", "", "dropbox access token")
	flag.StringVar(&bucket, "output-bucket", "", "optional S3 bucket")
	flag.BoolVar(&disableProgressbar, "disable-progressbar", false, "disable progressbar")
	flag.Parse()

	ctx, cancelFn := context.WithCancel(context.Background())
	config := dropbox.Config{Token: token}
	dbx := files.New(config)
	uploader := createUploader(outputDirectory, bucket)

	uploadStatuses := readUploadStatusesFile()
	defer writeUploadStatusesFile(uploadStatuses)

	handleStopSignals(uploadStatuses, cancelFn)

	for _, v := range listFilesRecursively(dbx, directory) {
		if e, ok := uploadStatuses[v.PathDisplay]; ok && e.Downloaded {
			fmt.Printf("file '%s' already processed\n", v.PathDisplay)
		} else if uploader == nil {
			fmt.Println(v.PathDisplay)
		} else if err := uploadFromDropbox(ctx, v.PathDisplay, dbx, uploader, disableProgressbar); err != nil {
			log.Printf("%+v\n", err)
			uploadStatuses[v.PathDisplay] = &UploadStatus{Error: fmt.Sprintf("%+v", err)}
		} else {
			log.Printf("successfully uploaded '%s'\n", v.PathDisplay)
			uploadStatuses[v.PathDisplay] = &UploadStatus{Downloaded: true}
		}
	}
}

func handleStopSignals(uploadStatuses UploadStatuses, cancelFn context.CancelFunc) {
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-stopChan
		cancelFn()
		writeUploadStatusesFile(uploadStatuses)
		os.Exit(1)
	}()
}

func writeUploadStatusesFile(uploadStatuses UploadStatuses) {
	if bytes, err := json.Marshal(uploadStatuses); err != nil {
		log.Printf("fail to write status file: %+v\n", err)
	} else if err = ioutil.WriteFile("upload-status.json", bytes, 0666); err != nil {
		log.Printf("fail to write status file: %+v\n", err)
	}
}

func readUploadStatusesFile() UploadStatuses {
	uploadStatuses := make(UploadStatuses)

	if bytes, err := ioutil.ReadFile("upload-status.json"); err != nil {
		log.Printf("fail to read status file: %+v\n", err)
	} else if err = json.Unmarshal(bytes, &uploadStatuses); err != nil {
		log.Printf("fail to read status file: %+v\n", err)
		uploadStatuses = make(UploadStatuses)
	}

	return uploadStatuses
}

func createUploader(outputDirectory string, bucket string) Uploader {
	if outputDirectory != "" {
		return NewDirectoryUploader(outputDirectory)
	} else if bucket != "" {
		return NewS3Uploader(bucket)
	}

	return nil
}

func uploadFromDropbox(ctx context.Context, file string, dbx files.Client, uploader Uploader, disableProgressbar bool) error {
	fmt.Printf("downloading '%s'\n", file)

	meta, content, err := downloadFromDropbox(dbx, file)

	if err != nil {
		return err
	}

	defer func() { _ = content.Close() }()

	if !disableProgressbar {
		progressBar := pb.Full.Start64(int64(meta.Size))
		defer progressBar.Finish()
		content = progressBar.NewProxyReader(content)
	}

	return uploader.Upload(ctx, meta, content)
}

func downloadFromDropbox(dbx files.Client, file string) (*files.FileMetadata, io.ReadCloser, error) {
	 meta, content, err := dbx.Download(files.NewDownloadArg(file))

	 if err != nil {
		return nil, nil, fmt.Errorf("fail to download file '%s': %w", file, err)
	}

	return meta, content, nil
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
	Upload(ctx context.Context, file *files.FileMetadata, reader io.Reader) error
}

type S3Uploader struct {
	uploader *s3manager.Uploader
	bucket *string
}

func (s *S3Uploader) Upload(ctx context.Context, file *files.FileMetadata, reader io.Reader) error {
	uploadInput := &s3manager.UploadInput{Bucket: s.bucket, Key: aws.String(file.PathDisplay), Body: reader}

	if file.Size > uint64(s.uploader.PartSize * int64(s.uploader.MaxUploadParts)) {
		prevPartSize := s.uploader.PartSize
		defer func() { s.uploader.PartSize = prevPartSize }()
		s.uploader.PartSize = int64(math.Ceil(float64(file.Size) / float64(s.uploader.MaxUploadParts)))
		log.Printf("S3 uploader: adjusting PartSize to %d\n (MaxUploadParts = %d)", s.uploader.PartSize, s.uploader.MaxUploadParts)
	}

	if _, err := s.uploader.UploadWithContext(ctx, uploadInput); err != nil {
		return fmt.Errorf("fail to upload file '%s': %w", file.PathDisplay, err)
	}

	return nil
}

func NewS3Uploader(bucket string) Uploader {
	return &S3Uploader{
		uploader: s3manager.NewUploader(session.Must(session.NewSession())),
		bucket: aws.String(bucket),
	}
}

type DirectoryUploader struct {
	dir string
}

func (d *DirectoryUploader) Upload(_ context.Context, file *files.FileMetadata, reader io.Reader) error {
	path := filepath.Join(d.dir, file.PathDisplay)
	containingDir := filepath.Dir(path)

	if err := os.MkdirAll(containingDir, 0666); err != nil {
		return fmt.Errorf("fail to create directory '%s': %w", containingDir, err)
	} else if outputFile, err := os.Create(path); err != nil {
		return fmt.Errorf("fail to create file '%s': %w", path, err)
	} else {
		defer func() { _ = outputFile.Close() }()

		if _, err = io.Copy(outputFile, reader); err != nil {
			return fmt.Errorf("fail to copy file '%s': %w", file.PathDisplay, err)
		}

		return nil
	}
}

func NewDirectoryUploader(dir string) Uploader {
	return &DirectoryUploader{dir: dir}
}
