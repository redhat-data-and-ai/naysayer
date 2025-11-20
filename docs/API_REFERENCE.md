# NAYSAYER API Reference

## 🌐 **API Endpoints Overview**

NAYSAYER provides HTTP endpoints for webhook processing, health monitoring, and status checking.

**Base URL**: `https://your-naysayer-domain.com`

> **🏗️ Architecture Details**: For system architecture and validation flow, see [Section-Based Architecture Guide](SECTION_BASED_ARCHITECTURE.md)

## 📡 **Webhook Endpoints**

### **POST /fivetran-terraform-rebase**

Webhook endpoint for Fivetran Terraform repository auto-rebase feature.

**Description**: Automatically rebases eligible merge requests when code is pushed to the main branch. Only processes push events to `main` or `master` branches.

**Request Headers**:
```http
Content-Type: application/json
```

**Request Body**: GitLab push webhook payload (JSON)

**Example Request**:
```bash
curl -X POST https://your-naysayer-domain.com/fivetran-terraform-rebase \
  -H "Content-Type: application/json" \
  -d '{
    "object_kind": "push",
    "ref": "refs/heads/main",
    "project": {
      "id": 456
    },
    "commits": [
      {
        "id": "abc123",
        "message": "Update configuration"
      }
    ]
  }'
```

**Response Codes**:
- `200 OK` - Webhook processed successfully
- `400 Bad Request` - Invalid request format, unsupported event type, or non-main branch push
- `500 Internal Server Error` - Internal processing error

**Success Response Example** (200):
```json
{
  "webhook_response": "processed",
  "status": "completed",
  "project_id": 456,
  "branch": "main",
  "total_mrs": 5,
  "eligible_mrs": 2,
  "successful": 2,
  "failed": 0,
  "skipped": 3,
  "skip_details": [
    {
      "mr_iid": 123,
      "reason": "pipeline_running",
      "pipeline_id": 45678
    },
    {
      "mr_iid": 124,
      "reason": "too_old",
      "created_at": "2025-11-01T10:00:00Z"
    },
    {
      "mr_iid": 125,
      "reason": "pipeline_failed",
      "pipeline_id": 45679
    }
  ]
}
```

**Response with Failures** (200):
```json
{
  "webhook_response": "processed",
  "status": "completed",
  "project_id": 456,
  "branch": "main",
  "total_mrs": 3,
  "eligible_mrs": 3,
  "successful": 2,
  "failed": 1,
  "skipped": 0,
  "failures": [
    {
      "mr_iid": 456,
      "error": "rebase failed: rebase already in progress or conflicts detected"
    }
  ]
}
```

**Non-Main Branch Response** (200):
```json
{
  "webhook_response": "processed",
  "status": "skipped",
  "reason": "Push to feature/update branch, only main/master triggers rebase",
  "branch": "feature/update"
}
```

**Response Fields**:
| Field | Type | Description |
|-------|------|-------------|
| `webhook_response` | string | Always `"processed"` |
| `status` | string | `"completed"` or `"skipped"` |
| `project_id` | number | GitLab project ID |
| `branch` | string | Branch name (`main` or `master`) |
| `total_mrs` | number | Total number of open MRs found |
| `eligible_mrs` | number | Number of MRs eligible for rebase |
| `successful` | number | Number of successfully rebased MRs |
| `failed` | number | Number of failed rebase attempts |
| `skipped` | number | Number of MRs skipped (not eligible) |
| `skip_details` | array | Details about skipped MRs (if any) |
| `failures` | array | Details about failed rebases (if any) |

**Skip Details Object**:
| Field | Type | Description |
|-------|------|-------------|
| `mr_iid` | number | Merge request IID |
| `reason` | string | Skip reason (`pipeline_running`, `pipeline_pending`, `pipeline_failed`, `too_old`) |
| `pipeline_id` | number | Pipeline ID (if skipped due to pipeline status) |
| `created_at` | string | MR creation date (if skipped due to age) |

**Failure Object**:
| Field | Type | Description |
|-------|------|-------------|
| `mr_iid` | number | Merge request IID |
| `error` | string | Error message describing the failure |

**Eligibility Criteria**:
- MR must be created within the last **7 days**
- MR pipeline status must be `success`, `skipped`, `canceled`, or `null` (no pipeline)
- MRs with `running`, `pending`, or `failed` pipelines are skipped
- Only push events to `main` or `master` branches trigger rebase operations

**Error Response Examples**:

**400 - Unsupported Event Type**:
```json
{
  "error": "Unsupported event type: merge_request. Only push events are supported."
}
```

**400 - Invalid Content Type**:
```json
{
  "error": "Content-Type must be application/json, got: text/plain"
}
```

**400 - Missing Project Information**:
```json
{
  "error": "Missing project information"
}
```

**500 - Failed to List MRs**:
```json
{
  "error": "Failed to list open MRs: GitLab API error 401: Unauthorized",
  "project_id": 456
}
```

### **POST /dataverse-product-config-review**

Main webhook endpoint for GitLab merge request events.

**Description**: Processes GitLab webhook events and automatically reviews dataproduct configuration changes.

**Request Headers**:
```http
Content-Type: application/json
X-Gitlab-Event: Merge Request Hook
X-Gitlab-Token: <optional-webhook-secret>
```

**Request Body**: GitLab merge request webhook payload (JSON)

**Example Request**:
```bash
curl -X POST https://your-naysayer-domain.com/dataverse-product-config-review \
  -H "Content-Type: application/json" \
  -H "X-Gitlab-Event: Merge Request Hook" \
  -d '{
    "object_kind": "merge_request",
    "object_attributes": {
      "id": 123,
      "iid": 456,
      "title": "Update warehouse configuration",
      "state": "opened",
      "target_branch": "main",
      "source_branch": "feature/warehouse-update",
      "author": {
        "username": "developer"
      }
    },
    "project": {
      "id": 789,
      "name": "dataproduct-config"
    },
    "changes": {
      "total": 1
    }
  }'
```

**Response Codes**:
- `200 OK` - Webhook processed successfully
- `400 Bad Request` - Invalid request format or unsupported event type
- `401 Unauthorized` - GitLab API authentication failed
- `500 Internal Server Error` - Internal processing error

**Success Response Example** (200):
```json
{
  "status": "success",
  "message": "Merge request processed successfully",
  "mr_id": 456,
  "project_id": 789,
  "decision": "auto_approve",
  "reason": "Warehouse size decrease detected (LARGE → SMALL)"
}
```

**Error Response Examples**:

**400 - Unsupported Event Type**:
```json
{
  "error": "Unsupported event type: push. Only merge_request events are supported"
}
```

**400 - Invalid Content Type**:
```json
{
  "error": "Content-Type must be application/json"
}
```

**400 - Missing object_kind**:
```json
{
  "error": "Missing object_kind"
}
```

## 🏥 **Health Monitoring Endpoints**

### **GET /health**

Comprehensive health status endpoint.

**Description**: Returns detailed health information including configuration status, SSL info, and system metrics.

**Request**: No parameters required

**Example Request**:
```bash
curl -s https://your-naysayer-domain.com/health | jq '.'
```

**Response** (200):
```json
{
  "status": "healthy",
  "service": "naysayer-webhook",
  "version": "v1.0.0",
  "uptime_seconds": 3600,
  "timestamp": "2024-01-15T10:30:00Z",
  "analysis_mode": "Full analysis enabled",
  "security_mode": "Token verification available",
  "gitlab_token": true,
  "webhook_secret": true,
  "ssl_info": {
    "ssl_enabled": true,
    "protocol": "http",
    "forwarded_proto": "https",
    "ssl_status": "✅ SSL properly configured"
  }
}
```

**Response Fields**:
| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Overall health status (`"healthy"`) |
| `service` | string | Service identifier |
| `version` | string | Application version |
| `uptime_seconds` | number | Service uptime in seconds |
| `timestamp` | string | Current timestamp (ISO 8601) |
| `analysis_mode` | string | Current analysis capabilities |
| `security_mode` | string | Webhook security configuration |
| `gitlab_token` | boolean | GitLab token availability |
| `webhook_secret` | boolean | Webhook secret configuration |
| `ssl_info` | object | SSL/TLS configuration details |

**SSL Info Object**:
| Field | Type | Description |
|-------|------|-------------|
| `ssl_enabled` | boolean | Whether SSL is detected |
| `protocol` | string | Request protocol (`"http"` or `"https"`) |
| `forwarded_proto` | string | X-Forwarded-Proto header value |
| `ssl_status` | string | SSL configuration status message |
| `ssl_warnings` | array | SSL warnings (if any) |

### **GET /ready**

Kubernetes readiness probe endpoint.

**Description**: Returns readiness status for load balancers and orchestrators. Used by Kubernetes for readiness probes.

**Request**: No parameters required

**Example Request**:
```bash
curl -s https://your-naysayer-domain.com/ready | jq '.'
```

**Success Response** (200):
```json
{
  "ready": true,
  "service": "naysayer-webhook",
  "timestamp": "2024-01-15T10:30:00Z",
  "gitlab_token": true,
  "webhook_secret": true,
  "ssl_info": {
    "ssl_enabled": true,
    "ssl_status": "✅ SSL properly configured"
  }
}
```

**Not Ready Response** (503):
```json
{
  "ready": false,
  "service": "naysayer-webhook",
  "timestamp": "2024-01-15T10:30:00Z",
  "reason": "GitLab token not configured",
  "gitlab_token": false,
  "webhook_secret": true,
  "ssl_info": {
    "ssl_enabled": true,
    "ssl_status": "✅ SSL properly configured"
  }
}
```

**Response Codes**:
- `200 OK` - Service is ready to accept traffic
- `503 Service Unavailable` - Service is not ready (missing configuration)

## ⚙️ **Configuration**

NAYSAYER is configured through environment variables and a `rules.yaml` file.

**Required Environment Variables**:
- `GITLAB_TOKEN` - GitLab personal access token with `api` scope
- `GITLAB_BASE_URL` - GitLab instance URL (default: `https://gitlab.com`)

**Optional Environment Variables**:
- `GITLAB_TOKEN_FIVETRAN` - Dedicated GitLab token for Fivetran Terraform repository (falls back to `GITLAB_TOKEN` if not set)
- `WEBHOOK_SECRET` - Webhook secret token for additional security
- `PORT` - Server port (default: `3000`)

> **📋 Configuration Details**: For complete configuration options and examples, see:
> - [Development Setup Guide](DEVELOPMENT_SETUP.md) - Environment variables and setup
> - [Section-Based Architecture Guide](SECTION_BASED_ARCHITECTURE.md) - rules.yaml configuration
> - [Fivetran Rule Documentation](rules/AUTOREBASE_RULE_AND_SETUP.md) - Fivetran-specific setup


## 🔍 **Error Handling**

### **Common Error Responses**

**GitLab API Errors** (logged, not returned to client):
```json
{
  "level": "error",
  "msg": "Failed to fetch MR changes",
  "mr_id": 456,
  "error": "GitLab API error 401: {\"message\":\"401 Unauthorized\"}"
}
```

**Rule Evaluation Errors** (logged):
```json
{
  "level": "error", 
  "msg": "Rule evaluation failed",
  "mr_id": 456,
  "error": "Failed to parse YAML: invalid syntax"
}
```

### **HTTP Status Code Reference**

| Code | Meaning | When It Occurs |
|------|---------|----------------|
| `200` | Success | Webhook processed successfully |
| `400` | Bad Request | Invalid JSON, wrong Content-Type, unsupported event |
| `401` | Unauthorized | GitLab API authentication failed (logged only) |
| `403` | Forbidden | GitLab API permission denied (logged only) |
| `404` | Not Found | Invalid endpoint path |
| `500` | Internal Server Error | Unexpected application error |
| `503` | Service Unavailable | Service not ready (readiness check) |

## 📊 **Monitoring**

NAYSAYER uses structured JSON logging with key fields: `mr_id`, `project_id`, `execution_time`, `decision`.

> **📊 Monitoring Details**: For complete logging configuration and monitoring setup, see [Development Setup Guide](DEVELOPMENT_SETUP.md)

## 🧪 **Testing**

**Test Health Endpoint**:
```bash
curl -f https://your-naysayer-domain.com/health
```

**Test Data Product Config Review Webhook**:
```bash
curl -X POST https://your-naysayer-domain.com/dataverse-product-config-review \
  -H "Content-Type: application/json" \
  -H "X-Gitlab-Event: Merge Request Hook" \
  -d '{"object_kind": "merge_request", "object_attributes": {"id": 123, "iid": 456}}'
```

**Test Fivetran Terraform Rebase Webhook**:
```bash
curl -X POST https://your-naysayer-domain.com/fivetran-terraform-rebase \
  -H "Content-Type: application/json" \
  -d '{
    "object_kind": "push",
    "ref": "refs/heads/main",
    "project": {"id": 456}
  }'
```

> **🧪 Development & Testing**: For comprehensive testing strategies and examples, see:
> - [Development Setup Guide](DEVELOPMENT_SETUP.md) - General testing guide

## 🔐 **Security**

NAYSAYER validates:
- **Content-Type**: Must be `application/json`
- **Event Type**: Must be `merge_request` events only
- **Payload Structure**: Must contain required GitLab webhook fields
- **SSL/TLS**: Logs warnings for HTTP requests

> **🔒 Security Details**: For complete security considerations, see [Troubleshooting Guide](TROUBLESHOOTING.md)

## 🔗 **Related Documentation**

- **[Development Setup Guide](DEVELOPMENT_SETUP.md)** - Setup, configuration, and testing
- **[Troubleshooting Guide](TROUBLESHOOTING.md)** - Common issues and debugging
- **[Section-Based Architecture](SECTION_BASED_ARCHITECTURE.md)** - System architecture
- **[Deployment Guide](../DEPLOYMENT.md)** - Production deployment

---

📡 **This API reference covers all public endpoints. For detailed examples and monitoring scripts, see the Development Setup Guide.** 