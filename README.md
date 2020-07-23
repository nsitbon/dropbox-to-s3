# Build
`DOCKER_BUILDKIT=1 docker build -t dropbox-to-s3 .`

# Run
`docker run -it --rm dropbox-to-s3`
```console
Usage of /dropbox-to-s3:
  -bucket string
    	S3 bucket
  -directory string
    	dropbox directory
  -token string
    	dropbox access token
```

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
