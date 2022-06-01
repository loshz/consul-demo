#################
# Build stage 0 #
#################
FROM golang:1.18-alpine3.16

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
FROM alpine:3.16

ARG USER=consul-demo

# Create group (-g/-G) and system user (-S)
# with specific UID (-u) and no password (-D)
RUN addgroup -g 2000 -S $USER \
  && adduser -G $USER -S -D -u 2000 $USER

# Copy operator binary from build stage 0
COPY --from=0 --chown=$USER /go/bin/consul-demo /bin/

WORKDIR /home/$USER

USER $USER

ENTRYPOINT ["consul-demo"]
