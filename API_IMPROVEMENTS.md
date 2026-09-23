# Aikido API Improvements for Terraform Provider

This document outlines API improvements that would resolve issues encountered when managing Aikido resources via Terraform.

## Missing Response Fields

### Container APIs (`GET /containers` and `GET /containers/{id}`)

Both the list and detail endpoints are missing fields that are configurable via separate write endpoints. This means Terraform cannot read back the full state after import, causing a one-time diff on the first apply.

| Missing Field | Type | Description | Write Endpoint |
|---|---|---|---|
| `active` | boolean | Whether scanning is enabled | `POST /containers/activate`, `POST /containers/deactivate` |
| `sensitivity` | string | Sensitivity level (`extreme`, `sensitive`, `normal`, `not_sensitive`, `no_data`) | `PUT /containers/{id}/sensitivity` |
| `internet_exposed` | string | Connectivity status (`connected`, `not_connected`, `unknown`) | `PUT /containers/{id}/internetConnection` |
| `tag_filter` | string | Tag filter pattern for scanning | `POST /containers/updateTagFilter` |

These fields need adding to both the list and detail responses or to a separate get API so that:
1. The `aikido_container_config` resource can read back full state after import (eliminating the one-time diff).
2. The `data_aikido_containers` data source can expose them for filtering or reference.

### Code Repo API (`GET /repositories/code/{id}`)

Missing one configurable field from the response:

| Missing Field | Type | Description | Write Endpoint |
|---|---|---|---|
| `dev_dep_scanning_enabled` | boolean | Whether dev dependency scanning is enabled | `PUT /repositories/code/{id}/devDepScanning` |

### CI Checks API (`GET /repositories/code/continuous_integration/checks`)

The API accepts this field on write but does not return it on read:

| Missing Field | Type | Description | Write Endpoint |
|---|---|---|---|
| `post_deep_audit_inline_comments_min_severity` | string | Minimum severity of new Deep Review issues for which inline comments are posted | `POST /repositories/code/continuous_integration/checks` |

The API documentation lists this field on the GET response, but a live call omits it. This means the `aikido_code_repo_ci_checks` resource can set the value but never read it back, so Terraform cannot detect drift in it and cannot populate it on import.

## Missing Delete Endpoints

### CI Checks (`/repositories/code/continuous_integration/checks`)

There is no `DELETE` for a repository's CI checks configuration, and no documented "off" state for one. Setting `minimum_severity` to `always_pass_check` with every `fail_on_*` false approximates disabling the checks, but it discards the rest of the configuration and cannot be reversed.

As a result, destroying an `aikido_code_repo_ci_checks` resource can only stop Terraform managing the configuration; the settings stay live in Aikido. A `DELETE` endpoint, or a documented reset, would let the resource clean up after itself.

## Specification Accuracy

### Paginated list responses are documented as single objects

Two list endpoints declare their `200` response schema as a single object, but return a JSON array:

- `GET /repositories/code`
- `GET /repositories/code/continuous_integration/checks`

Both accept `page` and `per_page`, so the array is clearly intended. Since this affects more than one endpoint it looks systematic rather than a one-off, and it means the specification cannot be used to generate a working client for these paths without manual correction.

## Type Inconsistency

### Container `linked_code_repo_id`

The `linked_code_repo_id` field in the container list/detail response is returned as a **number** by the API, but the API documentation types it as a **string**. The API should either:

- Document it as an integer (matching the actual response), or
- Return it as a string for consistency with how other ID fields are typed.

The provider currently expects a number, matching what the API actually returns.

## Missing Data Source Filters

Several data source list endpoints lack filters that would reduce API calls and enable more precise lookups.

### Users (`GET /users`)

**Current filters:** `include_inactive`, `team_id`

| Suggested Filter | Type | Rationale |
|---|---|---|
| `filter_email` | string | Currently the only way to find a user by email is to fetch the full list and filter client-side. The provider uses a Terraform `for` expression as a workaround. An email filter would allow direct single-user lookup. |
| `filter_name` | string | For consistency with other endpoints. |

This is the highest-impact improvement — it would eliminate the need for workaround expressions in team membership configuration.

### Domains (`GET /domains`)

**Current filters:** none

| Suggested Filter | Type | Rationale |
|---|---|---|
| `filter_domain` | string | Filter by domain name. |
| `filter_kind` | string | Filter by type (`front_end`, `rest_api`, `graphql_api`). |

### Clouds (`GET /clouds`)

**Current filters:** none

| Suggested Filter | Type | Rationale |
|---|---|---|
| `filter_provider` | string | Filter by cloud type (`aws`, `azure`, `gcp`, `kubernetes`). |
| `filter_name` | string | Filter by name. |

### Zen Apps (`GET /zen/apps`)

**Current filters:** none

| Suggested Filter | Type | Rationale |
|---|---|---|
| `filter_name` | string | Filter by app name. |
| `filter_environment` | string | Filter by environment (`production`, `staging`, `development`). |

### Code Repos (`GET /repositories/code`)

**Current filters:** `filter_name`, `filter_branch`, `include_inactive`

| Suggested Filter | Type | Rationale |
|---|---|---|
| `filter_team_id` | string | For consistency with the containers endpoint. |

### Autofix

There is currently no public API for configuring this.

## Error Response Formatting

API error responses contain unicode-escaped quotes in the JSON body:

```json
{"error":"You are missing the required scope for this request: \u0027clouds:read\u0027"}
```

Returning plain quotes would produce cleaner error messages:

```json
{"error":"You are missing the required scope for this request: 'clouds:read'"}
```
