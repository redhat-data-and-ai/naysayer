# Fivetran Terraform Rebase Webhook

## Overview

The Fivetran Terraform Rebase webhook enables automatic rebasing of merge requests in GitLab repositories. This feature is specifically designed for the `fivetran_terraform` repository to streamline the merge request workflow.

## Features

- **Automatic Rebase**: Automatically triggers rebase operations for open merge requests
- **Status Comments**: Optionally posts comments to merge requests indicating rebase success or failure
- **Error Handling**: Comprehensive error handling with detailed error messages
- **Event Validation**: Validates webhook payloads and only processes merge request events

## Configuration

### Naysayer Setup

1. **Environment Variables**: Ensure the following environment variables are set:
   ```bash
   GITLAB_TOKEN=<your-gitlab-token>
   GITLAB_BASE_URL=https://gitlab.cee.redhat.com
   ENABLE_MR_COMMENTS=true  # Optional: Enable/disable comments
   ```

2. **GitLab Token Permissions**: The GitLab token must have the following permissions:
   - `api` - Full API access
   - Maintainer role on the target repository

3. **Deploy Naysayer**: The webhook will be available at:
   ```
   POST /fivetran-terraform-rebase
   ```

### GitLab Webhook Setup

1. Navigate to your GitLab repository (e.g., `fivetran_terraform`)
2. Go to **Settings > Webhooks**
3. Add a new webhook with the following configuration:

   **URL**: `https://your-naysayer-instance/fivetran-terraform-rebase`
   
   **Trigger**: Select **Merge request events**
   
   **SSL Verification**: Enable (recommended)

4. Test the webhook using the "Test" button

## How It Works

### Webhook Flow

1. **Webhook Received**: GitLab sends a merge request event to Naysayer
2. **Validation**: Naysayer validates the webhook payload
3. **Event Check**: Verifies the event is a merge request event
4. **State Check**: Ensures the MR is in "opened" state
5. **Rebase Trigger**: Calls GitLab API to trigger rebase
6. **Comment (Optional)**: Posts a status comment to the MR
7. **Response**: Returns success/failure response to GitLab

### Supported Events

- **Merge Request Events**: The webhook only processes merge request events
- **Open MRs Only**: Rebase is only triggered for MRs in "opened" state

### Response Format

#### Success Response
```json
{
  "webhook_response": "processed",
  "event_type": "merge_request_rebase",
  "status": "success",
  "rebased": true,
  "project_id": 456,
  "mr_iid": 123,
  "message": "Rebase operation triggered successfully"
}
```

#### Skipped Response (Non-Open MR)
```json
{
  "webhook_response": "processed",
  "event_type": "merge_request_rebase",
  "status": "skipped",
  "reason": "MR state is 'merged', only processing open MRs",
  "rebased": false,
  "project_id": 456,
  "mr_iid": 123
}
```

#### Error Response
```json
{
  "error": "Failed to trigger rebase: rebase already in progress",
  "rebased": false,
  "project_id": 456,
  "mr_iid": 123
}
```

## MR Comments

When `ENABLE_MR_COMMENTS=true`, Naysayer will post comments to merge requests:

### Success Comment
```markdown
🔄 **Naysayer Rebase Triggered**

Automatic rebase has been initiated for this merge request.

The rebase operation is running in the background. Please wait a few moments for it to complete.
```

### Failure Comment
```markdown
🔄 **Naysayer Rebase Failed**

Failed to trigger automatic rebase for this merge request.

**Error:** [error details]

Please manually rebase or check the merge request status.
```

## API Reference

### Endpoint
`POST /fivetran-terraform-rebase`

### Headers
- `Content-Type: application/json`

### Request Body

GitLab webhook payload (merge request event):

```json
{
  "object_kind": "merge_request",
  "object_attributes": {
    "iid": 123,
    "state": "opened",
    "title": "Update Terraform config"
  },
  "project": {
    "id": 456
  },
  "user": {
    "username": "developer"
  }
}
```

### Response Codes

- `200 OK` - Rebase triggered successfully or skipped
- `400 Bad Request` - Invalid payload or unsupported event type
- `500 Internal Server Error` - Failed to trigger rebase

## Troubleshooting

### Common Issues

#### 1. Rebase Not Triggered

**Symptom**: Webhook receives event but rebase doesn't happen

**Solutions**:
- Verify the MR is in "opened" state
- Check GitLab token has maintainer permissions
- Review Naysayer logs for detailed error messages
- Ensure no rebase is already in progress

#### 2. Permission Errors

**Symptom**: Error message indicates insufficient permissions

**Solutions**:
- Verify GitLab token has `api` scope
- Ensure token user has maintainer role on repository
- Check if MR allows rebasing (not locked or protected)

#### 3. Conflict Errors

**Symptom**: Rebase fails with conflict message

**Solutions**:
- This is expected behavior when conflicts exist
- Manual resolution of conflicts is required
- Naysayer will add a comment indicating the failure

#### 4. No Comments Posted

**Symptom**: Rebase works but no comments appear

**Solutions**:
- Verify `ENABLE_MR_COMMENTS=true` in configuration
- Check GitLab token has permission to comment
- Review Naysayer logs for comment-related errors

### Debugging

Enable debug logging to see detailed webhook processing:

```bash
LOG_LEVEL=debug
```

Check Naysayer logs for entries like:
```
Processing rebase request | project_id=456 mr_iid=123
Triggering rebase operation | mr_iid=123
Rebase triggered successfully | mr_iid=123
```

## Security Considerations

1. **Token Security**: Store GitLab tokens securely
2. **SSL/TLS**: Always use HTTPS for webhook endpoints
3. **Webhook Secrets**: Consider using GitLab webhook secrets (future enhancement)
4. **IP Allowlisting**: Configure allowed IPs in Naysayer config if needed

## Testing

### Manual Testing

1. Create a test MR in your repository
2. Trigger the webhook manually from GitLab UI (Settings > Webhooks > Test)
3. Verify the rebase operation in GitLab
4. Check for status comments on the MR

### Automated Testing

Run the test suite:
```bash
go test ./internal/webhook/... -v -run TestFivetranTerraformRebaseHandler
```

## Examples

### Example 1: Basic Rebase Trigger

When an MR is updated and pushed to GitLab:
1. GitLab sends webhook event to Naysayer
2. Naysayer validates the event
3. Rebase is triggered automatically
4. Success comment is posted to MR

### Example 2: Conflict Detection

When a rebase would result in conflicts:
1. GitLab API returns 409 (Conflict) status
2. Naysayer captures the error
3. Failure comment is posted with error details
4. Developer resolves conflicts manually

## Future Enhancements

Potential future improvements:
- [ ] Webhook secret validation
- [ ] Configurable rebase strategies
- [ ] Retry logic for transient failures
- [ ] Metrics and monitoring
- [ ] Support for rebase options (skip_ci, etc.)

## Related Documentation

- [API Reference](./API_REFERENCE.md)
- [Development Setup](./DEVELOPMENT_SETUP.md)
- [Troubleshooting](./TROUBLESHOOTING.md)

## Support

For issues or questions:
- Review Naysayer logs
- Check GitLab webhook delivery logs
- Consult the troubleshooting section above

