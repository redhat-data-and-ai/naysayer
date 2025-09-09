# NAYSAYER API Reference

## 🌐 **API Endpoints Overview**

NAYSAYER provides HTTP endpoints for webhook processing, health monitoring, and status checking.

**Base URL**: `https://your-naysayer-domain.com`

> **🏗️ Architecture Details**: For system architecture and validation flow, see [Section-Based Architecture Guide](SECTION_BASED_ARCHITECTURE.md)

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

## ⚙️ **Configuration**

NAYSAYER is configured through environment variables and a `rules.yaml` file.

> **📋 Configuration Details**: For complete configuration options and examples, see:
> - [Development Setup Guide](DEVELOPMENT_SETUP.md) - Environment variables and setup
> - [Section-Based Architecture Guide](SECTION_BASED_ARCHITECTURE.md) - rules.yaml configuration


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

**Test Webhook**:
```bash
curl -X POST https://your-naysayer-domain.com/dataverse-product-config-review \
  -H "Content-Type: application/json" \
  -H "X-Gitlab-Event: Merge Request Hook" \
  -d '{"object_kind": "merge_request", "object_attributes": {"id": 123, "iid": 456}}'
```

> **🧪 Development & Testing**: For comprehensive testing strategies and examples, see [Development Setup Guide](DEVELOPMENT_SETUP.md)

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