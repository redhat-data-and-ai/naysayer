# 👨‍💻 Naysayer Development Guide

Complete guide for local development and building custom validation rules.

## 🏗️ Development Environment Setup

### Prerequisites

- Go 1.21 or later
- Git
- Make (optional, for convenience commands)
- GitLab token for testing (optional)

### Local Setup

```bash
# Clone repository
git clone https://github.com/your-org/naysayer.git
cd naysayer

# Install dependencies
go mod tidy

# Build application
go build -o naysayer cmd/main.go

# Run tests
go test ./...

# Start development server
export GITLAB_TOKEN=your-token  # Optional for testing
go run cmd/main.go
```

### Development Commands

```bash
# Build binary
make build

# Run tests with coverage
make test

# Run tests with race detection
go test -race ./...

# Build container image
make build-image

# Run linting
golangci-lint run

# Format code
go fmt ./...
```

## 🏗️ Project Structure

```
naysayer/
├── cmd/
│   └── main.go                    # Application entry point
├── internal/
│   ├── config/                    # Configuration management
│   │   ├── config.go             # Configuration loading
│   │   └── types.go              # Configuration types
│   ├── gitlab/                    # GitLab API client
│   │   ├── client.go             # HTTP client implementation
│   │   └── types.go              # GitLab API types
│   ├── rules/                     # Rule engine
│   │   ├── registry.go           # Rule registration and management
│   │   ├── shared/               # Common rule interfaces and types
│   │   │   ├── interfaces.go     # Rule interface definition
│   │   │   └── types.go          # Common types (MRContext, DecisionType)
│   │   ├── rule_a/               # Example validation rule
│   │   │   ├── rule.go           # Rule implementation
│   │   │   ├── rule_test.go      # Unit tests
│   │   │   └── types.go          # Rule-specific types
│   │   └── rule_b/               # Another validation rule
│   └── server/                    # HTTP server and handlers
│       ├── handlers.go           # Webhook and API handlers
│       ├── middleware.go         # HTTP middleware
│       └── server.go             # Server setup
├── docs/                          # Documentation
│   ├── rules/                    # User-facing rule documentation
│   ├── templates/                # Rule development templates
│   └── *.md                      # Development guides
├── config/                        # Kubernetes/OpenShift manifests
├── .github/                       # GitHub Actions workflows
├── Makefile                       # Build automation
├── go.mod                         # Go module dependencies
└── go.sum                         # Dependency checksums
```

## 🛡️ Building Custom Rules

### Quick Start: Create a New Rule

```bash
# 1. Create rule directory
mkdir internal/rules/myrule

# 2. Copy template
cp docs/templates/rule_templates/basic_rule_template.go.template \
   internal/rules/myrule/rule.go

# 3. Customize the template
# Edit internal/rules/myrule/rule.go
# - Replace package name
# - Update Name() and Description()
# - Implement validation logic

# 4. Add tests
cp docs/templates/rule_templates/rule_test_template.go.template \
   internal/rules/myrule/rule_test.go

# 5. Register rule
# Edit internal/rules/registry.go
# Add your rule to the registry

# 6. Test and deploy
go test ./internal/rules/myrule
```

### Rule Interface Implementation

Every rule must implement the `shared.Rule` interface:

```go
package shared

type Rule interface {
    Name() string                                          // Unique rule identifier
    Description() string                                   // Human-readable description
    Applies(mrCtx *MRContext) bool                        // Should this rule evaluate?
    ShouldApprove(mrCtx *MRContext) (DecisionType, string) // Auto-approve or manual review?
}

type DecisionType string

const (
    Approve      DecisionType = "approve"       // Auto-approve the MR
    ManualReview DecisionType = "manual_review" // Require human review
)

type MRContext struct {
    ProjectID int                    // GitLab project ID
    MRIID     int                    // Merge request IID
    Changes   []gitlab.FileChange    // Files changed in the MR
}
```

### Rule Development Patterns

#### 1. Basic File Pattern Rule

```go
package myrule

import (
    "strings"
    "github.com/naysayer/internal/rules/shared"
)

type Rule struct {
    client GitLabClientInterface
    config *Config
}

func (r *Rule) Name() string {
    return "my_validation_rule"
}

func (r *Rule) Description() string {
    return "Validates specific file patterns and content"
}

func (r *Rule) Applies(mrCtx *shared.MRContext) bool {
    for _, change := range mrCtx.Changes {
        if r.shouldValidateFile(change.NewPath) {
            return true
        }
    }
    return false
}

func (r *Rule) shouldValidateFile(path string) bool {
    return strings.HasSuffix(strings.ToLower(path), ".yaml") ||
           strings.HasSuffix(strings.ToLower(path), ".yml")
}

func (r *Rule) ShouldApprove(mrCtx *shared.MRContext) (shared.DecisionType, string) {
    // Implement your validation logic here
    // Return shared.Approve or shared.ManualReview
}
```

#### 2. Content Analysis Rule

```go
func (r *Rule) ShouldApprove(mrCtx *shared.MRContext) (shared.DecisionType, string) {
    for _, change := range mrCtx.Changes {
        if !r.shouldValidateFile(change.NewPath) {
            continue
        }
        
        // Fetch file content
        content, err := r.client.FetchFileContent(mrCtx.ProjectID, change.NewPath, "HEAD")
        if err != nil {
            return shared.ManualReview, fmt.Sprintf("Failed to fetch %s: %v", change.NewPath, err)
        }
        
        // Analyze content
        if !r.validateContent(change.NewPath, content.Content) {
            return shared.ManualReview, fmt.Sprintf("Validation failed for %s", change.NewPath)
        }
    }
    
    return shared.Approve, "All validations passed"
}

func (r *Rule) validateContent(filePath, content string) bool {
    // YAML parsing example
    var data map[string]interface{}
    if err := yaml.Unmarshal([]byte(content), &data); err != nil {
        return false
    }
    
    // Business logic validation
    if requiredField, exists := data["required_field"]; !exists || requiredField == "" {
        return false
    }
    
    return true
}
```

#### 3. Multi-Validator Rule

```go
type Rule struct {
    client     GitLabClientInterface
    config     *Config
    validators []Validator
}

type Validator interface {
    Validate(filePath, content string) ValidationResult
}

type ValidationResult struct {
    IsValid bool
    Issues  []string
}

func (r *Rule) ShouldApprove(mrCtx *shared.MRContext) (shared.DecisionType, string) {
    for _, change := range mrCtx.Changes {
        if !r.shouldValidateFile(change.NewPath) {
            continue
        }
        
        content, err := r.client.FetchFileContent(mrCtx.ProjectID, change.NewPath, "HEAD")
        if err != nil {
            return shared.ManualReview, fmt.Sprintf("Failed to fetch %s: %v", change.NewPath, err)
        }
        
        // Run all validators
        for _, validator := range r.validators {
            result := validator.Validate(change.NewPath, content.Content)
            if !result.IsValid {
                return shared.ManualReview, fmt.Sprintf("Validation failed: %s", strings.Join(result.Issues, ", "))
            }
        }
    }
    
    return shared.Approve, "All validations passed"
}
```

### Configuration Management

#### Adding Rule Configuration

1. **Update configuration types** in `internal/config/types.go`:

```go
type RulesConfig struct {
    // Existing rules...
    MyRule MyRuleConfig `mapstructure:"my_rule"`
}

type MyRuleConfig struct {
    Enabled           bool     `mapstructure:"enabled"`
    StrictMode        bool     `mapstructure:"strict_mode"`
    MaxFileSize       int      `mapstructure:"max_file_size"`
    AllowedExtensions []string `mapstructure:"allowed_extensions"`
}
```

2. **Load configuration** in your rule:

```go
func NewRule(client GitLabClientInterface) *Rule {
    cfg := config.Load()
    ruleConfig := &Config{
        Enabled:           cfg.Rules.MyRule.Enabled,
        StrictMode:        cfg.Rules.MyRule.StrictMode,
        MaxFileSize:       cfg.Rules.MyRule.MaxFileSize,
        AllowedExtensions: cfg.Rules.MyRule.AllowedExtensions,
    }
    
    return &Rule{
        client: client,
        config: ruleConfig,
    }
}
```

3. **Environment variable mapping**:

```bash
# Environment variables automatically map to config
MY_RULE_ENABLED=true
MY_RULE_STRICT_MODE=false
MY_RULE_MAX_FILE_SIZE=1048576
MY_RULE_ALLOWED_EXTENSIONS=.yaml,.yml,.json
```

### Rule Registration

Add your rule to the registry in `internal/rules/registry.go`:

```go
func (r *Registry) RegisterDefaultRules(client *gitlab.Client) {
    // Existing rules...
    
    // Register your rule
    r.RegisterRule(&RuleInfo{
        Name:        "my_rule",
        Description: "My custom validation rule",
        Version:     "1.0.0",
        Factory: func(client *gitlab.Client) shared.Rule {
            return myrule.NewRule(client)
        },
        Enabled:  true,
        Category: "validation",
    })
}
```

## 🧪 Testing

### Unit Testing

#### Basic Test Structure

```go
package myrule

import (
    "testing"
    
    "github.com/naysayer/internal/gitlab"
    "github.com/naysayer/internal/rules/shared"
    "github.com/stretchr/testify/assert"
)

func TestRule_Name(t *testing.T) {
    rule := NewRule(nil)
    assert.Equal(t, "my_rule", rule.Name())
}

func TestRule_ShouldApprove(t *testing.T) {
    tests := []struct {
        name             string
        fileContent      string
        expectedDecision shared.DecisionType
        expectedReason   string
    }{
        {
            name:             "valid content",
            fileContent:      "valid: content",
            expectedDecision: shared.Approve,
            expectedReason:   "validation passed",
        },
        {
            name:             "invalid content",
            fileContent:      "invalid: content",
            expectedDecision: shared.ManualReview,
            expectedReason:   "validation failed",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            client := &MockGitLabClient{fileContent: tt.fileContent}
            rule := NewRule(client)
            
            mrCtx := &shared.MRContext{
                ProjectID: 123,
                MRIID:     456,
                Changes: []gitlab.FileChange{
                    {NewPath: "test.yaml"},
                },
            }
            
            decision, reason := rule.ShouldApprove(mrCtx)
            assert.Equal(t, tt.expectedDecision, decision)
            assert.Contains(t, reason, tt.expectedReason)
        })
    }
}
```

#### Mock GitLab Client

```go
type MockGitLabClient struct {
    fileContent string
    returnError error
}

func (m *MockGitLabClient) FetchFileContent(projectID int, filePath, ref string) (*gitlab.FileContent, error) {
    if m.returnError != nil {
        return nil, m.returnError
    }
    return &gitlab.FileContent{Content: m.fileContent}, nil
}
```

### Integration Testing

```go
func TestRule_Integration(t *testing.T) {
    // Skip if no GitLab token available
    if os.Getenv("GITLAB_TOKEN") == "" {
        t.Skip("GITLAB_TOKEN not set, skipping integration test")
    }
    
    client := gitlab.NewClient(os.Getenv("GITLAB_TOKEN"))
    rule := NewRule(client)
    
    // Test with real GitLab project
    mrCtx := &shared.MRContext{
        ProjectID: 123,
        MRIID:     456,
        Changes: []gitlab.FileChange{
            {NewPath: "config/test.yaml"},
        },
    }
    
    decision, reason := rule.ShouldApprove(mrCtx)
    assert.NotEmpty(t, reason)
    assert.Contains(t, []shared.DecisionType{shared.Approve, shared.ManualReview}, decision)
}
```

### Performance Testing

```go
func BenchmarkRule_ShouldApprove(b *testing.B) {
    client := &MockGitLabClient{fileContent: "test: content"}
    rule := NewRule(client)
    
    mrCtx := &shared.MRContext{
        ProjectID: 123,
        MRIID:     456,
        Changes: []gitlab.FileChange{
            {NewPath: "test.yaml"},
        },
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        rule.ShouldApprove(mrCtx)
    }
}
```

## 🔧 API Development

### Webhook Handler Development

The main webhook handler is in `internal/server/handlers.go`. For rule-specific endpoints:

```go
func (h *Handlers) RuleSpecificEndpoint(c *fiber.Ctx) error {
    // Get rule from registry
    rule := h.registry.GetRule("my_rule")
    if rule == nil {
        return c.Status(404).JSON(fiber.Map{
            "error": "Rule not found",
        })
    }
    
    // Custom logic for your rule
    // Return appropriate response
    return c.JSON(fiber.Map{
        "rule": rule.Name(),
        "status": "active",
    })
}
```

### Adding Custom Endpoints

1. **Define handler** in your rule package:

```go
package myrule

func (r *Rule) StatusHandler() map[string]interface{} {
    return map[string]interface{}{
        "name":        r.Name(),
        "description": r.Description(),
        "config":      r.config,
        "enabled":     r.config.Enabled,
    }
}
```

2. **Register endpoint** in server setup:

```go
// In internal/server/server.go
func (s *Server) setupRoutes() {
    // Existing routes...
    
    // Rule-specific endpoints
    s.app.Get("/api/rules/my-rule/status", func(c *fiber.Ctx) error {
        rule := s.registry.GetRule("my_rule").(*myrule.Rule)
        return c.JSON(rule.StatusHandler())
    })
}
```

## 🚀 Building and Packaging

### Local Build

```bash
# Build for current platform
go build -o naysayer cmd/main.go

# Build for Linux (for containers)
GOOS=linux GOARCH=amd64 go build -o naysayer cmd/main.go

# Build with optimizations
go build -ldflags="-s -w" -o naysayer cmd/main.go
```

### Container Build

```bash
# Build container image
docker build -t naysayer:latest .

# Multi-platform build
docker buildx build --platform linux/amd64,linux/arm64 -t naysayer:latest .

# Using Make
make build-image
make push-image
```

### Release Process

```bash
# Tag release
git tag v1.0.0
git push origin v1.0.0

# Build release binaries
make release

# Create GitHub release (if using GitHub Actions)
gh release create v1.0.0 --generate-notes
```

## 🔍 Debugging

### Local Debugging

```bash
# Run with debug logging
export LOG_LEVEL=debug
go run cmd/main.go

# Debug specific rule
export RULE_DEBUG=true
export MY_RULE_DEBUG=true
go run cmd/main.go

# Use delve debugger
dlv debug cmd/main.go
```

### Testing Webhook Locally

```bash
# Use ngrok for external access
ngrok http 3000

# Test webhook payload
curl -X POST http://localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "object_attributes": {
      "iid": 123,
      "state": "opened"
    },
    "project": {
      "id": 456
    }
  }'
```

### Profiling

```bash
# CPU profiling
go run cmd/main.go -cpuprofile=cpu.prof

# Memory profiling
go run cmd/main.go -memprofile=mem.prof

# Analyze profiles
go tool pprof cpu.prof
go tool pprof mem.prof
```

## 📚 Development Resources

### Code Quality Tools

```bash
# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/go-delve/delve/cmd/dlv@latest

# Run linting
golangci-lint run

# Run security scanning
gosec ./...

# Check for unused dependencies
go mod tidy
```

### Documentation

- [Go Documentation](https://golang.org/doc/)
- [Fiber Framework](https://docs.gofiber.io/)
- [GitLab API](https://docs.gitlab.com/ee/api/)
- [Rule Creation Guide](docs/RULE_CREATION_GUIDE.md)
- [Rule Testing Guide](docs/RULE_TESTING_GUIDE.md)

### Development Workflow

1. **Feature Development**:
   - Create feature branch
   - Implement rule or feature
   - Write comprehensive tests
   - Update documentation

2. **Code Review**:
   - Run all tests and linting
   - Ensure test coverage > 80%
   - Update relevant documentation
   - Submit pull request

3. **Integration**:
   - CI/CD pipeline runs tests
   - Security scanning performed
   - Manual review completed
   - Merge to main branch

---

**🔗 Related Documentation:**
- [Main README](README.md) - Project overview
- [Deployment Guide](DEPLOYMENT.md) - Production deployment
- [Rule Creation Guide](docs/RULE_CREATION_GUIDE.md) - Detailed rule development
- [Rule Testing Guide](docs/RULE_TESTING_GUIDE.md) - Testing strategies