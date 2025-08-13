# NAYSAYER - Dataproduct Config Review Bot

A self-service GitLab webhook for automatically reviewing warehouse size changes in dataproduct configurations.

## Purpose

NAYSAYER helps the data platform team by automatically approving merge requests that only **decrease** warehouse sizes in `product.yaml` files, while requiring manual review for increases.

**Self-Service Rules:**
- ✅ **Warehouse size decrease** (LARGE → SMALL) → Auto-approve
- 🚫 **Warehouse size increase** (SMALL → LARGE) → Platform approval needed
- 🚫 **No warehouse changes** → Standard review process

## Quick Start

1. **Build and run locally**:
   ```bash
   make build
   export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
   make run
   ```

2. **Deploy to Kubernetes/OpenShift**:
   ```bash
   # Configure GitLab token in config/secrets.yaml
    kubectl apply -f config/
   ```

3. **Configure GitLab webhook** in your dataproduct-config repository:

   - URL: `https://your-naysayer-domain.com/webhook`
   - Trigger: Merge Request events
   - Secret: (optional)

## How It Works

### Working Diagram and Code walkthrough

```mermaid
flowchart TB
    %% External System
    GitLab["Redhat GitLab"]:::external
    GitLab -->|"Merge Request Event"| Webhook["POST /webhook (Webhook Receiver)"]:::transport

  subgraph "NAYSAYER Container / Pod"
        direction TB
        ConfigLoader["Config Loader 
        (internal/config)"]:::infra
        Logger["Structured Logger (internal/logging)"]:::infra
        Health["GET /health
        (Health Checker)"]:::transport
        Webhook
        RulesManager["Rules Manager  (internal/rules/manager)"]:::business
        WarehouseAnalyzer["Warehouse Analyzer \n(internal/rules/warehouse)"]:::business
        Decision{"All decreases?"}:::business
        AutoApprove["Auto-approve Path"]:::business
        ManualReview["Manual review Path"]:::business
        GitLabClient["GitLab API Client (internal/gitlab/client)"]:::infra
    end

    %% ConfigMap/Secrets
    ConfigMap[(ConfigMap/Secrets Config)]:::infra
    ConfigMap -->|loads| ConfigLoader

    %% Internal Flow
    Webhook -->|decode payload| RulesManager
    RulesManager -->|invoke analyzer| WarehouseAnalyzer
    WarehouseAnalyzer -->|evaluate diffs| Decision
    Decision -->|yes| AutoApprove
    Decision -->|no| ManualReview
    AutoApprove -->|POST approval| GitLabClient
    GitLabClient -->|GET changes| RulesManager

    %% Health Check Flow
    Health -->|returns 200| GitLab

    %% Click Events
    click Webhook "https://github.com/redhat-data-and-ai/naysayer/blob/main/cmd/main.go"
    click Health "https://github.com/redhat-data-and-ai/naysayer/blob/main/internal/webhook/health.go"
    click ConfigLoader "https://github.com/redhat-data-and-ai/naysayer/blob/main/internal/config/config.go"
    click ConfigMap "https://github.com/redhat-data-and-ai/naysayer/tree/main/config/"
    click GitLabClient "https://github.com/redhat-data-and-ai/naysayer/blob/main/internal/gitlab/client.go"
    click RulesManager "https://github.com/redhat-data-and-ai/naysayer/blob/main/internal/rules/manager.go"
    click WarehouseAnalyzer "https://github.com/redhat-data-and-ai/naysayer/blob/main/internal/rules/warehouse/analyzer.go"
    click Logger "https://github.com/redhat-data-and-ai/naysayer/blob/main/internal/logging/logger.go"

    %% Styles
    classDef external fill:#e0f7e9,stroke:#2e7d32,color:#2e7d32;
    classDef transport fill:#e3f2fd,stroke:#1565c0,color:#1565c0;
    classDef business fill:#fff8e1,stroke:#f9a825,color:#f9a825;
    classDef infra fill:#fff3e0,stroke:#ef6c00,color:#ef6c00;
```

NAYSAYER analyzes changes in `product.yaml` files within the dataproduct-config repository structure:

```
dataproducts/
├── source/product-name/env/product.yaml
├── aggregate/product-name/env/product.yaml
└── platform/product-name/env/product.yaml
```

### Dataproduct YAML Format

```yaml
name: your-dataproduct
kind: source-aligned  # or aggregated
rover_group: dataverse-source-your-dataproduct
warehouses:
  - type: user
    size: XSMALL          # ← NAYSAYER analyzes this
  - type: service_account
    size: LARGE           # ← and this
```

### Approval Logic

- **Auto-approve**: Warehouse size decreases only
  - `X6LARGE(10) → X5LARGE(9)` ✅
  - `X5LARGE(9) → X4LARGE(8)` ✅
  - `X4LARGE(8) → X3LARGE(7)` ✅
  - `X3LARGE(7) → XXLARGE(6)` ✅
  - `XXLARGE(6) → XLARGE(5)` ✅
  - `XLARGE(5) → LARGE(4)` ✅
  - `LARGE(4) → MEDIUM(3)` ✅
  - `MEDIUM(3) → SMALL(2)` ✅
  - `SMALL(2) → XSMALL(1)` ✅

- **Require approval**: Any increase or no warehouse changes
  - `SMALL(2) → MEDIUM(3)` ❌ (platform approval needed)
  - No warehouse changes ❌ (standard review process)

## Repository Integration

NAYSAYER is designed specifically for the dataproduct-config repository at:
`go/src/gitlab.com/ddis/repos/dataproduct-config`

It understands the DDIS dataproduct structure and focuses only on `product.yaml` files.

## Configuration

**Environment Variables:**
- `GITLAB_TOKEN` - GitLab API token (required for file analysis)
- `GITLAB_BASE_URL` - GitLab instance URL (default: https://gitlab.com)
- `PORT` - Server port (default: 3000)

## Deployment

### Kubernetes/OpenShift

1. **Configure secrets**:
   ```bash
   echo -n "your-gitlab-token" | base64
   # Update gitlab-token in config/secrets.yaml
   ```

2. **Deploy**:
   ```bash
   kubectl apply -f config/
   ```

3. **Image management** (push to Quay):
   ```bash
   make build-image
   make push-image
   ```

### Container Image

- Registry: `quay.io/ddis/naysayer`
- Tag: `latest`

## API Endpoints

- `POST /webhook` - GitLab webhook endpoint
- `GET /health` - Health check

## Testing

Basic functionality:
```bash
./test_simple.sh
```

File analysis with real GitLab API:
```bash
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
./test_file_analysis.sh
```

## Self-Service Benefits

- **Faster approvals** for warehouse downsizing
- **Platform team focus** on increases and complex changes  
- **Automated compliance** with resource optimization
- **Clear audit trail** in GitLab MR comments

## How It Works (Technical Details)

### File Analysis Process

1. **Webhook received** → Extract project ID and MR IID
2. **Fetch file changes** → Call GitLab API `/projects/:id/merge_requests/:iid/changes`
3. **Analyze config files** → Look for warehouse changes in YAML/JSON files
4. **Check diff patterns** → Find `-  warehouse: LARGE` → `+  warehouse: SMALL`
5. **Make decision** → Auto-approve only if all changes are decreases

### Supported File Types

- `.yaml` and `.yml` files
- `.json` files
- Looks for `warehouse:` configuration changes

### Example File Change

The bot analyzes diffs like this:

```diff
# config/dataproduct.yaml
- warehouse: LARGE
+ warehouse: SMALL
```

**Result:** ✅ Auto-approved (decrease detected)

```diff
# config/dataproduct.yaml
- warehouse: SMALL  
+ warehouse: LARGE
```

**Result:** 🚫 Requires approval (increase detected)

## Usage

### GitLab Webhook Setup

1. Go to your GitLab project → Settings → Webhooks
2. Add webhook URL: `http://your-server:3000/webhook`
3. Select "Merge request events"
4. Save

### Test It

```bash
# Test with mock GitLab webhook payload
curl -X POST localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "object_attributes": {
      "iid": 123
    },
    "project": {
      "id": 456
    }
  }'

# Response with GitLab token:
# {
#   "auto_approve": true,
#   "reason": "all warehouse changes are decreases",
#   "summary": "✅ Warehouse decrease(s) - auto-approved",
#   "details": "Found 1 warehouse decrease(s)"
# }

# Response without GitLab token:
# {
#   "auto_approve": false,
#   "reason": "GitLab token not configured",
#   "summary": "🚫 Cannot analyze files - missing GitLab token",
#   "details": "Set GITLAB_TOKEN environment variable to enable file analysis"
# }
```

## Warehouse Sizes

```
XSMALL (1) → SMALL (2) → MEDIUM (3) → LARGE (4) → XXLARGE (5)
```

**Decreases** (higher → lower) are auto-approved.  
**Increases** (lower → higher) require approval.

## API Endpoints

- **POST /webhook** - Main webhook endpoint
- **GET /health** - Health check

## Project Structure

```
naysayer/
├── cmd/main.go              # Complete application (360+ lines)
├── go.mod                   # Dependencies (GoFiber + YAML)
├── go.sum
├── Makefile                 # Build commands
└── README.md                # This file
```

## Error Handling

### Common Issues

**Missing GitLab Token:**
```json
{
  "auto_approve": false,
  "reason": "GitLab token not configured",
  "summary": "🚫 Cannot analyze files - missing GitLab token"
}
```

**GitLab API Error:**
```json
{
  "auto_approve": false,
  "reason": "Failed to fetch file changes",
  "summary": "🚫 API error - requires manual approval",
  "details": "Error: GitLab API error 401: Unauthorized"
}
```

**No Warehouse Changes:**
```json
{
  "auto_approve": false,
  "reason": "no warehouse changes detected in files",
  "summary": "🚫 No warehouse changes - requires approval"
}
```

## Deployment

### Docker

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o naysayer cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/naysayer .
EXPOSE 3000
CMD ["./naysayer"]
```

### Environment Setup

```bash
# Set required GitLab token
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx

# Optional: Set custom GitLab URL (for self-hosted)
export GITLAB_BASE_URL=https://gitlab.mycompany.com

# Optional: Set custom port
export PORT=8080
```

### Systemd Service

```ini
# /etc/systemd/system/naysayer.service
[Unit]
Description=NAYSAYER File-Based Webhook Service
After=network.target

[Service]
Type=simple
User=naysayer
ExecStart=/opt/naysayer/naysayer
Environment=PORT=3000
Environment=GITLAB_TOKEN=your_token_here
Environment=GITLAB_BASE_URL=https://gitlab.com
Restart=always

[Install]
WantedBy=multi-user.target
```

## Development

```bash
# Install dependencies
go mod tidy

# Build
go build -o naysayer cmd/main.go

# Run with debug logging
go run cmd/main.go

# Test health endpoint
curl http://localhost:3000/health
```

## Why File-Based Analysis?

**Previous Approach:** Analyzed MR titles for patterns like "Warehouse from LARGE to SMALL"
- ❌ Unreliable (depends on title format)
- ❌ Easy to bypass
- ❌ No validation of actual changes

**Current Approach:** Analyzes actual file diffs for warehouse configuration changes
- ✅ **Accurate** - sees real file changes
- ✅ **Secure** - can't be bypassed with clever titles
- ✅ **Reliable** - works regardless of MR title format
- ✅ **Detailed** - knows which files changed and how

## Troubleshooting

### Check GitLab Token

```bash
# Test your token manually
curl -H "Authorization: Bearer $GITLAB_TOKEN" \
  https://gitlab.com/api/v4/projects/YOUR_PROJECT_ID/merge_requests/YOUR_MR_IID/changes
```

### Verify Webhook Payload

```bash
# Check webhook is sending correct data
curl -X POST localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -d @webhook_payload.json
```

### Debug Mode

Set log level for detailed debugging:
```bash
# Run with verbose logging
go run cmd/main.go 2>&1 | tee naysayer.log
```

## Security

- GitLab API token should have minimal scopes (`read_repository`)
- Use environment variables for sensitive configuration
- Consider webhook signature validation for production use
- Run with restricted user permissions

## Contributing

The goal is to keep this focused on file-based warehouse analysis. Before adding features, ask: "Does this improve warehouse change detection?"

## License

Dual licensed under Apache 2.0 and MIT licenses.
