# NAYSAYER Makefile

.PHONY: build run test test-coverage clean install help docker fmt vet lint lint-fix

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
	@echo "  test           Run all tests"
	@echo "  test-coverage  Generate test coverage report"
	@echo ""
	@echo "Code Quality:"
	@echo "  lint           Run golangci-lint"
	@echo "  lint-fix       Run golangci-lint with automatic fixes"
	@echo "  fmt            Format code with gofmt"
	@echo "  vet            Run go vet"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean          Remove built binaries and coverage files"
	@echo "  install        Install dependencies"
	@echo ""

# Build the binary
build: lint fmt vet test
	@echo "Building naysayer..."
	go mod download && go mod tidy && go mod vendor
	go build -o naysayer cmd/main.go
	@echo "✅ Built naysayer binary"

# Build and run
run: build
	@echo "Starting naysayer server..."
	./naysayer

# Run unit tests
test:
	@echo "Running unit tests..."
	go test ./... -v -race -cover

# Generate test coverage report
test-coverage:
	@echo "Generating test coverage report..."
	@mkdir -p coverage
	go test ./... -coverprofile=coverage/coverage.out -covermode=atomic
	@echo "📊 Coverage Summary:"
	go tool cover -func=coverage/coverage.out | tail -1
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
		golangci-lint run --skip-dirs=vendor ./...; \
		echo "✅ Linting completed"; \
	else \
		echo "⚠️  golangci-lint not installed. Install with:"; \
		echo "   curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin v1.54.2"; \
	fi

# Run linter with automatic fixes
lint-fix:
	@echo "Running golangci-lint with automatic fixes..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run --fix --skip-dirs=vendor ./...; \
		echo "✅ Linting with fixes completed"; \
	else \
		echo "⚠️  golangci-lint not installed. Install with:"; \
		echo "   curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin v1.54.2"; \
	fi

# Clean built files and coverage files
clean:
	@echo "Cleaning..."
	rm -f naysayer
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
	docker build -t quay.io/redhat-data-and-ai/naysayer:latest .
	@echo "✅ Docker image built: quay.io/redhat-data-and-ai/naysayer:latest"


docker-push:
	docker push quay.io/redhat-data-and-ai/naysayer:latest
	@echo "✅ Docker image pushed: quay.io/redhat-data-and-ai/naysayer:latest"
