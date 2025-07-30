# Simple Makefile for NAYSAYER

.PHONY: build run test clean install help build-image push-image

# Default target
help:
	@echo "NAYSAYER Simple Build Commands:"
	@echo ""
	@echo "  build     Build the naysayer binary"
	@echo "  run       Build and run the server"
	@echo "  test      Run the test script"
	@echo "  clean      Remove built binaries"
	@echo "  install    Install dependencies"
	@echo "  docker     Build Docker image"
	@echo "  build-image Build and tag for Quay"
	@echo "  push-image Push image to Quay"
	@echo ""

# Build the binary
build:
	@echo "Building naysayer..."
	go build -o naysayer cmd/main.go
	@echo "✅ Built naysayer binary"

# Build and run
run: build
	@echo "Starting naysayer server..."
	./naysayer

# Run tests (requires server to be running)
test:
	@echo "Running tests..."
	@if ! curl -s http://localhost:3000/health > /dev/null; then \
		echo "❌ Server not running. Start with 'make run' first."; \
		exit 1; \
	fi
	./test_simple.sh

# Clean built files
clean:
	@echo "Cleaning..."
	rm -f naysayer
	@echo "✅ Cleaned"

# Install dependencies
install:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download
	@echo "✅ Dependencies installed"

# Build Docker image
docker:
	@echo "Building Docker image..."
	docker build -t naysayer:latest .
	@echo "✅ Docker image built: naysayer:latest"

# Quick development cycle
dev: clean build
	@echo "🚀 Development build complete"

# Production build with optimizations
build-prod:
	@echo "Building production binary..."
	CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o naysayer cmd/main.go
	@echo "✅ Production binary built"

# Build image for Quay
build-image:
	@echo "Building image for Quay..."
	docker build -t quay.io/ddis/naysayer:latest .
	@echo "✅ Image built: quay.io/ddis/naysayer:latest"

# Push image to Quay
push-image: build-image
	@echo "Pushing to Quay..."
	docker push quay.io/ddis/naysayer:latest
	@echo "✅ Image pushed to quay.io/ddis/naysayer:latest"