---
name: live-review
description: >-
  Fetches a GitLab merge request via glab, builds a merge_request webhook JSON for Naysayer,
  POSTs it to a running local server, and summarizes the decision. Use when the user invokes
  /live-review, asks to live-test an MR against Naysayer, or to replay webhook review locally.
---

# Live review (`/live-review`)

## When to use

Apply this skill when the user wants to **simulate GitLab’s merge-request webhook** against their **running Naysayer** process using a **real MR URL** (typically `gitlab.cee.redhat.com` / `dataverse-config`), without editing fixtures.

## Prerequisites

- **Naysayer running locally** with `GITLAB_TOKEN` and `GITLAB_BASE_URL` set so it can call GitLab (same as production review).
- **`glab`** authenticated for that GitLab host (`glab auth login --hostname <host>`).
- **`jq`**, **`curl`**, **`python3`** on `PATH`.

## Steps (agent)

1. **Resolve MR URL** — Accept a URL shaped like  
   `https://<host>/<namespace>/<project>/-/merge_requests/<iid>`  
   (optional trailing `/diffs` is stripped). Paths may include nested groups.

2. **Fetch MR JSON from GitLab** — Use **`glab api`** (not raw curl to GitLab unless glab fails):  
   `GET /api/v4/projects/<url-encoded-path>/merge_requests/<iid>`  
   with `--hostname <host>` from the MR URL.

3. **Build webhook body** — Minimal payload Naysayer accepts:

   - `object_kind`: `"merge_request"`
   - `object_attributes`: `iid`, `title`, `source_branch`, `target_branch`, `state`, `work_in_progress` (from API; default `false` if missing)
   - `project`: `{ "id": <numeric project_id from API> }` — use **`project_id`** from the MR response
   - `user`: `{ "username": <author.username> }` (webhook user; author is fine for replay)

4. **POST to the local server**

   - **Method / path:** `POST http://127.0.0.1:<PORT>/dataverse-product-config-review`
   - **Port:** `PORT` from env (Naysayer default **3000**), overridable via `NAYSAYER_URL` if you document a full URL.
   - **Headers:** `Content-Type: application/json`, `X-Gitlab-Event: Merge Request Hook`

5. **Report** — Print HTTP status and summarize JSON fields: `decision.type`, `decision.reason`, `mr_approved`, `execution_time` when present.

## Quick command (repository helper)

From the repo root:

```bash
./.cursor/skills/live-review/scripts/live-review.sh 'https://gitlab.example.com/group/repo/-/merge_requests/123'
```

Optional:

- `PORT=3000` — listener port (default `3000`).
- `NAYSAYER_URL` — full URL override, e.g. `http://127.0.0.1:3000/dataverse-product-config-review`.

## Constraints

- Do **not** fabricate MR metadata — load it with **`glab api`** so `project.id`, branches, and state match GitLab.
- If MR `state` is not `opened`, the webhook validator may reject or the handler may skip; say so clearly.
- Draft/WIP skipping follows server logic (`IsDraftMR` / title checks).

## Endpoint reference

| Item | Value |
|------|--------|
| Route | `POST /dataverse-product-config-review` |
| Source | `cmd/main.go` (`setupRoutes`) |
