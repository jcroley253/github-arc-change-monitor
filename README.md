# GitHub Actions Runner Monitoring Tool

A Golang service that proactively detects breaking or deprecating changes impacting GitHub Actions Runners (including ARC) before outages occur. This protects self-hosted GitHub Actions runner infrastructure from unexpected breaking changes and out-of-date components. By lightweight polling of GitHub APIs (with ETag caching), the monitor continuously tracks releases, Helm chart updates, and runner images across actions/runner, actions/runner-images, and actions/actions-runner-controller. An automated rules engine parses release notes for deprecation keywords (e.g., "breaking change", "EOL"), detects SemVer version drift, and enforces a strict 30-day support window for pinned versions. When actionable changes or high-severity signals are detected, deduplicated alerts with clear remediation instructions are dispatched directly to your team via Slack, Webhooks, or PagerDuty.

## Features

✅ **Release Monitoring** - Track new releases across key repositories (actions/runner, actions/runner-images, etc.)  
✅ **Breaking Change Detection** - Parse release notes for "breaking/deprecation" signals  
✅ **Changelog Monitoring** - Monitor GitHub's official changelog for runner deprecation announcements  
✅ **Health Checks** - Validate if your pinned runner versions are deprecated  
✅ **Support Window Enforcement** - Alert when versions exceed 30-day support window  
✅ **Smart Notifications** - Send actionable alerts to Google Chat and Email with deduplication

### Quick Start

```bash
# Set your GitHub token for higher rate limits (optional but recommended)
export GITHUB_TOKEN="your_token_here"

# Run the monitor
go run . -config config.yaml

# Test a specific runner version
go run . -config config.yaml -test-version v2.328.0

# Override versions from command line (useful for CI/CD pipelines)
go run . -config config.yaml \
  -runner-binary-version v2.330.0 \
  -runner-helm-version actions-runner-controller-0.23.7 \
  -runner-image-version ubuntu22/20241015
```

### Pipeline Integration

The recommended approach for CI/CD is to build the binary first and then run it. Below is a complete GitHub Actions workflow example:

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

You can also run ad-hoc with `go run` during local development:

```yaml
- name: Monitor Runner Versions
  run: |
    cd app
    go run . -config config.yaml \
      -runner-binary-version ${{ vars.ACTION_RUNNER_IMAGE_RELEASE_VERSION }} \
      -runner-helm-version ${{ vars.HELM_CHART_VERSION }} \
      -runner-image-version ${{ vars.RUNNER_IMAGE_VERSION }}
```

**Available Flags:**

- `-runner-binary-version` - Override the binary runner version (e.g., `v2.330.0`)
- `-runner-helm-version` - Override the helm chart version (e.g., `actions-runner-controller-0.23.7`)
- `-runner-image-version` - Override the runner image version (e.g., `ubuntu22/20241015`)

These flags override the `pinned_version` values in your config file, allowing you to check your currently deployed versions against the latest releases.

### State Storage

The app tracks seen releases and cached ETags in a state file to prevent duplicate alerts across runs. Two modes are supported:

#### Azure Blob Storage (recommended for CI/CD)

By default the app reads and writes state to an Azure Blob Storage container. This is the correct mode for scheduled workflows where state must persist between runs.

To configure it, update the following three values at the top of `app/main.go`:

```go
storageAccountURL := "https://<your-storage-account>.blob.core.windows.net/"
// ...
containerName := "<your-container-name>"
blobName := "state.json"
```

Authentication uses `DefaultAzureCredential`, which reads these environment variables (or GitHub Actions secrets):

| Variable | GitHub Actions secret |
|---|---|
| `AZURE_TENANT_ID` | `ARM_TENANT_ID` |
| `AZURE_CLIENT_ID` | `ARM_CLIENT_ID` |
| `AZURE_CLIENT_SECRET` | `ARM_CLIENT_SECRET` |

If the blob does not exist yet (first run), the app initializes a fresh empty state automatically — no manual setup required.

#### Local State (development / testing)

When running locally without Azure credentials, the app can use the `app/state.json` file in the repository as its state store. A clean template is already committed:

```json
{
  "etags": {},
  "seen": {},
  "seen_changelog_ids": {}
}
```

On the first local run all releases will be treated as new (no prior state), which is the expected behaviour for a fresh deployment. Subsequent local runs will start clean again unless the code is updated to read/write the local file directly instead of Azure Blob.
