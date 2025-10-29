# E2E Testing Framework

This directory contains the end-to-end (E2E) testing framework for Naysayer. The framework validates the complete MR review workflow by simulating real GitLab merge requests and verifying the approval decisions.

## Overview

The E2E tests work by:
1. Comparing "before" and "after" directory snapshots to generate file changes
2. Creating a mock GitLab client that serves these changes
3. Sending a webhook payload to the Naysayer handler
4. Validating the approval decision, MR comments, and GitLab API interactions

This approach allows us to test the entire review flow without requiring a live GitLab instance.

## Directory Structure

```
e2e/
├── README.md                    # This file
├── rules.yaml                   # Symlink to ../rules.yaml (production config)
├── scenarios_test.go            # Main test runner
├── diff_generator.go            # Generates file diffs from before/after folders
├── mock_gitlab_client.go        # Mock GitLab API client
├── scenario_loader.go           # Loads and parses scenario configs
└── testdata/
    └── scenarios/
        └── 01_single_rule_single_file/
            ├── warehouse_decrease/
            │   ├── scenario.yaml           # Test scenario configuration
            │   ├── before/                 # Initial repository state
            │   │   └── dataproducts/...
            │   ├── after/                  # Modified repository state
            │   │   └── dataproducts/...
            │   └── expected_comment.txt    # Expected MR comment
            └── warehouse_increase/
                └── ...
```

## Running E2E Tests

### Run all E2E tests
```bash
make test-e2e
```

### Run all tests (unit + E2E)
```bash
make test
```

### Run only unit tests (excluding E2E)
```bash
make test-unit
```

### Run a specific scenario
```bash
go test ./e2e -v -run TestE2E_Scenarios/warehouse_increase
```

### Run with verbose output and disable caching
```bash
go test ./e2e -v -count=1
```

Note: The `-count=1` flag disables test caching, ensuring tests always run fresh.


## Creating New Test Scenarios

### Step 1: Create scenario directory

```bash
mkdir -p e2e/testdata/scenarios/my_new_scenario/{before,after}
```

### Step 2: Create scenario.yaml

Create `e2e/testdata/scenarios/my_new_scenario/scenario.yaml`:

```yaml
name: "My New Scenario"
description: "Description of what this scenario tests"

mr_metadata:
  title: "Test: My feature change"
  author: "developer"
  source_branch: "feature/my-feature"
  target_branch: "main"

expected:
  decision: "approve"  # or "manual_review"
  approved: true       # or false
  reason: "Expected reason substring"
  comment_contains:
    - "phrase that should appear in comment"
    - "another expected phrase"
  rules_evaluated:
    - "warehouse_rule"
    - "metadata_rule"
```

### Step 3: Add before/after files

Place the initial state in `before/` and the modified state in `after/`:

```bash
# Example: Testing a warehouse size change
cp dataproducts/team/prod/product.yaml e2e/testdata/scenarios/my_new_scenario/before/
# Edit the file to change warehouse size
cp dataproducts/team/prod/product.yaml e2e/testdata/scenarios/my_new_scenario/after/
```

### Step 4: (Optional) Create expected comment

If you want to validate the exact comment text:

```bash
# Run the test once to see what comment is generated
go test ./e2e -v -run TestE2E_Scenarios/my_new_scenario

# Copy the actual comment to expected_comment.txt
cat > e2e/testdata/scenarios/my_new_scenario/expected_comment.txt << 'EOF'
<!-- naysayer-comment-id: auto-approved -->
✅ **Auto-approved**
...
EOF
```

### Step 5: Run the test

```bash
go test ./e2e -v -run TestE2E_Scenarios/my_new_scenario
```
