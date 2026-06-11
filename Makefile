# NAYSAYER Makefile

.PHONY: build run test test-unit test-e2e test-coverage clean install help docker fmt vet lint lint-fix

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
	@echo "  test           Run all tests (unit + e2e)"
	@echo "  test-unit      Run unit tests only"
	@echo "  test-e2e       Run E2E tests only"
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

# Run all tests (unit + e2e)
test:
	@echo "Running all tests (unit + e2e)..."
	go test ./... -v -race -cover

# Run unit tests only (excluding e2e)
test-unit:
	@echo "Running unit tests..."
	go test $$(go list ./... | grep -v /e2e) -v -race -cover

# Run E2E tests only
test-e2e:
	@echo "Running E2E tests..."
	go test ./e2e -v -count=1
	@echo "✅ E2E tests completed"

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
	@if GOFLAGS=-mod=mod go tool golangci-lint version > /dev/null 2>&1; then \
		GOFLAGS=-mod=mod go tool golangci-lint run ./... && echo "✅ Linting completed"; \
	elif [ -x "$$(go env GOPATH)/bin/golangci-lint" ]; then \
		$$(go env GOPATH)/bin/golangci-lint run ./... && echo "✅ Linting completed"; \
	elif command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run ./... && echo "✅ Linting completed"; \
	else \
		echo "⚠️  golangci-lint not available via Go toolchain or PATH. Install with:"; \
		echo "   go get -tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2"; \
		exit 1; \
	fi

# Run linter with automatic fixes
lint-fix:
	@echo "Running golangci-lint with automatic fixes..."
	@if GOFLAGS=-mod=mod go tool golangci-lint version > /dev/null 2>&1; then \
		GOFLAGS=-mod=mod go tool golangci-lint run --fix --skip-dirs=vendor ./...; \
		echo "✅ Linting with fixes completed"; \
	elif [ -x "$$(go env GOPATH)/bin/golangci-lint" ]; then \
		$$(go env GOPATH)/bin/golangci-lint run --fix --skip-dirs=vendor ./...; \
		echo "✅ Linting with fixes completed"; \
	elif command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run --fix --skip-dirs=vendor ./...; \
		echo "✅ Linting with fixes completed"; \
	else \
		echo "⚠️  golangci-lint not installed. Install with:"; \
		echo "   go get -tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2"; \
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
