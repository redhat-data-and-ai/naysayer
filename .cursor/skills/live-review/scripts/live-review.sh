#!/usr/bin/env bash
# Replay a GitLab MR as a merge_request webhook against local Naysayer.
# Usage: ./live-review.sh <merge-request-url>
# Env: PORT (default 3000), NAYSAYER_URL (optional full override), GITLAB_TOKEN for glab.

set -euo pipefail

MR_URL="${1:?Usage: $0 <merge-request-url>}"

eval "$(MR_URL="$MR_URL" python3 <<'PY'
import os
import re
import shlex
import sys
from urllib.parse import quote, urlparse, unquote

u = urlparse(os.environ["MR_URL"])
host = u.hostname or ""
if not host:
    print("error: URL must include a host", file=sys.stderr)
    sys.exit(2)

path = u.path.rstrip("/") or "/"
if path.endswith("/diffs"):
    path = path[: -len("/diffs")]
m = re.match(r"^/(.+)/-/merge_requests/(\d+)$", path)
if not m:
    print("error: expected .../-/merge_requests/<iid>", file=sys.stderr)
    sys.exit(2)

proj = unquote(m.group(1))
iid = m.group(2)

segments = [s for s in proj.split("/") if s]
enc = "%2F".join(quote(s, safe="") for s in segments)

print(f"GITLAB_HOST={shlex.quote(host)}")
print(f"ENC_PROJECT={shlex.quote(enc)}")
print(f"MR_IID={shlex.quote(iid)}")
PY
)"

PORT="${PORT:-3000}"
BASE_URL="${NAYSAYER_URL:-http://127.0.0.1:${PORT}/dataverse-product-config-review}"

command -v glab >/dev/null 2>&1 || {
	echo "glab not found" >&2
	exit 127
}
command -v jq >/dev/null 2>&1 || {
	echo "jq not found" >&2
	exit 127
}

MR_JSON="$(glab api "projects/${ENC_PROJECT}/merge_requests/${MR_IID}" --hostname "${GITLAB_HOST}")"

PAYLOAD="$(echo "$MR_JSON" | jq '{
  object_kind: "merge_request",
  object_attributes: {
    iid: .iid,
    title: .title,
    source_branch: .source_branch,
    target_branch: .target_branch,
    state: .state,
    work_in_progress: (.work_in_progress // false)
  },
  project: { id: .project_id },
  user: { username: (.author.username // "unknown"), name: (.author.name // "") }
}')"

echo "POST ${BASE_URL}" >&2
curl -sS -w "\nHTTP %{http_code}\n" -X POST "${BASE_URL}" \
	-H "Content-Type: application/json" \
	-H "X-Gitlab-Event: Merge Request Hook" \
	-d "${PAYLOAD}"
