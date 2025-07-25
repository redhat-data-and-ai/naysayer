# GitLab Merge Request Bot Backend

This application is a backend service for handling GitLab merge request (MR) events. It is built using the [Fiber](https://gofiber.io/) web framework and provides a webhook endpoint to process GitLab events, such as merge request creation. The service is designed to be extensible, allowing custom handlers to process specific events.

**Current Implementation**: NAYSAYER - A self-service approval bot for dataproduct-config repositories.

---

## Features

- **GitLab Webhook Integration**: Processes GitLab merge request events via webhook endpoints
- **Customizable Handlers**: Supports custom business logic for specific event types
- **Environment-Based Configuration**: Flexible configuration via environment variables
- **Pipeline Integration**: Validates pipeline status before approval decisions
- **Health Monitoring**: Built-in health checks and operational visibility
- **Dual Licensing**: Licensed under both Apache 2.0 and MIT licenses for flexibility

---

## NAYSAYER Implementation

NAYSAYER is the current implementation that enforces approval policies for dataproduct-config repositories, based on `0039-self-service-platform.md`.

### **🎯 Core Approval Policies**

#### **Warehouse Changes**
- ✅ **Warehouse decrease ONLY** → Auto-approved and merged immediately
- 🚫 **Warehouse increase** → Platform team approval required  
- 🚫 **Warehouse decrease + other changes** → Platform approval required (must separate into different MRs)

#### **Other Changes**
- 🚫 **New production deployment** → TOC approval required
- 🚫 **Platform migrations** → Platform team approval required
- ✅ **Self-service migrations** → Auto-approved (future)
- 🚧 **Pipeline failures** → Blocked until fixed

#### **Pipeline Requirements** 
- All pipelines must pass before any approval (configurable)
- Configurable allowed states: `success`, `skipped`
- Failed/pending pipelines block all approvals

### **Mock Testing Capabilities**
Test different scenarios with MR titles:
```
"Warehouse from LARGE to SMALL" → ✅ Auto-approved (decrease only)
"Warehouse SMALL to LARGE + migration" → 🚫 Platform approval (mixed changes)
"New production deploy" → 🚫 TOC approval required
"WIP: Pipeline fix" → 🚧 Waiting for pipeline completion
```

---

## How It Works

1. **Webhook Endpoint**: The `/dataproductconfig/review-mr` endpoint listens for GitLab MR events
2. **Event Parsing**: The payload is parsed into a `MergeRequestWebhook` struct
3. **Business Logic**: NAYSAYER's analysis engine processes the MR content and determines approval requirements
4. **Decision Engine**: Returns structured approval decisions based on configured policies
5. **Pipeline Integration**: Validates pipeline status and blocks approvals if needed

---

## Setup and Usage

### Prerequisites

- Go 1.23 or later
- GitLab instance with webhook access
- Environment configuration for your specific policies

### Configuration

#### **Environment Variables**
```bash
# Repository Configuration
DATAPRODUCT_REPO=dataverse/dataverse-config/dataproduct-config
GITLAB_BASE_URL=https://gitlab.cee.redhat.com
GITLAB_TOKEN=your-token-here

# Pipeline Policies  
REQUIRE_PIPELINE_SUCCESS=true
ALLOWED_PIPELINE_STATES=success,skipped
PIPELINE_TIMEOUT_MINUTES=30

# Feature Flags
ENABLE_GITLAB_API=false  # Phase 2
LOG_LEVEL=info

# Server
PORT=3000
```

#### **Webhook Setup**
Configure GitLab webhook to send MR events to:
```
POST /dataproductconfig/review-mr
Content-Type: application/json
```

### Build & Run

```bash
# Install dependencies
go mod tidy && go mod vendor

# Build
go build -o naysayer cmd/main.go

# Run
./naysayer
# OR
go run cmd/main.go
```

### Testing

Run the comprehensive test suite:
```bash
# Make script executable
chmod +x test_naysayer.sh

# Run all tests
./test_naysayer.sh
```

Or test specific scenarios manually:
```bash
# Warehouse decrease only - should auto-approve
curl -X POST localhost:3000/dataproductconfig/review-mr \
  -H "Content-Type: application/json" \
  -d '{"object_kind":"merge_request","object_attributes":{"title":"Warehouse from LARGE to SMALL","action":"open"}}'

# Mixed changes - should require approval  
curl -X POST localhost:3000/dataproductconfig/review-mr \
  -H "Content-Type: application/json" \
  -d '{"object_kind":"merge_request","object_attributes":{"title":"Warehouse LARGE to SMALL + new migration","action":"open"}}'
```

---

## API Endpoints

### **Webhook Handler**
```
POST /dataproductconfig/review-mr
```
Processes GitLab MR webhooks and returns approval decisions.

**Response Example:**
```json
{
  "mr_id": 123,
  "mr_title": "Warehouse from LARGE to SMALL",
  "decision": {
    "requires_approval": false,
    "approval_type": "none", 
    "auto_approve": true,
    "reason": "Warehouse decrease only - auto-approved",
    "summary": "✅ Warehouse decrease only (LARGE → SMALL) - auto-approved for merge"
  },
  "pipeline_status": {
    "status": "success",
    "passed": true
  }
}
```

### **Health Check**
```
GET /health
GET /dataproductconfig/health
```
Returns service status and configuration.

### **Legacy Naysayer**
```
GET /
```
Returns classic naysayer responses for fun.

---

## Architecture

The application follows a modular architecture:

```
naysayer/
├── cmd/main.go              # Application entry point
├── pkg/
│   ├── config/             # Environment-based configuration
│   └── analysis/           # Business logic and decision engine
├── api/
│   ├── handlers/           # Webhook request handlers
│   └── routes/             # HTTP routing configuration
└── test_naysayer.sh        # Comprehensive test suite
```

### **Extending for Custom Use Cases**

The backend is designed to be extensible. To implement your own bot logic:

1. **Create Custom Analysis**: Implement your business logic in `pkg/analysis/`
2. **Configure Policies**: Set environment variables for your specific policies  
3. **Update Handlers**: Modify `api/handlers/` to use your custom analysis
4. **Test Implementation**: Use the test framework to validate your logic

---

## 📋 **NAYSAYER Decision Matrix**

| Change Type | Alone | Mixed | Pipeline Status | Result |
|-------------|-------|--------|-----------------|---------|
| Warehouse ↓ | ✅ Auto | 🚫 Platform | ✅ Success | ✅ **Auto-Merge** |
| Warehouse ↓ | ✅ Auto | 🚫 Platform | ❌ Failed | 🚫 **Blocked** |  
| Warehouse ↑ | 🚫 Platform | 🚫 Platform | ✅ Success | ⏳ **Platform Approval** |
| Production | 🚫 TOC | 🚫 TOC | ✅ Success | ⏳ **TOC Approval** |
| Migration | 🚫 Platform | 🚫 Platform | ✅ Success | ⏳ **Platform Approval** |

### **Warehouse Size Hierarchy**
```
XSMALL → SMALL → MEDIUM → LARGE → XXLARGE
```
*Decreases auto-approve, increases require platform approval*

---

## Roadmap

### **Phase 2: GitLab API Integration** 
- **Sourcebinding Auto-Approval**: Auto-approve sourcebinding-only changes with dataproduct owner approval
- Real GitLab API calls instead of mocks
- Actual MR approval/rejection automation  
- File diff parsing for precise change detection
- Owner-based approval tracking and notifications
- See **[PHASE2_PLAN.md](./PHASE2_PLAN.md)** for complete roadmap

### **Phase 3: Advanced Policies**
- TOC approval workflows
- Self-service migration detection
- Enhanced validation rules
- Audit reporting

### **Framework Enhancements**
- Plugin architecture for custom handlers
- Configuration UI
- Multi-repository support
- Advanced webhook routing

---

## Documentation

- **[PHASE1_IMPLEMENTATION.md](./PHASE1_IMPLEMENTATION.md)** - Complete implementation guide
- **[CHANGES.md](./CHANGES.md)** - Technical change summary  
- **[TESTING.md](./TESTING.md)** - Comprehensive testing guide
- **[0039-self-service-platform.md](./0039-self-service-platform.md)** - Original design document

---

## License

This project is dual-licensed under the Apache 2.0 and MIT licenses. You may choose either license to use this software. See [LICENSE](LICENSE) and [LICENSE-MIT](LICENSE-MIT) for details.
