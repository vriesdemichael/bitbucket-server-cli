# Repository settings command tree and endpoint mapping

This document captures the initial issue #1 UX contract for repository settings parity.

## Command tree (initial slice)

- `bbsc repo settings security permissions users list [--repo PROJECT/slug] [--limit N]`
- `bbsc repo settings security permissions users grant <username> <REPO_READ|REPO_WRITE|REPO_ADMIN> [--repo PROJECT/slug]`
- `bbsc repo settings workflow webhooks list [--repo PROJECT/slug]`
- `bbsc repo settings workflow webhooks create <name> <url> [--event ...] [--active] [--repo PROJECT/slug]`
- `bbsc repo settings workflow webhooks delete <webhook-id> [--repo PROJECT/slug]`
- `bbsc repo settings pull-requests get [--repo PROJECT/slug]`
- `bbsc repo settings pull-requests update --required-all-tasks-complete=<bool> [--repo PROJECT/slug]`
- `bbsc repo settings pull-requests update-approvers --count <N> [--repo PROJECT/slug]`

Repository selection contract:

1. `--repo PROJECT/slug`
2. `BITBUCKET_PROJECT_KEY` + `BITBUCKET_REPO_SLUG`

Machine output contract:

- `--json` always returns structured objects.
- Security users list shape: `{ "users": [...] }`
- Workflow webhooks list shape: `{ "webhooks": ... }`
- Pull-request settings shape: `{ "pull_request_settings": ... }`

## Endpoint mapping

- `repo settings security permissions users list`
  - `GetUsersWithAnyPermission2WithResponse`
  - REST: `GET /rest/api/latest/projects/{projectKey}/repos/{repositorySlug}/permissions/users`

- `repo settings security permissions users grant`
  - `SetPermissionForUserWithResponse`
  - REST: `PUT /rest/api/latest/projects/{projectKey}/repos/{repositorySlug}/permissions/users?name=...&permission=...`

- `repo settings workflow webhooks list`
  - `FindWebhooks1WithResponse`
  - REST: `GET /rest/api/latest/projects/{projectKey}/repos/{repositorySlug}/webhooks`

- `repo settings workflow webhooks create`
  - `CreateWebhook1WithResponse`
  - REST: `POST /rest/api/latest/projects/{projectKey}/repos/{repositorySlug}/webhooks`

- `repo settings workflow webhooks delete`
  - `DeleteWebhook1WithResponse`
  - REST: `DELETE /rest/api/latest/projects/{projectKey}/repos/{repositorySlug}/webhooks/{webhookId}`

- `repo settings pull-requests get`
  - `GetPullRequestSettings1WithResponse`
  - REST: `GET /rest/api/latest/projects/{projectKey}/repos/{repositorySlug}/settings/pull-requests`

- `repo settings pull-requests update`
  - `UpdatePullRequestSettings1WithBody`
  - REST: `POST /rest/api/latest/projects/{projectKey}/repos/{repositorySlug}/settings/pull-requests`

- `repo settings pull-requests update-approvers`
  - `UpdatePullRequestSettings1WithBody`
  - REST: `POST /rest/api/latest/projects/{projectKey}/repos/{repositorySlug}/settings/pull-requests`

## Live coverage

- Security group: `TestLiveRepoSettingsSecurityPermissionsUsers`
- Security write: `TestLiveRepoSettingsGrantUserPermission`
- Workflow group: `TestLiveRepoSettingsWorkflowWebhooks`
- Workflow write: `TestLiveRepoSettingsCreateWebhook`
- Workflow write 2: `TestLiveRepoSettingsDeleteWebhook`
- Pull Requests group: `TestLiveRepoSettingsPullRequestSettings`
- Pull Requests write: `TestLiveRepoSettingsUpdatePullRequestRequiredAllTasks`
- Pull Requests write 2: `TestLiveRepoSettingsUpdatePullRequestRequiredApprovers`
