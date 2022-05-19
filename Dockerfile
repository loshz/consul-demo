#################
# Build stage 0 #
#################
FROM golang:1.18-alpine3.15

# Install build dependencies
RUN apk --no-cache add build-base

# Create work dir and copy sourcecode.
WORKDIR /go/src/github.com/loshz/consul-demo
COPY . .

# Build binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go/bin/consul-demo ./cmd/...

#################
# Build stage 1 #
#################
FROM alpine:3.15

# Install 3rd party dependencies
RUN apk --no-cache add ca-certificates curl

# Copy operator binary from build stage 0
COPY --from=0 /go/bin/consul-demo /bin/

ENTRYPOINT ["consul-demo"]
