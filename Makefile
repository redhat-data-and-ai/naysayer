# NAYSAYER Makefile

.PHONY: build run test test-unit test-integration test-coverage clean install help docker fmt vet lint

# Default target
help:
	@echo "NAYSAYER Build Commands:"
	@echo ""
	@echo "Build & Run:"
	@echo "  build          Build the naysayer binary"
	@echo "  run            Build and run the server"
	@echo "  docker         Build Docker image"
	@echo ""
	@echo "Testing:"
	@echo "  test           Run all tests (unit + integration)"
	@echo "  test-unit      Run unit tests only"
	@echo "  test-integration Run integration tests only"
	@echo "  test-coverage  Generate test coverage report"
	@echo ""
	@echo "Code Quality:"
	@echo "  lint           Run golangci-lint"
	@echo "  fmt            Format code with gofmt"
	@echo "  vet            Run go vet"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean          Remove built binaries and coverage files"
	@echo "  install        Install dependencies"
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

# Run unit tests
test-unit:
	@echo "Running unit tests..."
	go test ./... -v -race -cover

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	@if ! curl -s http://localhost:3000/health > /dev/null; then \
		echo "❌ Server not running. Start with 'make run' first."; \
		exit 1; \
	fi
	./test_mr.sh

# Run all tests
test: test-unit test-integration

# Generate test coverage report
test-coverage:
	@echo "Generating test coverage report..."
	go test ./... -coverprofile=coverage.out
	@echo "📊 Coverage Summary:"
	go tool cover -func=coverage.out
	@rm -f coverage.out
	@echo "✅ Coverage report completed"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "✅ Code formatted"

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...
	@echo "✅ go vet completed"

# Run linter
lint:
	@echo "Running golangci-lint..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
		echo "✅ Linting completed"; \
	else \
		echo "⚠️  golangci-lint not installed. Install with:"; \
		echo "   curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin v1.54.2"; \
	fi

# Clean built files and coverage files
clean:
	@echo "Cleaning..."
	rm -f naysayer coverage.out
	rm -rf coverage/
	@echo "✅ Cleaned"

# Install dependencies
install:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download
	@echo "✅ Dependencies installed"

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t naysayer:latest .
	@echo "✅ Docker image built: naysayer:latest"


