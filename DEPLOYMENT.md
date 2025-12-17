# 🚀 Naysayer Deployment Guide

## 📋 Prerequisites

- Kubernetes or OpenShift cluster access
- GitLab instance with webhook capabilities
- `kubectl` or `oc` CLI configured

## 🏗️ Deployment Architecture

Naysayer deployment configurations are maintained in this repository under `/config/`.

### Configuration Flow

```
Naysayer Repo (config/)
        ↓
   [Apply to Kubernetes/OpenShift]
        ↓
   Production Deployment
```

**Key Principle**: All deployment configs are maintained in the naysayer repository.

## 🎯 Deployment Options

Choose your deployment method based on use case:

### Option 1: Production Releases (Recommended)

**Use for:** Official releases, production deployments

**Process:**
1. Make code/config changes in naysayer repo
2. Create version tag (e.g., `v1.2.3`)
3. GitHub Actions automatically:
   - Builds and tests
   - Creates Docker image
   - Creates GitHub Release
4. Deploy using the versioned image

See [Release Process](#-automated-release-process) for details.

### Option 2: Direct Deployment (For Testing/Hotfixes)

**Use for:** Quick testing, development, hotfixes

**Process:**
```bash
# Deploy directly from naysayer/config/
kubectl apply -f config/
```

See [Manual Deployment](#⚡-initial-deployment) for details.

## 🚀 Automated Release Process

The recommended way to deploy Naysayer is through the automated release workflow.

### Step-by-Step Release

1. **Make your changes**
   ```bash
   # Edit code or deployment configs
   vim internal/rules/warehouse/warehouse_rule.go
   vim config/deployment.yaml

   # Commit changes
   git add .
   git commit -m "Add new warehouse size validation"
   git push origin main
   ```

2. **Create release tag**
   ```bash
   # Use semantic versioning
   git tag v1.2.3
   git push origin v1.2.3
   ```

3. **GitHub Actions runs automatically**
   - ✅ Runs full test suite (unit + E2E)
   - ✅ Builds Docker image with version tag
   - ✅ Pushes to `images.paas.redhat.com/ddis-asteroid/naysayer:v1.2.3`
   - ✅ Pushes to `images.paas.redhat.com/ddis-asteroid/naysayer:latest`
   - ✅ Creates GitHub Release with changelog

4. **Deploy to production**
   - Update deployment config with new image tag if needed
   - Apply configs using kubectl/oc

### Required GitHub Secrets

Configure these in your GitHub repository settings:

```
REGISTRY_USERNAME     # Container registry username
REGISTRY_PASSWORD     # Container registry password/token
```

### Versioning Strategy

Follow [Semantic Versioning](https://semver.org/):

- **v1.0.0** → **v2.0.0**: Breaking changes (API changes, removed features)
- **v1.0.0** → **v1.1.0**: New features (new endpoints, new rules)
- **v1.0.0** → **v1.0.1**: Bug fixes (rule fixes, performance improvements)

Examples:
```bash
git tag v1.0.0   # Initial release
git tag v1.1.0   # Added stale MR cleanup endpoint
git tag v1.1.1   # Fixed warehouse rule bug
git tag v2.0.0   # Changed API response format (breaking)
```

## ⚡ Initial Deployment

**Note**: Throughout this guide, replace `<your-namespace>` with your actual namespace and `<your-naysayer-route-hostname>` with your route hostname.

### 1. Configure Secrets

```bash
# Copy the example template
cp config/secrets.yaml.example config/secrets.yaml

# Edit with your actual credentials
vi config/secrets.yaml
```

Configure these values in `config/secrets.yaml`:
- `GITLAB_TOKEN`: Your GitLab API token
- `GITLAB_BASE_URL`: Your GitLab instance URL (e.g., https://gitlab.cee.redhat.com)
- `WEBHOOK_SECRET`: Webhook validation secret
- `GITLAB_TOKEN_FIVETRAN`: (Optional) Dedicated token for Fivetran rebase operations

**Note**: `secrets.yaml` is gitignored and won't be committed.

### 2. Deploy to Cluster

```bash
# Apply secrets first
kubectl apply -f config/secrets.yaml

# Deploy remaining components (excluding example files)
kubectl apply -f config/tenant-namespace.yaml \
  -f config/serviceaccount.yaml \
  -f config/deployment.yaml \
  -f config/service.yaml \
  -f config/route.yaml

# Wait for deployment to complete
kubectl rollout status deployment/naysayer -n <your-namespace>
```

### 3. Verify Deployment

```bash
# Check pod status
kubectl get pods -n <your-namespace> -l app=naysayer

# View logs
kubectl logs -n <your-namespace> -l app=naysayer --tail=50

# Test health endpoint
curl https://<your-naysayer-route-hostname>/health
```

## 🔄 Updating the Deployment

### Building and Deploying Changes

Use this workflow when you've made code changes or need to update secrets/configuration/rules:

```bash
# 1. Build and test locally
make build

# 2. Build Docker image
make docker-build

# 3. Push to registry
make docker-push

# 4. (Optional) Update secrets if needed
vi config/secrets.yaml
kubectl apply -f config/secrets.yaml

# 5. (Optional) Update deployment config if needed
vi config/deployment.yaml
kubectl apply -f config/deployment.yaml

# 6. (Optional) Update validation rules if needed
vi rules.yaml
kubectl create configmap naysayer-rules \
  --from-file=rules.yaml \
  --namespace=ddis-asteroid--naysayer \
  --dry-run=client -o yaml | kubectl apply -f -

# 7. Restart deployment to pull latest image and pick up any changes
kubectl rollout restart deployment/naysayer -n <your-namespace>

# 8. Wait for rollout to complete
kubectl rollout status deployment/naysayer -n <your-namespace>

# 9. Verify deployment
kubectl get pods -n <your-namespace> -l app=naysayer
kubectl logs -n <your-namespace> -l app=naysayer --tail=30
```

**Note**: If you only need to update secrets/config/rules (without rebuilding the image), skip steps 1-3.

## 🔍 Troubleshooting

### Check Pod Status

```bash
# Get pod details
kubectl get pods -n <your-namespace> -l app=naysayer

# Describe pod for events
kubectl describe pod -n <your-namespace> -l app=naysayer

# Check logs
kubectl logs -n <your-namespace> -l app=naysayer --tail=100
```

### Rollback Deployment

```bash
# Rollback to previous version
kubectl rollout undo deployment/naysayer -n <your-namespace>

# Check rollout history
kubectl rollout history deployment/naysayer -n <your-namespace>
```

### Check Secrets

```bash
# Verify secret exists
kubectl get secret naysayer-secrets -n <your-namespace>

# View secret keys (not values)
kubectl get secret naysayer-secrets -n <your-namespace> -o jsonpath='{.data}' | jq 'keys'
```

## 📊 Monitoring

### Health Check

```bash
# Port-forward to test locally
kubectl port-forward -n <your-namespace> deployment/naysayer 3000:3000

# Test health endpoint
curl http://localhost:3000/health
```

### View Metrics

```bash
# Get deployment details
kubectl get deployment naysayer -n <your-namespace> -o wide

# Get route/ingress
kubectl get route naysayer -n <your-namespace>

# Check resource usage
kubectl top pod -n <your-namespace> -l app=naysayer
```

## 📁 Configuration Files

### Kubernetes Manifests (`config/` directory)

- `secrets.yaml.example` - Secret template (copy to `secrets.yaml`)
- `deployment.yaml` - Main application deployment
- `service.yaml` - Kubernetes service
- `route.yaml` - OpenShift route
- `serviceaccount.yaml` - Service account
- `tenant-namespace.yaml` - Namespace definition

### Application Configuration (repository root)

- `rules.yaml` - Validation rules configuration (deployed as ConfigMap)

## 🔗 Related Documentation

- [Main README](README.md) - Project overview
- [config/README.md](config/README.md) - Configuration details
