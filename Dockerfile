FROM registry.access.redhat.com/ubi9/ubi-minimal:latest AS builder

RUN microdnf install -y tar gzip

RUN curl -OL https://go.dev/dl/go1.23.6.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz

WORKDIR app

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor/ vendor/

# Copy the go source
COPY cmd/ cmd/
# COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} /usr/local/go/bin/go build -a -o naysayer cmd/main.go

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
WORKDIR /

COPY --from=builder /app/naysayer .

USER 65532:65532

EXPOSE 3000

CMD ["/naysayer"]
