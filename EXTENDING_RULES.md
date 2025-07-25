# Extending NAYSAYER with New Rules

This guide explains how to create and add new rules to the NAYSAYER webhook system.

## Overview

NAYSAYER uses an extensible rule framework that allows you to easily add new approval logic for different types of merge request changes. Rules are automatically discovered and can be enabled/disabled as needed.

## Rule Architecture

### Core Components

1. **Rule Interface** (`internal/rules/shared/types.go`):
   - `Name()`: Unique identifier
   - `Description()`: Human-readable description
   - `Applies(mrCtx)`: Determines if rule should evaluate the MR
   - `ShouldApprove(mrCtx)`: Binary decision (approve/manual_review)

2. **Rule Registry** (`internal/rules/registry.go`):
   - Central registry for all available rules
   - Handles rule discovery and instantiation
   - Supports categorization and filtering

3. **Rule Manager** (`internal/rules/manager.go`):
   - Orchestrates rule evaluation
   - Handles early filtering (draft MRs, bots)
   - Aggregates rule results into final decision

## Creating a New Rule

### Step 1: Copy the Template

```bash
# Copy the template to create your new rule
cp -r internal/rules/template internal/rules/yourfeature
cd internal/rules/yourfeature
mv rule_template.go rule.go
```

### Step 2: Implement Your Rule

```go
package yourfeature

import (
    "github.com/redhat-data-and-ai/naysayer/internal/gitlab"
    "github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

type Rule struct {
    client *gitlab.Client
}

func NewRule(client *gitlab.Client) *Rule {
    return &Rule{client: client}
}

func (r *Rule) Name() string {
    return "your_feature_rule"
}

func (r *Rule) Description() string {
    return "Evaluates your feature changes"
}

func (r *Rule) Applies(mrCtx *shared.MRContext) bool {
    // Your logic to determine if this rule should run
    for _, change := range mrCtx.Changes {
        if strings.HasSuffix(change.NewPath, ".yourext") {
            return true
        }
    }
    return false
}

func (r *Rule) ShouldApprove(mrCtx *shared.MRContext) (shared.DecisionType, string) {
    // Your approval logic
    if r.hasIssues(mrCtx) {
        return shared.ManualReview, "Issues found requiring manual review"
    }
    return shared.Approve, "No issues detected"
}
```

### Step 3: Register Your Rule

Update `internal/rules/registry.go` to register your rule:

```go
func (r *RuleRegistry) registerBuiltInRules() {
    // Existing rules...
    
    // Your new rule
    r.RegisterRule(&RuleInfo{
        Name:        "your_feature_rule",
        Description: "Evaluates your feature changes",
        Version:     "1.0.0",
        Factory:     yourfeature.NewRule,
        Enabled:     true,
        Category:    "yourfeature",
    })
}
```

### Step 4: Add to Appropriate Rule Managers

Update rule managers to include your rule where appropriate:

```go
// For dataverse workflows
func (r *RuleRegistry) CreateDataverseRuleManager(client *gitlab.Client) shared.RuleManager {
    dataverseRules := []string{
        "warehouse_rule",
        "your_feature_rule", // Add if relevant to dataverse
    }
    // ...
}
```

### Step 5: Write Tests

Create `internal/rules/yourfeature/rule_test.go`:

```go
package yourfeature

import (
    "testing"
    "github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

func TestRule_Applies(t *testing.T) {
    rule := NewRule(nil)
    
    tests := []struct {
        name     string
        changes  []gitlab.FileChange
        expected bool
    }{
        {
            name: "should apply to .yourext files",
            changes: []gitlab.FileChange{
                {NewPath: "test.yourext"},
            },
            expected: true,
        },
        // More test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mrCtx := &shared.MRContext{Changes: tt.changes}
            if got := rule.Applies(mrCtx); got != tt.expected {
                t.Errorf("Applies() = %v, want %v", got, tt.expected)
            }
        })
    }
}
```

## Rule Categories

### Built-in Categories

- **warehouse**: Dataverse warehouse configuration rules
- **source**: Source binding and configuration rules
- **security**: Security-related validation rules
- **compliance**: Compliance and policy enforcement rules

### Creating Custom Categories

Simply specify a new category when registering your rule:

```go
r.RegisterRule(&RuleInfo{
    // ...
    Category: "mycustomcategory",
})
```

## Best Practices

### Performance

1. **Efficient Applies() Logic**: Make the `Applies()` method fast
2. **Lazy Evaluation**: Only fetch additional data in `ShouldApprove()`
3. **API Rate Limiting**: Be mindful of GitLab API calls
4. **Caching**: Cache expensive operations when possible

### Error Handling

1. **Graceful Degradation**: Prefer `ManualReview` over crashes
2. **Detailed Error Messages**: Provide context for manual reviewers
3. **Logging**: Log important decisions and errors

### Code Quality

1. **Single Responsibility**: Each rule should focus on one concern
2. **Descriptive Names**: Use clear, descriptive method and variable names
3. **Documentation**: Comment complex logic and edge cases
4. **Testing**: Write comprehensive tests including edge cases

## Advanced Features

### GitLab API Integration

Access GitLab API through the client:

```go
func (r *Rule) analyzeFile(projectID int, filePath, branch string) error {
    content, err := r.client.FetchFileContent(projectID, filePath, branch)
    if err != nil {
        return err
    }
    
    // Analyze content
    return nil
}
```

### Metrics Integration

Rules automatically get metrics through the rule manager, but you can add custom metrics:

```go
// In your rule
if r.metrics != nil {
    r.metrics.RecordCustomMetric("your_feature_analysis", duration)
}
```

### Configuration

Add rule-specific configuration through environment variables:

```go
type Rule struct {
    client    *gitlab.Client
    threshold int
}

func NewRule(client *gitlab.Client) *Rule {
    threshold := 10 // default
    if env := os.Getenv("YOUR_FEATURE_THRESHOLD"); env != "" {
        if val, err := strconv.Atoi(env); err == nil {
            threshold = val
        }
    }
    
    return &Rule{
        client:    client,
        threshold: threshold,
    }
}
```

## Rule Examples

### File Pattern Rule

```go
func (r *Rule) Applies(mrCtx *shared.MRContext) bool {
    patterns := []string{".sql", ".yaml", ".json"}
    
    for _, change := range mrCtx.Changes {
        for _, pattern := range patterns {
            if strings.HasSuffix(change.NewPath, pattern) {
                return true
            }
        }
    }
    return false
}
```

### Size-based Rule

```go
func (r *Rule) ShouldApprove(mrCtx *shared.MRContext) (shared.DecisionType, string) {
    totalChanges := len(mrCtx.Changes)
    
    if totalChanges > 50 {
        return shared.ManualReview, "Large MR with > 50 files requires manual review"
    }
    
    return shared.Approve, "Small MR auto-approved"
}
```

### Content Analysis Rule

```go
func (r *Rule) ShouldApprove(mrCtx *shared.MRContext) (shared.DecisionType, string) {
    for _, change := range mrCtx.Changes {
        if change.DeletedFile {
            continue
        }
        
        content, err := r.client.FetchFileContent(
            mrCtx.ProjectID, change.NewPath, "HEAD")
        if err != nil {
            return shared.ManualReview, fmt.Sprintf("Could not fetch %s: %v", 
                change.NewPath, err)
        }
        
        if r.hasSecrets(content.Content) {
            return shared.ManualReview, "Potential secrets detected"
        }
    }
    
    return shared.Approve, "No security issues detected"
}
```

## Testing Your Rule

### Unit Tests

```bash
# Run tests for your rule
go test ./internal/rules/yourfeature/...

# Run with coverage
go test -cover ./internal/rules/yourfeature/...
```

### Integration Tests

Test your rule with the full system:

```bash
# Build and test the webhook
go build ./cmd/
./cmd -port=8080 &

# Send test webhook payload
curl -X POST http://localhost:8080/webhook \
  -H "Content-Type: application/json" \
  -d @testdata/test-mr.json
```

### Manual Testing

1. Create a test repository
2. Configure NAYSAYER as a webhook
3. Create MRs that trigger your rule
4. Verify the behavior

## Deployment

### Environment Variables

Document any environment variables your rule uses:

```bash
# Example configuration
YOUR_FEATURE_THRESHOLD=20
YOUR_FEATURE_ENABLED=true
```

### Kubernetes Configuration

Update Kubernetes manifests if needed:

```yaml
env:
- name: YOUR_FEATURE_THRESHOLD
  value: "20"
```

### Monitoring

Your rule automatically gets basic metrics, but consider adding:

1. **Custom alerts** for rule-specific issues
2. **Dashboards** showing rule effectiveness
3. **Logs** for debugging rule decisions

## Troubleshooting

### Common Issues

1. **Rule not triggering**: Check `Applies()` logic
2. **Unexpected decisions**: Add debug logging to `ShouldApprove()`
3. **Performance issues**: Profile GitLab API calls
4. **Test failures**: Ensure test data matches actual GitLab payloads

### Debugging

```go
// Add debug logging
log.Printf("Rule %s: applies=%t, decision=%s", 
    r.Name(), applies, decision)
```

### Metrics

Check rule metrics at `/metrics`:

```
naysayer_rule_evaluations_total{rule_name="your_feature_rule"}
naysayer_rule_execution_time_seconds{rule_name="your_feature_rule"}
```

## Support

For questions or issues:

1. Check existing rules for examples
2. Review the template rule comments
3. Look at test cases for patterns
4. Check the monitoring dashboards
5. Open an issue with logs and reproduction steps