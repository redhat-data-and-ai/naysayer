# Data Product Consumer Rule (`dataproduct_consumer` package)

The Data Product Consumer Rule enables streamlined approval for granting consumer access to data products, service accounts, and rover groups across all environments without requiring TOC (Technical Oversight Committee) approval.

**Package**: `internal/rules/dataproduct_consumer`

## ⚖️ Rule Purpose

**Objective**: Auto-approve consumer access changes when only consumer fields are modified, allowing data product owners to manage access without TOC oversight

**Risk Level**: 🟢 **Low** - Consumer access changes are low-risk and require only data product owner approval

## 🔧 How It Works

### Detection Logic
The rule identifies when:
1. **Product File**: Changes are in a `product.yaml` file
2. **Consumer Section**: Only the `consumers` section under `data_product_db[*].presentation_schemas[*].consumers` is modified
3. **Consumer-Only Changes**: No other fields in the file are being modified

### Consumer Types Supported
The rule handles all consumer types:
- `data_product` - Other data products consuming this product
- `service_account` - Service accounts needing access
- `rover_group` - Rover groups requiring access

### Environment Coverage
Unlike the TOC approval rule, consumer changes are **auto-approved in ALL environments**:
- ✅ `dev` - Development environment
- ✅ `sandbox` - Sandbox/testing environment
- ✅ `preprod` - Pre-production environment
- ✅ `prod` - Production environment

### Examples of Consumer Changes
```yaml
# Adding a consumer to the consumers list
data_product_db:
- database: sfsales_db
  presentation_schemas:
  - name: sfsales_marts
    consumers:
    - name: forecasting
      kind: data_product
    - name: partnervettingonboarding  # NEW CONSUMER
      kind: data_product
```

## 🎯 Decision Logic

| **Condition** | **Decision** | **Reason** |
|---------------|--------------|------------|
| Consumer-only changes in any environment | ✅ **Auto-approve** | Data product owner approval sufficient, no TOC needed |
| Consumer + other field changes | 🔄 **Other Rules Apply** | Let other rules handle non-consumer changes |
| Non-product files | ✅ **Auto-approve** | Rule doesn't apply |

## ✅ Auto-Approval Scenarios

### When Consumer Changes Are Auto-Approved
- **Adding Consumers**: New data products, service accounts, or rover groups
- **Removing Consumers**: Revoking access from consumers
- **Modifying Consumers**: Changing consumer types or names
- **All Environments**: Works in dev, sandbox, preprod, and prod

### Business Benefits
1. **Faster Access Management**: No waiting for TOC approval
2. **Data Product Owner Control**: Owners manage their own consumers
3. **Reduced Bottlenecks**: Streamlined collaboration between teams
4. **Flexibility**: Works across all environments

## 🚫 When This Rule Doesn't Apply

### Mixed Changes Require Other Rules
If an MR contains consumer changes **AND** other changes, the other rules take precedence:

```yaml
# This MR has BOTH consumer and warehouse changes
warehouses:
- type: user
  size: MEDIUM  # ⚠️ Warehouse change - warehouse_rule applies

data_product_db:
- database: db
  presentation_schemas:
  - name: marts
    consumers:
    - name: newconsumer  # Consumer change
      kind: data_product
```
**Result**: ⚠️ Manual review required (due to warehouse size increase)

## ⚙️ Configuration

### Environment Variables
```bash
# Configure which environments allow consumer access (default: preprod,prod)
# Note: The rule now auto-approves in ALL environments regardless of this setting
DATAPRODUCT_CONSUMER_ENVS=preprod,prod
```

### Rules Configuration (rules.yaml)
```yaml
files:
  - name: "product_configs"
    path: "dataproducts/**/"
    filename: "product.{yaml,yml}"
    sections:
      # Consumer access changes - auto-approve with data product owner approval
      - name: consumers
        yaml_path: data_product_db[*].presentation_schemas[*].consumers
        rule_configs:
          - name: dataproduct_consumer_rule
            enabled: true
        auto_approve: true
```

## 🔍 Example Scenarios

### Scenario 1: Adding Consumer in Production
```yaml
# File: dataproducts/source/sfsales/prod/product.yaml
# Change: Adding new consumer "partnervettingonboarding"
data_product_db:
- database: sfsales_db
  presentation_schemas:
  - name: sfsales_marts
    consumers:
    - name: forecasting
      kind: data_product
    - name: partnervettingonboarding  # NEW
      kind: data_product
```
**Result**: ✅ Auto-approved - Consumer access changes in prod environment - data product owner approval sufficient (no TOC approval required)

### Scenario 2: Adding Rover Group Consumer in Preprod
```yaml
# File: dataproducts/source/sfsales/preprod/product.yaml
# Change: Adding rover group consumer
data_product_db:
- database: sfsales_db
  presentation_schemas:
  - name: sfsales_marts
    consumers:
    - name: dataverse-team-analytics  # NEW
      kind: rover_group
```
**Result**: ✅ Auto-approved - Consumer access changes in preprod environment - data product owner approval sufficient (no TOC approval required)

### Scenario 3: Adding Service Account Consumer in Dev
```yaml
# File: dataproducts/source/sfsales/dev/product.yaml
# Change: Adding service account consumer
data_product_db:
- database: sfsales_db
  presentation_schemas:
  - name: sfsales_marts
    consumers:
    - name: snowflake_workato_dev_appuser  # NEW
      kind: service_account
```
**Result**: ✅ Auto-approved - Consumer access changes in dev environment - data product owner approval sufficient (no TOC approval required)

### Scenario 4: Multiple Consumers Added
```yaml
# File: dataproducts/aggregate/forecasting/prod/product.yaml
# Change: Adding multiple consumers at once
data_product_db:
- database: forecasting_db
  presentation_schemas:
  - name: forecasting_marts
    consumers:
    - name: revenue_analytics  # NEW
      kind: data_product
    - name: sales_reporting    # NEW
      kind: data_product
    - name: bi_team_access     # NEW
      kind: rover_group
```
**Result**: ✅ Auto-approved - Multiple consumer additions are fine

### Scenario 5: Consumer + Warehouse Change (Mixed)
```yaml
# File: dataproducts/source/sfsales/prod/product.yaml
# Change: Both consumer AND warehouse changes
warehouses:
- type: user
  size: LARGE  # Changed from MEDIUM

data_product_db:
- database: sfsales_db
  presentation_schemas:
  - name: sfsales_marts
    consumers:
    - name: newconsumer  # NEW
      kind: data_product
```
**Result**: ⚠️ Manual review required - Warehouse size increase triggers warehouse_rule

## 📋 Governance & Compliance

### Why No TOC Approval?
Consumer access changes are considered **low-risk** because:
1. **Data product owners** already have authority over their data
2. **Access control** is managed at the data product level
3. **No infrastructure impact** - only permission changes
4. **Reversible** - access can be easily revoked

### Data Product Owner Responsibilities
Data product owners must ensure:
- ✅ Consumers have legitimate business need for access
- ✅ Proper data classification and sensitivity awareness
- ✅ Compliance with data governance policies
- ✅ Regular access reviews and cleanup

### Audit Trail
All consumer access changes are logged with:
- Timestamp of access grant
- Consumer name and type
- Data product and schema accessed
- Environment (dev, sandbox, preprod, prod)
- MR author (typically data product owner)

## 🛠️ Troubleshooting

### Common Issues

**Issue**: Consumer changes not auto-approved
```
Cause: MR contains non-consumer changes
Solution: Separate consumer changes into dedicated MR
✅ Good: Consumer-only MR
❌ Bad: Consumer + warehouse changes in same MR
```

**Issue**: Rule not detecting consumer section
```
Cause: YAML path doesn't match expected structure
Solution: Ensure consumers are under:
  data_product_db[*].presentation_schemas[*].consumers
```

### Debug Commands
```bash
# Check consumer section in product.yaml
yq '.data_product_db[].presentation_schemas[].consumers' product.yaml

# View only consumer-related changes in MR
git diff main -- '**/product.yaml' | grep -A 5 "consumers:"

# Validate YAML structure
yamllint dataproducts/source/*/prod/product.yaml
```

## 🎯 Best Practices

### Consumer Access Workflow
1. **Document Justification**: Include business reason in MR description
2. **Separate MRs**: Keep consumer changes in dedicated MRs
3. **Review Access**: Periodically audit and remove unused consumers
4. **Consistent Naming**: Use clear, descriptive consumer names

### MR Description Template
```markdown
## Consumer Access Request

**Data Product**: [name]
**Environment**: [dev/sandbox/preprod/prod]
**Consumer**: [name]
**Type**: [data_product/service_account/rover_group]

### Business Justification
[Describe why this consumer needs access]

### Expected Usage
[What data will be consumed and how]
```

### YAML Structure Best Practices
```yaml
# ✅ GOOD: Clear, organized structure
data_product_db:
- database: myproduct_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: analytics_team
      kind: rover_group
    - name: reporting_product
      kind: data_product

# ❌ AVOID: Mixing consumer types without organization
consumers:
  - foo
  - bar
  - baz
```

## 🔗 Related Rules

- **TOC Approval Rule**: Handles new product deployments (consumer rule is complementary)
- **Warehouse Rule**: Validates infrastructure changes (runs independently)
- **Metadata Rule**: Auto-approves documentation (works together with consumer rule)
- **Service Account Rule**: Validates service account configurations

## 🔄 Workflow Integration

### Typical Consumer Access Flow
1. **Data Product Owner** creates MR to add consumer
2. **Naysayer** detects consumer-only change
3. **Auto-Approval** by dataproduct_consumer_rule
4. **Owner Reviews** and merges (no TOC needed)
5. **Access Granted** through automated deployment

### Comparison with Other Rules

| **Change Type** | **Rule** | **Approval** |
|-----------------|----------|--------------|
| Add consumer | Consumer Rule | ✅ Auto (DP Owner) |
| New product in prod | TOC Rule | ⚠️ Manual (TOC) |
| Warehouse increase | Warehouse Rule | ⚠️ Manual |
| Documentation | Metadata Rule | ✅ Auto |

---

**✨ Key Takeaway**: The consumer rule empowers data product owners to manage access efficiently across all environments without TOC bottlenecks, while maintaining proper governance through owner-level approval.
