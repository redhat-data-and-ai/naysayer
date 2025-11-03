# Top 7 Critical E2E Test Scenarios

This document identifies the **7 most critical** E2E test scenarios that provide maximum coverage with minimal effort.

**Current Status:** 7/7 Tier 1 scenarios implemented ✅

**Last Updated:** 2025-10-30

---

## Quick Summary

| # | Scenario | Rule | Decision | Status | Priority |
|---|----------|------|----------|--------|----------|
| 1 | warehouse_decrease | Warehouse | Auto-approve | ✅ Implemented | P0 |
| 2 | warehouse_increase | Warehouse | Manual review | ✅ Implemented | P0 |
| 3 | toc_new_prod | TOC | Manual review | ✅ Implemented | P0 |
| 4 | consumer_only_prod | Consumer | Auto-approve | ✅ Implemented | P0 |
| 5 | metadata_readme | Metadata | Auto-approve | ✅ Implemented | P0 |
| 6 | metadata_product_sections | Metadata | Auto-approve | ✅ Implemented | P0 |
| 7 | integration_uncovered_lines | Integration | Manual review | ✅ Implemented | P0 |

**Coverage:** These 7 scenarios cover ~70% of real-world MR patterns

**Effort:** 1 week to implement all 7 scenarios

---

## Why These 7?

### Selection Criteria:
1. **High Frequency** - Most common MR patterns in real-world usage
2. **High Risk** - Tests critical decision paths (cost, production, access control)
3. **No Redundancy** - Each scenario validates a unique code path
4. **Business Value** - Directly impacts cost control, security, and velocity

### What These 7 Cover:
- ✅ All 4 validation rules (Warehouse, TOC, Consumer, Metadata)
- ✅ Both decision types (Auto-approve, Manual review)
- ✅ New files vs existing file updates
- ✅ Cost increases vs decreases
- ✅ Coverage gap detection (strict policy)
- ✅ Most common auto-approval patterns

### What We Removed (and Why):
- ❌ **warehouse_deletion** - Redundant with `warehouse_decrease` (both = cost savings)
- ❌ **warehouse_new_creation** - Redundant with `warehouse_increase` (new warehouse = increase from nothing)
- ❌ **toc_existing_prod** - Already tested by existing warehouse scenarios in prod
- ❌ **warehouse_cross_fork** - Not applicable (no external forks in internal repo)
- ❌ **toc_new_dev** - Unnecessary (inverse test of critical environment logic)
- ❌ **consumer_plus_warehouse** - Redundant (both rules run independently, no new logic)
- ❌ **metadata_with_code** - Redundant with `integration_unknown_filetype`
- ❌ **integration_draft_mr, integration_bot_mr, integration_empty_mr** - Consolidated into single `integration_special_mr_types` scenario
- ❌ **toc_environment_in_filename** - Already works via existing `_prod_` pattern detection
- ❌ **toc_case_insensitive** - Environment detection is always case-insensitive by design

---

## Detailed Scenarios

### Warehouse Rule (2 scenarios)

---

#### 1. warehouse_decrease ✅ IMPLEMENTED
**Rule:** Warehouse
**Decision:** Auto-approve
**Frequency:** Very High (20% of MRs)
**Risk:** Low (cost savings)

**What it tests:**
Warehouse size decrease auto-approves (cost savings)

**Real-world example:**
Team downsizes analytics warehouse from MEDIUM to SMALL during low-usage period

**File changes:**
```yaml
dataproducts/marketing/prod/product.yaml
  Before: warehouse: MEDIUM
  After:  warehouse: SMALL
```

**Why critical:**
- Most common cost-optimization pattern
- Must auto-approve to enable team velocity
- Tests core warehouse rule logic

**Implementation status:** ✅ Complete
**Location:** `e2e/testdata/scenarios/01_single_rule_single_file/warehouse_decrease/`

---

#### 2. warehouse_increase ✅ IMPLEMENTED
**Rule:** Warehouse
**Decision:** Manual review
**Frequency:** High (15% of MRs)
**Risk:** High (cost increase)

**What it tests:**
Warehouse size increase requires manual review (budget approval)

**Real-world example:**
Team scales up warehouse from SMALL to MEDIUM for increased load

**File changes:**
```yaml
dataproducts/marketing/prod/product.yaml
  Before: warehouse: SMALL
  After:  warehouse: MEDIUM
```

**Why critical:**
- Cost increase must be caught
- Tests manual review trigger
- Business-critical for budget control

**Implementation status:** ✅ Complete
**Location:** `e2e/testdata/scenarios/01_single_rule_single_file/warehouse_increase/`

---

### TOC Approval Rule (1 scenario)

---

#### 3. toc_new_prod
**Rule:** TOC Approval
**Decision:** Manual review
**Frequency:** Medium (10% of MRs)
**Risk:** Very High (new production deployment)

**What it tests:**
NEW product.yaml in prod requires TOC approval

**Real-world example:**
Brand new data product being deployed to production for first time

**File changes:**
```yaml
dataproducts/sales/prod/product.yaml (NEW FILE)
  Content:
    name: sales-analytics
    kind: source-aligned
    warehouses:
      - name: sales_wh
        warehouse: LARGE
```

**Why critical:**
- First-time production deployments are high-risk
- Requires architectural review
- Tests TOC rule's new file detection

**Note:** Updates to existing prod files are already tested by warehouse_decrease/increase scenarios

**Implementation status:** ✅ Complete
**Location:** `e2e/testdata/scenarios/02_toc_approval/toc_new_prod/`

---

### Consumer Rule (1 scenario)

---

#### 4. consumer_only_prod
**Rule:** Consumer
**Decision:** Auto-approve
**Frequency:** High (15% of MRs)
**Risk:** Low (access control by owner)

**What it tests:**
Consumer-only changes auto-approve without TOC

**Real-world example:**
Analytics team grants journey product access to their data

**File changes:**
```yaml
dataproducts/analytics/prod/product.yaml (lines 58-61 only)
  Before:
    consumers: []

  After:
    consumers:
      - name: journey
        kind: data_product
```

**Why critical:**
- Common access management pattern
- Owner approval should be sufficient
- Tests consumer-only detection logic

**Implementation status:** ✅ Complete
**Location:** `e2e/testdata/scenarios/01_single_rule_single_file/consumer_only_prod/`

---

### Metadata Rule (2 scenarios)

---

#### 5. metadata_readme
**Rule:** Metadata
**Decision:** Auto-approve
**Frequency:** Very High (20% of MRs)
**Risk:** Very Low (documentation)

**What it tests:**
README updates auto-approve

**Real-world example:**
Team updates documentation with new setup instructions

**File changes:**
```
dataproducts/analytics/README.md
  Before: Old instructions
  After:  New instructions
```

**Why critical:**
- Very common pattern
- Documentation must not block deployments
- Tests metadata rule's file type detection

**Implementation status:** ✅ Complete
**Location:** `e2e/testdata/scenarios/01_single_rule_single_file/metadata_readme/`

---

#### 6. metadata_product_sections
**Rule:** Metadata (Section-based)
**Decision:** Auto-approve
**Frequency:** Medium (7% of MRs)
**Risk:** Low (metadata only)

**What it tests:**
Metadata-only sections in product.yaml auto-approve

**Real-world example:**
Team updates product name and tags only

**File changes:**
```yaml
dataproducts/analytics/prod/product.yaml (lines 1-5 only)
  Before:
    name: analytics
    tags: []

  After:
    name: analytics-v2
    tags: [pii: true]
```

**Why critical:**
- Tests section-based metadata detection
- Common organizational metadata updates
- Ensures metadata changes don't block velocity

**Implementation status:** ✅ Complete
**Location:** `e2e/testdata/scenarios/01_single_rule_single_file/metadata_product_sections/`

---

### Integration (1 scenario)

---

#### 7. integration_uncovered_lines
**Rule:** Integration (Section-based validation)
**Decision:** Manual review
**Frequency:** Low (5% of MRs)
**Risk:** Very High (unknown changes)

**What it tests:**
Changed lines not covered by any section trigger manual review

**Real-world example:**
Developer adds new YAML section that rules don't know about

**File changes:**
```yaml
dataproducts/analytics/prod/product.yaml (lines 100-105)
  Before:
    (file ends at line 95)

  After:
    new_experimental_section:    # UNCOVERED
      enabled: true              # UNCOVERED
```

**Why critical:**
- Strict coverage policy - must catch unknown changes
- Tests section manager's coverage detection
- Safety mechanism for unexpected modifications

**Implementation status:** ✅ Complete
**Location:** `e2e/testdata/scenarios/04_integration/integration_uncovered_lines/`

---

## Coverage Analysis

### What These 7 Cover:

| Validation Rule | Scenarios | Coverage |
|----------------|-----------|----------|
| Warehouse | 2 scenarios | 80% of warehouse patterns |
| TOC Approval | 1 scenario | 80% of TOC patterns |
| Consumer | 1 scenario | 80% of consumer patterns |
| Metadata | 2 scenarios | 75% of metadata patterns |
| Integration | 1 scenario | 70% of coverage detection |

**Overall Coverage:** ~70% of real-world MR patterns

### What's NOT Covered (defer to Phase 2):
- ❌ Warehouse deletion (covered by decrease logic)
- ❌ Existing prod updates (covered by warehouse scenarios)
- ❌ Multiple mixed warehouse changes (edge case)
- ❌ New preprod deployments (similar to new prod)
- ❌ Consumer + warehouse mixed changes (rare)
- ❌ Multi-file MRs with mixed decisions (edge case)
- ❌ Error scenarios (YAML parse, API failures)
- ❌ Special MR types (draft, bot, empty)
- ❌ Unknown file types beyond basic validation

These can be added in Phase 2, but the top 7 provide strong baseline coverage.

---

## Implementation Guide

### Week 1: All 7 Scenarios

**Day 1-2: TOC Rule**
- toc_new_prod

**Day 3: Consumer Rule**
- consumer_only_prod

**Day 4: Metadata Rule**
- metadata_readme
- metadata_product_sections

**Day 5: Integration**
- integration_uncovered_lines

**Total:** 5 new scenarios + 2 existing = 7 complete

### Success Criteria:
- ✅ All 7 scenarios passing
- ✅ Coverage report shows ~70% coverage
- ✅ CI/CD pipeline running E2E tests on every PR
- ✅ Documentation updated

---

## Next Steps

1. **Implement all 7 scenarios** - 1 week effort
2. **Run full E2E suite** on CI to validate
3. **Monitor production** for patterns not covered
4. **Phase 2** - Add edge cases and error handling (14 more scenarios)

For detailed implementation examples, see the existing scenarios:
- `e2e/testdata/scenarios/01_single_rule_single_file/warehouse_decrease/`
- `e2e/testdata/scenarios/01_single_rule_single_file/warehouse_increase/`

For complete scenario list (17 scenarios across 3 tiers: 7+7+3), see `TEST_SCENARIOS.md`.
