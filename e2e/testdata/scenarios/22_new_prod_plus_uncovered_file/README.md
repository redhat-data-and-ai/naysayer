# Test Scenario: New Product (Prod) with Uncovered File

## Purpose
This scenario validates that Naysayer shows **both** rule check results AND uncovered files in manual review comments, even when both conditions exist simultaneously.

## Background
This test was created to reproduce and validate the fix for an issue found in MR 3273 where:
- The MR had a new `prod/product.yaml` file (requiring TOC approval)
- The MR also had an uncovered `consumers.yaml` file (no validation rules)
- The original bug: Only the uncovered file was shown in the comment, hiding the TOC rule failure

## What This Tests

### Files Changed
1. **New file**: `dataproducts/source/fammatrix/prod/product.yaml` - New product in prod environment
2. **New file**: `dataproducts/source/fammatrix/prod/sourcebinding.yaml` - Source binding configuration
3. **New file**: `dataproducts/source/fammatrix/consumers.yaml` - Consumer groups (no validation rules)
4. **Modified**: `dataproducts/source/fammatrix/preprod/product.yaml` - Tag update

### Expected Behavior
The comment should show:

1. **What was checked:**
   - ✅ Metadata rule: Approved
   - 🚫 TOC approval rule: Failed (new prod product requires TOC approval)
   - 🚫 Warehouse rule: Failed (new warehouses detected)

2. **Files without validation rules:**
   - `consumers.yaml` - No validation rules configured

### Key Validation
This ensures that when an MR has:
- Multiple rule failures (TOC, warehouse)
- AND uncovered files (consumers.yaml)

Both pieces of information are visible in the comment, not just the uncovered files.

## Related
- Original issue: MR 3273 in dataproduct-config repository
- Fix: `internal/webhook/messages.go` - Modified `buildDetailedManualReviewSummary` to always show "What was checked" section
