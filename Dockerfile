# syntax = docker/dockerfile:experimental
FROM golang:1.14.6-alpine3.12 as builder
WORKDIR /project
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -ldflags "-s -w" -o /dropbox-to-s3 .

FROM alpine:3.12.0
RUN apk --update add --no-cache ca-certificates
COPY --from=builder /dropbox-to-s3 /
ENTRYPOINT ["/dropbox-to-s3"]
CMD ["-h"]
