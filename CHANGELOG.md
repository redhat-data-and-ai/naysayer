# Changelog

All notable changes to Naysayer will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Documentation
- Added comprehensive deployment documentation

## How to Use This Changelog

When creating a new release:

1. Move items from `[Unreleased]` to a new version section
2. Create the version section with format: `## [X.Y.Z] - YYYY-MM-DD`
3. Organize changes under categories:
   - **Added** - New features
   - **Changed** - Changes to existing functionality
   - **Deprecated** - Soon-to-be removed features
   - **Removed** - Removed features
   - **Fixed** - Bug fixes
   - **Security** - Security improvements

Example:
```markdown
## [1.0.0] - 2025-01-15

### Added
- MR Validation & Auto-Approval endpoint
- Fivetran Terraform Auto-Rebase endpoint
- Stale MR Cleanup endpoint
- Section-based YAML validation
- Warehouse cost control rule
- Service account security rule
- Consumer access management rule
- TOC approval governance rule
- Metadata auto-approval rule

### Documentation
- Complete rule creation guide
- E2E testing framework documentation
- Architecture documentation
```
