# Sandbox Unstructured Data Product Rules (AI Factory)

**Package**: `internal/rules/sandbox_personal`

**Purpose**: Auto-approve safe, self-service changes for AI Factory `UnstructuredDataProduct` data products in the **sandbox** environment, while blocking changes that need human oversight.

These rules apply **only** when the product's sandbox configuration matches:

```yaml
# dataproducts/unstructured/{productname}/sandbox/unstructured-data-product.yaml
kind: UnstructuredDataProduct
metadata:
  name: aif-*   # Must start with "aif-"
```

For all other products (non-aif-* products, etc.), the **existing** Naysayer rules continue unchanged.

---

## When do sandbox rules activate?

Naysayer loads `sandbox/unstructured-data-product.yaml` from the MR **source branch** and checks:

| Field | Required value |
|-------|----------------|
| `kind` | `UnstructuredDataProduct` |
| `metadata.name` | Must start with `aif-` |

If either condition fails, every sandbox rule **no-ops** (returns auto-approve/skip) and normal rules handle the file.

**Typical product layout** (from `dataproduct-config`):

```
dataproducts/unstructured/aif-{productname}/
├── developers.yaml              ← product root (Rule 2)
├── groups/                      ← product root (Rule 4)
│   └── *.yaml
└── sandbox/
    ├── unstructured-data-product.yaml     ← activation file (Rule 1)
    └── unstructured-data-pipeline.yaml    ← required (Rule 3)
```

---

## Rule overview

Four dedicated rules are registered in `internal/rules/registry.go` and wired in `rules.yaml` under the **SANDBOX UNSTRUCTURED DATA PRODUCT RULES** section.

| # | Rule name | File(s) | NEW vs EXISTING | Decision |
|---|-----------|---------|-----------------|----------|
| 1 | `sandbox_unstructured_product_config` | `sandbox/unstructured-data-product.yaml` | Any | ✅ Always auto-approve |
| 2 | `sandbox_developers_rule` | Product-root `developers.yaml` | NEW: exactly 2 owners (1 human + 1 service account) matching CODEOWNERS; EXISTING: unchanged + matches CODEOWNERS | ✅ if valid / ⚠️ if invalid or changed |
| 3 | `sandbox_unstructured_pipeline_rule` | `sandbox/unstructured-data-pipeline.yaml` | Any (validates S3 prefix) | ✅ if prefix matches {productname}/source/ and {productname}/destination/ / ⚠️ if invalid |
| 4 | `sandbox_groups_strict_rule` | Product-root `groups/*.yaml` | Any change | ⚠️ Always manual review |

### What is **not** covered by sandbox rules

Files from non-aif-* products fall through to **existing** rules requiring manual review.

---

## Rule 1 — Unstructured data product config (`sandbox_unstructured_product_config`)

**Applies to**: `dataproducts/unstructured/aif-*/sandbox/unstructured-data-product.yaml`

**Behavior**: Always auto-approve configuration changes for aif-* products.

**Example** (auto-approved):

```yaml
# dataproducts/unstructured/aif-myproduct/sandbox/unstructured-data-product.yaml
kind: UnstructuredDataProduct
metadata:
  name: aif-myproduct
```

---

## Rule 2 — Developers (`sandbox_developers_rule`)

**Applies to**: Product-root `dataproducts/unstructured/aif-*/developers.yaml`  
**Does not apply to**: `developers.yaml` inside `sandbox/`, `dev/`, `preprod/`, or `prod/`

**Service Account**: Configurable via environment variable `SANDBOX_SERVICE_ACCOUNT_NAME`. This is loaded from config and passed to the rule during initialization.

### NEW `developers.yaml`

| Condition | Decision |
|-----------|----------|
| Exactly **2** owners (1 human + 1 service account) | ✅ Auto-approve |
| Members **exactly match** CODEOWNERS file | ✅ Auto-approve |
| Wrong count or missing service account | ⚠️ Manual review |
| CODEOWNERS mismatch | ⚠️ Manual review |

```yaml
group:
  owners:
  - alice
  - project_106670_bot_8fc70748bff819b7cfc1f20740c278a0
```

**CODEOWNERS must match**:
```
/dataproducts/unstructured/aif-myproduct/ @alice @project_106670_bot_8fc70748bff819b7cfc1f20740c278a0
```

### EXISTING `developers.yaml`

| Condition | Decision |
|-----------|----------|
| Exactly 2 owners, same members as before | ✅ Auto-approve |
| Members match CODEOWNERS | ✅ Auto-approve |
| Owner changed | ⚠️ Manual review |
| Count ≠ 2 (current or previous) | ⚠️ Manual review |
| CODEOWNERS mismatch | ⚠️ Manual review |

---

## Rule 3 — Unstructured data pipeline (`sandbox_unstructured_pipeline_rule`)

**Applies to**: `dataproducts/unstructured/aif-*/sandbox/unstructured-data-pipeline.yaml`

**Behavior**: Validates source and destination configurations based on their type. This file is **required**.

### Source Crawler Configuration

The `source_crawler_config` supports two types:

#### Type: `s3`

| Field | Required Pattern |
|-------|------------------|
| `source_crawler_config.s3Config.prefix` | Must start with `{productname}/source/` |

**Example** (auto-approved for product `aif-myproduct`):

```yaml
source_crawler_config:
  type: "s3"
  s3Config:
    bucket: "dataverse-sandbox-unstructured"
    prefix: "aif-myproduct/source/"

destination_syncer_config:
  type: "s3"
  s3DestinationConfig:
    bucket: "dataverse-sandbox-unstructured"
    prefix: "aif-myproduct/destination/"
```

**Invalid example** (manual review required):

```yaml
# Wrong source prefix
source_crawler_config:
  type: "s3"
  s3Config:
    prefix: "wrong-product/source/"   # ⚠️ Must start with aif-myproduct/source/
```

#### Type: `google_drive`

| Field | Required Pattern |
|-------|------------------|
| `source_crawler_config.googleDriveConfig.folder_ids` | Must contain at least one non-empty folder ID |

**Example** (auto-approved for product `aif-myproduct`):

```yaml
source_crawler_config:
  type: "google_drive"
  googleDriveConfig:
    folder_ids:
      - id: "abc123xyz456"
      - id: "def789uvw012"

destination_syncer_config:
  type: "s3"
  s3DestinationConfig:
    bucket: "dataverse-sandbox-unstructured"
    prefix: "aif-myproduct/destination/"
```

**Invalid example** (manual review required):

```yaml
# Empty folder_ids
source_crawler_config:
  type: "google_drive"
  googleDriveConfig:
    folder_ids: []   # ⚠️ Must have at least one folder ID
```

#### Unsupported Types

Any `source_crawler_config.type` other than `s3` or `google_drive` will require manual review.

### Destination Syncer Configuration

The `destination_syncer_config` **must always** be type `s3`:

| Field | Required Pattern |
|-------|------------------|
| `destination_syncer_config.type` | Must be `s3` |
| `destination_syncer_config.s3DestinationConfig.prefix` | Must start with `{productname}/destination/` |

**Invalid example** (manual review required):

```yaml
# Wrong destination prefix
destination_syncer_config:
  type: "s3"
  s3DestinationConfig:
    prefix: "aif-myproduct/wrongpath/"   # ⚠️ Must start with aif-myproduct/destination/
```

---

## Rule 4 — Groups folder (`sandbox_groups_strict_rule`)

**Applies to**: Product-root `dataproducts/unstructured/aif-*/groups/*.yaml`

**Behavior**: Any add/modify/delete under `groups/` requires **manual review**.

---

## Configuration (`rules.yaml`)

Sandbox rules use **aif-* specific** file patterns to only match AI Factory products. Products without the `aif-` prefix fall back to general validation rules.

**Pattern**: `dataproducts/unstructured/aif-*/sandbox/`

Implementation files:

| Component | Path |
|-----------|------|
| Rule implementations | `internal/rules/sandbox_personal/*.go` |
| Rule registration | `internal/rules/registry.go` |
| File/section mapping | `rules.yaml` (SANDBOX UNSTRUCTURED DATA PRODUCT RULES section) |
| Pattern precedence | `internal/rules/manager.go` → `getParserForFile()` |
| Service account config | Environment variable `SANDBOX_SERVICE_ACCOUNT_NAME` (loaded via `internal/config/config.go`) |

---

## Decision flow

```
MR changes received
        │
        ▼
Does sandbox/unstructured-data-product.yaml have
kind=UnstructuredDataProduct + name starts with "aif-"?
        │
   NO ──┴── YES → sandbox aif-* rules apply to matching files
        │              │
        │              ├── unstructured-data-product.yaml    → Rule 1 (always approve)
        │              ├── developers.yaml                   → Rule 2 (2 members + CODEOWNERS)
        │              ├── unstructured-data-pipeline.yaml   → Rule 3 (validate S3 prefix)
        │              └── groups/*.yaml                     → Rule 4 (always review)
        │
        ▼
Existing rules apply (manual review for non-aif products)
```

**MR-level decision**: If **any** file requires manual review, the entire MR requires manual review.

---

## E2E test scenarios

Run all sandbox scenarios:

```bash
go test ./e2e -run 'TestE2E_Scenarios/Sandbox' -v -count=1
```

| Scenario | E2E path | What it validates | Expected |
|----------|----------|-------------------|----------|
| 44 | `44_sandbox_personal_with_pipeline` | Valid pipeline with correct S3 prefixes | ✅ Approve |
| 45 | `45_sandbox_personal_without_pipeline` | Product config update only | ✅ Approve |
| 46 | `46_sandbox_developers_new_one` | NEW `developers.yaml`, 2 members matching CODEOWNERS | ✅ Approve |
| 47 | `47_sandbox_developers_new_multiple` | NEW `developers.yaml`, wrong count (3 members) | ⚠️ Manual review |
| 48 | `48_sandbox_developers_existing_unchanged` | EXISTING `developers.yaml`, unchanged | ✅ Approve |
| 49 | `49_sandbox_developers_existing_changed` | EXISTING `developers.yaml`, owner changed | ⚠️ Manual review |
| 50 | `50_sandbox_groups_folder_change` | NEW file under product-root `groups/` | ⚠️ Manual review |
| 51 | `51_sandbox_personal_sourcebinding` | Product config update | ✅ Approve |
| 52 | `52_sandbox_pipeline_wrong_prefix` | Pipeline with wrong source prefix (S3) | ⚠️ Manual review |
| 53 | `53_sandbox_developers_codeowners_mismatch` | developers.yaml doesn't match CODEOWNERS | ⚠️ Manual review |
| 54 | `54_sandbox_developers_no_service_account` | developers.yaml missing service account | ⚠️ Manual review |
| 55 | `55_non_aif_product` | Non-aif product (fallback to general rules) | ⚠️ Manual review |
| 56 | `56_sandbox_pipeline_google_drive` | Valid pipeline with google_drive source type | ✅ Approve |
| 57 | `57_sandbox_pipeline_google_drive_invalid` | google_drive with empty folder_ids | ⚠️ Manual review |
| 58 | `58_sandbox_pipeline_unsupported_type` | Unsupported source type (azure_blob) | ⚠️ Manual review |

**Service Account Placeholder**: All test files use `{{SERVICE_ACCOUNT_NAME}}` which is dynamically replaced with the value from the config (`SANDBOX_SERVICE_ACCOUNT_NAME` environment variable) at test runtime. This ensures tests remain valid when the service account name changes.

Test fixtures live under:

```
e2e/testdata/scenarios/<scenario_name>/
├── scenario.yaml
├── before/          # target branch state
└── after/           # source branch state
```

---

## Common MR patterns

### ✅ Typical auto-approved setup (new aif-* unstructured product)

```
dataproducts/unstructured/aif-myproduct/sandbox/unstructured-data-product.yaml    (NEW, kind=UnstructuredDataProduct, name=aif-myproduct)
dataproducts/unstructured/aif-myproduct/sandbox/unstructured-data-pipeline.yaml   (NEW, correct S3 prefixes)
dataproducts/unstructured/aif-myproduct/developers.yaml                           (NEW, 2 owners matching CODEOWNERS)
CODEOWNERS                                                                        (with entry: /dataproducts/unstructured/aif-myproduct/ @alice @{service_account})
```

### ⚠️ Always requires manual review

```
dataproducts/unstructured/aif-myproduct/groups/consumer-team.yaml     (any groups/ change)
dataproducts/unstructured/non-aif-product/...                         (non-aif products)
```

---

## Troubleshooting

### Sandbox rules not applying

1. Confirm `sandbox/unstructured-data-product.yaml` on the **source branch** has:
   - `kind: UnstructuredDataProduct`
   - `metadata.name` starts with `aif-`
2. Confirm the changed file path matches the expected layout (`dataproducts/unstructured/aif-{productname}/...`).
3. Check Naysayer logs for: `MR affects sandbox UnstructuredDataProduct with aif-* name at ...`

### MR blocked on `developers.yaml`

- New file: must have **exactly 2 owners** (1 human + 1 service account) and must **exactly match** CODEOWNERS.
- Existing file: owner list cannot change; count must stay at 2; must match CODEOWNERS.
- Verify CODEOWNERS has entry: `/dataproducts/unstructured/{productname}/ @member1 @service_account`

### MR blocked on `unstructured-data-pipeline.yaml`

**For S3 source type:**
- `source_crawler_config.s3Config.prefix` must start with `{productname}/source/`
- Example for product `aif-test`: prefix must be `aif-test/source/...`

**For Google Drive source type:**
- `source_crawler_config.googleDriveConfig.folder_ids` must contain at least one non-empty folder ID

**For destination (always S3):**
- `destination_syncer_config.s3DestinationConfig.prefix` must start with `{productname}/destination/`
- Example for product `aif-test`: prefix must be `aif-test/destination/...`

**Unsupported source types:**
- Only `s3` and `google_drive` are supported for `source_crawler_config.type`
- Any other type will require manual review

### MR blocked on `groups/`

- Expected behavior. Consumer group changes always require manual review for aif-* sandbox products.

### MR blocked with "Failed to verify" errors

- These errors indicate network or API issues when Naysayer attempts to fetch files from GitLab.
- Common causes: GitLab API rate limits, network timeouts, authentication failures.
- **Security note**: Naysayer fails-closed (requires manual review) when it cannot verify file state, preventing auto-approval on transient errors.
- Check Naysayer logs for the specific API error, then retry the MR validation once the issue is resolved.

### Changing the service account name

1. Set the environment variable: `export SANDBOX_SERVICE_ACCOUNT_NAME="your-new-service-account"`
2. Alternatively, configure via `config.yaml` or your deployment configuration
3. All validation logic and e2e tests automatically use the configured value

---

## Related documentation

- [Section-Based Architecture](../SECTION_BASED_ARCHITECTURE.md) — how `rules.yaml` sections work
- [Metadata Rule](METADATA_RULE.md) — `sourcebinding.yaml`, `snowpipeconfig.yaml`, etc.
- [Rule Creation Guide](../RULE_CREATION_GUIDE.md) — adding or extending rules
