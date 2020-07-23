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
