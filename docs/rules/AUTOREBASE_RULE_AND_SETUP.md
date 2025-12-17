# 🔄 Auto-Rebase Rule (Generic)

**Business Purpose**: Automatically rebases eligible merge requests when changes are pushed to the main branch, reducing manual maintenance overhead and keeping MRs up-to-date across any repository.

**Compliance Scope**: Provides automated Git operations for any repository to maintain merge request freshness and reduce merge conflicts. Can be configured with optional atlantis comment checking for repositories using Terraform/Atlantis.

## 📊 Policy Overview

```mermaid
graph TB
    subgraph "🔔 Webhook Trigger"
        A[Push to main/master branch] --> B{Webhook Received?}
        B -->|Yes| C[Process Push Event]
        B -->|No| D[Ignore]
    end
    
    subgraph "🔍 MR Filtering"
        C --> E[Get All Open MRs]
        E --> F{MR Created < 7 days?}
        F -->|No| G[Skip: Too Old]
        F -->|Yes| H{Pipeline Status?}
        H -->|Running/Pending| I[Skip: Active Pipeline]
        H -->|Success| J[Eligible for Rebase]
        H -->|Failed| K{All Jobs Succeeded?}
        K -->|No| I
        K -->|Yes| L{Check Atlantis?}
        L -->|No| I
        L -->|Yes| M{State Lock?}
        M -->|Yes| J
        M -->|No| I
    end
    
    subgraph "⚡ Rebase Action"
        J --> K[Rebase MR]
        K --> L{Rebase Success?}
        L -->|Yes| M[Add Comment to MR]
        L -->|No| N[Log Error]
    end
    
    classDef trigger fill:#e1f5fe,stroke:#0288d1,stroke-width:2px
    classDef filter fill:#fff3e0,stroke:#f57c00,stroke-width:2px
    classDef action fill:#d4edda,stroke:#155724,stroke-width:2px
    classDef skip fill:#f8d7da,stroke:#721c24,stroke-width:2px
    
    class A,B,C trigger
    class E,F,H,K,L,M filter
    class J,K,L,M action
    class D,G,I,N skip
```

## 🎯 What This Rule Does

**This feature**:
- Automatically rebases eligible merge requests when code is pushed to the main branch
- Filters MRs based on age (only MRs created within the last 7 days)
- Checks pipeline status and job success before rebasing
- Optionally checks atlantis comments for state lock vs plan failures (configurable)
- Posts comments to successfully rebased MRs to inform users

**Trigger**: Push events to `main` or `master` branch in any repository

**Endpoints**: 
- `POST /auto-rebase` (generic endpoint, recommended)
- `POST /fivetran-terraform-rebase` (legacy endpoint, backward compatibility)

## ⚙️ Setup Instructions

### 1. Create GitLab Access Token

**Step 1**: Navigate to GitLab Personal Access Tokens
- Go to **GitLab** → **User Settings** → **Access Tokens** (or **Preferences** → **Access Tokens`)
- Or visit: `https://your-gitlab-instance.com/-/user_settings/personal_access_tokens`

**Step 2**: Create New Token
1. Click **"Add new token"**
2. **Token name**: `naysayer-auto-rebase` (or descriptive name)
3. **Expiration date**: Set appropriate expiration (recommended: 1 year)
4. **Select scopes** (required):
   - ✅ `api` - Full API access
   - ✅ `read_api` - Read API access
   - ✅ `write_repository` - Write repository access (for rebase operations)

**Step 3**: Generate and Save Token
1. Click **"Create personal access token"**
2. **⚠️ IMPORTANT**: Copy the token immediately - it won't be shown again!
3. Save it securely (password manager, secret management system)

**Token Format**: `glpat-xxxxxxxxxxxxxxxxxxxx` (GitLab Personal Access Token)

### 2. Configure Environment Variables

**Option A: Repository-Specific Token (Recommended)**
```bash
# Set repository-specific token (supports both new and legacy env var names)
export AUTO_REBASE_REPOSITORY_TOKEN="glpat-your-repository-token-here"
# Or use legacy name for backward compatibility:
export GITLAB_TOKEN_FIVETRAN="glpat-your-repository-token-here"
```

**Option B: Use Main Token**
```bash
# If repository-specific token is not set, the system will use GITLAB_TOKEN
export GITLAB_TOKEN="glpat-your-main-token-here"
```

**Required Configuration**:
```bash
# GitLab instance URL
export GITLAB_BASE_URL="https://your-gitlab-instance.com"

# Server port (default: 3000)
export PORT="3000"

# Enable auto-rebase feature (default: true)
export AUTO_REBASE_ENABLED="true"

# Enable atlantis comment checking (default: false)
# Set to true for repositories using Terraform/Atlantis
export AUTO_REBASE_CHECK_ATLANTIS_COMMENTS="true"
```

### 3. Configure GitLab Webhook

**Step 1**: Navigate to Project Webhooks
- Go to your **repository** in GitLab
- Navigate to **Settings** → **Webhooks**
- Or visit: `https://your-gitlab-instance.com/your-group/your-repo/-/hooks`

**Step 2**: Add New Webhook
1. Click **"Add webhook"**
2. **URL**: `https://your-naysayer-domain.com/auto-rebase`
   - Replace `your-naysayer-domain.com` with your actual Naysayer deployment URL
   - Example: `https://naysayer.example.com/auto-rebase`
   - **Note**: Legacy endpoint `/fivetran-terraform-rebase` is also supported for backward compatibility
3. **Secret token** (optional but recommended):
   - Generate a secure random token
   - Store it in `WEBHOOK_SECRET` environment variable
   - Enter the same token in GitLab webhook configuration

**Step 3**: Select Trigger Events
- ✅ **Push events** - Required (this is what triggers the rebase)
- ❌ Merge request events - Not needed (feature uses push events)

**Step 4**: Configure Additional Settings
- ✅ **Enable SSL verification** - Recommended for production
- ✅ **Trigger on** - Select "Push events" only
- ✅ **Branch filter** (optional): `main` or `master` - Only rebase on pushes to main branch

**Step 5**: Save and Test
1. Click **"Add webhook"** or **"Save webhook"**
2. Click **"Test"** → **"Push events"** to verify webhook connectivity
3. Check Naysayer logs to confirm webhook is received

### 4. Verify Configuration

**Test Token Permissions**:
```bash
# Verify token has required scopes
curl -H "Authorization: Bearer $GITLAB_TOKEN_FIVETRAN" \
  "$GITLAB_BASE_URL/api/v4/personal_access_tokens/self" | jq '.scopes'

# Expected output should include: ["api"]
```

**Test Webhook Endpoint**:
```bash
# Test webhook endpoint to trigger rebase (generic endpoint)
curl -X POST https://your-naysayer-domain.com/auto-rebase \
  -H "Content-Type: application/json" \
  -d '{
    "object_kind": "push",
    "ref": "refs/heads/main",
    "project": {"id": YOUR_PROJECT_ID},
    "commits": [{"id": "abc123", "message": "Test commit"}]
  }'

# Legacy endpoint (also supported)
curl -X POST https://your-naysayer-domain.com/fivetran-terraform-rebase \
  -H "Content-Type: application/json" \
  -d '{
    "object_kind": "push",
    "ref": "refs/heads/main",
    "project": {"id": YOUR_PROJECT_ID},
    "commits": [{"id": "abc123", "message": "Test commit"}]
  }'
```

**Check Service Logs**:
```bash
# View Naysayer logs to verify webhook processing
kubectl logs -f deployment/naysayer | grep -i "auto-rebase\|rebase"

# Or if running locally
./naysayer | grep -i "auto-rebase\|rebase"
```

## 🔍 How It Works

### Eligibility Criteria

An MR is eligible for auto-rebase if **ALL** of the following are true:

1. ✅ **Age**: Created within the last **7 days**
2. ✅ **Pipeline Status**: 
   - Pipeline is `success` → Rebase directly
   - Pipeline is `failed` → Check jobs and optionally atlantis comments (see below)
   - Pipeline is `null` (no pipeline) → Rebase
3. ✅ **State**: MR is in `opened` state

### Skip Conditions

An MR is **skipped** (not rebased) if **ANY** of the following are true:

1. ❌ **Too Old**: Created more than 7 days ago
2. ❌ **Active Pipeline**: Pipeline status is `running` or `pending`
3. ❌ **Failed Pipeline**: 
   - If `AUTO_REBASE_CHECK_ATLANTIS_COMMENTS=false`: Skip all failed pipelines
   - If `AUTO_REBASE_CHECK_ATLANTIS_COMMENTS=true`: 
     - Skip if jobs failed
     - Skip if atlantis comment indicates plan error (not state lock)
     - Allow rebase if atlantis comment indicates state lock

### Atlantis Comment Checking (Optional)

When `AUTO_REBASE_CHECK_ATLANTIS_COMMENTS=true`:
- **Checks for atlantis-bot comments** in failed pipelines
- **State Lock Detection**: If comment contains "Error: Error acquiring the state lock" → Allow rebase
- **Plan Error Detection**: If comment contains other errors → Skip rebase
- **No Comment Found**: Skip rebase (safe default)

### Rebase Process

1. **Webhook Trigger**: Push to `main`/`master` branch triggers webhook
2. **MR Discovery**: System fetches all open MRs created in last 7 days
3. **Filtering**: MRs are filtered based on:
   - Pipeline status (success → rebase, failed → check jobs)
   - Job status (all jobs must succeed for failed pipelines)
   - Atlantis comments (if enabled, check for state lock vs plan errors)
4. **Rebase**: Eligible MRs are rebased sequentially
5. **Notification**: Successfully rebased MRs receive an automated comment

## 📋 Example Scenarios

### ✅ Scenario 1: Successful Auto-Rebase

**Setup**:
- MR created 3 days ago
- Pipeline status: `success`
- Push to main branch occurs

**Result**:
```json
{
  "webhook_response": "processed",
  "status": "completed",
  "total_mrs": 1,
  "eligible_mrs": 1,
  "successful": 1,
  "failed": 0,
  "skipped": 0
}
```

**MR Comment Added**:
> 🤖 **Automated Rebase**
> 
> This merge request has been automatically rebased with the latest changes from the target branch.
> 
> _This is an automated action triggered by a push to the main branch._

### ⏭️ Scenario 2: MR Skipped (Too Old)

**Setup**:
- MR created 10 days ago
- Pipeline status: `success`
- Push to main branch occurs

**Result**:
```json
{
  "webhook_response": "processed",
  "status": "completed",
  "total_mrs": 1,
  "eligible_mrs": 0,
  "successful": 0,
  "skipped": 1,
  "skip_details": [
    {
      "mr_iid": 123,
      "reason": "too_old",
      "created_at": "2025-11-01T10:00:00Z"
    }
  ]
}
```

### ⏭️ Scenario 3: MR Skipped (Running Pipeline)

**Setup**:
- MR created 2 days ago
- Pipeline status: `running`
- Push to main branch occurs

**Result**:
```json
{
  "webhook_response": "processed",
  "status": "completed",
  "total_mrs": 1,
  "eligible_mrs": 0,
  "successful": 0,
  "skipped": 1,
  "skip_details": [
    {
      "mr_iid": 123,
      "reason": "pipeline_running",
      "pipeline_id": 45678
    }
  ]
}
```

## 🔧 Troubleshooting

### Webhook Not Triggering

**Symptoms**: No rebase occurs when pushing to main branch

**Solutions**:
1. **Verify webhook URL**: Check webhook configuration in GitLab matches your Naysayer URL
2. **Check webhook logs**: In GitLab, go to **Settings** → **Webhooks** → Click webhook → View **Recent events**
3. **Verify endpoint**: Ensure `/fivetran-terraform-rebase` endpoint is accessible
4. **Check Naysayer logs**: Look for webhook receipt logs

**Debug Commands**:
```bash
# Test webhook endpoint directly (generic endpoint)
curl -X POST https://your-naysayer-domain.com/auto-rebase \
  -H "Content-Type: application/json" \
  -d '{"object_kind": "push", "ref": "refs/heads/main", "project": {"id": YOUR_PROJECT_ID}}'

# Check Naysayer service logs
kubectl logs -f deployment/naysayer | grep -i "auto-rebase\|rebase"
```

### Token Permission Errors

**Symptoms**: `403 Forbidden` or `401 Unauthorized` errors in logs

**Solutions**:
1. **Verify token scopes**: Ensure token has `api`, `read_api`, and `write_repository` scopes
2. **Check token expiration**: Verify token hasn't expired
3. **Verify token format**: Should start with `glpat-`
4. **Check environment variable**: Ensure `GITLAB_TOKEN_FIVETRAN` is set correctly

**Debug Commands**:
```bash
# Test token permissions (use your repository token)
curl -H "Authorization: Bearer $AUTO_REBASE_REPOSITORY_TOKEN" \
  "$GITLAB_BASE_URL/api/v4/user" | jq '.'

# Verify token scopes
curl -H "Authorization: Bearer $AUTO_REBASE_REPOSITORY_TOKEN" \
  "$GITLAB_BASE_URL/api/v4/personal_access_tokens/self" | jq '.scopes'
```

### MRs Not Being Rebased

**Symptoms**: Eligible MRs exist but aren't being rebased

**Solutions**:
1. **Check MR age**: Verify MRs are less than 7 days old
2. **Check pipeline status**: Ensure pipelines are not `running`, `pending`, or `failed`
3. **Verify MR state**: Ensure MRs are in `opened` state
4. **Check logs**: Look for skip reasons in Naysayer logs

**Debug Commands**:
```bash
# List open MRs and their details
curl -H "Authorization: Bearer $AUTO_REBASE_REPOSITORY_TOKEN" \
  "$GITLAB_BASE_URL/api/v4/projects/YOUR_PROJECT_ID/merge_requests?state=opened" \
  | jq '.[] | {iid: .iid, created_at: .created_at, pipeline: .pipeline}'

# Check specific MR details
curl -H "Authorization: Bearer $AUTO_REBASE_REPOSITORY_TOKEN" \
  "$GITLAB_BASE_URL/api/v4/projects/YOUR_PROJECT_ID/merge_requests/MR_IID" \
  | jq '{iid: .iid, created_at: .created_at, pipeline: .pipeline, state: .state}'
```

### Rebase Failures

**Symptoms**: Rebase attempts fail with errors

**Solutions**:
1. **Check MR mergeability**: Ensure MR can be rebased (no conflicts, not locked)
2. **Verify branch permissions**: Ensure token has write access to repository
3. **Check GitLab rate limits**: Verify you're not hitting API rate limits
4. **Review error details**: Check `failures` array in webhook response

**Debug Commands**:
```bash
# Check MR mergeability
curl -H "Authorization: Bearer $AUTO_REBASE_REPOSITORY_TOKEN" \
  "$GITLAB_BASE_URL/api/v4/projects/YOUR_PROJECT_ID/merge_requests/MR_IID" \
  | jq '{mergeable: .mergeable, merge_status: .merge_status, rebase_in_progress: .rebase_in_progress}'

# Manually trigger rebase to test
curl -X PUT -H "Authorization: Bearer $AUTO_REBASE_REPOSITORY_TOKEN" \
  "$GITLAB_BASE_URL/api/v4/projects/YOUR_PROJECT_ID/merge_requests/MR_IID/rebase"
```

## 🔒 Security Considerations

### Token Security

- **Never commit tokens to version control**
- **Use repository-specific tokens** (recommended) for better security isolation
- **Rotate tokens regularly** (recommended: every 90 days)
- **Use least privilege**: Only grant required scopes
- **Store tokens securely**: Use secret management systems (Kubernetes secrets, Vault, etc.)

### Webhook Security

- **Use HTTPS**: Always use HTTPS for webhook URLs
- **Enable SSL verification**: Verify SSL certificates in production
- **Use webhook secrets**: Configure `WEBHOOK_SECRET` for additional security
- **Restrict IPs** (optional): Use `WEBHOOK_ALLOWED_IPS` to restrict webhook sources

### Access Control

- **Repository permissions**: Ensure token only has access to the target repository
- **Branch protection**: Consider branch protection rules for main branch
- **Audit logging**: Monitor webhook and rebase activities in logs
- **Configuration**: Use `AUTO_REBASE_CHECK_ATLANTIS_COMMENTS` to enable/disable atlantis checking per repository

## 📊 Monitoring

### Key Metrics

Monitor these metrics to ensure the feature is working correctly:

- **Webhook receipt rate**: Number of webhooks received per day
- **Rebase success rate**: Percentage of successful rebases
- **Skip reasons**: Distribution of skip reasons (too old, pipeline status, etc.)
- **Error rate**: Number of failed rebase attempts

### Log Monitoring

**Key log messages to monitor**:
- `Push to main branch detected` - Webhook received
- `Found X eligible MRs to rebase` - MRs identified for rebase
- `Successfully triggered rebase for MR` - Successful rebase
- `Failed to rebase MR` - Rebase failure
- `Skipping MR with X pipeline` - MR skipped due to pipeline status
- `Skipping MR with failed pipeline due to plan error` - Plan error detected (atlantis check enabled)
- `Skipping MR with failed pipeline (no atlantis comment found)` - No atlantis comment (atlantis check enabled)

**Example log query**:
```bash
# Filter logs for auto-rebase activity
kubectl logs -f deployment/naysayer | grep -i "auto-rebase\|rebase"

# Count successful rebases
kubectl logs deployment/naysayer | grep "Successfully triggered rebase" | wc -l
```

## 🆘 Getting Help

### Escalation Path

1. **Self-service**: Review this documentation and troubleshooting section
2. **Check logs**: Review Naysayer and GitLab webhook logs
3. **Platform support**: Contact platform team for configuration issues
4. **GitLab support**: Contact GitLab support for API or webhook issues

### Information to Provide

When requesting help, provide:
- **Webhook response**: Full JSON response from webhook endpoint
- **MR details**: MR IID, creation date, pipeline status
- **Error messages**: Complete error text from logs
- **Configuration**: Environment variables (without exposing tokens)
- **Timeline**: When the issue occurred and frequency

## 📚 Related Documentation

- **[API Reference](../API_REFERENCE.md)** - Complete API documentation
- **[Troubleshooting Guide](../TROUBLESHOOTING.md)** - General troubleshooting

---

**💡 Pro Tip**: Set up monitoring alerts for rebase failures to catch issues early. Monitor the skip reasons to understand why MRs aren't being rebased and adjust thresholds if needed.

