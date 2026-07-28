---
name: create-naysayer-rule
description: >-
  Authors new Naysayer validation rules in Go following existing patterns:
  shared.Rule interface, registry registration, rules.yaml section wiring,
  unit tests, and e2e scenarios. Use when adding a validation rule, extending
  auto-approval, wiring rules.yaml, or implementing GetCoveredLines/ValidateLines.
---

# Create Naysayer Rule

## Before starting

1. Read [docs/RULE_CREATION_GUIDE.md](docs/RULE_CREATION_GUIDE.md) for full detail.
2. Find the closest existing rule and mirror its structure (see archetypes below).
3. Confirm the rule name ends with `_rule` and matches registry + `rules.yaml` exactly.

## Choose an archetype

| Need | Copy from | Package layout |
|------|-----------|----------------|
| Auto-approve safe/metadata files | `internal/rules/common/metadata_rule.go` | `common` or embed `common.NewBaseRule` |
| Single-file, path/pattern checks | `internal/rules/service_account_rule.go` | `internal/rules/<name>_rule.go` |
| Section + MR context (warehouses, consumers) | `internal/rules/warehouse/`, `internal/rules/dataproduct_consumer/` | `internal/rules/<name>/` with `rule.go`, optional `types.go`, `validator.go` |
| Full-file YAML CR validation | `internal/rules/tag/`, `internal/rules/masking/` | Subpackage + `Validator`, `SetMRContext` |

**Default for non-trivial logic:** subdirectory under `internal/rules/<name>/`.

## Required interface

Every rule implements `shared.Rule` in [internal/rules/shared/types.go](internal/rules/shared/types.go):

- `Name() string` — stable ID, e.g. `tag_rule`
- `Description() string`
- `GetCoveredLines(filePath, fileContent string) []LineRange` — return `nil` if rule does not apply
- `ValidateLines(filePath, fileContent string, lineRanges []LineRange) (DecisionType, string)` — `shared.Approve` or `shared.ManualReview`

Optional: `ContextAwareRule` with `SetMRContext(*MRContext)` when the rule needs other MR files (see `tag`, `codeowners`, `warehouse`).

Embed `common.BaseRule` via `common.NewBaseRule(name, description)` for name/description and `GetFullFileCoverage()`.

## Implementation checklist

```
- [ ] 1. Implement rule (correct archetype)
- [ ] 2. Unit tests: table-driven `ValidateLines` + `GetCoveredLines` edge cases
- [ ] 3. Register in internal/rules/registry.go → registerBuiltInRules()
- [ ] 4. Wire rules.yaml (see below)
- [ ] 5. If rule-specific config: internal/config/types.go + factory reads config.Load()
- [ ] 6. Optional: docs/rules/<RULE_NAME>.md
- [ ] 7. E2E scenario under e2e/testdata/scenarios/
- [ ] 8. go test ./internal/rules/... && make test-e2e (or targeted -run)
```

## Register the rule

In [internal/rules/registry.go](internal/rules/registry.go), add to `registerBuiltInRules()`:

```go
_ = r.RegisterRule(&RuleInfo{
    Name:        "my_rule",
    Description: "Human-readable description",
    Version:     "1.0.0",
    Factory: func(client gitlab.GitLabClient) shared.Rule {
        return mypackage.NewRule(client) // or NewRule(cfg) if config-driven
    },
    Enabled:  true,
    Category: "validation", // match siblings: warehouse, masking, auto_approval, etc.
})
```

Name must be unique; grep the repo for collisions before registering.

## Wire rules.yaml

Use **`rule_configs`** (not `rule_names`). Shape from [rules.yaml](rules.yaml) and [internal/config/sections.go](internal/config/sections.go):

```yaml
files:
  - name: "my_file_type"
    path: "dataproducts/**/"
    filename: "*.{yaml,yml}"
    parser_type: yaml
    enabled: true
    sections:
      - name: my_section
        yaml_path: .          # or dotted path, e.g. warehouses
        rule_configs:
          - name: my_rule
            enabled: true
        auto_approve: false   # true only when safe to auto-approve on pass
```

**Strict policy:** Any changed file/line not covered by an enabled section → manual review. New file patterns need an explicit `files:` entry; do not rely on implicit coverage.

When enabling a new file type in production `rules.yaml`, set `enabled: true` on the file block and add matching e2e coverage.

## Patterns to follow from existing rules

### GetCoveredLines

- Return `nil` when the rule does not apply to the path/content.
- Full-file rules: use `common.BaseRule.GetFullFileCoverage` or count lines like `tag_rule`.
- Deleted files: still return a minimal range so `ValidateLines` runs (see `tag/rule.go`).

### ValidateLines

- Early exit: `return shared.Approve, "Not a <x> file"` when non-applicable.
- Deletions / security-sensitive ops → `ManualReview` with a clear reason.
- Parse failures → `ManualReview`, not panic.
- Approve messages should be specific enough for MR comments.

### Config-driven rules

Examples: `toc_approval_rule`, `dataproduct_consumer_rule` — read `config.Load()` in the registry `Factory`, not inside `ValidateLines` on every call.

## Tests

**Unit:** `internal/rules/<pkg>/rule_test.go` — table tests for approve vs manual_review; test name/description constants.

**E2E:** [e2e/README.md](e2e/README.md) — `e2e/testdata/scenarios/<NN>_<name>/` with `before/`, `after/`, `scenario.yaml`:

```yaml
name: "my_scenario"
description: "..."
expected:
  decision: "approve"   # or manual_review
  approved: true        # or false
  comment_contains:
    - "expected substring"
```

Enable the rule on the file type in `rules.yaml` (or a test-only override in `e2e/rules.yaml` if the scenario needs it).

## Verify

```bash
go test ./internal/rules/<pkg> -v
go test ./internal/rules/... -v
go test ./e2e -v -run TestE2E_Scenarios/<scenario_name>
```

## Anti-patterns

- Do not implement legacy `Applies` / `ShouldApprove` (outdated; section manager uses `GetCoveredLines` / `ValidateLines`).
- Do not use `rule_names` in YAML — use `rule_configs` with `name` + `enabled`.
- Do not register without `rules.yaml` wiring — rule will never run on MRs.
- Do not auto-approve destructive changes (deletes, privilege grants) without matching existing rules' conservatism.

## Reference map

| Topic | Location |
|-------|----------|
| Full guide | docs/RULE_CREATION_GUIDE.md |
| Rule docs | docs/rules/*.md |
| Interface | internal/rules/shared/types.go |
| Base helpers | internal/rules/common/base.go |
| Config schema | internal/config/sections.go |
| Production wiring | rules.yaml |
| E2E | e2e/README.md, e2e/testdata/scenarios/ |
