# NAYSAYER Development Setup Guide

## 🚀 **Complete Local Development Environment**

This guide helps you set up a complete development environment for NAYSAYER, including IDE configuration, debugging, and testing tools.

## 📋 **Prerequisites**

### **Required Software**
- **Go 1.23+** - [Download](https://golang.org/dl/)
- **Git** - Version control
- **Docker** - For containerized development and testing
- **kubectl/oc** - For Kubernetes/OpenShift deployment testing
- **jq** - JSON processing for API testing
- **curl** - HTTP testing

### **Recommended IDE Setup**
- **VS Code** with Go extension
- **GoLand** (JetBrains)
- **Vim/Neovim** with vim-go

## 🛠️ **Initial Setup**

### **1. Clone and Prepare Repository**
```bash
# Clone repository
git clone git@github.com:redhat-data-and-ai/naysayer.git
cd naysayer

# Install Go dependencies
go mod download
go mod vendor

# Verify build works
make build
```

### **2. Environment Configuration**

Create your development environment file:
```bash
# Create .env.dev for local development
cat > .env.dev << EOF
# GitLab Configuration
GITLAB_TOKEN=glpat-your-development-token
GITLAB_BASE_URL=https://gitlab.com

# Webhook Configuration (optional for development)
WEBHOOK_SECRET=development-secret-key

# Server Configuration
PORT=3000
LOG_LEVEL=debug

# Development flags
DEV_MODE=true
EOF
```

### **3. Development GitLab Setup**

#### **Create Development GitLab Token**
1. Go to GitLab → **User Settings** → **Access Tokens**
2. Create token with scopes:
   - ✅ `api` - Full API access
   - ✅ `read_repository` - Read files
   - ✅ `write_repository` - Comment on MRs
3. Set expiry to 90 days for development
4. Copy token to `.env.dev`

#### **Create Test Project**
```bash
# Create test repository for webhook testing
curl -H "Authorization: Bearer $GITLAB_TOKEN" \
  -X POST "https://gitlab.com/api/v4/projects" \
  -d "name=naysayer-test&description=NAYSAYER webhook testing"
```

## 🖥️ **IDE Configuration**

### **VS Code Setup**

#### **Recommended Extensions**
```json
// .vscode/extensions.json
{
  "recommendations": [
    "golang.go",
    "ms-vscode.vscode-json",
    "redhat.vscode-yaml",
    "ms-kubernetes-tools.vscode-kubernetes-tools",
    "ms-vscode.test-adapter-converter"
  ]
}
```

#### **VS Code Settings**
```json
// .vscode/settings.json
{
  "go.toolsManagement.checkForUpdates": "local",
  "go.useLanguageServer": true,
  "go.formatTool": "goimports",
  "go.lintTool": "golangci-lint",
  "go.testFlags": ["-v", "-race"],
  "go.buildTags": "integration",
  "go.testTimeout": "30s",
  "files.exclude": {
    "vendor/": true,
    "*.log": true
  }
}
```

#### **Debug Configuration**
```json
// .vscode/launch.json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug NAYSAYER",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/cmd/main.go",
      "env": {
        "GITLAB_TOKEN": "${env:GITLAB_TOKEN}",
        "GITLAB_BASE_URL": "https://gitlab.com",
        "PORT": "3000",
        "LOG_LEVEL": "debug"
      },
      "args": []
    },
    {
      "name": "Debug Tests",
      "type": "go",
      "request": "launch",
      "mode": "test",
      "program": "${workspaceFolder}",
      "env": {
        "LOG_LEVEL": "debug"
      }
    }
  ]
}
```

### **GoLand Setup**

#### **Run Configuration**
1. **Run** → **Edit Configurations**
2. **Add** → **Go Build**
3. Configure:
   - **Name**: NAYSAYER Development
   - **Run kind**: File
   - **Files**: `cmd/main.go`
   - **Environment**: Add GitLab token and other env vars
   - **Working directory**: Project root

#### **Test Configuration**
1. **Add** → **Go Test**
2. Configure:
   - **Name**: All Tests
   - **Test kind**: Directory
   - **Directory**: Project root
   - **Pattern**: `./...`

## 🔧 **Development Tools Setup**

### **1. Go Tools Installation**
```bash
# Install development tools
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/onsi/ginkgo/v2/ginkgo@latest
go install golang.org/x/tools/cmd/godoc@latest
go install github.com/go-delve/delve/cmd/dlv@latest

# Verify installations
goimports -version
golangci-lint version
ginkgo version
```

### **2. Git Hooks Setup**
```bash
# Create pre-commit hook
cat > .git/hooks/pre-commit << 'EOF'
#!/bin/bash
set -e

echo "Running pre-commit checks..."

# Format code
echo "→ Formatting code..."
goimports -w .

# Lint code
echo "→ Linting code..."
golangci-lint run

# Run tests
echo "→ Running tests..."
go test ./... -short

echo "✅ Pre-commit checks passed!"
EOF

chmod +x .git/hooks/pre-commit
```

### **3. Makefile Development Targets**

Add these development-specific targets to your Makefile:
```makefile
# Development targets
.PHONY: dev-setup dev-run dev-test dev-lint dev-clean

dev-setup: ## Set up development environment
	@echo "Setting up development environment..."
	go mod download
	go mod vendor
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

dev-run: ## Run NAYSAYER in development mode
	@echo "Starting NAYSAYER in development mode..."
	source .env.dev && go run cmd/main.go

dev-test: ## Run tests with verbose output
	@echo "Running tests in development mode..."
	go test ./... -v -race -cover

dev-lint: ## Run linter in development mode
	@echo "Running linter..."
	golangci-lint run --config .golangci.yml

dev-clean: ## Clean development artifacts
	@echo "Cleaning development artifacts..."
	go clean -cache -testcache -modcache
	rm -f *.log
	rm -f coverage.out
```

## 🧪 **Testing Setup**

### **1. Test Environment Configuration**
```bash
# Create test environment
cat > .env.test << EOF
GITLAB_TOKEN=test-token-placeholder
GITLAB_BASE_URL=https://gitlab.example.com
WEBHOOK_SECRET=test-secret
PORT=3001
LOG_LEVEL=error
TEST_MODE=true
EOF
```

### **2. Mock GitLab Server Setup**
```bash
# Create mock server for testing (optional)
mkdir -p test/mock-gitlab
cat > test/mock-gitlab/main.go << 'EOF'
package main

import (
    "encoding/json"
    "net/http"
    "log"
)

func main() {
    http.HandleFunc("/api/v4/", mockGitLabAPI)
    log.Println("Mock GitLab server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func mockGitLabAPI(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    
    // Mock responses based on path
    switch r.URL.Path {
    case "/api/v4/user":
        json.NewEncoder(w).Encode(map[string]interface{}{
            "id": 1,
            "username": "test-user",
        })
    default:
        w.WriteHeader(404)
        json.NewEncoder(w).Encode(map[string]string{
            "message": "404 Not Found",
        })
    }
}
EOF
```

### **3. Test Data Management**
```bash
# Create test fixtures directory
mkdir -p test/fixtures

# Create sample webhook payloads
cat > test/fixtures/merge_request_webhook.json << 'EOF'
{
  "object_kind": "merge_request",
  "object_attributes": {
    "id": 123,
    "iid": 456,
    "title": "Test MR: Reduce warehouse size",
    "state": "opened",
    "target_branch": "main",
    "source_branch": "feature/warehouse-reduction",
    "author": {
      "username": "developer"
    }
  },
  "project": {
    "id": 789,
    "name": "test-project"
  },
  "changes": {
    "total": 1
  }
}
EOF

# Create sample product.yaml for testing
mkdir -p test/fixtures/dataproducts/test-product/dev
cat > test/fixtures/dataproducts/test-product/dev/product.yaml << 'EOF'
name: test-dataproduct
kind: source-aligned
rover_group: dataverse-source-test
warehouses:
  - name: test-warehouse
    warehouse: LARGE
    usage_type: source
EOF
```

## 🔍 **Debugging Setup**

### **1. Debugging with Delve**
```bash
# Install Delve debugger
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug main application
dlv debug cmd/main.go

# Debug specific test
dlv test ./internal/webhook -- -test.run TestWebhookHandler
```

### **2. Performance Profiling**
```bash
# Add profiling endpoints to main.go (development only)
if os.Getenv("DEV_MODE") == "true" {
    import _ "net/http/pprof"
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
}

# Access profiling data
go tool pprof http://localhost:6060/debug/pprof/profile
```

### **3. Structured Logging Debug**
```bash
# Enable debug logging
export LOG_LEVEL=debug

# Pretty print JSON logs
go run cmd/main.go 2>&1 | jq '.'

# Filter specific log messages
go run cmd/main.go 2>&1 | jq 'select(.msg | contains("webhook"))'
```

## 🌐 **Local Webhook Testing**

### **1. ngrok Setup for Webhook Testing**
```bash
# Install ngrok
# macOS: brew install ngrok
# Linux: Download from https://ngrok.com/

# Start NAYSAYER locally
make dev-run

# In another terminal, expose webhook endpoint
ngrok http 3000

# Use the ngrok HTTPS URL for GitLab webhook configuration
# Example: https://abc123.ngrok.io/dataverse-product-config-review
```

### **2. Manual Webhook Testing**
```bash
# Test webhook endpoint locally
curl -X POST http://localhost:3000/dataverse-product-config-review \
  -H "Content-Type: application/json" \
  -H "X-Gitlab-Event: Merge Request Hook" \
  -d @test/fixtures/merge_request_webhook.json

# Test health endpoints
curl http://localhost:3000/health | jq '.'
curl http://localhost:3000/ready | jq '.'
```

### **3. Integration Testing Script**
```bash
# Create integration test script
cat > scripts/integration_test.sh << 'EOF'
#!/bin/bash
set -e

echo "🧪 Running NAYSAYER Integration Tests"

# Start NAYSAYER in background
make dev-run &
NAYSAYER_PID=$!

# Wait for startup
sleep 2

# Test health endpoint
echo "→ Testing health endpoint..."
curl -f http://localhost:3000/health > /dev/null

# Test webhook endpoint
echo "→ Testing webhook endpoint..."
curl -f -X POST http://localhost:3000/dataverse-product-config-review \
  -H "Content-Type: application/json" \
  -H "X-Gitlab-Event: Merge Request Hook" \
  -d @test/fixtures/merge_request_webhook.json > /dev/null

# Cleanup
kill $NAYSAYER_PID

echo "✅ Integration tests passed!"
EOF

chmod +x scripts/integration_test.sh
```

## 📊 **Development Monitoring**

### **1. Log Monitoring**
```bash
# Tail logs with highlighting
make dev-run 2>&1 | grep --color=always -E "(ERROR|WARN|error|warn|Error|Warning)"

# Monitor specific components
make dev-run 2>&1 | jq 'select(.component == "webhook")'
```

### **2. Performance Monitoring**
```bash
# Monitor resource usage
watch 'ps aux | grep naysayer'

# Monitor port usage
lsof -i :3000

# Monitor file descriptors
lsof -p $(pgrep naysayer)
```

## 🔄 **Development Workflow**

### **Daily Development Flow**
```bash
# 1. Start development
git checkout main
git pull origin main
git checkout -b feature/your-feature

# 2. Set up environment
source .env.dev
make dev-setup

# 3. Develop with hot reloading
make dev-run  # Terminal 1
make dev-test # Terminal 2 (run tests on changes)

# 4. Before committing
make dev-lint
make dev-test
git add .
git commit -m "feat: your feature description"

# 5. Integration testing
./scripts/integration_test.sh
```

### **Code Quality Checks**
```bash
# Run all quality checks
make dev-lint
go test ./... -race -cover
go mod tidy
goimports -w .
```

## 🚨 **Common Development Issues**

### **Issue: GitLab API Rate Limiting**
```bash
# Solution: Use mock server for development
export GITLAB_BASE_URL=http://localhost:8080
go run test/mock-gitlab/main.go &
```

### **Issue: Port Already in Use**
```bash
# Find process using port 3000
lsof -i :3000

# Kill process
kill -9 $(lsof -t -i:3000)
```

### **Issue: Go Module Issues**
```bash
# Clean and reset modules
go clean -modcache
rm go.sum
go mod download
go mod tidy
```

## 🔗 **Related Documentation**

- [CONTRIBUTING.md](../CONTRIBUTING.md) - Contributing guidelines
- [Troubleshooting Guide](TROUBLESHOOTING.md) - Comprehensive debugging and testing strategies
- [Rule Creation Guide](RULE_CREATION_GUIDE.md) - Rule development and creation

## 🎯 **Next Steps**

Once your development environment is set up:

1. **Explore the codebase** - Start with `internal/webhook/` and `internal/rules/`
2. **Run existing tests** - `make dev-test` to understand current functionality
3. **Create a simple rule** - Follow [Rule Creation Guide](RULE_CREATION_GUIDE.md)
4. **Set up webhook testing** - Use ngrok for real GitLab integration
5. **Read the architecture** - Review [Section-Based Architecture](SECTION_BASED_ARCHITECTURE.md)

---

🎉 **Your NAYSAYER development environment is now ready for productive coding!** 