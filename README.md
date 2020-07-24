# Build on Docker
`DOCKER_BUILDKIT=1 docker build -t dropbox-to-s3 .`

# Build on Mac/Linux (target Linux)
`CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -mod vendor -ldflags "-s -w" -o dropbox-to-s3`

# Run
`docker run -it --rm dropbox-to-s3`
```console
Usage of /dropbox-to-s3:
  -output-bucket string
    	S3 bucket (if you want to upload to S3)
  -input-directory string
        Dropbox directory to upload
  -output-directory string
    	local output directory (if you want to upload to local file system)
  -disable-progressbar string
        disable progress bar
  -dropbox-token string
    	dropbox access token
```
You can pass either `-output-bucket` or `-output-directory` or none in which case the program will only print files to upload (can be used to generate a list).

# Generate Dropbox access token
## Create an app
- go to https://www.dropbox.com/developers/apps/create
- in _Choose an API_ select _Dropbox API_
- in _Choose the type of access you need_ select _Full Dropbox_
- give a name to your app
- click on the _Create app_ button

## Generate token
On the _OAuth 2_ panel under _Settings_ tab click on the _Generate_ button under _Generated access token_

# Configure AWS credentials
See https://docs.aws.amazon.com/sdk-for-go/api/aws/session/#hdr-Environment_Variables
