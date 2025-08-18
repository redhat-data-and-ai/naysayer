# 🚀 Naysayer Deployment Guide

Complete guide for deploying Naysayer in production environments.

## 📋 Prerequisites

- Kubernetes or OpenShift cluster
- GitLab instance with webhook capabilities
- GitLab API token with `read_repository` scope
- Container registry access (if building custom images)

## 🏗️ Deployment Options

### 1. Kubernetes/OpenShift (Recommended)

The Naysayer webhook deployment includes security, reliability, scalability, and monitoring features for production use.

#### Prerequisites
- **Kubernetes Cluster**: v1.20+
- **Ingress Controller**: For external webhook access
- **Prometheus Operator**: For monitoring (optional)
- **GitLab Instance**: With webhook configuration

#### Configure Secrets

```bash
# Create GitLab token secret with webhook security
export GITLAB_TOKEN="your-gitlab-token"
export WEBHOOK_SECRET="your-webhook-secret"

kubectl create secret generic naysayer-secrets \
  --from-literal=gitlab-token="$GITLAB_TOKEN" \
  --from-literal=webhook-secret="$WEBHOOK_SECRET"
```

#### Configure IP Restrictions (Optional)

```bash
# Allow specific IP ranges for webhook security
kubectl create configmap naysayer-config \
  --from-literal=allowed-ips="192.168.1.0/24,10.0.0.1"
```

#### Deploy Application

```bash
# Deploy core components
kubectl apply -f config/deployment.yaml
kubectl apply -f config/service.yaml
kubectl apply -f config/route.yaml

# Deploy monitoring (if Prometheus Operator is available)
kubectl apply -f config/monitoring.yaml
```

#### Verify Installation

```bash
# Check pod status
kubectl get pods -l app=naysayer

# Check service endpoints
kubectl get endpoints naysayer

# Check HPA status
kubectl get hpa naysayer-hpa

# View logs
kubectl logs -l app=naysayer --tail=100

# Check health endpoint
kubectl port-forward service/naysayer 3000:3000
curl http://localhost:3000/health
```

#### High Availability Configuration

The deployment includes:
- **Multiple replicas**: 3 minimum (ensures availability during rolling updates)
- **Auto-scaling**: HPA scales based on CPU (70%) and memory (80%) utilization
- **Anti-affinity**: Spreads pods across different nodes
- **Pod Disruption Budget**: Ensures minimum 2 pods remain available during maintenance

#### Security Features

- **Non-root execution**: Runs as user ID 1001
- **Read-only filesystem**: Prevents runtime file modifications
- **Dropped capabilities**: Removes all Linux capabilities
- **Network policies**: Restricts ingress/egress traffic
- **RBAC**: Minimal read-only permissions

#### Resource Configuration

Default resource allocation:
```yaml
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
    ephemeral-storage: "100Mi"
  limits:
    memory: "256Mi"
    cpu: "200m"
    ephemeral-storage: "500Mi"
```

Adjust based on your webhook traffic volume.

### 2. Container Deployment

#### Using Pre-built Image

```bash
# Pull and run official image
docker run -d \
  --name naysayer \
  -p 3000:3000 \
  -e GITLAB_TOKEN=your-token-here \
  -e GITLAB_BASE_URL=https://gitlab.com \
  quay.io/ddis/naysayer:latest
```

#### Building Custom Image

```bash
# Build image
make build-image

# Tag for your registry
docker tag naysayer:latest your-registry.com/naysayer:latest

# Push to registry
docker push your-registry.com/naysayer:latest
```

### 3. Systemd Service (Linux)

#### Service Configuration

```ini
# /etc/systemd/system/naysayer.service
[Unit]
Description=Naysayer GitLab MR Validation Service
After=network.target

[Service]
Type=simple
User=naysayer
ExecStart=/opt/naysayer/naysayer
Environment=PORT=3000
Environment=GITLAB_TOKEN=your_token_here
Environment=GITLAB_BASE_URL=https://gitlab.com
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

#### Installation

```bash
# Create user
sudo useradd -r -s /bin/false naysayer

# Install binary
sudo mkdir -p /opt/naysayer
sudo cp naysayer /opt/naysayer/
sudo chown -R naysayer:naysayer /opt/naysayer
sudo chmod +x /opt/naysayer/naysayer

# Install and start service
sudo systemctl daemon-reload
sudo systemctl enable naysayer
sudo systemctl start naysayer

# Check status
sudo systemctl status naysayer
```

## ⚙️ Configuration

### Environment Variables

#### Required Configuration

```bash
# GitLab Integration
GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx    # Required: GitLab API token
GITLAB_BASE_URL=https://gitlab.com          # Optional: GitLab instance URL
```

#### Optional Configuration

```bash
# Server Settings
PORT=3000                                   # Server port
BIND_ADDRESS=0.0.0.0                       # Bind address

# Rule Engine
RULES_ENABLED=true                          # Enable/disable all rules
RULES_TIMEOUT=30                            # Rule execution timeout (seconds)
RULES_MAX_FILE_SIZE=5242880                 # Max file size (5MB)
RULES_DEBUG=false                           # Debug logging

# Performance
MAX_CONCURRENT_RULES=10                     # Concurrent rule execution
REQUEST_TIMEOUT=30                          # HTTP request timeout
```

#### Rule-Specific Configuration

```bash
# Enable/disable individual rules
RULE_A_ENABLED=true
RULE_B_ENABLED=true

# Rule-specific settings
RULE_A_STRICT_MODE=false
RULE_A_MAX_FILE_SIZE=1048576
RULE_B_VALIDATE_FORMAT=true
RULE_B_REQUIRE_APPROVAL=false
```

### Configuration Files

#### Kubernetes ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: naysayer-config
data:
  GITLAB_BASE_URL: "https://gitlab.com"
  PORT: "3000"
  RULES_ENABLED: "true"
  RULES_TIMEOUT: "30"
  RULE_A_ENABLED: "true"
  RULE_B_ENABLED: "true"
```

#### Docker Compose

```yaml
version: '3.8'
services:
  naysayer:
    image: quay.io/ddis/naysayer:latest
    ports:
      - "3000:3000"
    environment:
      - GITLAB_TOKEN=${GITLAB_TOKEN}
      - GITLAB_BASE_URL=https://gitlab.com
      - RULES_ENABLED=true
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3000/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

## 🔒 Security

### Network Security

```bash
# Firewall configuration (iptables example)
# Allow inbound HTTPS traffic
iptables -A INPUT -p tcp --dport 443 -j ACCEPT

# Allow internal health checks
iptables -A INPUT -s 10.0.0.0/8 -p tcp --dport 3000 -j ACCEPT

# Deny all other inbound traffic to application port
iptables -A INPUT -p tcp --dport 3000 -j DROP
```

### TLS/SSL Configuration

#### Kubernetes Ingress with TLS

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: naysayer-ingress
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - naysayer.your-domain.com
    secretName: naysayer-tls
  rules:
  - host: naysayer.your-domain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: naysayer
            port:
              number: 3000
```

#### Reverse Proxy (Nginx)

```nginx
server {
    listen 443 ssl http2;
    server_name naysayer.your-domain.com;

    ssl_certificate /path/to/certificate.crt;
    ssl_certificate_key /path/to/private.key;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://localhost:3000/health;
        access_log off;
    }
}
```

### Secret Management

#### Kubernetes Secrets

```bash
# Create from literal
kubectl create secret generic naysayer-secrets \
  --from-literal=gitlab-token=your-token

# Create from file
kubectl create secret generic naysayer-secrets \
  --from-file=gitlab-token=/path/to/token-file

# Use external secret management
# Example: External Secrets Operator with Vault
```

#### HashiCorp Vault Integration

```yaml
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: vault-backend
spec:
  provider:
    vault:
      server: "https://vault.company.com"
      path: "secret"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "naysayer"
---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: naysayer-secrets
spec:
  secretStoreRef:
    name: vault-backend
    kind: SecretStore
  target:
    name: naysayer-secrets
  data:
  - secretKey: gitlab-token
    remoteRef:
      key: naysayer/gitlab
      property: token
```

## 📊 Monitoring

### Health Checks

#### Kubernetes Probes

```yaml
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
```

#### External Monitoring

```bash
# Prometheus scraping configuration
# Add to prometheus.yml
scrape_configs:
  - job_name: 'naysayer'
    static_configs:
      - targets: ['naysayer.your-domain.com:3000']
    metrics_path: /metrics
    scrape_interval: 30s
```

### Logging

#### Structured Logging Configuration

```bash
# Enable JSON logging for production
export LOG_FORMAT=json
export LOG_LEVEL=info

# Log aggregation (example with Fluentd)
export LOG_OUTPUT=stdout
```

#### Log Aggregation (ELK Stack)

```yaml
# Filebeat configuration for Kubernetes
apiVersion: v1
kind: ConfigMap
metadata:
  name: filebeat-config
data:
  filebeat.yml: |
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
    output.elasticsearch:
      hosts: ["elasticsearch.logging.svc.cluster.local:9200"]
      index: "naysayer-%{+yyyy.MM.dd}"
```

## 🔧 Troubleshooting

### Common Deployment Issues

#### 1. **Container Won't Start**

```bash
# Check logs
kubectl logs deployment/naysayer

# Common issues and solutions:
# - Missing GitLab token → Check secrets
# - Permission denied → Check user/group in container
# - Port already in use → Change PORT environment variable
```

#### 2. **GitLab Connectivity Issues**

```bash
# Test GitLab API access
kubectl exec deployment/naysayer -- curl -H "Authorization: Bearer $GITLAB_TOKEN" \
  https://gitlab.com/api/v4/user

# Check network policies
kubectl get networkpolicies
kubectl describe networkpolicy naysayer-netpol
```

#### 3. **Webhook Not Receiving Events**

```bash
# Verify webhook configuration in GitLab
# Check ingress/load balancer configuration
kubectl get ingress naysayer-ingress
kubectl describe ingress naysayer-ingress

# Test webhook endpoint
curl -X POST https://naysayer.your-domain.com/webhook \
  -H "Content-Type: application/json" \
  -d '{"object_attributes":{"iid":1},"project":{"id":1}}'
```

### Performance Tuning

#### Resource Limits

```yaml
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

#### Scaling

```yaml
# Horizontal Pod Autoscaler
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: naysayer-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: naysayer
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

## 🔄 Updates and Maintenance

### Rolling Updates

```bash
# Update deployment image
kubectl set image deployment/naysayer naysayer=quay.io/ddis/naysayer:v1.2.0

# Monitor rollout
kubectl rollout status deployment/naysayer

# Rollback if needed
kubectl rollout undo deployment/naysayer
```

### Backup and Recovery

#### Configuration Backup

```bash
# Backup Kubernetes configuration
kubectl get configmap naysayer-config -o yaml > naysayer-config-backup.yaml
kubectl get secret naysayer-secrets -o yaml > naysayer-secrets-backup.yaml

# Backup custom configurations
kubectl get deployment naysayer -o yaml > naysayer-deployment-backup.yaml
```

#### Disaster Recovery

```bash
# Restore from backup
kubectl apply -f naysayer-config-backup.yaml
kubectl apply -f naysayer-secrets-backup.yaml
kubectl apply -f naysayer-deployment-backup.yaml

# Verify restoration
kubectl get pods -l app=naysayer
curl https://naysayer.your-domain.com/health
```

## 📈 Production Checklist

### Pre-Deployment

- [ ] GitLab token configured with minimal required permissions
- [ ] TLS/SSL certificates installed and configured
- [ ] Resource limits and requests defined
- [ ] Health checks configured
- [ ] Monitoring and alerting set up
- [ ] Log aggregation configured
- [ ] Backup procedures documented
- [ ] Security scanning completed

### Post-Deployment

- [ ] Health endpoints responding
- [ ] GitLab webhook delivering events
- [ ] Rules executing successfully
- [ ] Logs flowing to aggregation system
- [ ] Metrics being collected
- [ ] Performance within acceptable limits
- [ ] Security hardening applied
- [ ] Documentation updated

### Ongoing Maintenance

- [ ] Regular security updates
- [ ] Performance monitoring
- [ ] Log rotation and cleanup
- [ ] Certificate renewal
- [ ] Backup verification
- [ ] Capacity planning
- [ ] Rule performance analysis

---

**🔗 Related Documentation:**
- [Main README](README.md) - Project overview and quick start
- [Development Guide](DEVELOPMENT.md) - Local development setup
- [Monitoring Guide](MONITORING.md) - Detailed monitoring and debugging
- [API Reference](docs/API_REFERENCE.md) - API endpoints and responses