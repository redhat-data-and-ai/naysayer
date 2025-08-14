# 🛡️ Naysayer - GitLab MR Validation System

A GitLab webhook service that automatically validates merge requests using configurable rules, helping teams maintain quality and compliance through smart automation.

> **🎯 Smart Decisions**: Auto-approves safe changes, flags risky ones for human review

## 🚀 What Naysayer Does

Naysayer analyzes GitLab merge requests and automatically:
- ✅ **Auto-approves** safe changes (cost reductions, compliant configurations)
- ⚠️ **Flags for review** risky changes (cost increases, security violations)
- 🔍 **Validates** file content against organizational policies
- 📝 **Documents** decisions with clear reasoning

## 🛡️ Current Validation Rules

| **Rule** | **Validates** | **Auto-Approves** | **Requires Review** |
|----------|---------------|-------------------|---------------------|
| **🏢 Warehouse** | Data warehouse configs (`product.yaml`) | Cost reductions | Cost increases |

> **📚 Detailed Rule Documentation**: See [Rules Documentation](docs/rules/README.md) for complete rule behaviors and troubleshooting guides.

## 🏗️ How It Works

Naysayer uses **line-level validation** with YAML section-aware parsing to ensure comprehensive coverage:

```mermaid
graph TD
    A[📥 GitLab MR] --> B[📄 File Analysis]
    B --> C[🧩 YAML Section Parsing]
    C --> D{📏 Line Coverage Check}
    D --> E[🏢 Warehouse Rule<br/>Covers entire product.yaml file]
    E --> H{🔍 Rule Coverage?}
    D --> F[❌ No Rule Coverage<br/>For non-warehouse files]
    F --> J
    H -->|✅ All lines covered & approved| I[🎉 Auto-Approve MR]
    H -->|❌ Uncovered lines or failed rules| J[🔍 Manual Review Required]
```

**Key Features**:
- 🎯 **Line-Level Validation**: Every changed line must be validated by at least one rule
- 🧩 **File-Level Coverage**: Warehouse rule validates entire `product.yaml` files
- 📊 **Coverage Enforcement**: Files without rule coverage automatically require manual review
- 📝 **Detailed Reporting**: File-by-file breakdown showing which rules passed/failed

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
Create an MR that modifies:
- `dataproducts/*/product.yaml` (triggers Warehouse Rule)

**Result**: Naysayer automatically approves or requests review based on the changes

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

> **📖 Complete Configuration**: See [Configuration Guide](docs/CONFIGURATION.md) for all rule-specific settings.

## 📚 Documentation

### 👥 For Users
- 🏠 **[Rules Overview](docs/rules/README.md)** - Understand what gets validated
- 🔧 **[Troubleshooting Guide](docs/TROUBLESHOOTING.md)** - Fix common issues
- ⚙️ **[Configuration Guide](docs/CONFIGURATION.md)** - Environment setup

### 👨‍💻 For Developers  
- 🎯 **[Rule Creation Guide](docs/RULE_CREATION_GUIDE.md)** - Build new validation rules
- 🧪 **[Rule Testing Guide](docs/RULE_TESTING_GUIDE.md)** - Testing strategies

### 🚀 For Operators
- 🐳 **[Deployment Guide](docs/DEPLOYMENT.md)** - Production setup
- 📊 **[Monitoring Guide](docs/MONITORING.md)** - Health checks and metrics

## 🛠️ Development

### Quick Development Setup
```bash
# Clone and setup
git clone https://github.com/your-org/naysayer.git
cd naysayer && go mod tidy

# Run tests
make test

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

### Adding New Rules
```bash
# 1. Create from template
mkdir internal/rules/myrule
cp docs/templates/rule_templates/enhanced_basic_rule_template.go.template internal/rules/myrule/rule.go

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

**Production**: See [Deployment Guide](docs/DEPLOYMENT.md) for complete Kubernetes/OpenShift setup

**Container**: `quay.io/ddis/naysayer:latest`

**Health Check**: `GET /health`

## 🤝 Contributing

1. Read [Rule Creation Guide](docs/RULE_CREATION_GUIDE.md)
2. Use [enhanced templates](docs/templates/rule_templates/)
3. Follow [testing guidelines](docs/RULE_TESTING_GUIDE.md)
4. Update documentation in `docs/rules/`

---

**🚀 Ready to get started?** 
- **Users**: Check [Rules Documentation](docs/rules/README.md) to understand what Naysayer validates
- **Developers**: See [Rule Creation Guide](docs/RULE_CREATION_GUIDE.md) to build custom rules
- **Operators**: Follow [Deployment Guide](docs/DEPLOYMENT.md) for production setup