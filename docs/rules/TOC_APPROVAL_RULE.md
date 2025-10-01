# TOC Approval Rule (`toc_approval` package)

The TOC (Technical Oversight Committee) Approval Rule ensures that new data products being promoted to production or pre-production environments receive proper governance oversight before deployment.

**Package**: `internal/rules/toc_approval`

## ⚖️ Rule Purpose

**Objective**: Require manual review and TOC approval for new `product.yaml` files being deployed to critical environments

**Risk Level**: 🔴 **High** - New products in production environments require governance oversight

## 🔧 How It Works

### Detection Logic
The rule identifies when:
1. **New File**: A `product.yaml` file is being added (not modified)
2. **Critical Environment**: The file path contains preprod or prod environment indicators
3. **Data Product**: The file is a data product configuration

### Environment Detection
The rule checks file paths for environment indicators:
- `/preprod/` - Pre-production environment
- `/prod/` - Production environment  
- `_preprod_` - Pre-production naming convention
- `_prod_` - Production naming convention

### Examples of Triggering Paths
```
✅ REQUIRES TOC APPROVAL:
dataproducts/analytics/prod/product.yaml        # New file in prod
dataproducts/source/preprod/product.yaml        # New file in preprod
dataproducts/ml/my_prod_setup/product.yaml      # New file with prod in path

❌ NO TOC APPROVAL NEEDED:
dataproducts/analytics/prod/product.yaml        # Existing file modification
dataproducts/analytics/dev/product.yaml         # New file in dev environment
dataproducts/analytics/test/product.yaml        # New file in test environment
```

## 🎯 Decision Logic

| **Condition** | **Decision** | **Reason** |
|---------------|--------------|------------|
| New `product.yaml` in prod/preprod | 🔍 **Manual Review** | TOC approval required for production deployments |
| Existing `product.yaml` in prod/preprod | ✅ **Auto-approve** | Modifications to existing products don't need TOC |
| New `product.yaml` in dev/test | ✅ **Auto-approve** | Development environments don't require TOC |
| Non-product files | ✅ **Auto-approve** | Rule only applies to product configurations |

## 🚫 Manual Review Triggers

### When TOC Approval is Required
- **New Data Product Promotion**: First-time deployment to production environments
- **Business Impact**: New products may have significant business or operational impact
- **Compliance**: Production deployments must follow governance processes

### Review Process
1. **Automatic Detection**: Naysayer flags the MR for manual review
2. **TOC Notification**: Technical Oversight Committee is notified
3. **Review Criteria**: TOC evaluates:
   - Business justification
   - Technical readiness
   - Security compliance
   - Resource requirements
4. **Manual Approval**: TOC member manually approves the MR

## ⚙️ Configuration

### Environment Variables
```bash
# Configure which environments require TOC approval
TOC_APPROVAL_ENVS=preprod,prod
```

### Rules Configuration (rules.yaml)
```yaml
files:
  - name: "product_configs"
    path: "**/dataproducts/**/"
    filename: "product.{yaml,yml}"
    sections:
      - name: full_file_toc_validation
        yaml_path: .
        rule_configs:
          - name: toc_approval_rule
            enabled: true
        auto_approve: false
```

## 🔍 Example Scenarios

### Scenario 1: New Production Deployment
```yaml
# File: dataproducts/analytics/prod/product.yaml (NEW FILE)
name: customer-analytics
version: 1.0.0
warehouses:
  - type: user
    size: LARGE
```
**Result**: 🔍 Manual review required - TOC approval needed

### Scenario 2: Development Environment
```yaml
# File: dataproducts/analytics/dev/product.yaml (NEW FILE)
name: customer-analytics
version: 1.0.0
```
**Result**: ✅ Auto-approved - Development environment

### Scenario 3: Existing Product Update
```yaml
# File: dataproducts/analytics/prod/product.yaml (EXISTING FILE)
name: customer-analytics
version: 1.1.0  # Version bump
```
**Result**: ✅ Auto-approved - Existing product modification

## 📋 Compliance & Audit

### Audit Trail
All TOC approval decisions are logged with:
- Timestamp of review request
- Environment being deployed to
- Product name and configuration
- TOC member who approved
- Business justification

### Governance Integration
- **JIRA Integration**: Automatic ticket creation for TOC review
- **Slack Notifications**: Real-time alerts to TOC channel
- **Dashboard Tracking**: Metrics on approval times and patterns

## 🛠️ Troubleshooting

### Common Issues

**Issue**: False positives for development environments
```
Solution: Ensure dev environment paths don't contain "prod" keywords
✅ Good: dataproducts/analytics/development/
❌ Avoid: dataproducts/analytics/dev-production/
```

**Issue**: TOC approval for non-critical changes
```
Solution: Use separate branches/paths for non-production testing
✅ Good: dataproducts/analytics/staging/
❌ Avoid: dataproducts/analytics/prod/ for testing
```

### Debug Commands
```bash
# Check which environments trigger TOC approval
grep -r "preprod\|prod" dataproducts/

# Verify file is detected as new
git status --porcelain | grep "^A"
```

## 🎯 Best Practices

### Development Workflow
1. **Test First**: Deploy to dev/test environments first
2. **Document Changes**: Include business justification in MR description  
3. **Early TOC Engagement**: Notify TOC before creating production MRs
4. **Staged Rollouts**: Use preprod for final validation before prod

### Path Naming Conventions
```bash
✅ RECOMMENDED:
dataproducts/[team]/[env]/product.yaml
dataproducts/analytics/prod/product.yaml
dataproducts/ml/preprod/product.yaml

❌ AVOID:
dataproducts/analytics/production-test/product.yaml  # Contains "prod"
dataproducts/ml/prod-backup/product.yaml            # Contains "prod"
```

## 🔗 Related Rules

- **Warehouse Rule**: Validates cost implications of new production deployments
- **Service Account Rule**: Ensures proper security for production environments
- **Metadata Rule**: Validates documentation completeness for production systems

---

**⚠️ Important**: This rule helps maintain production stability and governance compliance. Contact the TOC team for expedited reviews when needed.