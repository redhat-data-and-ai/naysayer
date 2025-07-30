# Kubernetes Deployment

This directory contains Kubernetes manifests for deploying NAYSAYER webhook service.

## Files

- `deployment.yaml` - Deployment and Secret for NAYSAYER
- `service.yaml` - Service to expose NAYSAYER internally
- `route.yaml` - OpenShift Route for external access

## Deployment Steps

1. **Configure GitLab Token**:
   ```bash
   echo -n "your-gitlab-token-here" | base64
   # Copy the output and update gitlab-token in deployment.yaml
   ```

2. **Update Route Domain** (if using OpenShift):
   Edit `route.yaml` and update the host field with your actual domain.

3. **Deploy to Kubernetes**:
   ```bash
   kubectl apply -f k8s/
   ```

4. **For OpenShift**:
   ```bash
   oc apply -f k8s/
   ```

## Configuration

The deployment uses environment variables:
- `GITLAB_TOKEN` - GitLab API token (from secret)
- `GITLAB_BASE_URL` - GitLab instance URL (defaults to https://gitlab.com)
- `PORT` - Service port (defaults to 3000)

## Monitoring

Health check is available at `/health` endpoint and is used for:
- Liveness probe (checks if app is running)
- Readiness probe (checks if app is ready to receive traffic)

## Image Management

The deployment references `quay.io/ddis/naysayer:latest`. To update:

1. Build and push new image:
   ```bash
   make build-image
   make push-image
   ```

2. Restart deployment:
   ```bash
   kubectl rollout restart deployment/naysayer
   ```