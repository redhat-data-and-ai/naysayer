# 📊 Naysayer Monitoring Guide

Comprehensive monitoring, health checks, and debugging guide for Naysayer in production.

## 🏥 Health Checks

### Basic Health Endpoint

```bash
# Check application health
curl https://naysayer.your-domain.com/health

# Expected response
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "1.0.0",
  "uptime": "72h15m30s"
}
```

### Detailed Health Check

```bash
# Comprehensive health information
curl https://naysayer.your-domain.com/api/health/detailed

# Expected response
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "components": {
    "gitlab_connection": {
      "status": "healthy",
      "last_check": "2024-01-15T10:29:55Z",
      "response_time_ms": 245
    },
    "rule_engine": {
      "status": "healthy",
      "active_rules": 3,
      "last_execution": "2024-01-15T10:25:12Z"
    },
    "memory": {
      "status": "healthy",
      "usage_mb": 128,
      "limit_mb": 512
    }
  }
}
```

### Kubernetes Health Probes

```yaml
# Deployment with health checks
apiVersion: apps/v1
kind: Deployment
metadata:
  name: naysayer
spec:
  template:
    spec:
      containers:
      - name: naysayer
        image: quay.io/ddis/naysayer:latest
        ports:
        - containerPort: 3000
        livenessProbe:
          httpGet:
            path: /health
            port: 3000
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /health
            port: 3000
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3
        startupProbe:
          httpGet:
            path: /health
            port: 3000
          initialDelaySeconds: 10
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 30
```

## 📈 Metrics and Monitoring

### Prometheus Metrics

Naysayer exposes metrics at `/metrics` endpoint:

```bash
# Scrape metrics
curl https://naysayer.your-domain.com/metrics
```

#### Key Metrics

**Application Metrics:**
```
# HTTP request metrics
naysayer_http_requests_total{method="POST",path="/webhook",status="200"} 1234
naysayer_http_request_duration_seconds{method="POST",path="/webhook"} 0.245

# Rule execution metrics
naysayer_rule_executions_total{rule="rule_a",decision="approve"} 856
naysayer_rule_executions_total{rule="rule_b",decision="manual_review"} 123
naysayer_rule_execution_duration_seconds{rule="rule_a"} 0.150

# GitLab API metrics
naysayer_gitlab_api_requests_total{endpoint="files",status="200"} 2345
naysayer_gitlab_api_request_duration_seconds{endpoint="files"} 0.320
naysayer_gitlab_api_errors_total{endpoint="files",error_type="timeout"} 12
```

**System Metrics:**
```
# Memory usage
naysayer_memory_usage_bytes 134217728
naysayer_memory_limit_bytes 536870912

# CPU usage
naysayer_cpu_usage_percent 15.5

# Goroutines
naysayer_goroutines_active 45
```

#### Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'naysayer'
    static_configs:
      - targets: ['naysayer.your-domain.com:3000']
    metrics_path: /metrics
    scrape_interval: 30s
    scrape_timeout: 10s
    scheme: https
    tls_config:
      insecure_skip_verify: false
```

### Grafana Dashboard

#### Key Panels

**1. System Overview:**
```json
{
  "title": "Naysayer System Overview",
  "panels": [
    {
      "title": "Request Rate",
      "type": "graph",
      "targets": [
        {
          "expr": "rate(naysayer_http_requests_total[5m])",
          "legendFormat": "{{method}} {{path}}"
        }
      ]
    },
    {
      "title": "Response Time",
      "type": "graph", 
      "targets": [
        {
          "expr": "histogram_quantile(0.95, rate(naysayer_http_request_duration_seconds_bucket[5m]))",
          "legendFormat": "95th percentile"
        }
      ]
    }
  ]
}
```

**2. Rule Performance:**
```json
{
  "title": "Rule Execution Performance",
  "panels": [
    {
      "title": "Rule Decisions",
      "type": "piechart",
      "targets": [
        {
          "expr": "increase(naysayer_rule_executions_total[1h])",
          "legendFormat": "{{rule}} - {{decision}}"
        }
      ]
    },
    {
      "title": "Rule Execution Time",
      "type": "graph",
      "targets": [
        {
          "expr": "naysayer_rule_execution_duration_seconds",
          "legendFormat": "{{rule}}"
        }
      ]
    }
  ]
}
```

### Alerting Rules

#### Prometheus Alerting

```yaml
# alerts.yml
groups:
- name: naysayer.rules
  rules:
  - alert: NaysayerDown
    expr: up{job="naysayer"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Naysayer instance is down"
      description: "Naysayer has been down for more than 1 minute"

  - alert: NaysayerHighErrorRate
    expr: rate(naysayer_http_requests_total{status=~"5.."}[5m]) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High error rate in Naysayer"
      description: "Error rate is {{ $value }} errors per second"

  - alert: NaysayerHighResponseTime
    expr: histogram_quantile(0.95, rate(naysayer_http_request_duration_seconds_bucket[5m])) > 2
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High response time in Naysayer"
      description: "95th percentile response time is {{ $value }}s"

  - alert: NaysayerRuleFailures
    expr: increase(naysayer_rule_executions_total{decision="error"}[10m]) > 5
    for: 1m
    labels:
      severity: warning
    annotations:
      summary: "Multiple rule execution failures"
      description: "{{ $value }} rule failures in the last 10 minutes"

  - alert: NaysayerGitLabAPIErrors
    expr: rate(naysayer_gitlab_api_errors_total[5m]) > 0.05
    for: 3m
    labels:
      severity: warning
    annotations:
      summary: "GitLab API errors in Naysayer"
      description: "GitLab API error rate is {{ $value }} per second"
```

### Log Monitoring

#### Structured Logging

```bash
# Enable JSON logging
export LOG_FORMAT=json
export LOG_LEVEL=info

# Example log output
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "info",
  "message": "Rule execution completed",
  "rule": "rule_a",
  "project_id": 123,
  "mr_iid": 456,
  "decision": "approve",
  "duration_ms": 150,
  "files_processed": 3
}
```

#### Log Aggregation (ELK Stack)

**Filebeat Configuration:**
```yaml
filebeat.inputs:
- type: container
  paths:
    - /var/log/containers/naysayer-*.log
  processors:
    - add_kubernetes_metadata:
        host: ${NODE_NAME}
        matchers:
        - logs_path:
            logs_path: "/var/log/containers/"
    - decode_json_fields:
        fields: ["message"]
        target: ""

output.elasticsearch:
  hosts: ["elasticsearch.logging.svc.cluster.local:9200"]
  index: "naysayer-%{+yyyy.MM.dd}"
```

**Logstash Pipeline:**
```ruby
input {
  beats {
    port => 5044
  }
}

filter {
  if [kubernetes][container][name] == "naysayer" {
    json {
      source => "message"
    }
    
    date {
      match => [ "timestamp", "ISO8601" ]
    }
    
    mutate {
      add_field => { "app" => "naysayer" }
    }
  }
}

output {
  elasticsearch {
    hosts => ["elasticsearch:9200"]
    index => "naysayer-%{+YYYY.MM.dd}"
  }
}
```

#### Log Analysis Queries

**Common Kibana Queries:**
```
# Rule execution errors
level:error AND rule:*

# Slow rule executions
duration_ms:>1000

# GitLab API issues
message:"gitlab" AND (level:error OR level:warn)

# High-volume projects
project_id:123 AND timestamp:[now-1h TO now]
```

## 🔍 Debugging

### Debug Mode

```bash
# Enable debug logging
export LOG_LEVEL=debug
export RULES_DEBUG=true

# Rule-specific debugging
export RULE_A_DEBUG=true
export RULE_B_DEBUG=true

# Start application
./naysayer
```

### Performance Debugging

#### CPU Profiling

```bash
# Enable CPU profiling
export ENABLE_PPROF=true
./naysayer

# Collect CPU profile
curl http://localhost:3000/debug/pprof/profile?seconds=30 > cpu.prof

# Analyze profile
go tool pprof cpu.prof
```

#### Memory Profiling

```bash
# Collect memory profile
curl http://localhost:3000/debug/pprof/heap > memory.prof

# Analyze memory usage
go tool pprof memory.prof

# Commands in pprof:
# top10          - Show top 10 memory consumers
# list <func>    - Show source code for function
# web           - Open web interface
```

#### Goroutine Analysis

```bash
# Check for goroutine leaks
curl http://localhost:3000/debug/pprof/goroutine > goroutines.prof
go tool pprof goroutines.prof
```

### Request Tracing

#### Distributed Tracing

```bash
# Enable tracing
export ENABLE_TRACING=true
export JAEGER_ENDPOINT=http://jaeger.tracing.svc.cluster.local:14268/api/traces

# Start with tracing
./naysayer
```

#### Custom Tracing

```go
import (
    "github.com/opentracing/opentracing-go"
    "github.com/uber/jaeger-client-go"
)

func (r *Rule) ShouldApprove(mrCtx *shared.MRContext) (shared.DecisionType, string) {
    span := opentracing.StartSpan("rule.should_approve")
    defer span.Finish()
    
    span.SetTag("rule.name", r.Name())
    span.SetTag("project.id", mrCtx.ProjectID)
    span.SetTag("mr.iid", mrCtx.MRIID)
    
    // Rule logic...
    
    span.SetTag("decision", decision)
    return decision, reason
}
```

### Common Issues and Solutions

#### 1. **High Memory Usage**

**Symptoms:**
- Memory usage continuously increasing
- OOM kills in Kubernetes
- Slow response times

**Debugging:**
```bash
# Check memory usage
curl http://localhost:3000/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# Look for memory leaks
curl http://localhost:3000/debug/pprof/allocs > allocs.prof
go tool pprof allocs.prof
```

**Solutions:**
- Reduce rule concurrent execution
- Implement file size limits
- Add memory-based circuit breakers

#### 2. **Slow Rule Execution**

**Symptoms:**
- High response times
- Timeout errors
- Queue backlog

**Debugging:**
```bash
# Profile CPU usage
curl http://localhost:3000/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Check rule execution times
grep "rule_execution_duration" logs/ | sort -k3 -nr
```

**Solutions:**
- Optimize regex patterns
- Implement caching
- Reduce GitLab API calls

#### 3. **GitLab API Issues**

**Symptoms:**
- Authentication errors
- Rate limiting
- Network timeouts

**Debugging:**
```bash
# Test GitLab connectivity
curl -H "Authorization: Bearer $GITLAB_TOKEN" \
  https://gitlab.com/api/v4/user

# Check API rate limits
curl -I -H "Authorization: Bearer $GITLAB_TOKEN" \
  https://gitlab.com/api/v4/user
```

**Solutions:**
- Verify token permissions
- Implement rate limiting
- Add retry logic with exponential backoff

#### 4. **Rule Registration Issues**

**Symptoms:**
- Rules not executing
- Missing from `/api/rules` endpoint
- Configuration not loading

**Debugging:**
```bash
# Check registered rules
curl http://localhost:3000/api/rules | jq '.'

# Verify configuration
env | grep -E "(RULE|GITLAB)_"

# Check rule loading logs
grep "register.*rule" logs/
```

### Performance Benchmarking

#### Load Testing

```bash
# Install bombardier
go install github.com/codesenberg/bombardier@latest

# Basic load test
bombardier -c 10 -n 1000 -H "Content-Type: application/json" \
  -m POST -f webhook_payload.json \
  http://localhost:3000/webhook

# Sustained load test
bombardier -c 5 -d 5m -H "Content-Type: application/json" \
  -m POST -f webhook_payload.json \
  http://localhost:3000/webhook
```

#### Benchmark Results Analysis

```bash
# Example benchmark output
Statistics        Avg      Stdev        Max
  Reqs/sec       125.67      23.45     189.23
  Latency       79.54ms    25.12ms   245.67ms
  HTTP codes:
    1xx - 0, 2xx - 995, 3xx - 0, 4xx - 3, 5xx - 2
  Throughput:    45.23MB/s
```

**Performance Targets:**
- Response time: < 500ms (95th percentile)
- Throughput: > 100 requests/second
- Error rate: < 1%
- Memory usage: < 256MB per instance

## 📱 Dashboards and Visualization

### Grafana Dashboard JSON

```json
{
  "dashboard": {
    "title": "Naysayer Operations Dashboard",
    "tags": ["naysayer", "validation", "gitlab"],
    "panels": [
      {
        "title": "System Health",
        "type": "stat",
        "targets": [
          {
            "expr": "up{job=\"naysayer\"}",
            "legendFormat": "Instance Status"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "mappings": [
              {"options": {"0": {"text": "DOWN"}}, "type": "value"},
              {"options": {"1": {"text": "UP"}}, "type": "value"}
            ]
          }
        }
      },
      {
        "title": "Request Volume",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(naysayer_http_requests_total[5m])",
            "legendFormat": "{{method}} {{path}}"
          }
        ]
      },
      {
        "title": "Rule Performance",
        "type": "table",
        "targets": [
          {
            "expr": "avg_over_time(naysayer_rule_execution_duration_seconds[1h])",
            "legendFormat": "{{rule}}"
          }
        ]
      }
    ]
  }
}
```

### Custom Metrics Dashboard

```bash
# Create custom metrics endpoint
curl http://localhost:3000/api/metrics/custom

# Response
{
  "rules": {
    "rule_a": {
      "executions_total": 1234,
      "avg_duration_ms": 150,
      "success_rate": 0.95,
      "last_execution": "2024-01-15T10:30:00Z"
    },
    "rule_b": {
      "executions_total": 567,
      "avg_duration_ms": 230,
      "success_rate": 0.92,
      "last_execution": "2024-01-15T10:29:45Z"
    }
  },
  "system": {
    "uptime_seconds": 259200,
    "memory_usage_mb": 128,
    "cpu_usage_percent": 15.5,
    "goroutines": 45
  }
}
```

## 🚨 Alerting and Notifications

### Slack Integration

```yaml
# Alertmanager configuration
route:
  group_by: ['alertname']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'naysayer-alerts'

receivers:
- name: 'naysayer-alerts'
  slack_configs:
  - api_url: 'YOUR_SLACK_WEBHOOK_URL'
    channel: '#naysayer-alerts'
    title: 'Naysayer Alert'
    text: '{{ range .Alerts }}{{ .Annotations.summary }}{{ end }}'
```

### Email Notifications

```yaml
receivers:
- name: 'naysayer-email'
  email_configs:
  - to: 'platform-team@company.com'
    from: 'alerts@company.com'
    subject: 'Naysayer Alert: {{ .GroupLabels.alertname }}'
    body: |
      {{ range .Alerts }}
      Alert: {{ .Annotations.summary }}
      Description: {{ .Annotations.description }}
      {{ end }}
```

### PagerDuty Integration

```yaml
receivers:
- name: 'naysayer-pagerduty'
  pagerduty_configs:
  - service_key: 'YOUR_PAGERDUTY_KEY'
    description: '{{ .GroupLabels.alertname }}: {{ .CommonAnnotations.summary }}'
```

---

**🔗 Related Documentation:**
- [Main README](README.md) - Project overview
- [Deployment Guide](DEPLOYMENT.md) - Production deployment
- [Development Guide](DEVELOPMENT.md) - Local development
- [API Reference](docs/API_REFERENCE.md) - API endpoints