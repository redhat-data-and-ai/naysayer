# 🛡️ Naysayer - GitLab MR Validation System

A GitLab webhook service that automatically validates merge requests using configurable rules, helping teams maintain quality and compliance through smart automation.

> **🎯 Smart Decisions**: Auto-approves safe changes, flags risky ones for human review

## 🚀 What Naysayer Does

Naysayer analyzes GitLab merge requests and automatically:
- ✅ **Auto-approves** safe changes (cost reductions, compliant configurations)
- ⚠️ **Flags for review** risky changes (cost increases, security violations)
- 🔍 **Validates** file content against organizational policies
- 📝 **Documents** decisions with clear reasoning

## 🛡️ Validation Rules

Naysayer includes built-in rules for:
- **🏢 Cost Control** - Warehouse configuration validation
- **🔒 Security** - Service account compliance  
- **⚖️ TOC Approval** - TOC oversight for production deployments
- **📄 Documentation** - Metadata and docs validation

> **📚 Complete Rule Details**: See [Rules Documentation](docs/rules/README.md) for what gets validated, troubleshooting, and examples.

## 🏗️ How It Works

Naysayer uses **Section-Based Validation Architecture** for granular, configurable rule processing:

```mermaid
graph TD
    A[📥 GitLab MR] --> B[📄 File Analysis]
    B --> C{Section-Based Config?}
    C -->|Yes| D[🧩 YAML Section Parsing]
    C -->|No| E[📏 Traditional Validation]
    
    D --> F[Extract File Sections]
    F --> G[Section 1: warehouses]
    F --> H[Section 2: service_account]
    F --> I[Section N: custom]
    
    G --> J[🏢 Apply warehouse_rule]
    H --> K[🔒 Apply service_account_rule]
    I --> L[🎯 Apply section rules]
    
    E --> M[Apply all rules to file]
    
    J --> N{All Sections Valid?}
    K --> N
    L --> N
    M --> O{All Rules Pass?}
    
    N -->|✅ All sections pass| P[🎉 Auto-Approve MR]
    N -->|❌ Section failures| Q[🔍 Manual Review Required]
    O -->|✅ All rules pass| P
    O -->|❌ Rule failures| Q
    
    classDef startEnd fill:#e1f5fe,stroke:#01579b,stroke-width:2px,color:#000
    classDef process fill:#f3e5f5,stroke:#4a148c,stroke-width:2px,color:#000
    classDef decision fill:#fff3e0,stroke:#e65100,stroke-width:2px,color:#000
    classDef section fill:#e8f5e8,stroke:#2e7d32,stroke-width:2px,color:#000
    classDef rule fill:#fff8e1,stroke:#f57f17,stroke-width:2px,color:#000
    classDef success fill:#e8f5e8,stroke:#388e3c,stroke-width:3px,color:#000
    classDef review fill:#ffebee,stroke:#d32f2f,stroke-width:2px,color:#000
    
    class A,P startEnd
    class B,D,E,F,M process
    class C,N,O decision
    class G,H,I section
    class J,K,L rule
    class P success
    class Q review
```

**🆕 Section-Based Features**:
- 🎯 **Granular Control**: Rules target specific YAML sections (e.g., `warehouses`, `service_account.dbt`)
- 🧩 **Configurable Mapping**: Rules and sections defined in `rules.yaml` configuration
- 📊 **Coverage Tracking**: Ensures all sections are validated by appropriate rules
- 🔧 **Modular Validation**: Different rules can validate different sections of the same file
- 📝 **Enhanced Reporting**: Section-by-section validation results with line-level detail

## 🚀 Quick Start

### 1. Deploy Naysayer
```bash
# Deploy to Kubernetes/OpenShift
kubectl apply -f config/

# Or run locally for development
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
make build && make run
```

### 2. Configure GitLab Webhook
1. Go to GitLab project → **Settings** → **Webhooks**
2. Add URL: `https://your-naysayer-domain.com/webhook`
3. Select **"Merge request events"**
4. Save configuration

### 3. Test It
Create an MR with configuration changes and Naysayer will automatically validate and provide feedback.

**Result**: Safe changes get auto-approved, risky changes get flagged for review

## ⚙️ Configuration

Basic configuration via environment variables:

```bash
# Required
GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx

# Optional
GITLAB_BASE_URL=https://gitlab.com
PORT=3000

# Rule toggles
WAREHOUSE_RULE_ENABLED=true
```

> **📖 Complete Configuration**: See [Development Setup Guide](docs/DEVELOPMENT_SETUP.md) for all rule-specific settings.

## 🎯 Auto-Approval Logic

### ✅ **When MRs Get Auto-Approved**

| **Change Type** | **Risk Level** | **Auto-Approval** | **Business Rationale** |
|-----------------|----------------|-------------------|------------------------|
| Documentation | 🟢 **Low** | ✅ Always | Zero operational risk |
| Warehouse Reduction | 🟢 **Low** | ✅ Yes | Cost savings (~$50k/month) |
| Service Account (Astro) | 🟡 **Medium** | ✅ Conditional | Automated accounts with naming compliance |
| Warehouse Increase | 🟡 **Medium** | ❌ Never | Requires budget approval |
| Service Account (Other) | 🔴 **High** | ❌ Never | Security review required |
| **New Product (Prod)** | 🔴 **High** | ❌ Never | Requires TOC governance approval |

### 🚫 **Manual Review Triggers**

- **Cost Increases** - Warehouse size increases require budget approval
- **New Production Deployments** - New product.yaml files in preprod/prod require TOC approval
- **Security Violations** - Hardcoded secrets, invalid domains
- **Configuration Errors** - YAML syntax errors, missing fields
- **Uncovered Changes** - Lines not validated by any rule

> **🔍 Compliance Details**: For complete audit trails, risk matrices, and compliance procedures, see detailed rule documentation.

## 📚 Documentation

### 🏗️ Architecture
- 🆕 **[Section-Based Architecture](docs/SECTION_BASED_ARCHITECTURE.md)** - Complete architecture overview with Mermaid diagrams
- 📡 **[API Reference](docs/API_REFERENCE.md)** - Updated endpoints and configuration

### 👥 For Users
- 🏠 **[Rules Overview](docs/rules/README.md)** - Understand what gets validated
- 🔧 **[Troubleshooting Guide](docs/TROUBLESHOOTING.md)** - Fix common issues
- ⚙️ **[Configuration Guide](docs/DEVELOPMENT_SETUP.md)** - Environment setup

### 👨‍💻 For Developers  
- 🎯 **[Rule Creation Guide](docs/RULE_CREATION_GUIDE.md)** - Build new validation rules
- 🧪 **[Development Setup Guide](docs/DEVELOPMENT_SETUP.md)** - Testing strategies and development setup

### 🚀 For Operators
- 🐳 **[Deployment Guide](DEPLOYMENT.md)** - Production setup
- 📊 **[Troubleshooting Guide](docs/TROUBLESHOOTING.md)** - Health checks and debugging

## 🛠️ Development

### Quick Development Setup
```bash
# Clone and setup
git clone https://github.com/your-org/naysayer.git
cd naysayer && go mod tidy

# Setup code quality tools
./scripts/setup-quality.sh

# Run code quality checks
make build

# Start development server
export GITLAB_TOKEN=your-token
go run cmd/main.go
```

### Project Structure
```
naysayer/
├── internal/rules/           # Rule engine and validation logic
│   └── warehouse/           # Warehouse configuration validation  
├── docs/                    # Complete documentation
│   ├── rules/              # User-facing rule guides
│   └── templates/          # Developer templates
└── config/                 # Kubernetes/OpenShift manifests
```

### Code Quality
```bash
# Run individual code quality checks
make lint       # Run golangci-lint
make lint-fix   # Run golangci-lint with automatic fixes
make fmt        # Format code
make vet        # Run go vet
make test       # Run tests

# Run all at once
make lint fmt vet test
```

### Adding New Rules
```bash
# 1. Create from template
mkdir internal/rules/myrule
# Follow the Rule Creation Guide for detailed template usage

# 2. Follow the Rule Creation Guide
# See: docs/RULE_CREATION_GUIDE.md
```

## 🔒 Security & Compliance

- **Minimal Permissions**: GitLab token only needs `read_repository` scope
- **Audit Trail**: All decisions logged with detailed reasoning  
- **Input Validation**: All webhook payloads validated
- **Policy Enforcement**: Consistent application of organizational standards

## 🎯 Benefits

- **⚡ Faster Reviews**: Safe changes approved automatically
- **🛡️ Risk Reduction**: Automated detection of policy violations  
- **📏 Consistency**: Uniform application of organizational standards
- **🔍 Transparency**: Clear explanations for all decisions

## 🚀 Deployment

**Production**: See [Deployment Guide](DEPLOYMENT.md) for complete Kubernetes/OpenShift setup

**Container**: `quay.io/ddis/naysayer:latest`

**Health Check**: `GET /health`

## 🤝 Contributing

1. Read [Rule Creation Guide](docs/RULE_CREATION_GUIDE.md)
2. Follow development guidelines and patterns
3. Update documentation in `docs/rules/`

---

**🚀 Ready to get started?** 
- **Users**: Check [Rules Documentation](docs/rules/README.md) to understand what Naysayer validates
- **Developers**: See [Rule Creation Guide](docs/RULE_CREATION_GUIDE.md) to build custom rules
- **Operators**: Follow [Deployment Guide](DEPLOYMENT.md) for production setup