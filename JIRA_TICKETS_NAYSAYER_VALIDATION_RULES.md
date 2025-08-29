# Naysayer YAML Validation Rules - JIRA Tickets Documentation

## Overview

This document provides detailed specifications for implementing YAML validation rules in the naysayer system. These rules are based on the analysis of `004_data_product_config_reviewer_checklist.md`, `0039-self-service-platform.md`, and the dataproduct-config repository structure.

## Implementation Strategy

- **Total Tickets:** 22 (all 1-point stories)
- **Completed:** 3 tickets ✅ | **Partially Done:** 5 tickets 🔄 | **Remaining:** 14 tickets ❌
- **Phases:** 6 phases for logical implementation order
- **Focus:** Each ticket implements one specific validation rule
- **Testing:** Every rule includes comprehensive test cases

## Implementation Status Legend
- ✅ **Completed:** Fully implemented and working
- 🔄 **Partially Done:** Framework/utilities exist, needs completion
- ❌ **Not Started:** Needs full implementation

## Implementation Guidelines

### **Rule Execution Order**
1. **File Path Validation** (Ticket 2) - First line of defense
2. **Basic Field Validation** (Ticket 1) - YAML structure and syntax
3. **Content-Specific Rules** (Tickets 3-22) - Domain-specific validation

### **Configuration Integration**
All rules must integrate with `internal/config/config.go` structure:
```go
type RulesConfig struct {
    EnabledRules       []string
    DisabledRules      []string
    StrictMode         bool     // Fail on warnings
    MaxFileSize        int      // Max YAML file size (bytes)
    MaxFilesPerMR      int      // Max files per MR
}
```

### **Error Message Standards**
All validation errors must follow this format:
```
❌ [RULE_NAME]: [Brief description]
💡 Resolution: [Specific steps to fix]
📖 Documentation: [Link to relevant docs]
```

### **Performance Requirements**
- GitLab API calls must be batched when possible
- File content should be cached within single MR evaluation
- Rules should fail fast on obvious violations
- Maximum validation time: 30 seconds per MR

---

## Phase 1: Core Infrastructure (4 tickets)

### ✅ Ticket 1: Basic YAML Field Validation Framework - COMPLETED
**Story Points:** 1  
**Epic:** Naysayer Validation Rules Infrastructure
**Status:** ✅ **COMPLETED** - Core infrastructure fully implemented

#### Description
Create the foundational validation framework for YAML field validation that will be used by all subsequent validation rules.

**✅ IMPLEMENTATION STATUS:**
- **Completed:** Core validation framework is fully implemented
- **Location:** `internal/rules/shared/`, `internal/rules/manager.go`, `internal/rules/registry.go`
- **Features Working:** Rule registry, line-level validation, YAML parsing with line tracking, error handling

#### Technical Requirements
- Add YAML field validation utilities to `internal/rules/shared` package
- Create helper functions for common validation patterns
- Implement field type validation (string, int, bool, array)
- Add error message standardization
- Create validation result structures
- **ENHANCED:** Add YAML syntax validation (malformed YAML, duplicate keys)
- **ENHANCED:** Add required fields validation framework
- **ENHANCED:** Add basic data type validation enhancement

#### Files to Create/Modify
- `internal/rules/shared/yaml_validator.go` (new)
- `internal/rules/shared/yaml_validator_test.go` (new)
- `internal/rules/shared/validation_errors.go` (new)

#### Implementation Details
```go
// Add to shared package
type YAMLValidator struct {
    fieldPath string
    value     interface{}
}

func (v *YAMLValidator) ValidateRequired() error
func (v *YAMLValidator) ValidateString() (string, error)
func (v *YAMLValidator) ValidateArray() ([]interface{}, error)
func (v *YAMLValidator) ValidateEmail() error
func (v *YAMLValidator) ValidateEnum(allowedValues []string) error
```

#### Acceptance Criteria
- [ ] Can validate required YAML fields exist
- [ ] Can validate field types (string, int, bool, array)
- [ ] Returns standardized validation error messages
- [ ] Has 100% test coverage
- [ ] Integration tests with sample YAML files
- [ ] **ENHANCED:** Validates YAML syntax and detects malformed YAML
- [ ] **ENHANCED:** Detects duplicate keys in YAML files
- [ ] **ENHANCED:** Validates required fields (name, kind, rover_group for products; email, role for service accounts)

#### Configuration Requirements
```go
// Add to config.go
type YAMLValidationConfig struct {
    EnableSyntaxValidation    bool     `json:"enable_syntax_validation"`
    EnableDuplicateKeyCheck  bool     `json:"enable_duplicate_key_check"`
    RequiredFields          []string  `json:"required_fields"`
    MaxYAMLDepth            int       `json:"max_yaml_depth"`
}
```

#### Error Handling
```go
// Standardized error types
type ValidationError struct {
    RuleName    string
    ErrorType   string // "syntax", "required_field", "type_mismatch"
    Field       string
    Message     string
    Resolution  string
    DocsLink    string
}
```

#### Test Cases
```yaml
# ✅ Valid test case
name: "test-product"
kind: "aggregated"
required_field: "value"

# ❌ Invalid test cases
name: 123  # Should be string
# missing required_field

# ❌ Syntax errors
name: "test"
invalid: yaml: syntax
```

---

### 🔄 Ticket 2: File Path Structure Validation Rule - PARTIALLY DONE
**Story Points:** 1  
**Epic:** Naysayer Validation Rules Infrastructure
**Status:** 🔄 **PARTIALLY DONE** - Utility functions exist, need rule implementation

#### Description
Implement validation for the dataproduct-config repository file path structure to ensure MRs follow the correct directory organization.

**🔄 IMPLEMENTATION STATUS:**
- **Existing:** Helper functions in `internal/rules/shared/utils.go`
  - `IsDataProductFile()`, `IsMigrationFile()`, `GetEnvironmentFromPath()`, `GetDataProductFromPath()`
- **Missing:** Actual rule implementation using these utilities
- **Effort:** Create rule class and register it in the system

#### Technical Requirements
- Validate `dataproducts/{source|aggregate|platform}/{name}/{env}/` pattern
- Check that `product.yaml` exists in environment directories
- Validate environment names (dev, sandbox, preprod, prod, platformtest)
- Validate data product types (source, aggregate, platform)

#### Files to Create/Modify
- `internal/rules/path_structure_rule.go` (new)
- `internal/rules/path_structure_rule_test.go` (new)
- Update `internal/webhook/dataverse_product_config_review.go` to register rule

#### Implementation Details
```go
type PathStructureRule struct {
    name string
}

func (r *PathStructureRule) Validate(ctx *shared.MRContext) []shared.LineValidationResult {
    // Validate each changed file path
    // Check directory structure
    // Ensure product.yaml exists in env directories
}
```

#### Acceptance Criteria
- [ ] Rejects MRs with files in incorrect directory structure
- [ ] Validates environment names (dev, sandbox, preprod, prod, platformtest)
- [ ] Validates data product types (source, aggregate, platform)
- [ ] Ensures product.yaml exists in environment directories
- [ ] Provides clear error messages for path violations

#### Configuration Requirements
```go
// Add to config.go PathValidationConfig
type PathValidationConfig struct {
    AllowedProductTypes  []string `json:"allowed_product_types"` // ["source", "aggregate", "platform"]
    AllowedEnvironments  []string `json:"allowed_environments"`  // ["dev", "sandbox", "preprod", "prod", "platformtest"]
    RequiredProductFile  string   `json:"required_product_file"` // "product.yaml"
    StrictPathValidation bool     `json:"strict_path_validation"`
}
```

#### Implementation Notes
- Use existing helper functions from `internal/rules/shared/utils.go`
- Validate path structure before other rules run
- Cache path validation results for performance

#### Error Handling
```go
// Path validation specific errors
type PathValidationError struct {
    InvalidPath     string
    ExpectedPattern string
    ErrorType       string // "invalid_type", "invalid_env", "missing_file"
    Resolution      string
}
```

#### Test Cases
```
✅ Valid paths:
- dataproducts/aggregate/helloaggregate/prod/product.yaml
- dataproducts/source/marketo/dev/product.yaml
- serviceaccounts/prod/test_appuser.yaml

❌ Invalid paths:
- dataproducts/invalid-type/test/prod/product.yaml
- dataproducts/aggregate/test/invalid-env/product.yaml
- random-file.yaml

# Error message example:
❌ PATH_VALIDATION: Invalid directory structure
💡 Resolution: Use pattern 'dataproducts/{source|aggregate|platform}/{name}/{env}/'
📖 Documentation: [link to naming conventions]
```

---

### ❌ Ticket 3: Product Name Consistency Rule - NOT STARTED
**Story Points:** 1  
**Epic:** Naysayer Validation Rules Infrastructure
**Status:** ❌ **NOT STARTED** - Needs full implementation

#### Description
Validate that the `tags.data_product` field in product.yaml matches the data product directory name for naming consistency.

#### Technical Requirements
- Parse `tags.data_product` field from product.yaml files
- Extract data product name from file path
- Compare for exact match
- Handle edge cases (missing tags, missing field)
- **ENHANCED:** Validate database naming conventions (`data_product_db[].database` should follow patterns)
- **ENHANCED:** Validate schema naming conventions (`presentation_schemas[].name`)
- **ENHANCED:** Ensure database names relate to data product names

#### Files to Create/Modify
- `internal/rules/product_name_rule.go` (new)
- `internal/rules/product_name_rule_test.go` (new)
- Update rule registration

#### Implementation Details
```go
type ProductNameRule struct {
    name string
}

func (r *ProductNameRule) Validate(ctx *shared.MRContext) []shared.LineValidationResult {
    // For each product.yaml file
    // Extract data product name from path: dataproducts/{type}/{NAME}/{env}/
    // Parse YAML and get tags.data_product
    // Compare for exact match
}
```

#### Acceptance Criteria
- [ ] Validates `tags.data_product` matches directory name
- [ ] Handles missing `tags` section gracefully
- [ ] Handles missing `data_product` field gracefully
- [ ] Provides clear error messages with expected vs actual values
- [ ] Works for all data product types (source, aggregate, platform)
- [ ] **ENHANCED:** Validates database naming conventions (e.g., `helloaggregate_db`)
- [ ] **ENHANCED:** Validates schema naming conventions (marts, staging, raw, etc.)
- [ ] **ENHANCED:** Prevents conflicting or reserved schema names

#### Configuration Requirements
```go
// Add to config.go NamingValidationConfig
type NamingValidationConfig struct {
    ValidateTagMatching         bool     `json:"validate_tag_matching"`
    ValidateDatabaseNaming      bool     `json:"validate_database_naming"`
    DatabaseNamingSuffix        string   `json:"database_naming_suffix"` // "_db"
    AllowedSchemaNames          []string `json:"allowed_schema_names"`   // ["marts", "staging", "raw"]
    StrictNamingConventions     bool     `json:"strict_naming_conventions"`
}
```

#### Implementation Notes
- Extract data product name from file path using `GetDataProductFromPath()`
- Validate both `tags.data_product` and database naming patterns
- Check schema names against approved list

#### Error Handling
```go
// Naming validation errors
type NamingValidationError struct {
    FieldName       string
    ExpectedValue   string
    ActualValue     string
    NamingType      string // "tag_mismatch", "database_naming", "schema_naming"
}
```

#### Test Cases
```yaml
# ✅ Valid: dataproducts/aggregate/helloaggregate/prod/product.yaml
name: helloaggregate
tags:
  data_product: helloaggregate
data_product_db:
- database: helloaggregate_db
  presentation_schemas:
  - name: marts  # ✅ Approved schema name

# ❌ Invalid: dataproducts/aggregate/helloaggregate/prod/product.yaml
tags:
  data_product: different-name  # ❌ Should match directory

# ❌ Invalid: Missing tags section
name: helloaggregate

# Error message example:
❌ NAMING_VALIDATION: Data product tag mismatch
💡 Resolution: Change 'tags.data_product' to 'helloaggregate' to match directory name
📖 Documentation: [link to naming conventions]
```

---

### ❌ Ticket 18: Cross-Reference Validation Rule - NOT STARTED
**Story Points:** 1  
**Epic:** Naysayer Validation Rules Infrastructure
**Status:** ❌ **NOT STARTED** - Needs full implementation

#### Description
Validate that service accounts and consumers referenced in configuration files actually exist and maintain environment consistency.

#### Technical Requirements
- Validate service accounts referenced in product.yaml `consumers[].name` exist
- Ensure environment consistency (prod product referencing prod service account)
- Check consumer references point to real data products or service accounts
- Cross-validate file references across the repository

#### Files to Create/Modify
- `internal/rules/cross_reference_rule.go` (new)
- `internal/rules/cross_reference_rule_test.go` (new)
- Update rule registration

#### Implementation Details
```go
type CrossReferenceRule struct {
    name string
}

func (r *CrossReferenceRule) validateServiceAccountExists(name, env string, ctx *shared.MRContext) error
func (r *CrossReferenceRule) validateConsumerExists(name, kind string, ctx *shared.MRContext) error
func (r *CrossReferenceRule) getEnvironmentFromPath(path string) string
```

#### Acceptance Criteria
- [ ] Validates service accounts referenced in product.yaml exist
- [ ] Ensures environment consistency (prod product → prod service account)
- [ ] Validates consumer references point to real entities
- [ ] Provides clear error messages for missing references
- [ ] Handles cross-environment reference validation

#### Test Cases
```yaml
# ✅ Valid: dataproducts/aggregate/test/prod/product.yaml
consumers:
  - name: test_astro_prod_appuser  # Must exist in serviceaccounts/prod/
    kind: service_account

# ❌ Invalid: References non-existent service account
consumers:
  - name: nonexistent_appuser
    kind: service_account

# ❌ Invalid: Environment mismatch
# prod/product.yaml referencing dev service account
consumers:
  - name: test_astro_dev_appuser
    kind: service_account
```

---

## Phase 2: Service Account Validation (5 tickets)

### 🔄 Ticket 4: Service Account Email Format Validation - PARTIALLY DONE
**Story Points:** 1  
**Epic:** Service Account Validation Rules
**Status:** 🔄 **PARTIALLY DONE** - Framework exists, need validator implementation

#### Description
Validate that service account email fields contain valid email formats and are from allowed domains.

**🔄 IMPLEMENTATION STATUS:**
- **Existing:** Service account analyzer framework in `internal/rules/serviceaccount/analyzer.go`
- **Existing:** Configuration structure and validation pipeline
- **Missing:** `EmailValidator` implementation (referenced but not implemented)
- **Effort:** Implement the actual email validation logic

#### Technical Requirements
- Validate email format using regex
- Check against allowed domains (configurable, default: redhat.com)
- Handle malformed YAML gracefully
- Provide specific error messages for different validation failures

#### Files to Create/Modify
- `internal/rules/service_account_email_rule.go` (new)
- `internal/rules/service_account_email_rule_test.go` (new)
- Update `internal/config/config.go` to add allowed domains configuration

#### Implementation Details
```go
type ServiceAccountEmailRule struct {
    name           string
    allowedDomains []string
}

func (r *ServiceAccountEmailRule) validateEmailFormat(email string) error
func (r *ServiceAccountEmailRule) validateDomain(email string) error
```

#### Configuration Addition
```go
// Add to ServiceAccountRuleConfig
type ServiceAccountRuleConfig struct {
    // ... existing fields
    AllowedDomains []string // Add this field
}
```

#### Acceptance Criteria
- [ ] Validates email format (contains @, valid characters)
- [ ] Validates email domain against allowed list
- [ ] Configurable allowed domains (default: redhat.com)
- [ ] Clear error messages for format vs domain failures
- [ ] Only validates service account YAML files

#### Configuration Requirements
```go
// Add to ServiceAccountRuleConfig
type ServiceAccountEmailConfig struct {
    AllowedDomains          []string `json:"allowed_domains"`          // ["redhat.com"]
    ValidateEmailFormat     bool     `json:"validate_email_format"`
    BlockGenericAddresses   bool     `json:"block_generic_addresses"`   // Block admin@, support@, etc.
    RequirePersonalEmail    bool     `json:"require_personal_email"`
}
```

#### Implementation Notes
- Leverage existing `EmailValidator` structure in `serviceaccount/analyzer.go`
- Implement actual validation logic in the validator
- Use regex for email format validation

#### Email Validation Patterns
```go
// Email validation regex
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// Generic email patterns to block
var genericEmailPatterns = []string{
    "admin@", "support@", "noreply@", "info@", "help@"
}
```

#### Error Handling
```go
type EmailValidationError struct {
    Email        string
    ErrorType    string // "invalid_format", "invalid_domain", "generic_address"
    AllowedDomains []string
}
```

#### Test Cases
```yaml
# ✅ Valid
email: john.doe@redhat.com
email: jane.smith@redhat.com

# ❌ Invalid format
email: invalid-email
email: user@
email: @redhat.com

# ❌ Invalid domain (if not in allowed list)
email: user@external.com
email: contractor@vendor.com

# ❌ Generic addresses
email: admin@redhat.com
email: support@redhat.com

# Error message example:
❌ EMAIL_VALIDATION: Invalid email domain
💡 Resolution: Use email from allowed domains: redhat.com
📖 Documentation: [link to email policy]
```

---

### 🔄 Ticket 5: Individual vs Group Email Validation - PARTIALLY DONE
**Story Points:** 1  
**Epic:** Service Account Validation Rules
**Status:** 🔄 **PARTIALLY DONE** - Framework exists, need validator implementation

#### Description
Ensure service account emails are individual user emails, not group/distribution list emails, as required for compliance.

#### Technical Requirements
- Create regex patterns to detect group email patterns
- Detect common group email patterns (team-, -list, -group, etc.)
- Validate against individual email requirement
- Configurable group email patterns

#### Files to Create/Modify
- `internal/rules/individual_email_rule.go` (new)
- `internal/rules/individual_email_rule_test.go` (new)
- Update configuration for group email patterns

#### Implementation Details
```go
type IndividualEmailRule struct {
    name                string
    groupEmailPatterns  []string
}

func (r *IndividualEmailRule) isGroupEmail(email string) bool {
    // Check against patterns like:
    // *-team@*, *-list@*, *-group@*, team-*@*, *-developers@*
}
```

#### Configuration Addition
```go
// Add to ServiceAccountRuleConfig
GroupEmailPatterns []string // Patterns to detect group emails
```

#### Acceptance Criteria
- [ ] Detects group email patterns (team-, -list, -group, etc.)
- [ ] Allows individual email addresses
- [ ] Configurable group email detection patterns
- [ ] Clear error messages explaining individual email requirement
- [ ] Works with existing email format validation

#### Configuration Requirements
```go
// Add to ServiceAccountRuleConfig
type GroupEmailDetectionConfig struct {
    GroupEmailPatterns      []string `json:"group_email_patterns"`
    StrictGroupDetection    bool     `json:"strict_group_detection"`
    AllowedGroupExceptions  []string `json:"allowed_group_exceptions"`
}
```

#### Group Email Detection Patterns
```go
// Comprehensive group email patterns
var groupEmailPatterns = []string{
    ".*-team@.*",           // team-data@redhat.com
    ".*-list@.*",           // developers-list@redhat.com
    ".*-group@.*",          // data-group@redhat.com
    "team-.*@.*",           // team-platform@redhat.com
    ".*-developers@.*",     // platform-developers@redhat.com
    ".*-engineering@.*",    // data-engineering@redhat.com
    "dl-.*@.*",             // dl-dataverse@redhat.com
    "ml-.*@.*",             // ml-platform@redhat.com
    "list-.*@.*",           // list-team@redhat.com
    ".*-shared@.*",         // platform-shared@redhat.com
}
```

#### Implementation Notes
- Use regex matching against comprehensive pattern list
- Allow configuration of additional patterns
- Support exceptions for legitimate group emails if needed

#### Test Cases
```yaml
# ✅ Valid individual emails
email: john.doe@redhat.com
email: jane.smith@redhat.com
email: jsmith@redhat.com
email: john.d.smith@redhat.com

# ❌ Invalid group emails
email: team-data@redhat.com
email: developers-list@redhat.com
email: data-team@redhat.com
email: platform-developers@redhat.com
email: dl-dataverse@redhat.com
email: ml-platform@redhat.com
email: data-engineering@redhat.com
email: platform-shared@redhat.com

# Error message example:
❌ GROUP_EMAIL_VALIDATION: Group email detected
💡 Resolution: Use individual email address instead of group/distribution list
📖 Documentation: [link to individual email policy]
```

---

### 🔄 Ticket 6: Service Account Naming Convention Rule - PARTIALLY DONE
**Story Points:** 1  
**Epic:** Service Account Validation Rules
**Status:** 🔄 **PARTIALLY DONE** - Framework exists, need validator implementation

#### Description
Validate that service account name field matches the filename and follows established naming conventions.

#### Technical Requirements
- Extract service account name from filename
- Parse `name` field from YAML
- Validate exact match between filename and name field
- Validate naming convention patterns (configurable)

#### Files to Create/Modify
- `internal/rules/service_account_naming_rule.go` (new)
- `internal/rules/service_account_naming_rule_test.go` (new)
- Update configuration for naming patterns

#### Implementation Details
```go
type ServiceAccountNamingRule struct {
    name            string
    namingPatterns  []string // Regex patterns for valid names
}

func (r *ServiceAccountNamingRule) extractNameFromPath(path string) string
func (r *ServiceAccountNamingRule) validateNamingConvention(name string) error
```

#### Acceptance Criteria
- [ ] Validates name field matches filename (without .yaml)
- [ ] Validates naming convention patterns
- [ ] Handles missing name field gracefully
- [ ] Clear error messages for naming violations
- [ ] Only applies to service account files

#### Configuration Requirements
```go
// Add to ServiceAccountRuleConfig
type ServiceAccountNamingConfig struct {
    EnforceFileNameMatch     bool     `json:"enforce_filename_match"`
    NamingConventionPatterns []string `json:"naming_convention_patterns"`
    RequiredNamingSuffix     string   `json:"required_naming_suffix"`    // "_appuser"
    AllowedSpecialChars      []string `json:"allowed_special_chars"`     // ["_", "-"]
}
```

#### Service Account Naming Patterns
```go
// Standard naming pattern: {dataproduct}_{integration}_{environment}_appuser
var serviceAccountPattern = regexp.MustCompile(`^[a-z0-9]+_[a-z0-9]+_[a-z0-9]+_appuser$`)

// Validation rules:
// 1. Lowercase only
// 2. Underscores as separators
// 3. Must end with '_appuser'
// 4. Environment must match directory
```

#### Implementation Notes
- Extract filename without extension
- Parse filename components: {dataproduct}_{integration}_{environment}_appuser
- Validate environment matches directory path
- Check against naming convention patterns

#### Test Cases
```
# ✅ Valid: serviceaccounts/prod/marketo_astro_prod_appuser.yaml
name: marketo_astro_prod_appuser

# ✅ Valid: serviceaccounts/dev/helloworld_workato_dev_appuser.yaml
name: helloworld_workato_dev_appuser

# ❌ Invalid: serviceaccounts/prod/test_astro_prod_appuser.yaml
name: different_name  # Doesn't match filename

# ❌ Invalid naming patterns
name: InvalidNamingPattern      # CamelCase not allowed
name: test-astro-prod-appuser   # Hyphens not allowed
name: test_astro_prod          # Missing _appuser suffix
name: test_astro_DEV_appuser   # Uppercase not allowed

# ❌ Environment mismatch
# File: serviceaccounts/prod/test_astro_dev_appuser.yaml  # prod != dev

# Error message example:
❌ NAMING_VALIDATION: Service account name doesn't match filename
💡 Resolution: Change 'name' field to match filename: 'test_astro_prod_appuser'
📖 Documentation: [link to service account naming conventions]
```

---

### 🔄 Ticket 7: Service Account Environment Restrictions - PARTIALLY DONE
**Story Points:** 1  
**Epic:** Service Account Validation Rules
**Status:** 🔄 **PARTIALLY DONE** - Framework exists, need validator implementation

#### Description
Validate that Astro service accounts are only created in preprod and prod environments, as per platform requirements.

#### Technical Requirements
- Detect Astro service accounts by naming pattern or role
- Validate they only exist in preprod/prod directories
- Allow other service account types in all environments
- Configurable environment restrictions

#### Files to Create/Modify
- `internal/rules/service_account_env_rule.go` (new)
- `internal/rules/service_account_env_rule_test.go` (new)
- Update configuration for environment restrictions

#### Implementation Details
```go
type ServiceAccountEnvRule struct {
    name                  string
    astroEnvironmentsOnly []string // preprod, prod
}

func (r *ServiceAccountEnvRule) isAstroServiceAccount(name, role string) bool
func (r *ServiceAccountEnvRule) getEnvironmentFromPath(path string) string
```

#### Acceptance Criteria
- [ ] Detects Astro service accounts (by name pattern or role)
- [ ] Allows Astro service accounts only in preprod/prod
- [ ] Allows non-Astro service accounts in all environments
- [ ] Clear error messages about environment restrictions
- [ ] Configurable environment restrictions

#### Configuration Requirements
```go
// Add to ServiceAccountRuleConfig
type EnvironmentRestrictionConfig struct {
    AstroEnvironmentsOnly    []string            `json:"astro_environments_only"`    // ["preprod", "prod"]
    IntegrationRestrictions  map[string][]string `json:"integration_restrictions"`   // astro: ["preprod", "prod"]
    AstroDetectionPatterns   []string            `json:"astro_detection_patterns"`   // ["*_astro_*", "*_ASTRO_*"]
}
```

#### Astro Service Account Detection
```go
// Patterns to identify Astro service accounts
var astroPatterns = []string{
    ".*_astro_.*",           // name contains "astro"
    ".*_ASTRO_.*",           // role contains "ASTRO"
    ".*ASTRO.*ROLE.*",       // role pattern
}

// Environment extraction from path and filename
func extractEnvironment(path, filename string) (string, error) {
    // From path: serviceaccounts/{env}/
    // From filename: {product}_{integration}_{env}_appuser
}
```

#### Implementation Notes
- Detect Astro service accounts by name pattern OR role pattern
- Extract environment from both directory path and filename
- Validate environment consistency
- Apply restrictions based on integration type

#### Test Cases
```yaml
# ✅ Valid: serviceaccounts/prod/marketo_astro_prod_appuser.yaml
name: marketo_astro_prod_appuser
role: MARKETO_ASTRO_ROLE
email: user@redhat.com

# ✅ Valid: serviceaccounts/preprod/test_astro_preprod_appuser.yaml
name: test_astro_preprod_appuser
role: TEST_ASTRO_ROLE

# ❌ Invalid: serviceaccounts/dev/test_astro_dev_appuser.yaml
name: test_astro_dev_appuser
role: TEST_ASTRO_ROLE

# ❌ Invalid: serviceaccounts/sandbox/marketo_astro_sandbox_appuser.yaml
name: marketo_astro_sandbox_appuser

# ✅ Valid: serviceaccounts/dev/test_workato_dev_appuser.yaml (non-Astro)
name: test_workato_dev_appuser
role: TEST_WORKATO_ROLE

# ✅ Valid: serviceaccounts/sandbox/hello_tableau_sandbox_appuser.yaml (non-Astro)
name: hello_tableau_sandbox_appuser

# Error message example:
❌ ENVIRONMENT_RESTRICTION: Astro service accounts only allowed in preprod/prod
💡 Resolution: Move to preprod/ or prod/ directory, or use different integration type
📖 Documentation: [link to Astro service account policy]
```

---

### ❌ Ticket 19: Environment Consistency Rule - NOT STARTED
**Story Points:** 1  
**Epic:** Service Account Validation Rules
**Status:** ❌ **NOT STARTED** - Needs full implementation

#### Description
Validate that the same data product has consistent core configuration across different environments while allowing environment-specific variations.

#### Technical Requirements
- Validate core fields are consistent across environments (name, kind, rover_group pattern)
- Ensure database naming patterns are consistent across environments
- Validate warehouse types are consistent (only sizes may differ)
- Allow environment-specific variations where appropriate

#### Files to Create/Modify
- `internal/rules/environment_consistency_rule.go` (new)
- `internal/rules/environment_consistency_rule_test.go` (new)
- Update rule registration

#### Implementation Details
```go
type EnvironmentConsistencyRule struct {
    name string
}

func (r *EnvironmentConsistencyRule) validateCoreFieldsConsistency(products map[string]*ProductConfig) error
func (r *EnvironmentConsistencyRule) validateDatabaseNamingConsistency(products map[string]*ProductConfig) error
func (r *EnvironmentConsistencyRule) validateWarehouseTypeConsistency(products map[string]*ProductConfig) error
```

#### Acceptance Criteria
- [ ] Validates core fields consistent across environments
- [ ] Ensures database naming patterns match across environments
- [ ] Allows warehouse size differences but consistent types
- [ ] Provides clear error messages for inconsistencies
- [ ] Handles partial environment configurations gracefully

#### Test Cases
```yaml
# ✅ Valid consistency across environments
# dev/product.yaml:
name: helloaggregate
kind: aggregated
rover_group: dataverse-aggregate-helloaggregate
data_product_db:
- database: helloaggregate_db

# prod/product.yaml:
name: helloaggregate  # ✅ Same
kind: aggregated     # ✅ Same
rover_group: dataverse-aggregate-helloaggregate  # ✅ Same
data_product_db:
- database: helloaggregate_db  # ✅ Same pattern

# ❌ Invalid: Inconsistent core fields
# prod/product.yaml:
name: different-name  # ❌ Should match dev
kind: source         # ❌ Should match dev
```

---

## Phase 3: Warehouse Size Validation (4 tickets)

### ✅ Ticket 8: Warehouse Size Change Detection - COMPLETED
**Story Points:** 1  
**Epic:** Warehouse Validation Rules
**Status:** ✅ **COMPLETED** - Fully implemented and working

#### Description
Implement detection of warehouse size changes in product.yaml files to identify when platform approval is needed.

**✅ IMPLEMENTATION STATUS:**
- **Completed:** Full warehouse change detection in `internal/rules/warehouse/analyzer.go`
- **Features Working:** 
  - YAML parsing for warehouse configurations
  - Size hierarchy mapping (`WarehouseSizes` in `types.go`)
  - Change detection (increases vs decreases)
  - Cross-branch comparison (old vs new content)
  - Handles new warehouses, deletions, modifications

#### Technical Requirements
- Parse warehouse configurations from product.yaml
- Compare old vs new warehouse sizes
- Classify changes as increases, decreases, or additions
- Handle multiple warehouse configurations per product

#### Files to Create/Modify
- `internal/rules/warehouse_change_detector.go` (new)
- `internal/rules/warehouse_change_detector_test.go` (new)
- `internal/rules/shared/warehouse_types.go` (new)

#### Implementation Details
```go
type WarehouseChangeDetector struct {
    sizeOrder map[string]int // XSMALL: 1, SMALL: 2, MEDIUM: 3, etc.
}

type WarehouseChange struct {
    Type         string // "user" or "service_account"
    OldSize      string
    NewSize      string
    ChangeType   string // "increase", "decrease", "addition", "removal"
}

func (d *WarehouseChangeDetector) DetectChanges(oldYAML, newYAML string) []WarehouseChange
```

#### Warehouse Size Ordering
```go
// Define size hierarchy
var WarehouseSizes = map[string]int{
    "XSMALL": 1,
    "SMALL":  2,
    "MEDIUM": 3,
    "LARGE":  4,
    "XLARGE": 5,
    "2XLARGE": 6,
    "3XLARGE": 7,
    "4XLARGE": 8,
}
```

#### Acceptance Criteria
- [ ] Detects warehouse size increases (SMALL → MEDIUM)
- [ ] Detects warehouse size decreases (LARGE → MEDIUM)
- [ ] Detects new warehouse additions
- [ ] Detects warehouse removals
- [ ] Handles multiple warehouses (user + service_account)
- [ ] Returns structured change information

#### Configuration Requirements
```go
// Warehouse detection already implemented, enhance with:
type WarehouseDetectionConfig struct {
    EnableChangeLogging     bool     `json:"enable_change_logging"`
    TrackWarehouseHistory   bool     `json:"track_warehouse_history"`
    MaxWarehouseSize        string   `json:"max_warehouse_size"`        // "X4LARGE"
    CostThresholdWarning    []string `json:"cost_threshold_warning"`    // Sizes that trigger cost warnings
}
```

#### Performance Optimization
- Existing implementation fetches file content efficiently
- Uses proper YAML parsing with error handling
- Handles cross-fork MRs correctly
- Caches parsed content within single evaluation

#### Monitoring and Logging
```go
// Add logging for warehouse changes
type WarehouseChangeLog struct {
    ProjectID     int
    MRIID         int
    FilePath      string
    WarehouseType string
    OldSize       string
    NewSize       string
    ChangeType    string // "increase", "decrease", "addition", "removal"
    Author        string
    Timestamp     time.Time
}
```

#### Test Cases
```yaml
# ✅ Valid detection scenarios
# Old version
warehouses:
- type: user
  size: SMALL
- type: service_account
  size: MEDIUM

# New version (increase detected)
warehouses:
- type: user
  size: LARGE        # INCREASE: SMALL → LARGE
- type: service_account
  size: MEDIUM        # NO CHANGE

# ✅ New warehouse addition
warehouses:
- type: user
  size: SMALL
- type: service_account  # NEW
  size: XSMALL

# ✅ Warehouse removal
warehouses:
- type: user
  size: SMALL
# service_account warehouse removed
```

---

### ✅ Ticket 9: Warehouse Size Increase Validation - COMPLETED
**Story Points:** 1  
**Epic:** Warehouse Validation Rules
**Status:** ✅ **COMPLETED** - Fully implemented and working

#### Description
Implement the warehouse rule that requires platform approval for size increases but auto-approves decreases.

**✅ IMPLEMENTATION STATUS:**
- **Completed:** Full warehouse rule implementation in `internal/rules/warehouse/rule.go`
- **Features Working:**
  - Auto-approves warehouse size decreases
  - Requires manual review for size increases
  - Handles new warehouse additions as increases
  - Detects non-warehouse changes and requires manual review
  - Integrated with GitLab API for file content fetching
  - Registered in rule registry as `warehouse_rule`

#### Technical Requirements
- Use warehouse change detection from Ticket 8
- Trigger manual review for any warehouse size increases
- Auto-approve warehouse size decreases
- Handle new warehouse additions as increases (require approval)

#### Files to Create/Modify
- `internal/rules/warehouse_rule.go` (new)
- `internal/rules/warehouse_rule_test.go` (new)
- Update rule registration

#### Implementation Details
```go
type WarehouseRule struct {
    name             string
    changeDetector   *WarehouseChangeDetector
}

func (r *WarehouseRule) Validate(ctx *shared.MRContext) []shared.LineValidationResult {
    // For each product.yaml change
    // Detect warehouse changes
    // Return ManualReview for increases
    // Return Approve for decreases/no changes
}
```

#### Acceptance Criteria
- [ ] Triggers manual review for warehouse size increases
- [ ] Auto-approves warehouse size decreases
- [ ] Auto-approves when no warehouse changes
- [ ] Treats new warehouse additions as increases (manual review)
- [ ] Clear error messages explaining why manual review is needed
- [ ] Handles multiple warehouse changes in single MR

#### Configuration Requirements
```go
// Warehouse rule already implemented, enhance with:
type WarehouseApprovalConfig struct {
    AutoApproveDecreases    bool     `json:"auto_approve_decreases"`    // true
    RequireApprovalIncreases bool    `json:"require_approval_increases"` // true
    RequireApprovalAdditions bool    `json:"require_approval_additions"` // true
    CostImpactThreshold     float64  `json:"cost_impact_threshold"`     // Cost increase % threshold
    ApprovalBypassUsers     []string `json:"approval_bypass_users"`     // Emergency bypass
}
```

#### Implementation Enhancement
- Existing rule works correctly for basic size validation
- Add cost impact calculation for better decision making
- Include detailed reasoning in approval messages
- Support for emergency bypass mechanisms

#### Cost Impact Calculation
```go
// Add cost calculation to existing warehouse rule
type WarehouseCostCalculator struct {
    sizeCosts map[string]float64  // XSMALL: 1.0, SMALL: 2.0, etc.
}

func (c *WarehouseCostCalculator) CalculateCostImpact(oldSize, newSize string) float64 {
    oldCost := c.sizeCosts[oldSize]
    newCost := c.sizeCosts[newSize]
    return ((newCost - oldCost) / oldCost) * 100  // Percentage increase
}
```

#### Test Cases
```yaml
# ❌ Manual Review: Size increase
# Old: SMALL → New: LARGE (200% cost increase)

# ✅ Auto-approve: Size decrease  
# Old: LARGE → New: MEDIUM (cost decrease)

# ❌ Manual Review: New warehouse addition
# Old: (none) → New: SMALL (new cost)

# ✅ Auto-approve: No changes
# Old: MEDIUM → New: MEDIUM (no cost impact)

# Error message example:
❌ WAREHOUSE_VALIDATION: Warehouse size increase detected
💡 Resolution: Size increase from SMALL to LARGE requires platform team approval (200% cost increase)
📖 Documentation: [link to warehouse approval process]
```

---

### ❌ Ticket 10: Warehouse Size Environment Logic - NOT STARTED
**Story Points:** 1  
**Epic:** Warehouse Validation Rules
**Status:** ❌ **NOT STARTED** - Needs implementation on top of existing warehouse rule

#### Description
Apply different warehouse validation rules based on environment (dev/sandbox vs preprod/prod) with configurable policies.

#### Technical Requirements
- Extract environment from file path
- Apply different validation rules per environment
- Configurable environment-specific policies
- More lenient rules for dev/sandbox environments

#### Files to Create/Modify
- Update `internal/rules/warehouse_rule.go`
- Update `internal/config/config.go` for environment-specific config
- Add tests for environment-specific logic

#### Implementation Details
```go
type WarehouseRuleConfig struct {
    AllowTOCBypass       bool
    PlatformEnvironments []string // preprod, prod
    AutoApproveEnvs      []string // dev, sandbox
}

func (r *WarehouseRule) getEnvironmentPolicy(env string) EnvironmentPolicy
```

#### Configuration Addition
```go
// Add to config.go WarehouseRuleConfig
PlatformEnvironments []string // Environments requiring platform approval
AutoApproveEnvs      []string // Environments allowing auto-approval
```

#### Acceptance Criteria
- [ ] Different validation rules for dev/sandbox vs preprod/prod
- [ ] Configurable environment-specific policies
- [ ] More lenient rules for development environments
- [ ] Stricter rules for production environments
- [ ] Clear documentation of environment-specific behavior

#### Configuration Requirements
```go
// Add environment-specific warehouse policies
type EnvironmentWarehousePolicies struct {
    PlatformEnvironments     []string            `json:"platform_environments"`     // ["preprod", "prod"]
    AutoApproveEnvironments  []string            `json:"auto_approve_environments"`  // ["dev", "sandbox"]
    MaxSizeByEnvironment     map[string]string   `json:"max_size_by_environment"`   // dev: "MEDIUM", prod: "4XLARGE"
    StrictModeEnvironments   []string            `json:"strict_mode_environments"`   // ["prod"]
}
```

#### Environment-Specific Logic
```go
// Enhance existing warehouse rule with environment awareness
func (r *WarehouseRule) getEnvironmentPolicy(env string) EnvironmentPolicy {
    switch env {
    case "dev", "sandbox":
        return EnvironmentPolicy{
            MaxSize:           "MEDIUM",
            AutoApproveIncrease: true,  // Up to max size
            RequiresPlatformApproval: false,
        }
    case "preprod":
        return EnvironmentPolicy{
            MaxSize:           "LARGE",
            AutoApproveIncrease: false,
            RequiresPlatformApproval: true,
        }
    case "prod":
        return EnvironmentPolicy{
            MaxSize:           "4XLARGE",
            AutoApproveIncrease: false,
            RequiresPlatformApproval: true,
            StrictValidation: true,
        }
    }
}
```

#### Implementation Notes
- Use existing `GetEnvironmentFromPath()` function
- Apply different validation rules based on environment
- Support configurable policies per environment
- More restrictive rules for production environments

#### Test Cases
```yaml
# ✅ Dev/Sandbox: More lenient
# dataproducts/aggregate/test/dev/product.yaml
warehouses:
- type: user
  size: MEDIUM  # ✅ Auto-approved increase up to MEDIUM

# ❌ Dev: Over limit
warehouses:
- type: user
  size: LARGE   # ❌ Exceeds dev max size (MEDIUM)

# ❌ Preprod/Prod: Strict rules
# dataproducts/aggregate/test/prod/product.yaml
warehouses:
- type: user
  size: LARGE   # ❌ Any increase requires platform approval

# Environment detection from path:
# dataproducts/aggregate/test/dev/ → dev environment
# dataproducts/aggregate/test/prod/ → prod environment

# Error message example:
❌ WAREHOUSE_ENVIRONMENT: Size exceeds environment limit
💡 Resolution: Dev environment max size is MEDIUM. Use smaller size or promote to preprod
📖 Documentation: [link to environment policies]
```

---

### ❌ Ticket 20: Resource Allocation Validation Rule - NOT STARTED
**Story Points:** 1  
**Epic:** Warehouse Validation Rules
**Status:** ❌ **NOT STARTED** - Needs full implementation

#### Description
Validate that warehouse sizes and resource allocations are reasonable for the environment and follow best practices.

#### Technical Requirements
- Validate warehouse sizes are appropriate for environment type
- Flag unusual resource allocation patterns
- Prevent over-provisioning in non-production environments
- Configurable resource allocation policies per environment

#### Files to Create/Modify
- `internal/rules/resource_allocation_rule.go` (new)
- `internal/rules/resource_allocation_rule_test.go` (new)
- Update configuration for resource policies

#### Implementation Details
```go
type ResourceAllocationRule struct {
    name                    string
    maxSizesByEnvironment   map[string]string // dev: "MEDIUM", prod: "4XLARGE"
    recommendedSizes        map[string]string // recommendations per env
}

func (r *ResourceAllocationRule) validateSizeForEnvironment(size, env string) error
func (r *ResourceAllocationRule) flagUnusualPatterns(oldSize, newSize, env string) []string
```

#### Configuration Addition
```go
// Add to WarehouseRuleConfig
MaxSizesByEnvironment   map[string]string // Maximum allowed sizes per environment
RecommendedSizes        map[string]string // Recommended sizes per environment
```

#### Acceptance Criteria
- [ ] Validates warehouse sizes are reasonable for environment
- [ ] Flags unusual resource allocation patterns (e.g., jumping multiple sizes)
- [ ] Prevents over-provisioning in dev/sandbox environments
- [ ] Provides recommendations for appropriate sizing
- [ ] Configurable resource policies per environment

#### Test Cases
```yaml
# ✅ Valid: Appropriate sizes for environment
# dev environment:
warehouses:
- type: user
  size: SMALL          # ✅ Reasonable for dev

# prod environment:
warehouses:
- type: user
  size: LARGE          # ✅ Reasonable for prod

# ❌ Potentially problematic
# dev environment:
warehouses:
- type: user
  size: 4XLARGE        # ❌ Over-provisioned for dev

# Unusual pattern:
# Old: SMALL → New: 4XLARGE  # ❌ Jumped too many sizes
```

---

## Phase 4: Migration File Validation (4 tickets)

### ❌ Ticket 11: Migration File Path Validation - NOT STARTED
**Story Points:** 1  
**Epic:** Migration Validation Rules
**Status:** ❌ **NOT STARTED** - Helper function exists, need rule implementation

#### Description
Validate that migration files are in the correct subdirectories and follow proper naming conventions.

#### Technical Requirements
- Validate migration files are in `migrations/platform/` or `migrations/product/` subdirectories
- Validate naming pattern: `V{number}__{description}.sql`
- Ensure sequential version numbering
- Detect and flag incorrect migration paths

#### Files to Create/Modify
- `internal/rules/migration_path_rule.go` (new)
- `internal/rules/migration_path_rule_test.go` (new)
- Update rule registration

#### Implementation Details
```go
type MigrationPathRule struct {
    name string
}

func (r *MigrationPathRule) validateMigrationPath(path string) error
func (r *MigrationPathRule) validateNamingPattern(filename string) error
func (r *MigrationPathRule) validateVersionSequence(files []string) error
```

#### Accepted Migration Paths
```
✅ Valid:
- dataproducts/aggregate/test/prod/migrations/platform/V1__setup.sql
- dataproducts/aggregate/test/prod/migrations/product/V2__grants.sql

❌ Invalid:
- dataproducts/aggregate/test/prod/migrations/V1__setup.sql (missing subdir)
- dataproducts/aggregate/test/prod/migrations/platform/setup.sql (wrong naming)
- dataproducts/aggregate/test/prod/migrations/invalid/V1__setup.sql (wrong subdir)
```

#### Acceptance Criteria
- [ ] Validates migrations are in platform/ or product/ subdirectories
- [ ] Validates V{number}__{description}.sql naming pattern
- [ ] Detects version numbering gaps or duplicates
- [ ] Clear error messages for path and naming violations
- [ ] Only validates .sql files in migrations directories

#### Configuration Requirements
```go
// Add to MigrationsRuleConfig
type MigrationPathValidationConfig struct {
    RequiredSubdirectories   []string `json:"required_subdirectories"`   // ["platform", "product"]
    VersionPattern          string   `json:"version_pattern"`           // "V\\d+__.*\\.sql"
    MaxVersionGap           int      `json:"max_version_gap"`           // 5 (warn if version jumps > 5)
    AllowVersionSkips       bool     `json:"allow_version_skips"`       // false
    ValidateSequentialOrder bool     `json:"validate_sequential_order"`  // true
}
```

#### Migration Path Validation Logic
```go
// Migration file pattern validation
var migrationPatterns = struct {
    ValidPath     *regexp.Regexp
    ValidNaming   *regexp.Regexp
    VersionExtract *regexp.Regexp
}{
    ValidPath:     regexp.MustCompile(`.*/(migrations)/(platform|product)/.*\.sql$`),
    ValidNaming:   regexp.MustCompile(`^V\d+__.+\.sql$`),
    VersionExtract: regexp.MustCompile(`^V(\d+)__.*$`),
}

// Validation functions
func validateMigrationPath(path string) error
func validateMigrationNaming(filename string) error
func validateVersionSequence(files []string) error
```

#### Implementation Notes
- Use existing `IsMigrationFile()` helper function
- Validate against both platform/ and product/ subdirectories
- Check version numbering consistency
- Support environment-specific migration rules

#### Test Cases
```
✅ Valid paths:
- dataproducts/aggregate/test/prod/migrations/platform/V1__setup.sql
- dataproducts/source/marketo/dev/migrations/product/V2__grants.sql
- dataproducts/platform/admin/prod/migrations/platform/V10__permissions.sql

❌ Invalid paths:
- dataproducts/aggregate/test/migrations/V1__setup.sql (missing subdir)
- dataproducts/aggregate/test/prod/migrations/invalid/V1__setup.sql (wrong subdir)
- dataproducts/aggregate/test/prod/scripts/V1__setup.sql (not in migrations/)

✅ Valid naming patterns:
- V1__initial_setup.sql
- V2__add_permissions.sql
- V10__major_update.sql
- V100__large_migration.sql

❌ Invalid naming patterns:
- setup.sql (no version)
- V1_setup.sql (single underscore)
- v1__setup.sql (lowercase v)
- V1__setup.txt (wrong extension)
- 1__setup.sql (missing V prefix)
- V01__setup.sql (leading zero)

# Error message example:
❌ MIGRATION_PATH: Invalid migration file path
💡 Resolution: Move to migrations/platform/ or migrations/product/ subdirectory
📖 Documentation: [link to migration standards]
```

---

### ❌ Ticket 12: Platform Migration Approval Rule - NOT STARTED
**Story Points:** 1  
**Epic:** Migration Validation Rules
**Status:** ❌ **NOT STARTED** - Needs full implementation

#### Description
Implement rule that requires platform approval for platform migrations while auto-approving product migrations.

#### Technical Requirements
- Detect migration files in `migrations/platform/` directories
- Trigger manual review for platform migrations (ACCOUNTADMIN privileges)
- Auto-approve migrations in `migrations/product/` directories
- Clear reasoning in validation messages

#### Files to Create/Modify
- `internal/rules/platform_migration_rule.go` (new)
- `internal/rules/platform_migration_rule_test.go` (new)
- Update rule registration

#### Implementation Details
```go
type PlatformMigrationRule struct {
    name string
}

func (r *PlatformMigrationRule) Validate(ctx *shared.MRContext) []shared.LineValidationResult {
    // For each .sql file in migrations directories
    // Check if in platform/ subdirectory
    // Return ManualReview for platform migrations
    // Return Approve for product migrations
}
```

#### Approval Logic
```
Platform migrations (migrations/platform/):
→ Manual Review (requires platform team approval)
→ Reason: "ACCOUNTADMIN privileges required"

Product migrations (migrations/product/):
→ Auto-approve
→ Reason: "Self-service migration approved"
```

#### Acceptance Criteria
- [ ] Triggers manual review for migrations/platform/ files
- [ ] Auto-approves migrations/product/ files
- [ ] Clear reasoning about ACCOUNTADMIN vs self-service
- [ ] Handles mixed platform/product migrations in single MR
- [ ] Only applies to .sql files

#### Configuration Requirements
```go
// Add to MigrationsRuleConfig
type PlatformMigrationConfig struct {
    RequirePlatformApproval   bool     `json:"require_platform_approval"`   // true
    AutoApproveProductMigrations bool  `json:"auto_approve_product_migrations"` // true
    PlatformApprovalUsers     []string `json:"platform_approval_users"`     // Users who can approve
    EmergencyBypassEnabled    bool     `json:"emergency_bypass_enabled"`    // For critical fixes
}
```

#### Platform vs Product Migration Logic
```go
// Migration approval decision logic
func (r *PlatformMigrationRule) determineMigrationApproval(filePath string) (DecisionType, string) {
    if strings.Contains(filePath, "/migrations/platform/") {
        return shared.ManualReview, "Platform migration requires manual approval (ACCOUNTADMIN privileges)"
    }
    
    if strings.Contains(filePath, "/migrations/product/") {
        return shared.Approve, "Product migration auto-approved (self-service)"
    }
    
    return shared.ManualReview, "Migration not in recognized subdirectory"
}
```

#### Security Considerations
```go
// Platform migrations run with elevated privileges
type MigrationSecurityLevel struct {
    Platform struct {
        Privileges   string   // "ACCOUNTADMIN"
        RequiresApproval bool // true
        AuditRequired   bool // true
    }
    Product struct {
        Privileges   string   // "DBT_SERVICE_ACCOUNT"
        RequiresApproval bool // false
        AuditRequired   bool // false
    }
}
```

#### Test Cases
```
❌ Manual Review (Platform migrations):
- dataproducts/aggregate/test/prod/migrations/platform/V1__create_database.sql
- dataproducts/source/marketo/prod/migrations/platform/V2__grant_admin_access.sql
- dataproducts/platform/admin/prod/migrations/platform/V1__system_setup.sql

✅ Auto-approve (Product migrations):
- dataproducts/aggregate/test/dev/migrations/product/V1__create_tables.sql
- dataproducts/source/marketo/prod/migrations/product/V2__stored_procedures.sql
- dataproducts/aggregate/analytics/prod/migrations/product/V3__update_views.sql

Mixed MR scenarios:
- ✅ migrations/product/V1__setup.sql
- ❌ migrations/platform/V2__admin_grants.sql
→ Overall: Manual Review (platform migration present)

# Error message example:
❌ PLATFORM_MIGRATION: Platform migration requires manual approval
💡 Resolution: Platform migrations run with ACCOUNTADMIN privileges and need platform team review
📖 Documentation: [link to migration approval process]
```

---

### ❌ Ticket 13: SQL Content Basic Security Validation - NOT STARTED
**Story Points:** 1  
**Epic:** Migration Validation Rules
**Status:** ❌ **NOT STARTED** - Needs full implementation

#### Description
Implement basic security checks for SQL migration content to detect potentially dangerous operations.

#### Technical Requirements
- Scan SQL content for dangerous patterns
- Flag operations like DROP DATABASE, GRANT ALL, etc.
- Configurable list of dangerous SQL patterns
- Provide specific warnings for detected issues

#### Files to Create/Modify
- `internal/rules/sql_security_rule.go` (new)
- `internal/rules/sql_security_rule_test.go` (new)
- Update configuration for dangerous SQL patterns

#### Implementation Details
```go
type SQLSecurityRule struct {
    name               string
    dangerousPatterns  []string
}

func (r *SQLSecurityRule) scanSQLContent(content string) []SecurityIssue
func (r *SQLSecurityRule) isDangerousPattern(statement string) bool
```

#### Dangerous SQL Patterns
```go
var DangerousPatterns = []string{
    "DROP DATABASE",
    "DROP SCHEMA",
    "GRANT ALL",
    "GRANT.*TO.*WITH GRANT OPTION",
    "CREATE USER",
    "ALTER USER.*SET PASSWORD",
    "SHOW GRANTS",
    // Add more patterns as needed
}
```

#### Acceptance Criteria
- [ ] Detects dangerous SQL operations (DROP DATABASE, GRANT ALL, etc.)
- [ ] Configurable list of dangerous patterns
- [ ] Provides specific warnings for detected issues
- [ ] Works with both platform and product migrations
- [ ] Doesn't block legitimate operations with false positives

#### Configuration Requirements
```go
// Add to MigrationsRuleConfig
type SQLSecurityConfig struct {
    EnableSecurityScanning    bool     `json:"enable_security_scanning"`
    DangerousPatterns        []string `json:"dangerous_patterns"`
    BlockedOperations        []string `json:"blocked_operations"`
    AllowProductMigrations   bool     `json:"allow_product_migrations"`  // Less strict for product migrations
    SecurityScanTimeout      int      `json:"security_scan_timeout"`     // Seconds
}
```

#### Dangerous SQL Pattern Detection
```go
// Comprehensive dangerous SQL patterns
var dangerousPatterns = []struct {
    Pattern     string
    Severity    string
    Description string
}{
    {`DROP\s+DATABASE`, "HIGH", "Database deletion"},
    {`DROP\s+SCHEMA`, "HIGH", "Schema deletion"},
    {`GRANT\s+ALL`, "HIGH", "Excessive privileges"},
    {`GRANT.*WITH\s+GRANT\s+OPTION`, "HIGH", "Grant propagation"},
    {`CREATE\s+USER`, "MEDIUM", "User creation"},
    {`ALTER\s+USER.*PASSWORD`, "MEDIUM", "Password modification"},
    {`TRUNCATE\s+TABLE`, "MEDIUM", "Data deletion"},
    {`DELETE\s+FROM.*WHERE\s+1=1`, "HIGH", "Mass deletion"},
    {`UPDATE.*SET.*WHERE\s+1=1`, "HIGH", "Mass update"},
    {`--\s*password`, "LOW", "Password in comments"},
}

// SQL content scanning function
func scanSQLContent(content string) []SecurityIssue
```

#### Implementation Notes
- Scan all .sql files in migration directories
- Different security levels for platform vs product migrations
- Support configurable pattern lists
- Include line numbers in security findings

#### Test Cases
```sql
-- ❌ HIGH SEVERITY: Flagged as dangerous
DROP DATABASE sensitive_db;                    -- Database deletion
GRANT ALL ON *.* TO 'user'@'%';               -- Excessive privileges  
CREATE USER 'newuser'@'%' IDENTIFIED BY 'password'; -- User creation
DELETE FROM users WHERE 1=1;                  -- Mass deletion
GRANT SELECT ON db.* TO role WITH GRANT OPTION; -- Grant propagation

-- ❌ MEDIUM SEVERITY: Requires attention
TRUNCATE TABLE temp_data;                      -- Data deletion
ALTER USER existing_user SET PASSWORD = 'new'; -- Password change

-- ❌ LOW SEVERITY: Security concern
-- Default password is 'admin123'              -- Password in comments

-- ✅ SAFE: Approved operations
CREATE TABLE analytics_data (id INT, name VARCHAR(100));
GRANT SELECT ON database.specific_table TO specific_role;
INSERT INTO products VALUES (1, 'Product A');
UPDATE users SET last_login = NOW() WHERE user_id = 123;
CREATE VIEW user_summary AS SELECT id, name FROM users;

# Error message example:
❌ SQL_SECURITY: Dangerous SQL operation detected
💡 Resolution: HIGH SEVERITY - Line 5: 'DROP DATABASE' is not allowed in migrations
📖 Documentation: [link to SQL security guidelines]
```

---

### ❌ Ticket 21: Secrets Detection Rule - NOT STARTED
**Story Points:** 1  
**Epic:** Migration Validation Rules
**Status:** ❌ **NOT STARTED** - Needs full implementation

#### Description
Implement security scanning to detect and prevent secrets, credentials, and sensitive data in configuration and migration files.

#### Technical Requirements
- Scan YAML and SQL content for credential patterns
- Detect passwords, API keys, tokens, and other secrets
- Flag suspicious patterns in comments and descriptions
- Configurable secret detection patterns

#### Files to Create/Modify
- `internal/rules/secrets_detection_rule.go` (new)
- `internal/rules/secrets_detection_rule_test.go` (new)
- Update configuration for secret patterns

#### Implementation Details
```go
type SecretsDetectionRule struct {
    name            string
    secretPatterns  []string
}

func (r *SecretsDetectionRule) scanForSecrets(content string) []SecurityIssue
func (r *SecretsDetectionRule) isSecretPattern(text string) bool
```

#### Secret Detection Patterns
```go
var SecretPatterns = []string{
    "password\s*[:=]\s*[\"']?[^\s\"']+",
    "api[_-]?key\s*[:=]\s*[\"']?[^\s\"']+",
    "token\s*[:=]\s*[\"']?[^\s\"']+",
    "secret\s*[:=]\s*[\"']?[^\s\"']+",
    "IDENTIFIED BY\s+[\"']?[^\s\"']+",
    // Add more patterns as needed
}
```

#### Acceptance Criteria
- [ ] Detects common credential patterns in YAML files
- [ ] Scans SQL content for hardcoded passwords
- [ ] Flags suspicious patterns in comments
- [ ] Configurable secret detection patterns
- [ ] Provides guidance on secure credential handling

#### Test Cases
```yaml
# ❌ Flagged as containing secrets
email: user@redhat.com
password: "secretpassword123"  # ❌ Hardcoded password
api_key: "abc123xyz"           # ❌ API key

# SQL content:
CREATE USER 'test' IDENTIFIED BY 'password123';  # ❌ Hardcoded password

# ✅ Safe configurations
email: user@redhat.com
role: TEST_ROLE                # ✅ No sensitive data
```

---

## Phase 5: TOC Approval and Environment Rules (3 tickets)

### ❌ Ticket 14: TOC Approval for Preprod/Prod Promotion - NOT STARTED
**Story Points:** 1  
**Epic:** Approval Workflow Rules
**Status:** ❌ **NOT STARTED** - Needs full implementation

#### Description
Detect new data product promotions to preprod/prod environments and trigger TOC approval workflow.

#### Technical Requirements
- Detect new product.yaml files in preprod/prod directories
- Identify first-time promotions vs updates
- Trigger TOC approval for new promotions
- Auto-approve updates to existing products

#### Files to Create/Modify
- `internal/rules/toc_promotion_rule.go` (new)
- `internal/rules/toc_promotion_rule_test.go` (new)
- Update rule registration

#### Implementation Details
```go
type TOCPromotionRule struct {
    name              string
    tocEnvironments   []string // preprod, prod
}

func (r *TOCPromotionRule) isNewPromotion(ctx *shared.MRContext, filePath string) bool
func (r *TOCPromotionRule) getEnvironmentFromPath(path string) string
func (r *TOCPromotionRule) isProductYamlFile(path string) bool
```

#### Detection Logic
```
New file in preprod/prod + product.yaml:
→ Manual Review (TOC approval required)

Existing file modified in preprod/prod:
→ Auto-approve (update to existing product)

New/modified files in dev/sandbox:
→ Auto-approve (no TOC approval needed)
```

#### Acceptance Criteria
- [ ] Detects new product.yaml files in preprod/prod
- [ ] Triggers manual review for new promotions
- [ ] Auto-approves updates to existing products
- [ ] Auto-approves all dev/sandbox changes
- [ ] Clear messaging about TOC approval requirement

#### Configuration Requirements
```go
// Add to ApprovalConfig
type TOCPromotionConfig struct {
    TOCEnvironments          []string `json:"toc_environments"`          // ["preprod", "prod"]
    RequireTOCApproval       bool     `json:"require_toc_approval"`
    TOCBypassUsers           []string `json:"toc_bypass_users"`           // Emergency bypass
    NewPromotionDetection    bool     `json:"new_promotion_detection"`    // true
    TOCApprovalTimeoutHours  int      `json:"toc_approval_timeout_hours"` // 72
}
```

#### TOC Approval Detection Logic
```go
// Detect new data product promotions
func (r *TOCPromotionRule) isNewPromotion(ctx *shared.MRContext, filePath string) bool {
    // Check if this is a new file in preprod/prod
    if !r.isTOCEnvironment(filePath) {
        return false
    }
    
    // Check if file exists in target branch
    exists := r.fileExistsInTargetBranch(ctx, filePath)
    return !exists  // New file = new promotion
}

func (r *TOCPromotionRule) isTOCEnvironment(filePath string) bool {
    env := shared.GetEnvironmentFromPath(filePath)
    return contains(r.tocEnvironments, env)
}
```

#### Implementation Notes
- Integrate with existing GitLab API to check file existence
- Use MR context to determine if file is new vs modified
- Support TOC bypass for emergency situations
- Track promotion approvals for audit purposes

#### Test Cases
```
❌ Manual Review (TOC approval required):
- Added: dataproducts/aggregate/newproduct/prod/product.yaml      # New prod promotion
- Added: dataproducts/source/newsource/preprod/product.yaml       # New preprod promotion
- Added: dataproducts/platform/newplatform/prod/product.yaml      # New platform prod

✅ Auto-approve (no TOC needed):
- Modified: dataproducts/aggregate/existing/prod/product.yaml      # Update existing prod
- Modified: dataproducts/source/existing/preprod/product.yaml     # Update existing preprod
- Added: dataproducts/aggregate/newproduct/dev/product.yaml       # New dev (not TOC env)
- Added: dataproducts/aggregate/newproduct/sandbox/product.yaml   # New sandbox (not TOC env)
- Modified: dataproducts/aggregate/existing/dev/product.yaml      # Dev updates

# Detection scenarios:
# File exists in target branch + MR modifies it = Update (auto-approve)
# File doesn't exist in target branch + MR adds it = New promotion (TOC approval)

# Error message example:
❌ TOC_PROMOTION: New data product promotion requires TOC approval
💡 Resolution: This is a new data product promotion to prod environment - TOC team approval required
📖 Documentation: [link to TOC approval process]
```

---

### ❌ Ticket 15: Consumer Group Environment Restrictions - NOT STARTED
**Story Points:** 1  
**Epic:** Approval Workflow Rules
**Status:** ❌ **NOT STARTED** - Needs full implementation

#### Description
Validate that consumer groups are only defined in preprod and prod environments, not in dev/sandbox.

#### Technical Requirements
- Detect consumer.yaml files and consumer definitions
- Validate they only exist in preprod/prod directories
- Check for consumer configurations in product.yaml files
- Provide clear guidance about environment restrictions

#### Files to Create/Modify
- `internal/rules/consumer_env_rule.go` (new)
- `internal/rules/consumer_env_rule_test.go` (new)
- Update rule registration

#### Implementation Details
```go
type ConsumerEnvRule struct {
    name                    string
    allowedConsumerEnvs     []string // preprod, prod
}

func (r *ConsumerEnvRule) isConsumerFile(path string) bool
func (r *ConsumerEnvRule) hasConsumerConfig(yamlContent string) bool
func (r *ConsumerEnvRule) getEnvironmentFromPath(path string) string
```

#### Validation Scope
```
Check for consumers in:
1. consumers.yaml files
2. product.yaml consumer configurations
3. presentation_schemas[].consumers[] definitions
```

#### Acceptance Criteria
- [ ] Validates consumer.yaml files only in preprod/prod
- [ ] Validates consumer configs in product.yaml only in preprod/prod
- [ ] Rejects consumer definitions in dev/sandbox
- [ ] Clear error messages about environment restrictions
- [ ] Handles both direct consumer files and inline configs

#### Configuration Requirements
```go
// Add to ApprovalConfig
type ConsumerEnvironmentConfig struct {
    AllowedConsumerEnvironments   []string `json:"allowed_consumer_environments"`   // ["preprod", "prod"]
    BlockConsumerInDevEnvs        bool     `json:"block_consumer_in_dev_envs"`
    ValidateConsumerFiles         bool     `json:"validate_consumer_files"`
    ValidateInlineConsumers       bool     `json:"validate_inline_consumers"`      // In product.yaml
}
```

#### Consumer Detection Logic
```go
// Comprehensive consumer detection patterns
func (r *ConsumerEnvRule) detectConsumerConfigurations(yamlContent string) []ConsumerReference {
    // 1. Direct consumer.yaml files
    // 2. product.yaml consumers[] sections
    // 3. presentation_schemas[].consumers[] definitions
    // 4. data_product_db[].consumers[] configurations
}

// Consumer reference types
type ConsumerReference struct {
    Type        string // "file", "inline", "schema_consumer"
    Path        string
    LineNumber  int
    Environment string
    ConfigType  string // "consumers_yaml", "product_yaml", "schema_yaml"
}
```

#### Implementation Notes
- Use existing `GetEnvironmentFromPath()` function
- Scan YAML content for consumer-related sections
- Validate both standalone consumer files and inline configurations
- Support different consumer configuration patterns

#### Environment Validation Logic
```go
// Environment-specific consumer validation
func (r *ConsumerEnvRule) validateConsumerEnvironment(consumerRef ConsumerReference) ValidationResult {
    env := shared.GetEnvironmentFromPath(consumerRef.Path)
    
    if !contains(r.allowedConsumerEnvs, env) {
        return ValidationResult{
            Type:    "environment_restriction",
            Severity: "HIGH",
            Message: fmt.Sprintf("Consumer configuration not allowed in %s environment", env),
            Resolution: "Move consumer configuration to preprod or prod environment",
        }
    }
    
    return ValidationResult{Type: "valid"}
}
```

#### Test Cases
```yaml
# ✅ Valid: Consumer configurations in allowed environments
# dataproducts/aggregate/test/prod/consumers.yaml
consumers:
  - name: analytics_team
    type: user_group
    permissions: ["read"]

# dataproducts/aggregate/test/preprod/product.yaml
name: test_product
consumers:
  - name: test_service_account
    kind: service_account

# ❌ Invalid: Consumer configurations in restricted environments
# dataproducts/aggregate/test/dev/consumers.yaml
consumers:
  - name: dev_consumer  # ❌ Not allowed in dev

# dataproducts/aggregate/test/sandbox/product.yaml
name: test_product
consumers:
  - name: sandbox_consumer  # ❌ Not allowed in sandbox

# ❌ Invalid: Schema-level consumers in dev
# dataproducts/aggregate/test/dev/product.yaml
data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts
    consumers:  # ❌ Schema consumers not allowed in dev
    - name: dev_analytics

# Error message example:
❌ CONSUMER_ENVIRONMENT: Consumer configuration not allowed in dev environment
💡 Resolution: Consumer groups are only permitted in preprod and prod environments
📖 Documentation: [link to consumer environment policy]
```

---

### ❌ Ticket 22: Consumer Access Pattern Rule - NOT STARTED
**Story Points:** 1  
**Epic:** Approval Workflow Rules
**Status:** ❌ **NOT STARTED** - Needs full implementation

#### Description
Validate that consumer access patterns are logical, prevent circular dependencies, and ensure consumers exist in appropriate environments.

#### Technical Requirements
- Validate consumer access patterns make logical sense
- Check for circular dependencies between data products
- Ensure consumers exist in appropriate environments
- Validate data product to consumer compatibility

#### Files to Create/Modify
- `internal/rules/consumer_access_pattern_rule.go` (new)
- `internal/rules/consumer_access_pattern_rule_test.go` (new)
- Update rule registration

#### Implementation Details
```go
type ConsumerAccessPatternRule struct {
    name                    string
    allowedConsumerEnvs     []string
}

func (r *ConsumerAccessPatternRule) validateAccessPattern(producer, consumer, env string) error
func (r *ConsumerAccessPatternRule) detectCircularDependency(dependencies map[string][]string) error
func (r *ConsumerAccessPatternRule) validateConsumerEnvironment(consumer, env string) error
```

#### Validation Logic
```
Access Pattern Validation:
1. Source data products can be consumed by aggregate data products
2. Aggregate data products can be consumed by other aggregates
3. Platform data products have special access rules
4. Service accounts can consume appropriate data products

Circular Dependency Detection:
- Product A → Product B → Product A (circular)
- Build dependency graph and detect cycles
```

#### Acceptance Criteria
- [ ] Validates logical consumer access patterns
- [ ] Detects and prevents circular dependencies
- [ ] Ensures consumers exist in appropriate environments
- [ ] Validates data product type compatibility for consumption
- [ ] Provides clear error messages for access violations

#### Configuration Requirements
```go
// Add to ApprovalConfig
type ConsumerAccessPatternConfig struct {
    EnableCircularDependencyCheck  bool                `json:"enable_circular_dependency_check"`
    AllowedAccessPatterns          map[string][]string `json:"allowed_access_patterns"`        // source: ["aggregate"], aggregate: ["aggregate"]
    CrossEnvironmentAccess         bool                `json:"cross_environment_access"`       // Allow prod → dev access
    MaxDependencyDepth             int                 `json:"max_dependency_depth"`           // 5
    ValidateConsumerExistence      bool                `json:"validate_consumer_existence"`
}
```

#### Access Pattern Matrix
```go
// Define allowed consumption patterns
var accessPatternMatrix = map[string][]string{
    "source": {"aggregate", "platform"},                    // Source can be consumed by aggregate/platform
    "aggregate": {"aggregate", "platform", "analytics"},    // Aggregate can be consumed by other aggregates
    "platform": {"aggregate", "platform", "source"},       // Platform has broad access
}

// Service account access patterns
var serviceAccountAccessPatterns = map[string][]string{
    "astro": {"source", "aggregate"},          // Astro can consume source/aggregate
    "workato": {"source"},                     // Workato only source
    "tableau": {"aggregate", "platform"},      // Tableau analytical access
}
```

#### Circular Dependency Detection
```go
// Graph-based circular dependency detection
type DependencyGraph struct {
    nodes map[string]*DependencyNode
    edges map[string][]string  // producer -> []consumers
}

type DependencyNode struct {
    Name        string
    Type        string // "source", "aggregate", "platform"
    Environment string
    Consumers   []string
}

func (r *ConsumerAccessPatternRule) buildDependencyGraph(mrCtx *shared.MRContext) *DependencyGraph
func (g *DependencyGraph) detectCycles() [][]string  // Returns all circular paths found
func (g *DependencyGraph) validateDepth() error      // Check max dependency depth
```

#### Implementation Notes
- Build complete dependency graph from all changed files
- Use depth-first search for cycle detection
- Validate access patterns against allowed matrix
- Check environment consistency for cross-references

#### Test Cases
```yaml
# ✅ Valid access patterns
# Source → Aggregate consumption
# dataproducts/source/marketo/prod/product.yaml references:
# dataproducts/aggregate/marketing/prod as consumer ✅

# Aggregate → Aggregate consumption
# dataproducts/aggregate/sales/prod/product.yaml references:
# dataproducts/aggregate/reporting/prod as consumer ✅

# Service account consumption
# serviceaccounts/prod/marketo_astro_prod_appuser.yaml references:
# dataproducts/source/marketo/prod ✅ (Astro can consume source)

# ❌ Invalid patterns
# Circular dependency chain
# Product A → Product B → Product C → Product A ❌
products:
  aggregate/a/prod:
    consumers: ["aggregate/b/prod"]
  aggregate/b/prod:
    consumers: ["aggregate/c/prod"]
  aggregate/c/prod:
    consumers: ["aggregate/a/prod"]  # ❌ Creates cycle

# Invalid access pattern
# dataproducts/aggregate/marketing/prod references:
# dataproducts/source/crm/prod as consumer ❌ (aggregate cannot consume source)

# Environment mismatch
# dataproducts/aggregate/test/dev/product.yaml references:
# serviceaccounts/prod/test_astro_prod_appuser ❌ (dev → prod reference)

# Excessive dependency depth
# A → B → C → D → E → F ❌ (exceeds max depth of 5)

# Error message examples:
❌ CIRCULAR_DEPENDENCY: Circular dependency detected
💡 Resolution: Remove circular reference: aggregate/a/prod → aggregate/b/prod → aggregate/a/prod
📖 Documentation: [link to dependency management]

❌ ACCESS_PATTERN: Invalid consumer access pattern
💡 Resolution: Aggregate data products cannot consume source data products directly
📖 Documentation: [link to access pattern rules]

❌ ENVIRONMENT_MISMATCH: Cross-environment consumer reference
💡 Resolution: Dev environment cannot reference prod consumers
📖 Documentation: [link to environment isolation]
```

---

## Phase 6: Masking Policy Validation (2 tickets)

### ❌ Ticket 16: Masking Policy Datatype Consistency - NOT STARTED
**Story Points:** 1  
**Epic:** Masking Policy Validation Rules
**Status:** ❌ **NOT STARTED** - Needs full implementation

#### Description
Validate that masking policy mask values are consistent with their specified datatypes.

#### Technical Requirements
- Parse masking_policies.yaml files
- Validate mask value matches datatype (string/float/etc.)
- Check for proper casting syntax for non-string types
- Handle different masking value formats

#### Files to Create/Modify
- `internal/rules/masking_datatype_rule.go` (new)
- `internal/rules/masking_datatype_rule_test.go` (new)
- Update rule registration

#### Implementation Details
```go
type MaskingDatatypeRule struct {
    name string
}

func (r *MaskingDatatypeRule) validateMaskValue(datatype, mask string) error
func (r *MaskingDatatypeRule) isValidStringMask(mask string) bool
func (r *MaskingDatatypeRule) isValidFloatMask(mask string) bool
```

#### Validation Rules
```yaml
# String datatype - mask should be string
datatype: string
mask: "==MASKED=="  # ✅ Valid

# Float datatype - mask should be float or cast expression
datatype: float
mask: "cast(0.0 as float)"  # ✅ Valid
mask: "0.0"                 # ✅ Valid
mask: "==MASKED=="          # ❌ Invalid (string for float)
```

#### Acceptance Criteria
- [ ] Validates string masks for string datatypes
- [ ] Validates float masks for float datatypes
- [ ] Accepts proper cast expressions for numeric types
- [ ] Rejects mismatched datatype/mask combinations
- [ ] Clear error messages with examples of correct format

#### Configuration Requirements
```go
// Add to MaskingPolicyConfig
type MaskingDatatypeConfig struct {
    EnableDatatypeValidation    bool              `json:"enable_datatype_validation"`
    SupportedDatatypes         []string          `json:"supported_datatypes"`         // ["string", "float", "int", "boolean"]
    MaskFormatPatterns         map[string]string `json:"mask_format_patterns"`        // datatype -> regex pattern
    AllowCastExpressions       bool              `json:"allow_cast_expressions"`
    ValidateMaskSyntax         bool              `json:"validate_mask_syntax"`
}
```

#### Datatype Validation Matrix
```go
// Comprehensive datatype validation patterns
var datatypeMaskPatterns = map[string]struct{
    AllowedFormats []string
    Examples       []string
    CastPattern    string
}{
    "string": {
        AllowedFormats: []string{`^".*"$`, `^'.*'$`},           // Quoted strings
        Examples:       []string{`"==MASKED=="`, `'***'`},
        CastPattern:    `^".*"$`,
    },
    "float": {
        AllowedFormats: []string{`^\d+\.\d+$`, `^cast\(.*as float\)$`},  // Numeric or cast
        Examples:       []string{`0.0`, `cast(0.0 as float)`},
        CastPattern:    `^cast\([\d\.]+\s+as\s+float\)$`,
    },
    "int": {
        AllowedFormats: []string{`^\d+$`, `^cast\(.*as int\)$`},        // Integer or cast
        Examples:       []string{`0`, `-1`, `cast(0 as int)`},
        CastPattern:    `^cast\(\d+\s+as\s+int\)$`,
    },
    "boolean": {
        AllowedFormats: []string{`^(true|false)$`, `^cast\(.*as boolean\)$`},
        Examples:       []string{`false`, `cast(false as boolean)`},
        CastPattern:    `^cast\((true|false)\s+as\s+boolean\)$`,
    },
}
```

#### Implementation Notes
- Parse masking_policies.yaml files using existing YAML parsing utilities
- Validate each masking policy entry against datatype requirements
- Support both simple values and cast expressions
- Provide specific error messages with correct format examples

#### Mask Value Validation Logic
```go
// Comprehensive mask value validation
func (r *MaskingDatatypeRule) validateMaskValue(datatype, mask string) ValidationResult {
    patterns, exists := datatypeMaskPatterns[datatype]
    if !exists {
        return ValidationResult{
            Type: "unsupported_datatype",
            Message: fmt.Sprintf("Unsupported datatype: %s", datatype),
        }
    }
    
    // Check against allowed formats
    for _, pattern := range patterns.AllowedFormats {
        if matched, _ := regexp.MatchString(pattern, mask); matched {
            return ValidationResult{Type: "valid"}
        }
    }
    
    return ValidationResult{
        Type: "invalid_mask_format",
        Message: fmt.Sprintf("Invalid mask format for %s datatype", datatype),
        Examples: patterns.Examples,
    }
}
```

#### Test Cases
```yaml
# ✅ Valid combinations
# String datatype with string mask
datatype: string
mask: "==MASKED=="          # ✅ Quoted string

# Float datatype with numeric mask
datatype: float
mask: "0.0"                 # ✅ Numeric value

# Float datatype with cast expression
datatype: float
mask: "cast(0.0 as float)"  # ✅ Cast expression

# Integer datatype
datatype: int
mask: "0"                   # ✅ Integer value
mask: "cast(-1 as int)"     # ✅ Cast expression

# Boolean datatype
datatype: boolean
mask: "false"               # ✅ Boolean value
mask: "cast(false as boolean)"  # ✅ Cast expression

# ❌ Invalid combinations
# String datatype with numeric mask
datatype: string
mask: 0.0                   # ❌ Should be quoted string

# Float datatype with string mask
datatype: float
mask: "==MASKED=="          # ❌ Should be numeric or cast

# Invalid cast expressions
datatype: float
mask: "cast(invalid as float)"  # ❌ Invalid cast syntax

# Type mismatch in cast
datatype: int
mask: "cast(0.0 as float)"  # ❌ Float cast for int datatype

# Error message examples:
❌ DATATYPE_MISMATCH: Invalid mask format for float datatype
💡 Resolution: Use numeric value like '0.0' or cast expression like 'cast(0.0 as float)'
📖 Documentation: [link to masking policy format guide]

❌ UNSUPPORTED_DATATYPE: Unsupported datatype 'timestamp'
💡 Resolution: Use supported datatypes: string, float, int, boolean
📖 Documentation: [link to supported datatypes]
```

datatype: float
mask: "text_mask"
```

---

### ❌ Ticket 17: Masking Strategy Validation - NOT STARTED
**Story Points:** 1  
**Epic:** Masking Policy Validation Rules
**Status:** ❌ **NOT STARTED** - Needs full implementation

#### Description
Validate that masking strategies are supported for their specified datatypes (e.g., HASH_SHA1 only for strings).

#### Technical Requirements
- Parse masking strategy definitions
- Validate strategy compatibility with datatypes
- Check supported strategy combinations
- Provide guidance on valid strategy/datatype pairs

#### Files to Create/Modify
- `internal/rules/masking_strategy_rule.go` (new)
- `internal/rules/masking_strategy_rule_test.go` (new)
- Update rule registration

#### Implementation Details
```go
type MaskingStrategyRule struct {
    name                     string
    strategyDatatypeMap      map[string][]string
}

func (r *MaskingStrategyRule) validateStrategy(strategy, datatype string) error
func (r *MaskingStrategyRule) getSupportedStrategies(datatype string) []string
```

#### Strategy Compatibility Matrix
```go
var StrategyDatatypeMap = map[string][]string{
    "UNMASKED":  {"string", "float", "int", "boolean"},
    "HASH_SHA1": {"string"},  // Only for strings
    "ENCRYPT":   {"string"},
    "NULLIFY":   {"string", "float", "int"},
}
```

#### Acceptance Criteria
- [ ] Validates HASH_SHA1 only used with string datatype
- [ ] Validates other strategies against compatible datatypes
- [ ] Provides list of supported strategies for each datatype
- [ ] Clear error messages with compatibility information
- [ ] Configurable strategy/datatype compatibility matrix

#### Configuration Requirements
```go
// Add to MaskingPolicyConfig
type MaskingStrategyConfig struct {
    EnableStrategyValidation    bool                        `json:"enable_strategy_validation"`
    StrategyDatatypeMatrix      map[string][]string         `json:"strategy_datatype_matrix"`
    CustomStrategies           map[string][]string         `json:"custom_strategies"`           // User-defined strategies
    StrictStrategyValidation   bool                        `json:"strict_strategy_validation"`
    DeprecatedStrategies       []string                    `json:"deprecated_strategies"`
}
```

#### Comprehensive Strategy Matrix
```go
// Extended strategy compatibility matrix
var strategyDatatypeMatrix = map[string]struct{
    SupportedDatatypes  []string
    Description         string
    SecurityLevel       string
    PerformanceImpact   string
}{
    "UNMASKED": {
        SupportedDatatypes: []string{"string", "float", "int", "boolean", "timestamp"},
        Description:        "No masking applied - data visible as-is",
        SecurityLevel:      "LOW",
        PerformanceImpact:  "NONE",
    },
    "HASH_SHA1": {
        SupportedDatatypes: []string{"string"},
        Description:        "SHA-1 hash of string values",
        SecurityLevel:      "MEDIUM",
        PerformanceImpact:  "LOW",
    },
    "HASH_SHA256": {
        SupportedDatatypes: []string{"string"},
        Description:        "SHA-256 hash of string values",
        SecurityLevel:      "HIGH",
        PerformanceImpact:  "LOW",
    },
    "ENCRYPT": {
        SupportedDatatypes: []string{"string"},
        Description:        "AES encryption of string values",
        SecurityLevel:      "HIGH",
        PerformanceImpact:  "MEDIUM",
    },
    "NULLIFY": {
        SupportedDatatypes: []string{"string", "float", "int", "boolean"},
        Description:        "Replace with NULL values",
        SecurityLevel:      "HIGH",
        PerformanceImpact:  "NONE",
    },
    "RANDOM_NUMERIC": {
        SupportedDatatypes: []string{"int", "float"},
        Description:        "Random numeric values within range",
        SecurityLevel:      "MEDIUM",
        PerformanceImpact:  "LOW",
    },
    "PARTIAL_MASK": {
        SupportedDatatypes: []string{"string"},
        Description:        "Show partial data (e.g., first 2 chars)",
        SecurityLevel:      "MEDIUM",
        PerformanceImpact:  "LOW",
    },
}
```

#### Implementation Notes
- Parse masking policies YAML files using existing utilities
- Validate each strategy against supported datatypes
- Provide detailed compatibility information
- Support custom strategy definitions
- Warn about deprecated strategies

#### Strategy Validation Logic
```go
// Enhanced strategy validation with detailed feedback
func (r *MaskingStrategyRule) validateStrategyCompatibility(strategy, datatype string) ValidationResult {
    strategyInfo, exists := strategyDatatypeMatrix[strategy]
    if !exists {
        return ValidationResult{
            Type: "unknown_strategy",
            Message: fmt.Sprintf("Unknown masking strategy: %s", strategy),
            AvailableStrategies: getAllStrategies(),
        }
    }
    
    if !contains(strategyInfo.SupportedDatatypes, datatype) {
        return ValidationResult{
            Type: "strategy_datatype_mismatch",
            Message: fmt.Sprintf("Strategy %s not supported for datatype %s", strategy, datatype),
            SupportedDatatypes: strategyInfo.SupportedDatatypes,
            AlternativeStrategies: getStrategiesForDatatype(datatype),
        }
    }
    
    return ValidationResult{
        Type: "valid",
        SecurityLevel: strategyInfo.SecurityLevel,
        PerformanceImpact: strategyInfo.PerformanceImpact,
    }
}
```

#### Test Cases
```yaml
# ✅ Valid strategy/datatype combinations
# String datatype with string-compatible strategies
datatype: string
cases:
  - strategy: HASH_SHA1        # ✅ Valid for string
  - strategy: HASH_SHA256      # ✅ Valid for string  
  - strategy: ENCRYPT          # ✅ Valid for string
  - strategy: UNMASKED         # ✅ Valid for all types
  - strategy: NULLIFY          # ✅ Valid for string
  - strategy: PARTIAL_MASK     # ✅ Valid for string

# Numeric datatypes with numeric-compatible strategies
datatype: int
cases:
  - strategy: UNMASKED         # ✅ Valid for all types
  - strategy: NULLIFY          # ✅ Valid for int
  - strategy: RANDOM_NUMERIC   # ✅ Valid for int/float

datatype: float
cases:
  - strategy: UNMASKED         # ✅ Valid for all types
  - strategy: NULLIFY          # ✅ Valid for float
  - strategy: RANDOM_NUMERIC   # ✅ Valid for int/float

# Boolean datatype
datatype: boolean
cases:
  - strategy: UNMASKED         # ✅ Valid for all types
  - strategy: NULLIFY          # ✅ Valid for boolean

# ❌ Invalid strategy/datatype combinations
# String strategies with numeric datatypes
datatype: float
cases:
  - strategy: HASH_SHA1        # ❌ Invalid - only for strings
  - strategy: ENCRYPT          # ❌ Invalid - only for strings
  - strategy: PARTIAL_MASK     # ❌ Invalid - only for strings

datatype: int
cases:
  - strategy: HASH_SHA256      # ❌ Invalid - only for strings
  - strategy: ENCRYPT          # ❌ Invalid - only for strings

# Numeric strategies with string datatype
datatype: string
cases:
  - strategy: RANDOM_NUMERIC   # ❌ Invalid - only for numeric types

# Unknown strategies
datatype: string
cases:
  - strategy: INVALID_STRATEGY # ❌ Unknown strategy

# Error message examples:
❌ STRATEGY_MISMATCH: Strategy HASH_SHA1 not supported for datatype float
💡 Resolution: Use supported strategies for float: UNMASKED, NULLIFY, RANDOM_NUMERIC
📖 Documentation: [link to strategy compatibility matrix]

❌ UNKNOWN_STRATEGY: Unknown masking strategy 'INVALID_STRATEGY'
💡 Resolution: Use supported strategies: UNMASKED, HASH_SHA1, HASH_SHA256, ENCRYPT, NULLIFY, RANDOM_NUMERIC, PARTIAL_MASK
📖 Documentation: [link to masking strategy reference]

❌ DEPRECATED_STRATEGY: Strategy MD5_HASH is deprecated
💡 Resolution: Use HASH_SHA256 for improved security
📖 Documentation: [link to security best practices]
```

---

## Implementation Guidelines

### Development Workflow
1. **Start with Phase 1** - Build infrastructure first
2. **Test each rule independently** - Ensure isolation
3. **Use shared utilities** - Leverage YAML validation framework
4. **Follow naming conventions** - Consistent rule naming and structure
5. **Comprehensive testing** - Include edge cases and error conditions

### Configuration Strategy
```go
// Each rule should be configurable
type RulesConfig struct {
    EnabledRules       []string
    DisabledRules      []string
    WarehouseRule      WarehouseRuleConfig
    ServiceAccountRule ServiceAccountRuleConfig
    MigrationsRule     MigrationsRuleConfig
    // ... etc
}
```

### Error Message Standards
- **Clear and actionable** - Tell user exactly what's wrong
- **Include examples** - Show correct format when possible
- **Reference documentation** - Link to naming conventions, etc.
- **Consistent formatting** - Use standard error message templates

### Testing Strategy
- **Unit tests** - Test each validation function independently
- **Integration tests** - Test with real YAML files from dataproduct-config
- **Edge case tests** - Missing fields, malformed YAML, etc.
- **Performance tests** - Ensure rules don't slow down MR processing

This comprehensive plan provides **22 well-defined, 1-point tickets** that will implement complete YAML validation for the naysayer system while maintaining the self-service platform vision.

## Enhanced Implementation Guidance

All tickets have been enhanced with:

### **Essential Implementation Details**
- **Configuration Requirements:** Comprehensive config structures for each rule with environment-specific settings
- **Implementation Notes:** Specific guidance on leveraging existing infrastructure and integration patterns
- **Error Handling Patterns:** Standardized error types with detailed resolution guidance
- **Performance Considerations:** Optimization strategies and monitoring requirements

### **Operational Excellence**  
- **Test Cases:** Comprehensive examples covering valid/invalid scenarios with specific error messages
- **Integration Patterns:** Clear guidance on integrating with existing naysayer infrastructure
- **Monitoring & Logging:** Structured logging patterns for rule execution and audit trails
- **Security Considerations:** Best practices for handling sensitive data and validation

### **Developer Experience**
- **Code Examples:** Detailed implementation patterns following existing codebase conventions
- **Configuration Templates:** Ready-to-use configuration structures
- **Error Message Templates:** Standardized user-friendly error formatting
- **Documentation Links:** References to relevant platform documentation and policies

This enhanced documentation ensures each ticket is **immediately actionable** with clear implementation guidance, reducing development time and ensuring consistency across all validation rules.

## Current Implementation Status Summary

### ✅ **COMPLETED TICKETS (3/22):**
- **Ticket 1:** Basic YAML Field Validation Framework ✅
- **Ticket 8:** Warehouse Size Change Detection ✅
- **Ticket 9:** Warehouse Size Increase Validation ✅

### 🔄 **PARTIALLY COMPLETED TICKETS (5/22):**
- **Ticket 2:** File Path Structure Validation (utilities exist)
- **Ticket 4:** Service Account Email Format Validation (framework exists)
- **Ticket 5:** Individual vs Group Email Validation (framework exists)
- **Ticket 6:** Service Account Naming Convention Rule (framework exists)
- **Ticket 7:** Service Account Environment Restrictions (framework exists)

### ❌ **REMAINING TICKETS (14/22):**
- All other validation rules need full implementation

### **Effective Work Remaining:**
- **High Priority:** Complete service account validators (Tickets 4-7)
- **Medium Priority:** Path validation, cross-reference, environment consistency
- **Lower Priority:** Migration, TOC approval, masking policy rules

### **Current System Status:**
- ✅ **Core Infrastructure:** Fully functional rule system with line-level validation
- ✅ **Warehouse Rules:** Complete warehouse size validation working in production
- 🔄 **Service Account Rules:** Framework ready, need validator implementations
- ❌ **Other Rules:** Need implementation on top of existing infrastructure

### **Implementation Notes:**
- The foundational validation system is mature and well-structured
- Warehouse validation demonstrates the system working end-to-end
- Service account framework shows the pattern for implementing remaining rules
- Most remaining work is creating specific validation logic, not infrastructure