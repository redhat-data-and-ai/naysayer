# OpenShift Deployment

This directory contains OpenShift manifests for deploying NAYSAYER webhook service, following Red Hat internal deployment patterns.

## Quick Deployment

1. **Create the namespace** (if using Red Hat PaaS):
   ```bash
   oc apply -f tenant-namespace.yaml
   ```

2. **Copy and configure secrets**:
   ```bash
   cp secrets.yaml.example secrets.yaml
   # Edit secrets.yaml and replace the placeholder values
   vi secrets.yaml
   ```

3. **Deploy all components**:
   ```bash
   oc apply -f serviceaccount.yaml
   oc apply -f secrets.yaml
   oc apply -f deployment.yaml
   oc apply -f service.yaml
   oc apply -f route.yaml
   ```

4. **Update the route hostname** in `route.yaml` to match your domain.

## Files

- `tenant-namespace.yaml` - Red Hat PaaS tenant namespace definition
- `serviceaccount.yaml` - Service account for the deployment
- `secrets.yaml.example` - Template for required secrets (copy to `secrets.yaml`)
- `deployment.yaml` - Main application deployment
- `service.yaml` - Service to expose the application
- `route.yaml` - OpenShift route for external access

## Configuration

### Required Secrets

The application requires these secrets to be configured in `secrets.yaml`:

- `GITLAB_TOKEN` - GitLab API token for webhook access
- `GITLAB_BASE_URL` - GitLab instance URL (defaults to https://gitlab.com)
- `WEBHOOK_SECRET` - Secret for webhook signature verification

### Environment Variables

The deployment configures these environment variables:

- `PORT` - Service port (3000)

- `LOG_LEVEL` - Logging level (info)
- `ENVIRONMENT` - Deployment environment (production)

## Resource Requirements

- **Requests**: 128Mi memory, 100m CPU
- **Limits**: 256Mi memory, 200m CPU
- **Replicas**: 1 (suitable for most use cases)

## Health Checks

- **Liveness Probe**: `/health` endpoint (checks if app is running)
- **Readiness Probe**: `/ready` endpoint (checks if app is ready for traffic)

## Namespace

This deployment uses the namespace: `ddis-asteroid--naysayer`

Update all manifests if you need to use a different namespace.

## Monitoring

The application exposes health endpoints for monitoring:
- `GET /health` - Health check
- `GET /ready` - Readiness check

## Webhook URL

Once deployed, the webhook will be available at:
```
https://naysayer-webhook.your-domain.com/dataverse-product-config-review
```

Configure this URL in your GitLab project webhook settings.