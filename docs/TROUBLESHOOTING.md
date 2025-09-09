# NAYSAYER Troubleshooting Guide

## 🚨 **Common Issues & Solutions**

This guide helps you diagnose and fix common problems with NAYSAYER webhook integration.

## 📊 **Quick Diagnostics**

### **1. Health Check First**
```bash
# Check overall health
curl -s https://YOUR-NAYSAYER-DOMAIN/health | jq '.'

# Check readiness status  
curl -s https://YOUR-NAYSAYER-DOMAIN/ready | jq '.'
```

### **2. Check NAYSAYER Logs**
```bash
# Get recent logs
kubectl logs -n ddis-asteroid--naysayer deployment/naysayer-deployment --tail=50

# Follow logs in real-time
kubectl logs -n ddis-asteroid--naysayer deployment/naysayer-deployment -f

# Search for specific issues
kubectl logs -n ddis-asteroid--naysayer deployment/naysayer-deployment | grep -i error
```

### **3. Verify Configuration**
```bash
# Check secrets
kubectl get secret naysayer-secrets -n ddis-asteroid--naysayer -o yaml

# Check deployment status
kubectl get deployment naysayer-deployment -n ddis-asteroid--naysayer

# Check service and route
kubectl get service,route -n ddis-asteroid--naysayer
```

## 🔧 **Issue Categories**

---

## 🚫 **1. Webhook Delivery Issues**

### **Problem: Webhooks not reaching NAYSAYER**

**Symptoms:**
- No logs in NAYSAYER when MR is created
- GitLab webhook shows delivery failures
- Webhook test in GitLab fails

**Diagnosis:**
```bash
# Check GitLab webhook delivery logs
# Go to GitLab Project → Settings → Webhooks → View delivery logs

# Test webhook URL manually
curl -X POST https://YOUR-NAYSAYER-DOMAIN/dataverse-product-config-review \
  -H "Content-Type: application/json" \
  -H "X-Gitlab-Event: Merge Request Hook" \
  -d '{"object_kind": "merge_request", "object_attributes": {"id": 123}}'
```

**Common Causes & Solutions:**

| Cause | Solution |
|-------|----------|
| **Wrong webhook URL** | Verify URL: `/dataverse-product-config-review` not `/webhook` |
| **SSL certificate issues** | Check GitLab webhook SSL settings and certificate validity |
| **Network/firewall blocking** | Check network policies, firewall rules |
| **NAYSAYER pod not running** | `kubectl get pods -n ddis-asteroid--naysayer` |
| **Service not exposing port** | `kubectl get service naysayer -o yaml` |

### **Problem: Webhook delivers but returns 400/500 errors**

**Symptoms:**
```bash
# GitLab webhook logs show:
# HTTP 400: Bad Request
# HTTP 500: Internal Server Error
```

**Diagnosis:**
```bash
# Check NAYSAYER error logs
kubectl logs -n ddis-asteroid--naysayer deployment/naysayer-deployment | grep "error\|Error\|ERROR"

# Test with proper headers
curl -X POST https://YOUR-NAYSAYER-DOMAIN/dataverse-product-config-review \
  -H "Content-Type: application/json" \
  -H "X-Gitlab-Event: Merge Request Hook" \
  -d @test-webhook.json
```

**Solutions:**
- **400 errors**: Check Content-Type header, JSON payload format
- **500 errors**: Check NAYSAYER logs for internal errors, configuration issues

---

## 🔑 **2. Authentication Issues**

### **Problem: 401 Unauthorized from GitLab API**

**Symptoms:**
```json
{
  "level": "error",
  "msg": "Failed to fetch MR changes", 
  "error": "GitLab API error 401: {\"message\":\"401 Unauthorized\"}"
}
```

**Diagnosis:**
```bash
# Test GitLab token manually
export GITLAB_TOKEN="your-token-here"
curl -H "Authorization: Bearer $GITLAB_TOKEN" \
  https://gitlab.com/api/v4/user

# Check token in NAYSAYER
kubectl get secret naysayer-secrets -n ddis-asteroid--naysayer -o jsonpath='{.data.GITLAB_TOKEN}' | base64 -d
```

**Solutions:**

| Issue | Solution |
|-------|----------|
| **Invalid token** | Regenerate GitLab token with correct scopes |
| **Expired token** | Create new token (check expiry date) |
| **Insufficient permissions** | Ensure token has `api`, `read_repository`, `write_repository` scopes |
| **Wrong GitLab instance** | Check `GITLAB_BASE_URL` matches your GitLab |
| **Token not in secret** | Update Kubernetes secret with valid token |

### **Problem: Token has insufficient permissions**

**Symptoms:**
```json
{
  "level": "error",
  "msg": "Failed to approve MR",
  "error": "GitLab API error 403: {\"message\":\"403 Forbidden\"}"
}
```

**Solutions:**
1. **For Personal Access Tokens**: Ensure user has Developer/Maintainer role on project
2. **For Project Access Tokens**: Set role to Developer or Maintainer
3. **Check token scopes**: Must include `api`, `read_repository`, `write_repository`

---

## 🔐 **3. SSL/TLS Issues**

### **Problem: SSL verification fails in GitLab**

**Symptoms:**
- GitLab webhook test shows SSL error
- No webhook deliveries
- Error: "SSL verification failed"

**Diagnosis:**
```bash
# Test SSL certificate
openssl s_client -connect YOUR-NAYSAYER-DOMAIN:443 -verify_return_error

# Check certificate details
echo | openssl s_client -connect YOUR-NAYSAYER-DOMAIN:443 2>/dev/null | \
  openssl x509 -noout -text | grep -A 2 "Subject:"

# Test from different locations
curl -I https://YOUR-NAYSAYER-DOMAIN
```

**Solutions:**
1. Verify GitLab webhook SSL configuration
2. Check certificate validity and trust chain
3. Ensure proper TLS/SSL protocol versions
4. Test webhook endpoint with curl over HTTPS

### **Problem: NAYSAYER receives HTTP instead of HTTPS**

**Symptoms:**
```json
{
  "ssl_enabled": false,
  "ssl_warnings": ["⚠️ Request received over HTTP"]
}
```

**Solutions:**
1. **Check OpenShift route**:
   ```bash
   oc get route naysayer -o yaml | grep -A 5 tls
   ```
2. **Verify edge termination**:
   ```yaml
   tls:
     termination: edge
     insecureEdgeTerminationPolicy: Redirect
   ```
3. **Check proxy headers**: Ensure `X-Forwarded-Proto: https` is set

---

## 🏗️ **4. Configuration Issues**

### **Problem: Health check shows "not ready"**

**Symptoms:**
```json
{
  "ready": false,
  "reason": "GitLab token not configured"
}
```

**Solutions:**
```bash
# Check if secret exists
kubectl get secret naysayer-secrets -n ddis-asteroid--naysayer

# Verify secret contents
kubectl get secret naysayer-secrets -n ddis-asteroid--naysayer -o yaml

# Update secret if missing
kubectl patch secret naysayer-secrets -n ddis-asteroid--naysayer \
  --patch='{"stringData":{"GITLAB_TOKEN":"glpat-your-token"}}'

# Restart deployment to pick up changes
kubectl rollout restart deployment naysayer-deployment -n ddis-asteroid--naysayer
```

### **Problem: Wrong webhook endpoint configured**

**Symptoms:**
- Webhook delivers to wrong path
- 404 Not Found errors
- NAYSAYER logs show no activity

**Common Mistakes:**
```bash
❌ https://domain.com/webhook
❌ https://domain.com/dataverse-product-config-review-webhook  
❌ https://domain.com/api/webhook
✅ https://domain.com/dataverse-product-config-review
```

**Solution:**
Update GitLab webhook URL to exactly: `/dataverse-product-config-review`

---

## 📝 **5. Rule Processing Issues**

### **Problem: NAYSAYER processes webhook but doesn't approve/comment**

**Symptoms:**
- Webhook delivers successfully (200 response)
- Logs show "Manual review required"
- No auto-approval despite warehouse decreases

**Diagnosis:**
```bash
# Check rule evaluation logs
kubectl logs -n ddis-asteroid--naysayer deployment/naysayer-deployment | grep -A 5 "Rule evaluation"

# Look for specific decision reasoning
kubectl logs -n ddis-asteroid--naysayer deployment/naysayer-deployment | grep "Decision"
```

**Common Causes:**

| Cause | Log Message | Solution |
|-------|-------------|----------|
| **No warehouse changes detected** | `"No warehouse changes detected"` | Verify file paths and YAML format |
| **Warehouse size increase** | `"Warehouse size increase detected"` | Expected behavior - manual review needed |
| **File fetching failed** | `"Failed to fetch MR changes"` | Check GitLab token permissions |
| **Invalid YAML format** | `"Failed to parse YAML"` | Fix YAML syntax in product.yaml |
| **Wrong file paths** | `"No applicable rules"` | Check if files match expected patterns |

### **Problem: NAYSAYER doesn't detect warehouse changes**

**Symptoms:**
```json
{
  "msg": "Decision",
  "type": "manual_review", 
  "reason": "No warehouse changes detected"
}
```

**Diagnosis:**
```bash
# Check if files match expected patterns
kubectl logs -n ddis-asteroid--naysayer deployment/naysayer-deployment | grep "file_changes"

# Verify file paths in MR
# Expected: dataproducts/*/product.yaml
```

**Solutions:**
1. **Check file structure**: Files must be in `dataproducts/*/product.yaml` format
2. **Verify YAML format**: Must contain `warehouses:` section
3. **Check MR changes**: Ensure files are actually modified, not just added/deleted

---

## 🚀 **6. Deployment Issues**

### **Problem: NAYSAYER pod won't start**

**Symptoms:**
```bash
kubectl get pods -n ddis-asteroid--naysayer
# NAME                                  READY   STATUS    RESTARTS   AGE
# naysayer-deployment-xxx               0/1     Error     0          1m
```

**Diagnosis:**
```bash
# Check pod events
kubectl describe pod -n ddis-asteroid--naysayer naysayer-deployment-xxx

# Check pod logs
kubectl logs -n ddis-asteroid--naysayer naysayer-deployment-xxx

# Check deployment events
kubectl describe deployment naysayer-deployment -n ddis-asteroid--naysayer
```

**Common Solutions:**

| Error | Solution |
|-------|----------|
| **ImagePullBackOff** | Check image name/tag, registry access |
| **CrashLoopBackOff** | Check environment variables, secrets |
| **ConfigMap/Secret not found** | Ensure secrets.yaml is applied |
| **Insufficient resources** | Check CPU/memory limits and requests |

### **Problem: Service not accessible**

**Symptoms:**
- Health check fails from outside cluster
- Webhook URL returns connection refused

**Diagnosis:**
```bash
# Check service
kubectl get service naysayer -n ddis-asteroid--naysayer -o yaml

# Check endpoints
kubectl get endpoints naysayer -n ddis-asteroid--naysayer

# Check route (OpenShift)
oc get route naysayer -n ddis-asteroid--naysayer
```

**Solutions:**
1. **Verify service selector** matches pod labels
2. **Check port configuration** (should be 3000)
3. **Ensure route/ingress** is properly configured

---

## 🔍 **7. Advanced Debugging**

### **Enable Debug Logging**
```bash
# Set log level to debug
kubectl patch deployment naysayer-deployment -n ddis-asteroid--naysayer \
  --patch='{"spec":{"template":{"spec":{"containers":[{"name":"naysayer","env":[{"name":"LOG_LEVEL","value":"debug"}]}]}}}}'

# Restart to apply changes
kubectl rollout restart deployment naysayer-deployment -n ddis-asteroid--naysayer
```

### **Test with Minimal Webhook**
```bash
# Create minimal test webhook payload
cat > test-webhook.json << EOF
{
  "object_kind": "merge_request",
  "object_attributes": {
    "id": 123,
    "iid": 456,
    "title": "Test MR",
    "state": "opened",
    "target_branch": "main",
    "source_branch": "feature"
  },
  "project": {
    "id": 789
  }
}
EOF

# Test webhook
curl -X POST https://YOUR-NAYSAYER-DOMAIN/dataverse-product-config-review \
  -H "Content-Type: application/json" \
  -H "X-Gitlab-Event: Merge Request Hook" \
  -d @test-webhook.json
```

### **Network Connectivity Tests**
```bash
# Test from within cluster
kubectl run test-pod --image=curlimages/curl -it --rm -- \
  curl -v http://naysayer:3000/health

# Test external connectivity
kubectl run test-pod --image=curlimages/curl -it --rm -- \
  curl -v https://YOUR-NAYSAYER-DOMAIN/health
```

## 📋 **Diagnostic Checklist**

When debugging issues, check these in order:

### **Basic Connectivity**
- [ ] NAYSAYER pod is running and ready
- [ ] Service is exposing correct port (3000)
- [ ] Route/Ingress is configured with correct hostname
- [ ] Health endpoint returns 200 from external URL

### **Authentication**
- [ ] GitLab token is valid and not expired
- [ ] Token has required scopes (api, read_repository, write_repository)
- [ ] Token is properly stored in Kubernetes secret
- [ ] GITLAB_BASE_URL matches your GitLab instance

### **SSL/Security**
- [ ] SSL certificate is valid and not expired
- [ ] Certificate covers webhook domain
- [ ] SSL verification passes from external test
- [ ] Webhook secret is configured (if using verification)

### **Webhook Configuration**
- [ ] Webhook URL path is `/dataverse-product-config-review`
- [ ] GitLab webhook is configured for "Merge request events" only
- [ ] SSL verification is enabled in GitLab
- [ ] Webhook secret matches NAYSAYER configuration

### **Rule Processing**
- [ ] Files match expected patterns (`product.yaml` in dataproducts structure)
- [ ] YAML files have valid warehouse configurations
- [ ] Changes are actual modifications (not just additions/deletions)
- [ ] Rule evaluation logs show expected behavior

## 🆘 **Getting Help**

If you're still having issues:

1. **Collect logs**:
   ```bash
   kubectl logs -n ddis-asteroid--naysayer deployment/naysayer-deployment --tail=100 > naysayer-logs.txt
   ```

2. **Run health checks**:
   ```bash
   curl -s https://YOUR-NAYSAYER-DOMAIN/health | jq '.' > health-status.json
   curl -s https://YOUR-NAYSAYER-DOMAIN/ready | jq '.' > ready-status.json
   ```

3. **Check configuration**:
   ```bash
   kubectl get deployment,service,route -n ddis-asteroid--naysayer -o yaml > naysayer-config.yaml
   ```

4. **Test webhook delivery**:
   - Go to GitLab → Project → Settings → Webhooks
   - Click "Test" → "Merge request events"
   - Note the response code and any error messages

Include this information when seeking help or filing issues.

## 🔗 **Related Documentation**

- [Development Setup Guide](DEVELOPMENT_SETUP.md) - Initial setup guide
- [API Reference](API_REFERENCE.md) - API endpoints and responses
- [Deployment Guide](../DEPLOYMENT.md) - Production deployment guide

---

## 🎯 Rule Development Issues

### 🔧 Rule Not Triggering

**Symptoms**: Your custom rule doesn't run on relevant MRs

**Diagnostic Steps**:
```bash
# 1. Check if rule is registered
curl http://localhost:3000/api/rules | jq '.rules[] | select(.name=="your_rule")'

# 2. Check rule is enabled
env | grep YOUR_RULE_ENABLED

# 3. Check if Applies() is working
grep "Rule.*applies.*false" logs/
```

**Common Solutions**:
1. **File Pattern Mismatch** - Update `shouldValidateFile()` method
2. **Rule Not Registered** - Add to `internal/rules/registry.go`
3. **Rule Disabled** - Set `YOUR_RULE_ENABLED=true`

### 🚨 False Positives/Negatives  

**Symptoms**: Rule makes incorrect approve/manual review decisions

**Solutions**:
1. **Add Debug Logging** to validation logic
2. **Test Edge Cases** (empty files, malformed content, large files)
3. **Review Validation Logic** for regex patterns and business rules

### ⚡ Performance Issues

**Symptoms**: Rules are slow or timing out

**Solutions**:
1. **Add File Size Limits** in validation
2. **Optimize Regex Patterns** (compile once, reuse)
3. **Profile Performance** with `go test -bench=.`

### 📚 Rule Development Resources

For detailed rule development help:
- 🎯 [Rule Creation Guide](RULE_CREATION_GUIDE.md) - Complete implementation guide
- 🧪 [Development Setup Guide](DEVELOPMENT_SETUP.md) - Testing strategies and development setup
- 📁 [Adding New Rules Guide](ADDING_NEW_RULES.md) - Step-by-step rule creation

---

🔧 **Remember**: Most webhook issues are related to SSL configuration, authentication, or webhook URL configuration. For rule development issues, start with the [Rule Creation Guide](RULE_CREATION_GUIDE.md) and use debug logging extensively! 