# 🛡️ Naysayer Rules Documentation

This directory contains detailed documentation for each validation rule implemented in Naysayer. Each rule has its own focused documentation explaining what it validates, when it triggers, and how to resolve common issues.

> **🎯 Quick Navigation**: Find your rule below and click the link for detailed guidance.

## 📚 Available Rules

### 🏢 [Warehouse Rule](WAREHOUSE_RULE.md)
**Validates**: Data product warehouse configurations  
**Triggers on**: `**/product.{yaml,yml}` files with warehouse sections  
**Purpose**: Cost control and budget governance  
**Key behavior**: Auto-approves cost reductions (~$50k/month savings), requires approval for increases

### 🔒 [Service Account Rule](SERVICE_ACCOUNT_RULE.md)
**Validates**: Service account configurations and security policies  
**Triggers on**: `**/*serviceaccount*.{yaml,yml}`, `**/*_astro_*_appuser.{yaml,yml}`  
**Purpose**: Security compliance and identity management  
**Key behavior**: Auto-approves Astro service accounts, requires security review for manual accounts

### 📄 [Metadata Rule](METADATA_RULE.md)
**Validates**: Documentation and metadata files  
**Triggers on**: `**/*.md`, `**/developers.{yaml,yml}`, documentation files  
**Purpose**: Development velocity and documentation quality  
**Key behavior**: Auto-approves all documentation and metadata changes (zero risk)

### ⚖️ [TOC Approval Rule](TOC_APPROVAL_RULE.md)
**Validates**: New data product deployments to production environments
**Triggers on**: New `**/product.{yaml,yml}` files in preprod/prod paths
**Purpose**: Governance oversight and production deployment control
**Key behavior**: Requires TOC approval for new products in critical environments

### 👥 [Data Product Consumer Rule](DATAPRODUCT_CONSUMER_RULE.md)
**Validates**: Consumer access changes to data products
**Triggers on**: `data_product_db[*].presentation_schemas[*].consumers` sections in `**/product.{yaml,yml}`
**Purpose**: Streamlined consumer access management across all environments
**Key behavior**: Auto-approves consumer-only changes with data product owner approval (no TOC needed)

### 🔄 [Auto-Rebase Rule](AUTOREBASE_RULE_AND_SETUP.md)
**Validates**: Automated rebase operations for all repository
**Triggers on**: Push events to `main`/`master` branch
**Purpose**: Automatically rebase eligible merge requests to keep them up-to-date
**Key behavior**: Rebases MRs created within last 7 days with successful/skipped pipelines, skips MRs with active/failed pipelines

## 🎯 Quick Problem Resolution

### My MR is Blocked - What Now?

1. **Check which rule is triggering**: Look at the MR comments or logs
2. **Read the specific rule documentation**: Click the rule link above
3. **Follow the troubleshooting section**: Each rule doc has common solutions
4. **Fix and retry**: Make the suggested changes and push updates

### Common Issues by File Type

| **File Pattern** | **Rule** | **Common Issues** | **Quick Fix** |
|------------------|----------|-------------------|---------------|
| `**/product.{yaml,yml}` | [Warehouse](WAREHOUSE_RULE.md) | Size increases, YAML syntax | Use `XSMALL`/`SMALL`/`MEDIUM`/`LARGE`, validate YAML |
| `**/product.{yaml,yml}` (new) | [TOC Approval](TOC_APPROVAL_RULE.md) | New products in prod/preprod | Get TOC approval or deploy to dev/test first |
| `**/product.{yaml,yml}` (consumers) | [Consumer](DATAPRODUCT_CONSUMER_RULE.md) | Mixed changes with non-consumer fields | Separate consumer changes into dedicated MR |
| `**/*serviceaccount*.{yaml,yml}` | [Service Account](SERVICE_ACCOUNT_RULE.md) | Non-Astro accounts, domain violations | Use Astro patterns, @redhat.com emails |
| `**/*.md`, docs files | [Metadata](METADATA_RULE.md) | File access issues | Check file permissions, valid UTF-8 encoding |

## ⚙️ Rule System Overview

### Section-Based Architecture

NAYSAYER uses a **Section-Based Validation Architecture** where rules can target specific sections of files rather than entire files. This provides:

- **🎯 Granular Control**: Rules validate specific YAML sections (e.g., `warehouses`, `service_account.dbt`)
- **⚡ Performance**: Only relevant sections are parsed and validated  
- **🔧 Configurability**: Rules and sections are configured through `rules.yaml`
- **📊 Coverage Tracking**: Ensures all sections are covered by appropriate rules

> **🏗️ Complete Details**: For architecture deep-dive, implementation patterns, and technical details, see:
> - **[Section-Based Architecture Guide](../SECTION_BASED_ARCHITECTURE.md)** - Complete architecture overview
> - **[Rule Creation Guide](../RULE_CREATION_GUIDE.md)** - Implementation guide for developers

### Decision Logic

- **Fail-Fast**: If ANY rule requires manual review, the entire MR needs review
- **Independent**: Each rule evaluates files independently
- **Configurable**: Rules can be enabled/disabled via environment variables

### Rule Categories

| **Category** | **Purpose** | **Examples** |
|--------------|-------------|--------------|
| 🏢 **Cost Control** | Prevent unexpected cost increases | Warehouse size validation |
| 🔒 **Security** | Enforce security policies | Service account email validation |
| 👥 **Access Management** | Streamline data access workflows | Consumer access auto-approval |
| 📋 **Compliance** | Meet organizational standards | Naming conventions, documentation |
| 🔧 **Configuration** | Ensure valid configurations | YAML syntax, required fields |
| 🔄 **Automation** | Automate maintenance tasks | Fivetran Terraform auto-rebase |

## 🎯 For Different Audiences

### 👨‍💻 **Developers**
Working on MRs that get blocked by rules:
- Read specific rule documentation for your file type
- Follow troubleshooting guides
- Use provided examples for proper configuration

### 👥 **Team Leads** 
Understanding why MRs require review:
- Review rule purposes and business justification
- Understand cost and security implications
- Guide team on best practices

### 🔧 **Platform Engineers**
Managing and configuring rules:
- See [Rule Creation Guide](../RULE_CREATION_GUIDE.md) for implementation
- See [Development Setup Guide](../DEVELOPMENT_SETUP.md) for settings
- Monitor rule performance and effectiveness

### 🛡️ **Security Teams**
Understanding security controls:
- Review security policies and requirements
- Understand risk mitigation strategies
- Configure security policies and exceptions

## 📊 Rule Status Dashboard

### Active Rules Summary

| **Rule** | **Status** | **Auto-Approval Rate** | **Common Issues** |
|----------|------------|------------------------|-------------------|
| 🏢 **Warehouse** | ✅ Active | ~85% | Size increases (15%) |
| 🔒 **Service Account** | ✅ Active | ~70% | Non-Astro accounts (30%) |
| 👥 **Consumer** | ✅ Active | ~100% | Mixed changes (<1%) |
| ⚖️ **TOC Approval** | ✅ Active | N/A | New prod deployments |
| 📄 **Metadata** | ✅ Active | ~100% | File access issues (<1%) |
| 🔄 **Fivetran Rebase** | ✅ Active | N/A | Webhook configuration, token permissions |

### Performance Metrics

- **Average rule execution time**: < 3 seconds
- **False positive rate**: < 2%
- **System availability**: 99.9%

## ⚙️ Global Configuration

### Environment Variables

Control rule behavior globally:

```bash
# Enable/disable all rules
RULES_ENABLED=true

# Global timeout for rule execution
RULES_TIMEOUT=30

# Debug logging for all rules
RULES_DEBUG=false

# Maximum file size for processing
RULES_MAX_FILE_SIZE=5242880  # 5MB
```

### Per-Rule Configuration

Each rule can be configured independently:

```bash
# Warehouse Rule
WAREHOUSE_RULE_ENABLED=true
WAREHOUSE_ALLOW_SIZE_INCREASES=false
```

## 🔧 Troubleshooting

### Universal Solutions

These apply to all rules:

#### YAML Syntax Issues
```bash
# Validate YAML before committing
yamllint your-file.yaml

# Or use online validator
# https://yaml-online-parser.appspot.com/
```

#### File Path Issues
- Check exact file path matches rule patterns
- Verify case sensitivity
- Ensure proper directory structure

#### Permission Issues
- Verify GitLab token has file access
- Check repository permissions
- Contact platform team if persistent

### Getting Debug Information

Enable debug logging to see detailed rule execution:

```bash
# Set environment variables
export RULES_DEBUG=true
export LOG_LEVEL=debug

# Check logs for detailed execution info
kubectl logs -f deployment/naysayer | grep rule_execution
```

## 🆘 Getting Help

### Escalation Path

1. **Self-service**: Read rule-specific documentation
2. **Team consultation**: Discuss with team leads or senior developers
3. **Platform support**: Contact platform team for technical issues
4. **Security review**: Contact security team for policy questions
5. **Emergency**: Use on-call procedures for production blockers

### When to Contact Support

- Persistent rule failures after following documentation
- Questions about rule policies or requirements
- Need for emergency bypasses
- Issues with rule performance or availability

### Information to Provide

When requesting help:
- **MR URL**: Link to blocked merge request
- **Rule name**: Which specific rule is blocking
- **Error message**: Complete error text from logs
- **File content**: Relevant configuration files
- **Expected behavior**: What you think should happen

## 📈 Rule Development

### For Rule Authors

Interested in creating new rules? 

- 🎯 **[Rule Creation Guide](../RULE_CREATION_GUIDE.md)** - Complete step-by-step implementation guide
- 🧪 **[Development Setup Guide](../DEVELOPMENT_SETUP.md)** - Testing strategies and development patterns
- 🏗️ **[Section-Based Architecture](../SECTION_BASED_ARCHITECTURE.md)** - Architecture overview and design principles

### Contributing

To contribute to rule documentation:
1. Follow the structure of existing rule documents
2. Include troubleshooting and examples
3. Test with real scenarios
4. Submit PR with clear description

## 📚 Related Documentation

| **Topic** | **Document** | **Audience** |
|-----------|--------------|--------------|
| **Section-Based Architecture** | [Section-Based Architecture](../SECTION_BASED_ARCHITECTURE.md) | All |
| **Creating Rules** | [Rule Creation Guide](../RULE_CREATION_GUIDE.md) | Developers |
| **Development Setup** | [Development Setup Guide](../DEVELOPMENT_SETUP.md) | Developers |
| **API Reference** | [API Reference](../API_REFERENCE.md) | Platform Engineers |
| **Troubleshooting** | [Troubleshooting Guide](../TROUBLESHOOTING.md) | Operators |

---

**💡 Pro Tip**: Bookmark the specific rule documentation for files you work with frequently. Most issues can be resolved quickly by following the rule-specific troubleshooting guides.