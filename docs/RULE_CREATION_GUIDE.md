# 🎯 Rule Creation Guide for Developers

This guide walks you through creating new validation rules in NAYSAYER. Follow the patterns and examples to implement rules quickly and reliably.

## 📋 Table of Contents

1. [🚀 Quick Start](#quick-start) - Get a rule working in 15 minutes
2. [🏗️ Rule Interface](#rule-interface) - Core implementation requirements
3. [📁 Rule Templates](#rule-templates) - Choose your starting point
4. [⚙️ Implementation Guide](#implementation-guide) - Step-by-step development
5. [🧪 Testing Your Rule](#testing-your-rule) - Ensure reliability
6. [📡 Registration & Deployment](#registration--deployment) - Go live
7. [🔧 Troubleshooting](#troubleshooting) - Common issues and solutions

## 🚀 Quick Start

### Choose Your Rule Template

| **Rule Type** | **Use Case** | **Template** | **Time** |
|---------------|--------------|--------------|----------|
| 📄 **Simple File Validation** | File patterns, naming, basic content | Service Account Rule | 15 mins |
| 🔧 **Section-Based Validation** | YAML sections, cost control, resources | Warehouse Rule | 20 mins |
| ✅ **Auto-Approval** | Documentation, metadata, zero-risk files | Metadata Rule | 10 mins |

### 30-Second Setup

```bash
# 1. Create rule directory (choose one approach)

# Option A: Create subdirectory for complex rules
mkdir internal/rules/myvalidation

# Option B: Add single file rule directly to internal/rules/
# (like service_account_rule.go, documentation_auto_approval_rule.go)

# 2. Copy appropriate template
# For single-file rules:
cp internal/rules/service_account_rule.go internal/rules/my_new_rule.go
# For complex rules with subdirectory:
cp internal/rules/warehouse/rule.go internal/rules/myvalidation/rule.go
# For simple auto-approval:
cp internal/rules/common/metadata_rule.go internal/rules/myvalidation/rule.go

# 3. Customize and register (see sections below)
```

## 🏗️ Rule Interface

Every rule implements this core interface from `internal/rules/shared/types.go`:

```go
type Rule interface {
    // Name returns the unique identifier for this rule
    Name() string
    
    // Description returns a human-readable description
    Description() string
    
    // Returns which line ranges this rule validates in a file
    GetCoveredLines(filePath string, fileContent string) []LineRange
    
    // Validates only the specified line ranges
    ValidateLines(filePath string, fileContent string, lineRanges []LineRange) (DecisionType, string)
}

// Optional: For rules that need MR context (like warehouse rule)
type ContextAwareRule interface {
    Rule
    
    // SetMRContext provides the full MR context to the rule for advanced analysis
    SetMRContext(mrCtx *MRContext)
}
```

### Key Concepts

- **GetCoveredLines()**: Declare which file lines your rule validates
- **ValidateLines()**: Perform validation on specific line ranges  
- **ContextAwareRule**: Optional interface for rules needing GitLab MR context
- **Section-Based Only**: ALL validation uses section-based architecture via `rules.yaml`
- **No Fallbacks**: Files without section configuration require manual review
- **Coverage Enforcement**: All file lines must be covered by at least one rule

## 📁 Rule Templates

### Template 1: Service Account Rule (File-Level Validation)

**Best for**: Validating entire files, pattern matching, security checks

```go
// internal/rules/myvalidation/rule.go
package myvalidation

import (
    "strings"
    "github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

type Rule struct {
    name        string
    description string
}

func NewRule() *Rule {
    return &Rule{
        name:        "my_validation_rule",
        description: "Validates specific file patterns and content",
    }
}

func (r *Rule) Name() string {
    return r.name
}

func (r *Rule) Description() string {
    return r.description
}

func (r *Rule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
    if !r.appliesToFile(filePath) {
        return []shared.LineRange{} // Rule doesn't apply
    }
    
    // Validate entire file
    totalLines := shared.CountLines(fileContent)
    return []shared.LineRange{{
        StartLine: 1,
        EndLine:   totalLines,
        FilePath:  filePath,
    }}
}

func (r *Rule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
    if !r.appliesToFile(filePath) {
        return shared.Approve, "Rule does not apply to this file"
    }
    
    // Your validation logic here
    if r.isValidContent(fileContent) {
        return shared.Approve, "File validation passed"
    }
    
    return shared.ManualReview, "File requires manual review"
}

func (r *Rule) appliesToFile(filePath string) bool {
    // Define which files this rule should validate
    return strings.HasSuffix(strings.ToLower(filePath), ".yaml") ||
           strings.HasSuffix(strings.ToLower(filePath), ".yml")
}

func (r *Rule) isValidContent(content string) bool {
    // Your validation logic
    return len(strings.TrimSpace(content)) > 0
}
```

### Template 2: Warehouse Rule (Section-Based Validation)

**Best for**: Validating specific YAML sections, cost control, resource management

```go
// internal/rules/myvalidation/rule.go
package myvalidation

import (
    "strings"
    "github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

type Rule struct {
    name        string
    description string
}

func NewRule() *Rule {
    return &Rule{
        name:        "my_section_rule",
        description: "Validates specific sections in product.yaml files",
    }
}

func (r *Rule) Name() string {
    return r.name
}

func (r *Rule) Description() string {
    return r.description
}

func (r *Rule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
    if !r.isTargetFile(filePath) {
        return []shared.LineRange{}
    }
    
    // For section-based validation, return placeholder
    // Actual sections configured in rules.yaml
    return []shared.LineRange{{
        StartLine: 1,
        EndLine:   1,
        FilePath:  filePath,
    }}
}

func (r *Rule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
    if !r.isTargetFile(filePath) {
        return shared.Approve, "Not a target file"
    }
    
    // fileContent contains section content when called by section manager
    if r.validateSectionContent(fileContent) {
        return shared.Approve, "Section validation passed"
    }
    
    return shared.ManualReview, "Section requires manual review"
}

func (r *Rule) isTargetFile(filePath string) bool {
    return strings.HasSuffix(strings.ToLower(filePath), "product.yaml") ||
           strings.HasSuffix(strings.ToLower(filePath), "product.yml")
}

func (r *Rule) validateSectionContent(sectionContent string) bool {
    // Your section-specific validation logic
    // This content is extracted by the section manager
    return len(strings.TrimSpace(sectionContent)) > 0
}
```

### Template 3: Metadata Rule (Auto-Approval)

**Best for**: Documentation files, metadata, zero-risk changes

```go
// internal/rules/myvalidation/rule.go
package myvalidation

import (
    "strings"
    "github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

type Rule struct {
    name        string
    description string
}

func NewRule() *Rule {
    return &Rule{
        name:        "my_metadata_rule",
        description: "Auto-approves documentation and metadata changes",
    }
}

func (r *Rule) Name() string {
    return r.name
}

func (r *Rule) Description() string {
    return r.description
}

func (r *Rule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
    if !r.isMetadataFile(filePath) {
        return []shared.LineRange{}
    }
    
    // Cover entire file for metadata
    totalLines := shared.CountLines(fileContent)
    return []shared.LineRange{{
        StartLine: 1,
        EndLine:   totalLines,
        FilePath:  filePath,
    }}
}

func (r *Rule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
    if r.isMetadataFile(filePath) {
        return shared.Approve, r.getApprovalReason(filePath)
    }
    
    return shared.ManualReview, "Not a metadata file"
}

func (r *Rule) isMetadataFile(filePath string) bool {
    lowerPath := strings.ToLower(filePath)
    return strings.HasSuffix(lowerPath, ".md") ||
           strings.HasSuffix(lowerPath, ".txt") ||
           strings.Contains(lowerPath, "readme") ||
           strings.Contains(lowerPath, "docs/")
}

func (r *Rule) getApprovalReason(filePath string) string {
    if strings.HasSuffix(strings.ToLower(filePath), ".md") {
        return "Auto-approved: Markdown documentation changes are safe"
    }
    return "Auto-approved: Metadata file changes are safe"
}
```

## ⚙️ Implementation Guide

### Step 1: Choose and Customize Template

1. **Pick Template**: Choose based on your validation needs
2. **Update Names**: Change rule name, description, and package name
3. **Define File Patterns**: Implement `appliesToFile()` or similar
4. **Add Validation Logic**: Implement your specific validation requirements

### Step 2: Add Configuration (Optional)

```go
type Config struct {
    Enabled     bool     `yaml:"enabled"`
    StrictMode  bool     `yaml:"strict_mode"`
    AllowedExts []string `yaml:"allowed_extensions"`
}

func (r *Rule) loadConfig() *Config {
    return &Config{
        Enabled:     getEnvBool("MY_RULE_ENABLED", true),
        StrictMode:  getEnvBool("MY_RULE_STRICT_MODE", false),
        AllowedExts: getEnvStringSlice("MY_RULE_ALLOWED_EXTENSIONS", []string{".yaml", ".yml"}),
    }
}
```

### Step 3: Handle Edge Cases

```go
func (r *Rule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
    // Handle empty content
    if len(strings.TrimSpace(fileContent)) == 0 {
        return shared.ManualReview, "Empty file requires review"
    }
    
    // Handle large files
    if len(fileContent) > 1024*1024 { // 1MB
        return shared.ManualReview, "File too large for automatic validation"
    }
    
    // Your validation logic
    return r.validateContent(fileContent)
}
```

## 🧪 Testing Your Rule

### Basic Test Structure

```go
// internal/rules/myvalidation/rule_test.go
package myvalidation

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

func TestRule_Name(t *testing.T) {
    rule := NewRule()
    assert.Equal(t, "my_validation_rule", rule.Name())
}

func TestRule_ValidateLines(t *testing.T) {
    rule := NewRule()
    
    tests := []struct {
        name           string
        filePath       string
        fileContent    string
        expectedResult shared.DecisionType
        expectedReason string
    }{
        {
            name:           "valid file",
            filePath:       "test.yaml",
            fileContent:    "name: test\nvalue: 123",
            expectedResult: shared.Approve,
            expectedReason: "File validation passed",
        },
        {
            name:           "invalid file",
            filePath:       "test.yaml",
            fileContent:    "",
            expectedResult: shared.ManualReview,
            expectedReason: "Empty file requires review",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            lineRanges := []shared.LineRange{{StartLine: 1, EndLine: 10, FilePath: tt.filePath}}
            decision, reason := rule.ValidateLines(tt.filePath, tt.fileContent, lineRanges)
            assert.Equal(t, tt.expectedResult, decision)
            assert.Contains(t, reason, tt.expectedReason)
        })
    }
}
```

### Test Commands

```bash
# Run tests
go test ./internal/rules/myvalidation -v

# Run with coverage
go test ./internal/rules/myvalidation -cover

# Run specific test
go test ./internal/rules/myvalidation -run TestRule_ValidateLines -v
```

## 📡 Registration & Deployment

### Step 1: Register Rule

Add to `internal/rules/registry.go` in the `registerBuiltInRules()` function:

```go
func (r *RuleRegistry) registerBuiltInRules() {
    // ... existing rules ...
    
    // Your new rule
    _ = r.RegisterRule(&RuleInfo{
        Name:        "my_validation_rule",
        Description: "Validates specific file patterns and content",
        Version:     "1.0.0",
        Factory: func(client *gitlab.Client) shared.Rule {
            return myvalidation.NewRule(client) // Pass client if needed
        },
        Enabled:  true,
        Category: "validation",
    })
}

### Step 2: Configure rules.yaml

For **file-level validation** (Service Account template):
```yaml
files:
  - name: "my_files"
    path: "**/"
    filename: "*.{yaml,yml}"
    parser_type: yaml
    enabled: true
    sections:
      - name: full_file
        yaml_path: "."  # Entire file
        required: true
        rule_names:
          - my_validation_rule
        description: "Full file validation"
```

For **section-based validation** (Warehouse template):
```yaml
files:
  - name: "product_configs"
    path: "**/"
    filename: "product.{yaml,yml}"
    parser_type: yaml
    enabled: true
    sections:
      - name: my_section
        yaml_path: my_section  # Specific YAML path
        required: true
        rule_names:
          - my_section_rule
        description: "My section validation"
```

For **auto-approval** (Metadata template):
```yaml
files:
  - name: "documentation"
    path: "**/"
    filename: "*.{md,txt}"
    parser_type: text
    enabled: true
    sections:
      - name: full_file
        yaml_path: "."
        required: true
        rule_names:
          - my_metadata_rule
        description: "Documentation auto-approval"
```

### Step 3: Test Integration

```bash
# Build and test
go test ./internal/rules/... -v

# Test your specific rule
go test ./internal/rules/myvalidation -v

# Run full integration test
make test

# Deploy to development environment
kubectl apply -f config/deployment.yaml

# Verify rule is loaded in logs
kubectl logs deployment/naysayer | grep "Registered rule: my_validation_rule"
```

## 🎯 Complete Example: Adding a Simple File Extension Rule

Here's a complete walkthrough of adding a real rule to validate file extensions:

### Step 1: Create the Rule File

Create `internal/rules/file_extension_rule.go`:

```go
package rules

import (
    "path/filepath"
    "strings"
    "github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

type FileExtensionRule struct {
    name        string
    description string
}

func NewFileExtensionRule() *FileExtensionRule {
    return &FileExtensionRule{
        name:        "file_extension_rule", 
        description: "Prevents .tmp and .backup files from being committed",
    }
}

func (r *FileExtensionRule) Name() string {
    return r.name
}

func (r *FileExtensionRule) Description() string {
    return r.description
}

func (r *FileExtensionRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
    if r.hasBadExtension(filePath) {
        // Only cover the file if it has a bad extension
        totalLines := shared.CountLines(fileContent)
        return []shared.LineRange{{
            StartLine: 1,
            EndLine:   totalLines,
            FilePath:  filePath,
        }}
    }
    return []shared.LineRange{} // Don't cover files with good extensions
}

func (r *FileExtensionRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
    if r.hasBadExtension(filePath) {
        return shared.ManualReview, "Temporary and backup files should not be committed: " + filepath.Base(filePath)
    }
    return shared.Approve, "File extension is acceptable"
}

func (r *FileExtensionRule) hasBadExtension(filePath string) bool {
    ext := strings.ToLower(filepath.Ext(filePath))
    badExtensions := []string{".tmp", ".backup", ".bak", ".swp"}
    
    for _, badExt := range badExtensions {
        if ext == badExt {
            return true
        }
    }
    return false
}
```

### Step 2: Add Test File

Create `internal/rules/file_extension_rule_test.go`:

```go
package rules

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

func TestFileExtensionRule_Name(t *testing.T) {
    rule := NewFileExtensionRule()
    assert.Equal(t, "file_extension_rule", rule.Name())
}

func TestFileExtensionRule_ValidateLines(t *testing.T) {
    rule := NewFileExtensionRule()
    
    tests := []struct {
        name           string
        filePath       string
        expectedResult shared.DecisionType
        expectedReason string
    }{
        {
            name:           "good file",
            filePath:       "config/settings.yaml",
            expectedResult: shared.Approve,
            expectedReason: "File extension is acceptable",
        },
        {
            name:           "tmp file",
            filePath:       "temp/data.tmp",
            expectedResult: shared.ManualReview,
            expectedReason: "Temporary and backup files should not be committed",
        },
        {
            name:           "backup file",
            filePath:       "config/settings.yaml.backup",
            expectedResult: shared.ManualReview,
            expectedReason: "Temporary and backup files should not be committed",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            lineRanges := []shared.LineRange{{StartLine: 1, EndLine: 10, FilePath: tt.filePath}}
            decision, reason := rule.ValidateLines(tt.filePath, "some content", lineRanges)
            assert.Equal(t, tt.expectedResult, decision)
            assert.Contains(t, reason, tt.expectedReason)
        })
    }
}
```

### Step 3: Register in Registry

Add to `internal/rules/registry.go` in `registerBuiltInRules()`:

```go
// File extension rule
_ = r.RegisterRule(&RuleInfo{
    Name:        "file_extension_rule",
    Description: "Prevents .tmp and .backup files from being committed",
    Version:     "1.0.0",
    Factory: func(client *gitlab.Client) shared.Rule {
        return NewFileExtensionRule()
    },
    Enabled:  true,
    Category: "file_validation",
})
```

### Step 4: Configure in rules.yaml

Add to `rules.yaml`:

```yaml
# Add to the files array
- name: "all_files"
  path: "**/"
  filename: "*"
  parser_type: yaml
  description: "All files for extension validation"
  enabled: true
  sections:
    - name: full_file
      yaml_path: .
      required: false
      rule_names:
        - file_extension_rule
      description: "File extension validation"
```

### Step 5: Test Your Rule

```bash
# Test the rule
go test ./internal/rules -run TestFileExtensionRule -v

# Test integration
go test ./internal/rules/... -v

# Check rule is registered
go run main.go --list-rules | grep file_extension_rule
```

**That's it!** Your rule is now part of the system and will automatically validate file extensions in all merge requests.

## 🔧 Troubleshooting

### Common Issues

**Rule Not Triggering**
```bash
# Check file patterns match
echo "File: config/test.yaml" | grep -E "\.ya?ml$"

# Verify rule registration
grep "my_validation_rule" internal/rules/registry.go

# Check logs
kubectl logs deployment/naysayer | grep "my_validation_rule"
```

**False Positives/Negatives**
```bash
# Add debug logging
func (r *Rule) ValidateLines(...) {
    log.Printf("DEBUG: Validating file=%s, content_len=%d", filePath, len(fileContent))
    // ... your logic
}

# Test with specific cases
go test ./internal/rules/myvalidation -run TestSpecificCase -v
```

**Configuration Issues**
```bash
# Check environment variables
env | grep MY_RULE

# Verify rules.yaml syntax
kubectl apply --dry-run=client -f config/rules.yaml
```

## ✅ Quick Reference

### Rule Development Checklist

- [ ] Choose appropriate template
- [ ] Implement `Name()` and `Description()`
- [ ] Implement `GetCoveredLines()` for file/section coverage
- [ ] Implement `ValidateLines()` with your validation logic
- [ ] Add comprehensive tests
- [ ] Register rule in registry.go
- [ ] Configure rules.yaml appropriately
- [ ] Test integration end-to-end

### Best Practices

1. **Start Simple**: Begin with basic validation, add complexity gradually
2. **Clear Errors**: Provide specific, actionable error messages
3. **Performance**: Handle large files and edge cases gracefully
4. **Testing**: Cover happy path, error cases, and edge conditions
5. **Documentation**: Clear rule descriptions and configuration options

### File Structure

**Option A: Single File Rules** (like service_account_rule.go):
```
internal/rules/
├── my_new_rule.go       # Main implementation
├── my_new_rule_test.go  # Unit tests
└── registry.go          # Register your rule here
```

**Option B: Complex Rules with Subdirectory** (like warehouse/):
```
internal/rules/myvalidation/
├── rule.go              # Main implementation (type Rule struct{})
├── rule_test.go         # Unit tests  
├── analyzer.go          # Complex logic (optional)
└── types.go             # Data structures (optional)
```

**Actual Examples in Codebase**:
- `internal/rules/service_account_rule.go` - Single file rule
- `internal/rules/warehouse/` - Complex rule with subdirectory
- `internal/rules/common/metadata_rule.go` - Shared utility rule

---

**🎉 You're ready to create production-quality validation rules!** Start with a template, customize for your needs, and follow the registration steps to deploy your rule.