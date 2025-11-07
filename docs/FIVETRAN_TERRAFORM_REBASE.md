# Fivetran Terraform Automatic Rebase

## Overview

The Fivetran Terraform Automatic Rebase feature keeps all open merge requests (MRs) up-to-date with the main branch. Whenever a commit is pushed to the `main` or `master` branch (typically when an MR is merged), **all remaining open MRs are automatically rebased** to ensure they're synchronized with the latest changes.

## Key Features

- **Automatic Multi-MR Rebase**: Rebases **all** open MRs when main branch is updated
- **Push-to-Main Trigger**: Activates automatically on any push to main/master branch
- **Batch Processing**: Handles multiple MRs efficiently in a single webhook event
- **Error Resilience**: Continues rebasing other MRs even if some fail
- **Detailed Reporting**: Provides comprehensive success/failure statistics
- **Separate Token Support**: Optional dedicated GitLab token for Fivetran operations

## How It Works

### Trigger Flow

```
Developer merges MR #123 → main
         ↓
GitLab fires "push to main" webhook
         ↓
Naysayer receives webhook
         ↓
Naysayer queries GitLab for all open MRs
         ↓
Naysayer rebases MR #456, #789, #101, ... (all open MRs)
         ↓
Returns summary: successful/failed counts
```

### Why This Approach?

When any change lands on `main`, all other open MRs become potentially out-of-sync:

```
Before merge:
main:           A---B---C
MR #456:             D---E  (out of sync)
MR #789:             F---G  (out of sync)

After MR #123 merged:
main:           A---B---C---H  ← New commit
MR #456:             D---E      ← Still based on old main
MR #789:             F---G      ← Still based on old main

After automatic rebase:
main:           A---B---C---H
MR #456:                    D'---E'  ← Rebased on latest main
MR #789:                    F'---G'  ← Rebased on latest main
```

This approach:
- ✅ Reduces merge conflicts
- ✅ Keeps all MRs current with latest main
- ✅ Simplifies code review (reviewers see code against latest main)
- ✅ Catches integration issues early

## Configuration

### 1. Naysayer Configuration

#### Environment Variables

```bash
# Required: GitLab API access
GITLAB_TOKEN=<your-gitlab-token>
GITLAB_BASE_URL=https://your-gitlab-instance.com

# Optional: Dedicated token for Fivetran operations
GITLAB_TOKEN_FIVETRAN=<fivetran-specific-token>

# Optional: Enable/disable MR comments (currently not used for push events)
ENABLE_MR_COMMENTS=true
```

**Token Selection Logic:**
- If `GITLAB_TOKEN_FIVETRAN` is set, it will be used for Fivetran rebase operations
- If not set, falls back to `GITLAB_TOKEN`
- This allows separate permissions for different projects

#### GitLab Token Requirements

The GitLab token (either `GITLAB_TOKEN` or `GITLAB_TOKEN_FIVETRAN`) must have:

| Requirement | Description |
|------------|-------------|
| **Scope** | `api` - Full API access |
| **Role** | `Maintainer` or higher on the `fivetran_terraform` project |
| **Permissions** | Ability to rebase MRs and list project MRs |

**Recommended: Use a Project Access Token**
1. Go to `fivetran_terraform` project → Settings → Access Tokens
2. Create token with:
   - Role: `Maintainer`
   - Scopes: `api`, `write_repository`
   - Name: `naysayer-rebase-bot`
3. Set as `GITLAB_TOKEN_FIVETRAN` in Naysayer

#### Webhook Endpoint

Once deployed, the webhook will be available at:
```
POST /fivetran-terraform-rebase
```

### 2. GitLab Webhook Setup

Configure the webhook in your GitLab `fivetran_terraform` repository:

1. **Navigate to Webhooks**
   - Go to: `fivetran_terraform` → Settings → Webhooks

2. **Add New Webhook**
   - **URL**: `https://your-naysayer-instance.com/fivetran-terraform-rebase`
   - **Trigger**: ☑️ **Push events** ONLY
   - **Branch filter**: `main` (or `master` if that's your default branch)
   - **SSL Verification**: ✅ Enable (recommended)
   - Click **Add webhook**

3. **Test the Webhook**
   - Use the "Test" button → "Push events"
   - Verify you get a 200 OK response
   - Check that the response indicates how many MRs would be rebased

⚠️ **Important**: Do NOT enable "Merge request events" - this webhook responds to push events only.

## API Reference

### Endpoint
```
POST /fivetran-terraform-rebase
```

### Headers
```
Content-Type: application/json
```

### Request Body

GitLab **push event** payload:

```json
{
  "object_kind": "push",
  "ref": "refs/heads/main",
  "project": {
    "id": 94023,
    "name": "fivetran_terraform",
    "web_url": "https://gitlab.cee.redhat.com/dataverse/platform-tooling/fivetran_terraform"
  },
  "user_username": "developer",
  "commits": [
    {
      "id": "abc123...",
      "message": "Merged MR !123",
      "timestamp": "2025-11-07T10:30:00Z"
    }
  ]
}
```

### Response Format

#### Success - All MRs Rebased
```json
{
  "webhook_response": "processed",
  "status": "completed",
  "project_id": 94023,
  "branch": "main",
  "total_mrs": 5,
  "successful": 5,
  "failed": 0
}
```

#### Success - Some MRs Failed
```json
{
  "webhook_response": "processed",
  "status": "completed",
  "project_id": 94023,
  "branch": "main",
  "total_mrs": 5,
  "successful": 3,
  "failed": 2,
  "failures": [
    {
      "mr_iid": 456,
      "error": "rebase failed: conflicts detected"
    },
    {
      "mr_iid": 789,
      "error": "insufficient permissions or rebase not allowed"
    }
  ]
}
```

#### Success - No Open MRs
```json
{
  "webhook_response": "processed",
  "status": "completed",
  "project_id": 94023,
  "branch": "main",
  "total_mrs": 0,
  "successful": 0,
  "failed": 0
}
```

#### Skipped - Push to Non-Main Branch
```json
{
  "webhook_response": "processed",
  "status": "skipped",
  "reason": "Push to feature-branch branch, only main/master triggers rebase",
  "branch": "feature-branch"
}
```

#### Error - Invalid Payload
```json
{
  "error": "Missing project information"
}
```

#### Error - Unsupported Event
```json
{
  "error": "Unsupported event type: merge_request. Only push events are supported."
}
```

### Response Codes

| Code | Meaning |
|------|---------|
| `200 OK` | Webhook processed successfully (may include partial failures) |
| `400 Bad Request` | Invalid payload, wrong event type, or missing required fields |
| `500 Internal Server Error` | Failed to list open MRs or other server error |

## Supported Events

| Event Type | Branch | Action |
|-----------|--------|--------|
| ✅ Push | `main` | Rebase all open MRs |
| ✅ Push | `master` | Rebase all open MRs |
| ❌ Push | Other branches | Skipped (no action) |
| ❌ Merge Request | Any | Rejected (use push events) |
| ❌ Other events | Any | Rejected |

## Real-World Usage Examples

### Example 1: Normal Merge Workflow

**Scenario**: Developer merges MR !123 into main

```
1. Developer clicks "Merge" on MR !123
2. GitLab merges the MR and fires push-to-main webhook
3. Naysayer receives webhook
4. Naysayer finds 4 other open MRs: !456, !789, !101, !202
5. Naysayer rebases all 4 MRs
6. Response: "successful": 4, "failed": 0
```

### Example 2: Some Rebases Fail

**Scenario**: Multiple MRs exist, some have conflicts

```
1. MR !100 is merged to main
2. Webhook triggers rebase of open MRs: !101, !102, !103
3. Results:
   - !101: ✅ Rebased successfully
   - !102: ❌ Failed (conflicts with new main)
   - !103: ✅ Rebased successfully
4. Response shows: "successful": 2, "failed": 1
5. Developer of !102 must manually resolve conflicts
```

### Example 3: Direct Push to Main

**Scenario**: Hotfix pushed directly to main (bypassing MR)

```
1. Developer runs: git push origin main
2. GitLab fires push-to-main webhook
3. All open MRs are rebased automatically
4. Ensures all MRs include the hotfix changes
```

### Example 4: No Open MRs

**Scenario**: All MRs are closed/merged

```
1. Last MR is merged to main
2. Webhook fires
3. Naysayer queries for open MRs
4. Finds: 0 open MRs
5. Response: "No open MRs to rebase"
```

## Troubleshooting

### Issue 1: "401 Unauthorized" Error

**Symptom**: Logs show `rebase failed with status 401`

**Solutions**:
1. Verify `GITLAB_TOKEN_FIVETRAN` (or `GITLAB_TOKEN`) is set correctly
2. Check token hasn't expired
3. Ensure token has `api` scope
4. Test token manually:
   ```bash
   curl -H "Authorization: Bearer $GITLAB_TOKEN_FIVETRAN" \
     https://your-gitlab.com/api/v4/user
   ```

### Issue 2: "403 Forbidden - Cannot push to source branch"

**Symptom**: Rebase fails with permission error

**Root Causes**:
- Token user doesn't have Maintainer role
- Using a Personal Access Token (PAT) of a user without proper access
- Branch protection rules preventing rebase

**Solutions**:
1. **Use Project Access Token** (recommended):
   - Create at: Project → Settings → Access Tokens
   - Role: `Maintainer`
   - Scopes: `api`, `write_repository`

2. **Or grant user Maintainer access**:
   - Go to: Project → Members
   - Add user or update role to `Maintainer`

3. **Check branch protection**:
   - Ensure source branches allow rebasing

### Issue 3: Rebase Not Triggered

**Symptom**: Push to main happens but no rebase occurs

**Checklist**:
- [ ] Webhook is configured for **Push events** (not Merge Request events)
- [ ] Branch filter is set to `main` (or `master`)
- [ ] Webhook URL is correct
- [ ] Webhook is enabled (not disabled)
- [ ] Naysayer service is running and accessible
- [ ] Check GitLab webhook delivery logs for errors

### Issue 4: Some MRs Always Fail

**Common Reasons**:

| Reason | How to Check | Solution |
|--------|--------------|----------|
| **Conflicts** | Check GitLab MR page | Manually resolve conflicts |
| **Fork MRs** | Check if source is forked repo | GitLab API limitation, manual rebase needed |
| **Protected branches** | Check branch protection rules | Adjust rules or exclude MR |
| **Large MRs** | Check size/changes | May timeout, split into smaller MRs |

### Issue 5: Webhook Times Out

**Symptom**: Many open MRs cause slow response

**Solutions**:
- This is expected with 50+ open MRs
- GitLab will retry automatically
- Consider reviewing and closing stale MRs
- MRs are rebased even if webhook times out

## Debugging

### Enable Debug Logging

```bash
LOG_LEVEL=debug
```

### Key Log Messages

Look for these in Naysayer logs:

```
# Webhook received
Push to main branch detected, rebasing all open MRs

# Counting MRs
Found 5 open MRs to rebase

# Per-MR processing
Attempting to rebase MR | mr_iid=456
Successfully triggered rebase for MR | mr_iid=456
Failed to rebase MR | mr_iid=789 | error=conflicts detected

# Summary
Rebase operation completed | total=5 successful=4 failed=1
```

### Testing Locally

Use the provided test scripts:

```bash
# Just count open MRs (no rebase)
./count_open_mrs_only.sh

# Full test with local Naysayer
./test_count_open_mrs.sh
```

See `TESTING_GUIDE.md` for detailed instructions.

## Performance Considerations

### How Many MRs Can It Handle?

- **< 10 MRs**: Near-instant processing
- **10-30 MRs**: Processes in 5-15 seconds
- **30-50 MRs**: May take 30-60 seconds
- **50+ MRs**: May timeout, but GitLab retries automatically

### Optimization Tips

1. **Close stale MRs**: Regularly review and close old MRs
2. **Merge frequently**: Don't let MRs pile up
3. **Use draft MRs**: Mark WIP MRs as draft (currently still rebased, future enhancement)

## Security Best Practices

1. **Token Management**
   - Use Project Access Tokens over Personal Access Tokens
   - Rotate tokens regularly
   - Store tokens in secure secret management (Kubernetes secrets, Vault)
   - Use minimum required scopes

2. **Network Security**
   - Always use HTTPS for webhook endpoint
   - Enable SSL verification in GitLab webhook config
   - Consider IP allowlisting if possible

3. **Access Control**
   - Grant Maintainer role only where needed
   - Audit token usage regularly
   - Monitor webhook delivery logs

## Monitoring & Metrics

### Key Metrics to Track

- **Rebase success rate**: `successful / total_mrs`
- **Average rebase time**: Time to process all MRs
- **Common failure reasons**: Track error patterns
- **Webhook delivery success**: Monitor in GitLab

### Recommended Alerts

- Rebase success rate < 80%
- Webhook delivery failures
- Token expiration warnings
- Unusual number of open MRs

## Limitations

### Current Limitations

1. **Fork MRs**: Cannot rebase MRs from forked repositories (GitLab API limitation)
2. **Draft MRs**: Currently rebases draft MRs (may change in future)
3. **No retry logic**: Failed rebases must be triggered manually
4. **No webhook secrets**: Payload validation not yet implemented

### Known Issues

- **Conflict handling**: MRs with conflicts will fail (expected behavior)
- **Closed MRs**: Closed/merged MRs are correctly skipped
- **Branch protection**: Very strict protection rules may prevent rebase

## Future Enhancements

Potential improvements:

- [ ] Support webhook secret validation
- [ ] Skip draft MRs option
- [ ] Configurable retry logic for transient failures
- [ ] Rate limiting for large projects
- [ ] Metrics endpoint (Prometheus format)
- [ ] Selective rebase (filter by label, assignee, etc.)
- [ ] Notification integrations (Slack, email)

## Related Documentation

- [Main API Reference](./API_REFERENCE.md) - All Naysayer endpoints
- [Testing Guide](../TESTING_GUIDE.md) - How to test locally
- [Push-to-Main Implementation](../PUSH_TO_MAIN_IMPLEMENTATION.md) - Technical details
- [Deployment Checklist](../DEPLOYMENT_CHECKLIST.md) - Production deployment steps

## Support & Contributing

### Getting Help

1. Check this documentation
2. Review Naysayer logs
3. Check GitLab webhook delivery logs
4. Review [Troubleshooting](#troubleshooting) section above

### Contributing

Found a bug or have a feature request? Please open an issue on GitHub!

---

**Last Updated**: November 2025  
**Feature Version**: 1.0 (Push-to-Main)
