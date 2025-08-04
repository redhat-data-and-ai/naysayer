# NAYSAYER API Reference

Complete API documentation for the NAYSAYER webhook service.

## Base Information

- **Service Name:** NAYSAYER Dataproduct Config
- **Version:** yaml-analysis
- **Port:** 3000 (configurable via `PORT` environment variable)
- **Content-Type:** application/json

## Endpoints

### Health Check

**GET /health**

Returns the current status and configuration of the NAYSAYER service.

#### Request

```bash
curl http://localhost:3000/health
```

#### Response

```json
{
  "analysis_mode": "Full YAML analysis",
  "gitlab_token": true,
  "service": "naysayer-dataproduct-config", 
  "status": "healthy",
  "version": "yaml-analysis"
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Service health status (`"healthy"`) |
| `service` | string | Service identifier |
| `version` | string | Current version/mode |
| `analysis_mode` | string | Analysis capability (`"Full YAML analysis"` or `"Limited (no GitLab token)"`) |
| `gitlab_token` | boolean | Whether GitLab token is configured |

### Webhook Endpoint

**POST /webhook**

Main endpoint for processing GitLab merge request webhooks.

#### Request

```bash
curl -X POST http://localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "object_attributes": {
      "iid": 1551
    },
    "project": {
      "id": 106670
    }
  }'
```

#### Request Body

GitLab webhook payload containing merge request information:

```json
{
  "object_attributes": {
    "iid": 1551,
    "title": "Update warehouse size for discounting",
    "state": "opened"
  },
  "project": {
    "id": 106670,
    "name": "dataproduct-config"
  }
}
```

#### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `object_attributes.iid` | integer | Merge request internal ID |
| `project.id` | integer | GitLab project ID |

#### Response - Auto Approved

```json
{
  "auto_approve": true,
  "reason": "all warehouse changes are decreases",
  "summary": "✅ Warehouse decrease(s) - auto-approved",
  "details": "Found 1 warehouse decrease(s)"
}
```

#### Response - Requires Approval

```json
{
  "auto_approve": false,
  "reason": "warehouse increase detected: XSMALL → LARGE",
  "summary": "🚫 Warehouse increase - platform approval required",
  "details": "File: dataproducts/aggregate/discounting/preprod/product.yaml (type: service_account)"
}
```

#### Response - No Changes

```json
{
  "auto_approve": false,
  "reason": "no warehouse changes detected in YAML files",
  "summary": "🚫 No warehouse changes in YAML - requires approval"
}
```

#### Response - Configuration Error

```json
{
  "auto_approve": false,
  "reason": "GitLab token not configured",
  "summary": "🚫 Cannot analyze YAML files - missing GitLab token",
  "details": "Set GITLAB_TOKEN environment variable to enable YAML analysis"
}
```

#### Response - API Error

```json
{
  "auto_approve": false,
  "reason": "Failed to fetch file changes",
  "summary": "🚫 API error - requires manual approval",
  "details": "Error: GitLab API error 401: Unauthorized"
}
```

#### Response - Analysis Error

```json
{
  "auto_approve": false,
  "reason": "YAML analysis failed",
  "summary": "🚫 Analysis error - requires manual approval",
  "details": "Could not analyze warehouse changes: file not found in source branch"
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `auto_approve` | boolean | Whether MR should be auto-approved |
| `reason` | string | Technical reason for the decision |
| `summary` | string | Human-readable summary with emoji |
| `details` | string | Additional context (optional) |

#### Error Response Scenarios

| Scenario | Reason | Summary | When it occurs |
|----------|--------|---------|----------------|
| No GitLab token | `GitLab token not configured` | `🚫 Cannot analyze YAML files - missing GitLab token` | `GITLAB_TOKEN` environment variable not set |
| GitLab API failure | `Failed to fetch file changes` | `🚫 API error - requires manual approval` | Network issues, 401/404 from GitLab API |
| YAML analysis failure | `YAML analysis failed` | `🚫 Analysis error - requires manual approval` | File not found, invalid YAML, deleted branch |
| No warehouse changes | `no warehouse changes detected in YAML files` | `🚫 No warehouse changes in YAML - requires approval` | No product.yaml files modified |
| Warehouse increase | `warehouse increase detected: XSMALL → LARGE` | `🚫 Warehouse increase - platform approval required` | Any warehouse size increase detected |

#### HTTP Status Codes

| Code | Description |
|------|-------------|
| 200 | Success - decision made |
| 400 | Bad Request - invalid JSON or missing fields |
| 500 | Internal Server Error |


## Error Handling

### Invalid JSON Payload

**Request:**
```bash
curl -X POST http://localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -d 'invalid json'
```

**Response:**
```json
{
  "error": "Invalid JSON payload"
}
```
**Status:** 400 Bad Request

### Missing MR Information

**Request:**
```json
{
  "project": {
    "id": 106670
  }
  // Missing object_attributes.iid
}
```

**Response:**
```json
{
  "error": "Missing MR information: missing project ID (106670) or MR IID (0)"
}
```
**Status:** 400 Bad Request

## Authentication

NAYSAYER authenticates with GitLab using the configured token:

```bash
# Environment variable
export GITLAB_TOKEN=tDnsuUeVxy-n3PfhTvQG

# GitLab API calls use Bearer authentication
Authorization: Bearer tDnsuUeVxy-n3PfhTvQG
```

## Rate Limiting

No built-in rate limiting. Consider implementing at the infrastructure level for production deployments.

## CORS

CORS is enabled for all origins by default. Suitable for webhook integrations.

## Examples

### Complete GitLab Webhook Flow

**1. GitLab sends webhook when MR is opened:**
```json
POST /webhook
{
  "object_kind": "merge_request",
  "object_attributes": {
    "id": 1790103,
    "iid": 1551,
    "project_id": 106670,
    "title": "Update warehouse size for discounting preprod",
    "state": "opened",
    "source_branch": "feature-warehouse-update",
    "target_branch": "main"
  },
  "project": {
    "id": 106670,
    "name": "dataproduct-config",
    "path_with_namespace": "dataverse/dataverse-config/dataproduct-config"
  }
}
```

**2. NAYSAYER analyzes changes and responds:**
```json
{
  "auto_approve": false,
  "reason": "warehouse increase detected: XSMALL → LARGE",
  "summary": "🚫 Warehouse increase - platform approval required",
  "details": "File: dataproducts/aggregate/discounting/preprod/product.yaml (type: service_account)"
}
```

**3. GitLab webhook receiver uses response to:**
- Auto-approve the MR (if `auto_approve: true`)
- Add a comment explaining the decision
- Request manual review (if `auto_approve: false`)

### Testing with Different Scenarios

**Scenario 1: Warehouse Decrease (Auto-approve)**
```bash
# This would represent an MR that changes LARGE → MEDIUM
curl -X POST http://localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "object_attributes": {"iid": 123},
    "project": {"id": 106670}
  }'

# Expected: auto_approve: true
```

**Scenario 2: No GitLab Token**
```bash
# Remove token temporarily
unset GITLAB_TOKEN

curl -X POST http://localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "object_attributes": {"iid": 123},
    "project": {"id": 106670}
  }'

# Expected: auto_approve: false, reason: "GitLab token not configured"
```

**Scenario 3: Invalid MR (Testing Error Handling)**
```bash
curl -X POST http://localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "object_attributes": {"iid": 99999},
    "project": {"id": 106670}
  }'

# Expected: auto_approve: false, reason: "Failed to fetch file changes"
```

**Scenario 4: Analysis Failure (YAML Error)**
```bash
# This would represent an MR where YAML analysis fails
curl -X POST http://localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "object_attributes": {"iid": 456},
    "project": {"id": 106670}
  }'

# Expected: auto_approve: false, reason: "YAML analysis failed"
```

## Integration Guide

### GitLab Webhook Configuration

1. **Navigate to your dataproduct-config repository**
2. **Go to Settings → Webhooks**
3. **Configure webhook:**
   - URL: `https://your-naysayer-domain.com/webhook`
   - Trigger: ✅ Merge request events
   - SSL verification: ✅ Enabled

### Processing NAYSAYER Response

**In your GitLab webhook handler:**

```python
import requests

def process_naysayer_decision(mr_data):
    # Call NAYSAYER
    response = requests.post(
        "https://naysayer.your-domain.com/webhook",
        json=mr_data
    )
    
    decision = response.json()
    
    if decision["auto_approve"]:
        # Auto-approve the MR
        approve_merge_request(mr_data["project"]["id"], mr_data["object_attributes"]["iid"])
        add_comment(f"🎉 {decision['summary']}")
    else:
        # Add comment explaining why approval is needed
        add_comment(f"⏸️ {decision['summary']}\nReason: {decision['reason']}")
        request_review()
```

### Monitoring and Alerting

**Key metrics to monitor:**
- Response time for `/webhook` endpoint
- Error rate (5xx responses)
- GitLab API failures
- Auto-approval rate

**Health check for monitoring:**
```bash
# Add to your monitoring system
curl -f http://naysayer:3000/health || exit 1
```

### Logging

NAYSAYER logs key events:
```
2025/07/28 22:08:49 🚀 NAYSAYER Dataproduct Config starting on port 3000
2025/07/28 22:08:49 📁 Analysis mode: Full YAML analysis
2025/07/28 22:08:51 Processing MR: Project=106670, MR=1551
2025/07/28 22:08:52 Decision: auto_approve=false, reason=warehouse increase detected: XSMALL → LARGE
```

Parse these logs for debugging and monitoring.