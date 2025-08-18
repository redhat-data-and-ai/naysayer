# NAYSAYER API Reference

## 🌐 **API Endpoints Overview**

NAYSAYER provides several HTTP endpoints for webhook processing, health monitoring, and status checking.

**Base URL**: `https://your-naysayer-domain.com`

## 📡 **Webhook Endpoints**

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

## 🔧 **Configuration & Environment**

### **Analysis Modes**

The `analysis_mode` field in health responses indicates current capabilities:

| Mode | Description | Cause |
|------|-------------|-------|
| `"Full analysis enabled"` | All features available | Valid GitLab token configured |
| `"Limited (no GitLab token)"` | Basic webhook processing only | No GitLab token configured |

### **Security Modes**

The `security_mode` field indicates webhook security configuration:

| Mode | Description |
|------|-------------|
| `"Token verification available"` | Webhook secret configured |
| `"No secret configured"` | No webhook secret set |

### **SSL Status Messages**

| Status | Meaning |
|--------|---------|
| `"✅ SSL properly configured"` | HTTPS request detected |
| `"⚠️ Request received over HTTP - GitLab SSL verification will reject HTTP webhooks"` | HTTP request (potential issue) |

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

## 📊 **Monitoring & Observability**

### **Structured Logging**

NAYSAYER uses structured JSON logging with these key fields:

**Example Log Entry**:
```json
{
  "level": "info",
  "ts": 1642234567.123,
  "caller": "webhook/handler.go:45",
  "msg": "Processing MR event",
  "component": "NAYSAYER",
  "service": "naysayer",
  "mr_id": 456,
  "project_id": 789,
  "author": "developer",
  "execution_time": "125ms",
  "decision": "auto_approve"
}
```

**Log Levels**:
- `debug` - Detailed debugging information
- `info` - General information messages
- `warn` - Warning conditions
- `error` - Error conditions

### **Key Metrics Fields**

| Field | Description | Example |
|-------|-------------|---------|
| `mr_id` | Merge request IID | `456` |
| `project_id` | GitLab project ID | `789` |
| `execution_time` | Processing duration | `"125ms"` |
| `decision` | Rule decision type | `"auto_approve"`, `"manual_review"` |
| `file_changes` | Number of files changed | `3` |
| `rules_evaluated` | Number of rules processed | `1` |

## 🧪 **Testing & Development**

### **Manual Testing Commands**

**Test Health Endpoint**:
```bash
curl -f https://your-naysayer-domain.com/health
echo $?  # Should be 0 for success
```

**Test Readiness**:
```bash
curl -f https://your-naysayer-domain.com/ready
echo $?  # Should be 0 if ready
```

**Test Webhook with Minimal Payload**:
```bash
curl -X POST https://your-naysayer-domain.com/dataverse-product-config-review \
  -H "Content-Type: application/json" \
  -H "X-Gitlab-Event: Merge Request Hook" \
  -d '{"object_kind": "merge_request", "object_attributes": {"id": 123, "iid": 456}}'
```

### **Webhook Payload Examples**

**Minimal Valid Payload**:
```json
{
  "object_kind": "merge_request",
  "object_attributes": {
    "id": 123,
    "iid": 456,
    "state": "opened"
  },
  "project": {
    "id": 789
  }
}
```

**Complete Warehouse Change Payload**:
```json
{
  "object_kind": "merge_request",
  "object_attributes": {
    "id": 123,
    "iid": 456,
    "title": "Reduce warehouse size for cost optimization",
    "state": "opened",
    "target_branch": "main",
    "source_branch": "feature/warehouse-reduction",
    "author": {
      "username": "cost-optimizer"
    }
  },
  "project": {
    "id": 789,
    "name": "dataproduct-config",
    "web_url": "https://gitlab.com/company/dataproduct-config"
  },
  "changes": {
    "total": 2
  }
}
```

## 🔐 **Security Considerations**

### **Request Validation**

NAYSAYER validates incoming requests:
1. **Content-Type**: Must be `application/json`
2. **Event Type**: Must be `merge_request` events only
3. **Payload Structure**: Must contain required GitLab webhook fields
4. **SSL/TLS**: Logs warnings for HTTP requests when SSL verification is expected

### **Authentication Flow**

```
GitLab Webhook → NAYSAYER → GitLab API
       ↓              ↓           ↑
   (optional)    (validates)  (authenticates)
 webhook secret   payload     with token
```

### **Rate Limiting**

NAYSAYER does not implement built-in rate limiting. Consider implementing rate limiting at the load balancer or proxy level for production deployments.

## 🔗 **Related Documentation**

- [USER_SETUP_GUIDE.md](USER_SETUP_GUIDE.md) - Setup and configuration
- [SSL_WEBHOOK_SECURITY.md](SSL_WEBHOOK_SECURITY.md) - SSL requirements
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Common issues and debugging
- [KUBERNETES_DEPLOYMENT.md](../KUBERNETES_DEPLOYMENT.md) - Deployment guide

## 📝 **API Usage Examples**

### **Health Check Monitoring Script**
```bash
#!/bin/bash
NAYSAYER_URL="https://your-naysayer-domain.com"

# Check health
if curl -f -s "$NAYSAYER_URL/health" > /dev/null; then
  echo "✅ NAYSAYER is healthy"
else
  echo "❌ NAYSAYER health check failed"
  exit 1
fi

# Check readiness
if curl -f -s "$NAYSAYER_URL/ready" > /dev/null; then
  echo "✅ NAYSAYER is ready"
else
  echo "⚠️ NAYSAYER is not ready"
  exit 1
fi
```

### **Webhook Debugging Script**
```bash
#!/bin/bash
WEBHOOK_URL="https://your-naysayer-domain.com/dataverse-product-config-review"

# Test webhook endpoint
response=$(curl -s -w "HTTPSTATUS:%{http_code}" \
  -X POST "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -H "X-Gitlab-Event: Merge Request Hook" \
  -d '{"object_kind": "merge_request", "object_attributes": {"id": 123}}')

body=$(echo "$response" | sed -E 's/HTTPSTATUS\:[0-9]{3}$//')
status=$(echo "$response" | tr -d '\n' | sed -E 's/.*HTTPSTATUS:([0-9]{3})$/\1/')

echo "Status: $status"
echo "Response: $body"

if [ "$status" -eq 200 ]; then
  echo "✅ Webhook endpoint is working"
else
  echo "❌ Webhook endpoint failed"
fi
```

---

📡 **This API reference covers all public endpoints and their expected behaviors. For implementation details, see the source code in the `internal/webhook/` directory.** 