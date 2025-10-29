# NAYSAYER Development Guide

## Prerequisites

- **Go 1.23+**
- **Git**
- **golangci-lint** - Install: `brew install golangci-lint` or see [installation guide](https://golangci-lint.run/usage/install/)
- **GitLab token** - For testing webhook integration

## Quick Start

```bash
# Clone repository
git clone git@github.com:redhat-data-and-ai/naysayer.git
cd naysayer

# Install dependencies
make install

# Run tests
make test

# Start development server
export GITLAB_TOKEN=glpat-your-token
make run
```

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run only unit tests
make test-unit

# Run only E2E tests
make test-e2e

# Generate coverage report
make test-coverage
```

### Running Specific Tests

```bash
# Test specific package
go test ./internal/rules/warehouse -v

# Test specific function
go test ./internal/webhook -run TestWebhookHandler -v

# Test specific E2E scenario
go test ./e2e -v -run TestE2E_Scenarios/warehouse_increase
```

### Creating E2E Tests

See the [E2E Testing Guide](../e2e/README.md) for complete instructions on creating scenario-based tests.

Quick example:
```bash
# Create scenario directory
mkdir -p e2e/testdata/scenarios/my_scenario/{before,after}

# Add scenario.yaml with test configuration
# Add files to before/ and after/ directories

# Run your scenario
go test ./e2e -v -run TestE2E_Scenarios/my_scenario
```

## Code Quality

### Make Commands

```bash
make lint      # Run golangci-lint
make lint-fix  # Run golangci-lint with auto-fixes
make fmt       # Format code with gofmt
make vet       # Run go vet
make test      # Run all tests
```

## Local Webhook Testing

### Using ngrok

```bash
# Start naysayer
export GITLAB_TOKEN=glpat-your-token
make run

# In another terminal, expose webhook
ngrok http 3000

# Use ngrok URL in GitLab webhook settings
# Example: https://abc123.ngrok.io/webhook
```

### Manual Testing

```bash
# Test webhook endpoint
curl -X POST http://localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -H "X-Gitlab-Event: Merge Request Hook" \
  -d @test_payload.json

# Test health endpoint
curl http://localhost:3000/health
```

## IDE Setup

### VS Code

Recommended extensions:
- `golang.go` - Go language support
- `redhat.vscode-yaml` - YAML support

VS Code will automatically use the Go extension for debugging, testing, and formatting.


### Using IDE Debugger

Both VS Code and GoLand support debugging Go applications out of the box. Set breakpoints and use the debug panel.

## Environment Variables

Create `.env.dev` for local development:

```bash
GITLAB_TOKEN=glpat-your-development-token
GITLAB_BASE_URL=https://gitlab.com
PORT=3000
LOG_LEVEL=debug
```

Then source it before running:
```bash
source .env.dev && make run
```

## Common Issues

### Port Already in Use

```bash
# Find and kill process on port 3000
lsof -i :3000
kill -9 $(lsof -t -i:3000)
```

### Go Module Issues

```bash
# Clean and re-download modules
go clean -modcache
rm go.sum
go mod download
go mod tidy
```

### Tests Failing

```bash
# Clear test cache
go clean -testcache

# Run tests with verbose output
make test-unit -v
```

## Adding New Rules

See the [Rule Creation Guide](RULE_CREATION_GUIDE.md) for detailed instructions on creating new validation rules.

Quick overview:
1. Create new package in `internal/rules/your_rule/`
2. Implement the `Rule` interface from `internal/rules/shared/types.go`
3. Register rule in `internal/rules/registry.go`
4. Add tests
5. Update documentation

## Documentation

### What to Document

When adding features, update:
- Code comments for public functions
- `docs/rules/` if adding new rules
- This development guide if changing workflow
- `README.md` if changing user-facing behavior
- E2E tests to validate the feature

### Documentation Style

- Use clear, concise language
- Include code examples where helpful
- Keep documentation close to the code it describes
- Avoid duplicating information

## CI/CD

### GitHub Actions

CI runs on all PRs and includes:
- Linting (`golangci-lint`)
- Unit tests
- E2E tests (PR only)
- Security scanning
- Code coverage

See `.github/workflows/githubci.yml` for details.

### Viewing CI Results

- Check the "Actions" tab in GitHub
- CI must pass before merging
- Coverage reports uploaded to Codecov

## Related Documentation

- [E2E Testing Guide](../e2e/README.md) - End-to-end testing framework
- [Rule Creation Guide](RULE_CREATION_GUIDE.md) - Creating new validation rules
- [Troubleshooting Guide](TROUBLESHOOTING.md) - Debugging and fixing issues
- [Section-Based Architecture](SECTION_BASED_ARCHITECTURE.md) - Architecture overview
