# NAYSAYER File-Based Analysis - Multi-stage Docker build
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest AS builder

# Install build dependencies
RUN microdnf install -y tar gzip ca-certificates

# Install Go
RUN curl -OL https://go.dev/dl/go1.23.6.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz

WORKDIR /app

# Copy dependency manifests and vendor directory
COPY go.mod go.sum ./
COPY vendor/ vendor/

# Copy source code (file-based analysis implementation)
COPY cmd/ cmd/

# Build the binary with file analysis capabilities
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    /usr/local/go/bin/go build -a -ldflags="-w -s" -o naysayer cmd/main.go

# Runtime stage
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# Install CA certificates for GitLab API calls
RUN microdnf install -y ca-certificates && \
    microdnf clean all

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/naysayer .

# Create non-root user
RUN groupadd -r naysayer && useradd -r -g naysayer naysayer
USER naysayer:naysayer

# Expose port
EXPOSE 3000

# Health check using process check (simpler approach)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pgrep naysayer || exit 1

# Environment variable documentation
ENV PORT=3000
ENV GITLAB_BASE_URL=https://gitlab.com
# ENV GITLAB_TOKEN=<set-your-token-here>

# Labels for documentation
LABEL org.opencontainers.image.title="NAYSAYER File-Based Analysis"
LABEL org.opencontainers.image.description="GitLab webhook service for file-based warehouse approval decisions"
LABEL org.opencontainers.image.version="file-analysis"
LABEL org.opencontainers.image.source="https://github.com/redhat-data-and-ai/naysayer"

CMD ["./naysayer"]