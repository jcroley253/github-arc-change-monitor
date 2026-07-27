---
applyTo: "**"
---

# Copilot Instructions — GitHub ARC Change Monitor

This is a Go CLI application that polls the GitHub REST API to detect breaking or deprecating changes in GitHub Actions Runner (ARC) related repositories. It compares pinned versions against the latest releases, scans release notes for deprecation signals, enforces a configurable support window, and sends deduplicated alerts to Google Chat and/or email.

## Project Structure

```
app/
  main.go          # Entry point: blob client, flag parsing, run orchestration
  config.go        # Config structs and loadConfig()
  types.go         # Alert struct (shared across packages)
  state.go         # State struct, loadStateFromBlob(), saveStateToBlob()
  github.go        # GitHub REST API calls (fetchReleases, etc.)
  analyzer.go      # Release analysis: keyword scan, SemVer diff, age checks
  notifier.go      # Google Chat and email notification dispatch
  health.go        # Runner version health checks
  changelog.go     # GitHub changelog RSS feed monitoring
  config.yaml      # Runtime configuration (repos, notifications, thresholds)
  state.json       # Local state template (empty — used as baseline for new deployments)
  go.mod           # Module: runnerwatch, Go 1.24.1
.github/
  workflows/       # GitHub Actions workflow(s) for scheduled execution
  instructions/    # Copilot instruction files
```

## Key Types

### Alert (types.go)
```go
type Alert struct {
    Repo        string
    Type        string    // "binary" | "helm" | "image"
    TagName     string
    Severity    string    // "critical" | "warning" | "info"
    Reason      string
    PublishedAt time.Time
    URL         string
    Remediation string
}
```

### State (state.go)
```go
type State struct {
    ETags            map[string]string              `json:"etags"`
    Seen             map[string]map[string]struct{} `json:"seen"`
    SeenChangelogIDs map[string]struct{}            `json:"seen_changelog_ids"`
}
```

### Config (config.go)
```go
type RepoConfig struct {
    Owner            string
    Repo             string
    Type             string   // "binary" | "helm" | "image"
    PinnedVersion    string
    CriticalKeywords []string
}
```

## Configuration (config.yaml)

The app is driven entirely by `config.yaml`. Key fields:

| Field | Description |
|---|---|
| `poll_interval` | Polling cadence (e.g. `"5m"`, `"1h"`) |
| `support_window_days` | Days before a pinned version is considered stale (default: 30) |
| `enable_changelog` | Monitor GitHub's changelog RSS for deprecation announcements |
| `enable_health_checks` | Validate whether pinned runner versions are still connectable |
| `changelog_keywords` | Strings to match in RSS feed entries |
| `repos` | List of repositories to monitor (see below) |
| `notifications` | Google Chat and email settings |

### Adding a Repository to Monitor

Add an entry under `repos` in `config.yaml`:

```yaml
repos:
  - owner: actions
    repo: runner
    type: binary           # binary | helm | image
    pinned_version: "v2.331.0"
    critical_keywords:
      - "breaking change"
      - "deprecat"
      - "end of support"
      - "removed"
```

`pinned_version` is only used when running locally without CLI flags. In CI/CD, the version is always passed via `-runner-binary-version`, `-runner-helm-version`, or `-runner-image-version`.

### Notification Setup

**Google Chat:**
```yaml
notifications:
  google_chat:
    webhook_url: "https://chat.googleapis.com/v1/spaces/<SPACE>/messages?key=<KEY>&token=<TOKEN>"
    space: "GitHub Action Runner Alerts"
```

**Email (SMTP):**
```yaml
notifications:
  email:
    smtp_host: "smtp.example.com"
    from: "Runner Alerts <no-reply@example.com>"
    to:
      - "team@example.com"
```

## CLI Flags

| Flag | Default | Description |
|---|---|---|
| `-config` | `config.yaml` | Path to config file |
| `-test-version` | `""` | Test a specific runner version and exit (e.g. `v2.328.0`) |
| `-runner-binary-version` | `""` | Override `pinned_version` for `type: binary` repos |
| `-runner-helm-version` | `""` | Override `pinned_version` for `type: helm` repos |
| `-runner-image-version` | `""` | Override `pinned_version` for `type: image` repos |

CLI flags take precedence over `pinned_version` values in `config.yaml`.

## State Storage

### Azure Blob Storage (CI/CD — default)

State persists between workflow runs via Azure Blob. The connection is configured in `app/main.go`:

```go
storageAccountURL := "https://<your-storage-account>.blob.core.windows.net/"
containerName     := "<your-container-name>"
blobName          := "state.json"
```

Authentication uses `DefaultAzureCredential`, reading these environment variables:

| Env var | GitHub Actions secret |
|---|---|
| `AZURE_TENANT_ID` | `ARM_TENANT_ID` |
| `AZURE_CLIENT_ID` | `ARM_CLIENT_ID` |
| `AZURE_CLIENT_SECRET` | `ARM_CLIENT_SECRET` |

If the blob does not exist, the app initializes a fresh empty state automatically.

### Local State (development)

`app/state.json` is a clean template committed to the repo:
```json
{ "etags": {}, "seen": {}, "seen_changelog_ids": {} }
```
On a local run without Azure credentials the app will fail to connect; to run locally, point `storageAccountURL` at a real or emulated account, or adapt the code to use `loadStateFromFile` / `saveStateToFile`.

## GitHub Actions Workflow Integration

The recommended pattern is: checkout → setup-go → build → run.

```yaml
jobs:
  monitor:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Repo
        uses: actions/checkout@v4

      - name: Set Up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24.1'

      - name: Build Monitor
        working-directory: app
        run: go build -o github_runner_change_monitor .

      - name: Run Monitor
        working-directory: app
        env:
          AZURE_CLIENT_ID: ${{ secrets.ARM_CLIENT_ID }}
          AZURE_CLIENT_SECRET: ${{ secrets.ARM_CLIENT_SECRET }}
          AZURE_TENANT_ID: ${{ secrets.ARM_TENANT_ID }}
        run: |
          ./github_runner_change_monitor \
            -runner-binary-version "${{ vars.ACTION_RUNNER_IMAGE_RELEASE_VERSION }}" \
            -runner-helm-version "actions-runner-controller-${{ vars.HELM_CHART_VERSION }}"
```

The binary writes `binary_version_changed=true` and `new_binary_version=<tag>` to `GITHUB_OUTPUT` when a new binary release is detected, which downstream jobs can consume.

## Coding Conventions

- All files are in `package main` — this is a single-binary CLI app, not a library.
- Use `log.Fatalf` for unrecoverable startup errors (credential failure, config parse failure).
- Use `fmt.Fprintf(os.Stderr, ...)` for non-fatal per-repo errors inside the monitoring loop so the run continues for remaining repos.
- State is only saved to blob when `hasChanges == true` to avoid unnecessary writes.
- Alert deduplication: call `stableID(repoKey, tag, reason)` (SHA-1 hash) and check `state.Seen` before emitting an alert.
- ETags are stored per `owner/repo` key (e.g. `"actions/runner"`) in `state.ETags` and passed as `If-None-Match` headers to reduce GitHub API calls.
- `GITHUB_TOKEN` is optional but strongly recommended — without it the unauthenticated rate limit is 60 requests/hour.

## Severity Classification

| Severity | When |
|---|---|
| `critical` | Release body contains keywords from `critical_keywords` in config, OR major SemVer bump |
| `warning` | Minor version is 5+ behind, OR 2+ minor versions behind AND >60 days old |
| `info` | New release detected, within support window, no breaking signals |
