# Contributing to NAYSAYER

Welcome to NAYSAYER! This guide will help you understand the codebase structure and how to contribute effectively.

## 📁 Project Structure

### Root Directory
```
├── cmd/                    # Application entry points
├── config/                 # OpenShift deployment manifests
├── docs/                   # Project documentation
├── internal/               # Private application code
├── vendor/                 # Go module dependencies (vendored)
├── go.mod, go.sum         # Go module files
├── Dockerfile             # Container build configuration
├── Makefile              # Build and development tasks
└── README.md             # Project overview
```

### `/cmd` - Application Entry Points
Contains the main applications for this project.
- **`main.go`** - Primary NAYSAYER webhook service entry point
- **Purpose**: Initialize configuration, set up handlers, start HTTP server
- **Add here**: New application entry points (if building additional tools)

### `/config` - Deployment Configuration
Kubernetes/OpenShift configuration files for deployment.
- **`deployment.yaml`** - Kubernetes deployment configuration
- **`service.yaml`** - Kubernetes service configuration  
- **`route.yaml`** - OpenShift route configuration
- **Purpose**: Infrastructure as code for deployment
- **Add here**: New Kubernetes resources, environment-specific configs

### `/docs` - Documentation
All project documentation and guides.
- **`IMPLEMENTATION_GUIDE.md`** - Detailed technical implementation
- **`FLOW_DIAGRAM.md`** - Code flow and architecture diagrams
- **`API_REFERENCE.md`** - API documentation
- **Purpose**: Keep all documentation organized and accessible
- **Add here**: New guides, API docs, architectural decisions

### `/internal` - Private Application Code
Private packages that cannot be imported by external projects.

#### `/internal/webhook` - HTTP Request Handling
Handles incoming HTTP requests (GitLab webhooks, health checks).
- **`dataverse_product_config_review.go`** - Main webhook handler for MR reviews
- **`health.go`** - Health check endpoint handler
- **Purpose**: HTTP layer, request/response handling, routing
- **Add here**: New HTTP endpoints, middleware, request handlers

#### `/internal/rules` - Business Rules Engine
Core business logic for evaluating merge requests.
- **`rules.go`** - Go-based rule configuration (replaces YAML config)
- **`shared/`** - Common types and interfaces used by all rules
  - **`types.go`** - Core interfaces (Rule, RuleEngine) and result types
  - **`engine.go`** - Rule execution engine with unanimous approval strategy
- **`warehouse/`** - Warehouse size change approval logic
  - **`rule.go`** - Main warehouse rule implementation
  - **`types.go`** - Warehouse-specific types (WarehouseChange, WarehouseSizes)
  - **`analyzer.go`** - YAML analysis logic for dataproduct files
- **Purpose**: Business logic, approval decisions, rule evaluation
- **Add here**: New rules, rule modifications, business logic changes

#### `/internal/gitlab` - GitLab Integration
GitLab API client and integration logic.
- **`client.go`** - GitLab API client implementation
- **`file_content.go`** - File content fetching utilities
- **`types.go`** - GitLab-specific data structures
- **Purpose**: External API integration, GitLab data fetching
- **Add here**: New GitLab API endpoints, data structures, integrations

#### `/internal/config` - Configuration Management
Application configuration loading and management.
- **`config.go`** - Configuration structure and loading logic
- **Purpose**: Environment variables, config validation, app settings
- **Add here**: New configuration options, validation logic


## 🚀 Development Workflow

### Adding a New Rule
1. Create new rule directory in `/internal/rules/{rule_name}/`
2. Create `rule.go` implementing the `Rule` interface from `shared/`
3. Add rule-specific types in `types.go` if needed
4. Add rule to `rules.go` configuration
5. Add tests for the rule logic
6. Update documentation

### Adding a New HTTP Endpoint
1. Add handler function in `/internal/webhook/`
2. Register route in `cmd/main.go`
3. Add tests for the endpoint
4. Update API documentation

### Adding New Analysis Capabilities
1. Extend functionality within the relevant rule's directory (e.g., `/internal/rules/warehouse/analyzer.go`)
2. Update rule logic if needed
3. Add tests for analysis logic
4. Update implementation guide

## 🧪 Testing Guidelines

### Rule Testing
- Test rule logic with various MR scenarios
- Mock external dependencies (GitLab API)
- Test both approval and rejection cases

### Integration Testing
- Test webhook endpoints with sample payloads
- Verify GitLab API integration
- Test configuration loading

### Unit Testing
- Test individual functions and methods
- Use dependency injection for testability
- Aim for high test coverage

## 📝 Code Style Guidelines

### Go Conventions
- Follow standard Go naming conventions
- Use gofmt for formatting
- Add meaningful comments for exported functions
- Handle errors explicitly

### Architecture Principles
- Keep business logic in `/internal/rules/`
- Use interfaces for external dependencies
- Maintain clean separation between layers
- Prefer composition over inheritance

### Commit Messages
- Use conventional commit format
- Include scope: `feat(rules): add new warehouse validation`
- Be descriptive about changes made

## 🔧 Local Development

### Prerequisites
- Go 1.19+
- GitLab access token (for API integration)
- Docker (for containerized development)

### Setup
```bash
# Clone repository
git clone <repository-url>
cd naysayer

# Install dependencies
go mod download

# Set environment variables
export GITLAB_TOKEN=your-token-here
export GITLAB_BASE_URL=https://gitlab.example.com

# Run locally
go run ./cmd
```

### Building
```bash
# Build binary
make build

# Build container
make docker-build

# Run tests
make test
```

## 🎯 Design Decisions

### Why Go Configuration Instead of YAML?
- **Type Safety**: Compile-time validation vs runtime errors
- **IDE Support**: Full autocomplete and refactoring
- **Maintainability**: Rules are part of the codebase
- **Testing**: Can unit test rule configurations

### Why Unanimous Approval Strategy?
- **Conservative**: Ensures all rules agree before auto-approval
- **Predictable**: Clear decision logic
- **Extensible**: Easy to add new rules without changing strategy

### Why Separate Packages?
- **Single Responsibility**: Each package has one clear purpose
- **Testability**: Easy to mock and test individual components
- **Maintainability**: Changes are isolated to relevant packages

## 📚 Additional Resources

- [Go Project Layout](https://github.com/golang-standards/project-layout)
- [GitLab API Documentation](https://docs.gitlab.com/ee/api/)
- [Kubernetes API Reference](https://kubernetes.io/docs/reference/)

## 🤝 Getting Help

- Check existing documentation in `/docs`
- Review implementation guide for technical details
- Look at existing code patterns for consistency
- Ask questions in project discussions or issues

---

Thank you for contributing to NAYSAYER! 🚀