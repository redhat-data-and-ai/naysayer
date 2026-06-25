# Sandbox Personal Unstructured Data Product Rules

**Package**: `internal/rules/sandbox_personal`

**Purpose**: Auto-approve safe, self-service changes for **Personal** `UnstructuredDataProduct` data products in the **sandbox** environment, while blocking changes that need human oversight.

These rules apply **only** when the product's sandbox configuration matches:

```yaml
# dataproducts/<type>/<product>/sandbox/product.yaml
kind: UnstructuredDataProduct
type: Personal
```

For all other products (source-aligned, aggregate, non-Personal types, etc.), the **existing** Naysayer rules continue unchanged.

---

## When do sandbox rules activate?

Naysayer loads `sandbox/product.yaml` from the MR **source branch** and checks:

| Field | Required value |
|-------|----------------|
| `kind` | `UnstructuredDataProduct` |
| `type` | `Personal` |

If either condition fails, every sandbox rule **no-ops** (returns auto-approve/skip) and normal rules handle the file.

**Typical product layout** (from `dataproduct-config`):

```
dataproducts/source/<product_name>/
├── developers.yaml              ← product root (Rule 3)
├── groups/                      ← product root (Rule 4)
│   └── *.yaml
├── sandbox/
│   ├── product.yaml             ← activation + warehouse validation (Rule 2)
│   ├── unstructured-data-pipeline.yaml   ← optional (Rule 1)
│   ├── sourcebinding.yaml       ← existing metadata_rule
│   ├── snowpipeconfig.yaml      ← existing metadata_rule
│   └── migrations/              ← existing strict policy (manual review)
├── dev/
├── preprod/
└── prod/
```

---

## Rule overview

Four dedicated rules are registered in `internal/rules/registry.go` and wired in `rules.yaml` under the **SANDBOX PERSONAL UNSTRUCTURED DATA PRODUCT RULES** section.

| # | Rule name | File(s) | NEW vs EXISTING | Decision |
|---|-----------|---------|-----------------|----------|
| 1 | `sandbox_unstructured_pipeline_rule` | `sandbox/unstructured-data-pipeline.yaml` | Any (file is optional) | ✅ Always auto-approve |
| 2 | `sandbox_product_warehouse_rule` | `sandbox/product.yaml` → `warehouses` | NEW: validate XSMALL; EXISTING: delegate to warehouse logic | ✅ if XSMALL / ⚠️ if wrong size or warehouse change |
| 3 | `sandbox_developers_rule` | Product-root `developers.yaml` | NEW: exactly 1 owner; EXISTING: unchanged | ✅ if valid / ⚠️ if invalid or changed |
| 4 | `sandbox_groups_strict_rule` | Product-root `groups/*.yaml` | Any change | ⚠️ Always manual review |

### What is **not** covered by sandbox rules

These files are **not** special-cased by sandbox rules. They fall through to **existing** rules:

| File | Fallback rule |
|------|---------------|
| `sandbox/sourcebinding.yaml` | `metadata_rule` → auto-approve |
| `sandbox/snowpipeconfig.yaml` | `metadata_rule` → auto-approve |
| `sandbox/migrations/*.sql` | Strict policy → manual review |
| Any other uncovered file type | Strict policy → manual review |

---

## Rule 1 — Unstructured data pipeline (`sandbox_unstructured_pipeline_rule`)

**Applies to**: `dataproducts/**/sandbox/unstructured-data-pipeline.yaml`

**Behavior**: Always auto-approve when the product is a sandbox Personal UnstructuredDataProduct. The file is **optional** — MRs without this file are still valid.

**Example** (auto-approved):

```yaml
# dataproducts/source/myproduct/sandbox/unstructured-data-pipeline.yaml
pipeline:
  name: my-pipeline
  steps: []
```

---

## Rule 2 — Sandbox product warehouses (`sandbox_product_warehouse_rule`)

**Applies to**: `warehouses` section in `dataproducts/**/sandbox/product.yaml`

### NEW `sandbox/product.yaml`

All warehouse entries must use size **`XSMALL`**:

```yaml
warehouses:
- type: user
  size: XSMALL
- type: service_account
  size: XSMALL
```

| Condition | Decision |
|-----------|----------|
| All warehouses `XSMALL` | ✅ Auto-approve |
| Any warehouse ≠ `XSMALL` | ⚠️ Manual review |

### EXISTING `sandbox/product.yaml`

Warehouse changes are evaluated using the standard **`warehouse_rule`** logic (delegated inside `sandbox_product_warehouse_rule` — the warehouse package itself is **not** modified). Size increases and other warehouse changes require manual review, same as non-sandbox products.

---

## Rule 3 — Developers (`sandbox_developers_rule`)

**Applies to**: Product-root `dataproducts/**/developers.yaml`  
**Does not apply to**: `developers.yaml` inside `sandbox/`, `dev/`, `preprod/`, or `prod/`

### NEW `developers.yaml`

| Condition | Decision |
|-----------|----------|
| Exactly **1** owner in `group.owners` | ✅ Auto-approve |
| 0 or 2+ owners | ⚠️ Manual review |

```yaml
group:
  owners:
  - alice@company.com   # must be exactly one
```

### EXISTING `developers.yaml`

| Condition | Decision |
|-----------|----------|
| Still exactly 1 owner, same email as before | ✅ Auto-approve |
| Owner changed | ⚠️ Manual review |
| Count ≠ 1 (current or previous) | ⚠️ Manual review |

---

## Rule 4 — Groups folder (`sandbox_groups_strict_rule`)

**Applies to**: Product-root `dataproducts/**/groups/*.yaml`

**Behavior**: Any add/modify/delete under `groups/` requires **manual review**, even if the generic `metadata_rule` would otherwise auto-approve group files.

---

## Configuration (`rules.yaml`)

Sandbox rules use **more specific** file patterns than the generic `product_configs` entry. The section manager picks the **longest matching pattern**, so `dataproducts/**/sandbox/product.yaml` overrides the generic `dataproducts/**/product.yaml` for sandbox personal products.

Implementation files:

| Component | Path |
|-----------|------|
| Rule implementations | `internal/rules/sandbox_personal/*.go` |
| Rule registration | `internal/rules/registry.go` |
| File/section mapping | `rules.yaml` (sandbox section at bottom) |
| Pattern precedence | `internal/rules/manager.go` → `getParserForFile()` |

---

## Decision flow

```
MR changes received
        │
        ▼
Does sandbox/product.yaml have
kind=UnstructuredDataProduct + type=Personal?
        │
   NO ──┴── YES → sandbox personal rules apply to matching files
        │              │
        │              ├── pipeline.yaml        → Rule 1 (always approve)
        │              ├── sandbox/product.yaml → Rule 2 (warehouses)
        │              ├── developers.yaml      → Rule 3
        │              └── groups/*.yaml         → Rule 4 (always review)
        │
        ▼
Existing rules apply (warehouse_rule, metadata_rule,
toc_approval_rule, strict policy, etc.)
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
| 44 | `44_sandbox_personal_new_correct_warehouses` | NEW `sandbox/product.yaml`, all `XSMALL` warehouses | ✅ Approve |
| 45 | `45_sandbox_personal_new_wrong_warehouse` | NEW product with `SMALL` warehouse | ⚠️ Manual review |
| 46 | `46_sandbox_personal_with_pipeline` | NEW product + `unstructured-data-pipeline.yaml` | ✅ Approve |
| 47 | `47_sandbox_personal_without_pipeline` | NEW product, no pipeline file (optional) | ✅ Approve |
| 48 | `48_sandbox_developers_new_one` | NEW product-root `developers.yaml`, 1 owner | ✅ Approve |
| 49 | `49_sandbox_developers_new_multiple` | NEW `developers.yaml`, 2 owners | ⚠️ Manual review |
| 50 | `50_sandbox_developers_existing_unchanged` | EXISTING `developers.yaml`, no owner change | ✅ Approve |
| 51 | `51_sandbox_developers_existing_changed` | EXISTING `developers.yaml`, owner changed | ⚠️ Manual review |
| 52 | `52_sandbox_groups_folder_change` | NEW file under product-root `groups/` | ⚠️ Manual review |
| 53 | `53_sandbox_existing_product_warehouse_change` | EXISTING product, warehouse size increase | ⚠️ Manual review (warehouse logic) |
| 54 | `54_sandbox_personal_sourcebinding` | `sandbox/sourcebinding.yaml` change | ✅ Approve (`metadata_rule`) |

Test fixtures live under:

```
e2e/testdata/scenarios/<scenario_name>/
├── scenario.yaml
├── before/          # target branch state
└── after/           # source branch state
```

---

## Common MR patterns

### ✅ Typical auto-approved setup (new Personal unstructured product)

```
dataproducts/source/myproduct/sandbox/product.yaml          (NEW, XSMALL warehouses)
dataproducts/source/myproduct/developers.yaml               (NEW, 1 owner)
dataproducts/source/myproduct/sandbox/unstructured-data-pipeline.yaml  (optional)
```

### ⚠️ Always requires manual review

```
dataproducts/source/myproduct/groups/consumer-team.yaml     (any groups/ change)
dataproducts/source/myproduct/sandbox/sourcebinding.yaml    (only if metadata_rule doesn't apply — normally auto-approved)
```

---

## Troubleshooting

### Sandbox rules not applying

1. Confirm `sandbox/product.yaml` on the **source branch** has both:
   - `kind: UnstructuredDataProduct`
   - `type: Personal`
2. Confirm the changed file path matches the expected layout (`dataproducts/source/<product>/...`).
3. Check Naysayer logs for: `MR affects sandbox Personal UnstructuredDataProduct at ...`

### MR blocked on warehouse for NEW product

- Every warehouse entry must be `size: XSMALL` (case-insensitive).
- Check for typos like `SMALL`, `MEDIUM`, or missing warehouse types.

### MR blocked on `developers.yaml`

- New file: must have **exactly one** owner.
- Existing file: owner email cannot change; count must stay at 1.

### MR blocked on `groups/`

- Expected behavior. Consumer group changes require platform review for Personal sandbox products.

---

## Related documentation

- [Section-Based Architecture](../SECTION_BASED_ARCHITECTURE.md) — how `rules.yaml` sections work
- [Warehouse Rule](WAREHOUSE_RULE.md) — warehouse change logic for **existing** sandbox products
- [Metadata Rule](METADATA_RULE.md) — `sourcebinding.yaml`, `snowpipeconfig.yaml`, etc.
- [Rule Creation Guide](../RULE_CREATION_GUIDE.md) — adding or extending rules
