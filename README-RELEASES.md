# Release Management Guide

This repository uses [Release Please](https://github.com/googleapis/release-please) to automate the creation of releases based on [Conventional Commit](https://www.conventionalcommits.org/) messages.

## How to Generate a Release (End-to-End)

### Step 1: Write Conventional Commits

When making changes to the repository, ensure your commit messages follow the Conventional Commits specification:

```bash
git commit -m "feat: add new runner configuration for OpenShift"
git commit -m "fix: resolve authentication issue in AKS deployment"
git commit -m 'feat!: change runner image format (breaking change)'
```

### Step 2: Create and Merge Your Pull Request

After committing your changes, open a pull request targeting the `main` branch. Once your PR is reviewed and approved, merge it into `main`:

1. Push your branch to GitHub:
   ```bash
   git push origin <your-branch-name>
   ```
2. Open a pull request from your branch to `main`.
3. Request reviews and address any feedback.
4. Once approved, merge the PR into `main` (following your team's branch protection rules).

> **Note:** Direct pushes to `main` are not allowed due to branch protection policies. All changes must go through pull requests.

### Step 3: Release Please Workflow Triggers

Release Please runs automatically when PRs are merged to main, and can also be triggered manually:

**Automatic (Default):**

- Automatically runs when changes are pushed to the `main` branch that affect specific paths:
  - `.github/workflows/runner-change-monitor.yml`
  - `.release-please-manifest.json`
  - `app/**`
  - `CHANGELOG.md`
- Analyzes new commits since the last release
- Creates or updates Release PR if there are releasable commits (`feat:`, `fix:`, `chore:`, etc.)
- No action required from you

**Manual (Optional):**

1. Navigate to the **Actions** tab in GitHub
2. Select the **🎉 Release Generator** workflow
3. Click **Run workflow**
4. Choose the `main` branch
5. Click **Run workflow** button

> **Tip:** Manual triggering is useful for testing or forcing a re-run if something failed.

### Step 4: Review and Merge Release PR

Release Please will:

- Analyze commit history since the last release
- Create a Release PR with updated CHANGELOG.md and version bumps
- The PR title will be something like "chore: release 0.2.0"

**Review the Release PR and merge it when ready.**

### Step 5: Automatic GitHub Release

Once the Release PR is merged, Release Please automatically:

- Tags the commit with the version number (e.g., `v0.2.0`)
- Creates a GitHub Release with release notes
- Updates the repository with the new version

## Conventional Commit Messages & Semantic Versioning

| Commit Prefix | Description | Semantic Version Impact | Changelog Section | Example |
|---------------|-------------|------------------------|-------------------|---------|
| `feat:` | New features | **MINOR** (0.1.0 → 0.2.0) | Features | `feat: add GKE runner configuration` |
| `fix:` | Bug fixes | **PATCH** (0.1.0 → 0.1.1) | Bug Fixes | `fix: resolve AKS authentication timeout` |
| `chore:` | Maintenance tasks | **PATCH** (0.1.0 → 0.1.1) | Miscellaneous Chores | `chore: update runner dependencies` |
| `deps:` | Dependency updates | **PATCH** (0.1.0 → 0.1.1) | Dependencies | `deps: update actions/checkout to v4` |
| `docs:` | Documentation changes | **PATCH** (0.1.0 → 0.1.1) | Documentation | `docs: update runner deployment guide` |
| `refactor:` | Code refactoring | **PATCH** (0.1.0 → 0.1.1) | Code Refactoring | `refactor: simplify script configuration logic` |
| `perf:` | Performance improvements | **PATCH** (0.1.0 → 0.1.1) | Performance Improvements | `perf: optimize image build process` |
| `build:` | Build system changes | **PATCH** (0.1.0 → 0.1.1) | Build System | `build: update container build configuration` |
| `ci:` | CI/CD changes | **PATCH** (0.1.0 → 0.1.1) | Hidden in changelog | `ci: add automated deployment testing` |
| `style:` | Code style changes | **PATCH** (0.1.0 → 0.1.1) | Hidden in changelog | `style: fix YAML formatting` |
| `test:` | Test additions/changes | **PATCH** (0.1.0 → 0.1.1) | Hidden in changelog | `test: add unit tests for configuration` |
| `revert:` | Revert previous changes | **PATCH** (0.1.0 → 0.1.1) | Reverts | `revert: revert feature X` |
| `feat!:` | Breaking changes | **MAJOR** (0.1.0 → 1.0.0) | Features | `feat!: redesign runner deployment API` |
| `fix!:` | Breaking bug fixes | **MAJOR** (0.1.0 → 1.0.0) | Bug Fixes | `fix!: remove deprecated runner labels` |
| `BREAKING CHANGE:` | Breaking changes (in footer) | **MAJOR** (0.1.0 → 1.0.0) | Respective section | See example below |

*Note: All commit types listed above are releasable and will trigger version bumps. Types marked as "Hidden in changelog" (`ci:`, `style:`, `test:`) will still create releases but won't appear in the changelog.*

### Breaking Change Examples

Using the `!` suffix:
```bash
git commit -m 'feat!: change runner authentication to federated credentials'
```

Using the `BREAKING CHANGE:` footer:
```bash
git commit -m "feat: update runner configuration format

BREAKING CHANGE: The runner configuration format has changed from YAML to JSON. 
Existing configurations must be migrated to the new format."
```

## Advanced Release Management

### Force a Specific Version

Sometimes you may need to override the automatic semantic versioning and force a specific version number. Use the `Release-As` trailer in your commit message:

```bash
# Force a specific version (e.g., for alignment or policy reasons)
git commit --allow-empty -m "chore: release 1.0.0" -m "Release-As: 1.0.0"
```

**Common use cases:**

- Aligning version numbers across related repositories
- Skipping version numbers for organizational reasons
- Setting an initial major version (e.g., moving from 0.x to 1.0.0)
- Correcting version sequence after manual intervention

**Example scenarios:**
```bash
# Move to 1.0.0 for production readiness
git commit --allow-empty -m "chore: promote to v1.0.0" -m "Release-As: 1.0.0"

# Align with organizational versioning
git commit --allow-empty -m "chore: align with platform version" -m "Release-As: 2.0.0"

# Jump to specific version
git commit --allow-empty -m "chore: version alignment" -m "Release-As: 1.5.0"
```

> **Note:** The `--allow-empty` flag creates a commit without file changes, useful when you only need to trigger a release with a specific version.

### Skip Release for a Commit

To prevent a commit from triggering a release or being included in the changelog, include `[skip ci]` or `[ci skip]` in the commit message:

```bash
git commit -m "docs: minor typo fix [skip ci]"
git commit -m "[ci skip] chore: update local development notes"
```

## Key Considerations

### 🔐 Pull Request Validation

**All PRs are automatically validated for conventional commit compliance:**

- **PR Title Validated**: The PR title must follow conventional commit format
- **Automated Feedback**: Validation results are shown in PR checks
- **Required Check**: PRs cannot be merged until validation passes

**Validation workflow:**

1. PR is opened or updated
2. PR Title Validation workflow runs automatically
3. Title is checked against allowed types and format
4. Fix the PR title if validation fails before merging

**Validation rules:**

- **Allowed types**: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`
- **Scopes**: Optional (not required)
- **Subject case**: Must start with a lowercase letter
- **Format**: `type(optional-scope): description starting with lowercase`

**Valid PR title examples:**

- ✅ `feat: add new runner for OpenShift`
- ✅ `fix(aks): resolve authentication issue`
- ✅ `chore: update dependencies`
- ✅ `docs(readme): improve installation guide`

**Invalid PR title examples:**

- ❌ `Feature: Add new runner` (wrong format)
- ❌ `fix: Fix authentication issue` (subject starts with uppercase)
- ❌ `Update dependencies` (missing type prefix)

> **Note:** Individual commit messages within the PR are not validated - only the PR title matters. However, following conventional commits for all commits is still recommended for better history tracking.

### 🔄 No Duplicate PRs

**Release Please is intelligent about PR management:**

- **Only creates Release PRs when there are releasable commits** (feat, fix, chore, etc.)
- **Updates existing Release PRs** instead of creating new ones
- **All commit types in the configuration trigger releases and cause version bumps** (except hidden types still bump version)
- **One Release PR at a time** - subsequent releasable commits update the existing PR

### 📊 When Release PRs Are Created

Release Please only tracks changes in specific paths:

- ✅ `.github/workflows/runner-change-monitor.yml`
- ✅ `.release-please-manifest.json`
- ✅ `app/**`
- ✅ `CHANGELOG.md`

**Examples:**

- ✅ **WILL trigger PR**: `feat: add new runner configuration` + changes to `resources/aks/default/values.yaml`
- ✅ **WILL trigger PR**: `fix: resolve deployment issue` + changes to `scripts/configure_runner_scale_set.sh`
- ✅ **WILL update existing PR**: New releasable commit when Release PR already exists
- ❌ **WILL NOT trigger PR**: Changes only to files outside the tracked paths (e.g., `.github/workflows/unrelated.yml`)
- ❌ **WILL NOT trigger PR**: Commits with `[skip ci]` in the message

### 🏷️ PR Lifecycle Labels

Release Please uses labels to track PR status:

- `autorelease: pending` - Release PR is open and waiting to be merged
- `autorelease: tagged` - Release PR has been merged and release is tagged

### 🚀 Repository Setup Requirements

- **Permissions**: Repository allows GitHub Actions to create PRs
  - Settings → Actions → General
  - "Allow GitHub Actions to create and approve pull requests" is enabled
- **Branch Protection**: Release PRs can be merged to main branch
- **Conventional Commits**: Team must follow conventional commit format for PR titles
- **Runner Access**: Workflow runs on `cw-az-westus2-pd` (production runner)
- **Workflow Files**: 
  - `.github/workflows/release-generator.yml` - Release Please workflow
  - `.github/workflows/pr-title-validation.yml` - PR title validation
  - `release-please-config.json` - Release configuration
  - `.release-please-manifest.json` - Current version tracking

### ⚙️ Configuration Details

**Current Release Configuration:**

- **Release Type**: `simple` (for non-language-specific projects)
- **Version Format**: `v0.2.0` (includes `v` prefix)
- **Current Version**: `0.1.0` (tracked in `.release-please-manifest.json`)
- **Changelog**: Auto-generated in `CHANGELOG.md`
- **Pre-1.0 Behavior**: 
  - Minor version bumps are enabled (`bump-minor-pre-major: true`)
  - Patch bumps for pre-1.0 minor changes are disabled

**Workflow Behavior:**

- **Automatic Trigger**: Runs on push to `main` branch when tracked files change
- **Manual Trigger**: Available via workflow_dispatch
- **Permissions**: Can write to contents, pull requests, and issues
- **Runner**: Uses `cw-az-westus2-pd` (GitHub self-hosted runner)
- **Branch Protection**: Direct pushes to `main` are blocked; all changes via PR

**Path Triggers:**
The workflow only runs when these files/directories are modified:

- `.github/workflows/runner-change-monitor.yml`
- `.release-please-manifest.json`
- `app/**`
- `CHANGELOG.md`

## Troubleshooting

### Release PR Not Created?

1. **Check tracked paths**: Ensure your changes affect one of the tracked paths listed above
2. **Check for releasable commits**: Verify commits use proper conventional commit format
3. **Look for existing Release PR**: Search for PRs with `autorelease: pending` label - new releasable commits will update the existing PR
4. **Check workflow logs**: Review the GitHub Actions run for any errors
5. **Verify workflow ran**: Confirm the Release Generator workflow executed after your push to `main`

### PR Title Validation Failing?

1. **Check PR title format**: Must use format `type: description` or `type(scope): description`
2. **Verify commit type**: Ensure type is one of: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`
3. **Subject case**: Description must start with lowercase letter
4. **Review check details**: Click on the failed check for specific error messages

### Force Release Please to Run

If you need to force Release Please to reprocess commits:

**Method 1: Manual Trigger (Recommended)**

1. Go to Actions tab → 🎉 Release Generator workflow
2. Click "Run workflow"
3. Select `main` branch
4. Click "Run workflow" button

**Method 2: Empty Commit**

```bash
git checkout -b trigger-release
git commit --allow-empty -m "chore: trigger release please"
git push origin trigger-release
# Then create and merge PR to main
```

**Method 3: Update Tracked File**

```bash
git checkout -b trigger-release
# Make a minor change to CHANGELOG.md or another tracked file
git add .
git commit -m "chore: trigger release workflow"
git push origin trigger-release
# Then create and merge PR to main
```

### Version Not Incrementing as Expected?

1. **Check commit types**: Ensure commits use types that trigger the desired version bump
2. **Review configuration**: Check `release-please-config.json` for version bump rules
3. **Force specific version**: Use `Release-As` trailer if needed
4. **Check manifest**: Verify `.release-please-manifest.json` has the expected current version
5. **Verify tracked paths**: Ensure changed files are in the tracked paths

### Workflow Not Running?

1. **Check changed paths**: Verify your PR modified files in one of the tracked paths
2. **Review workflow permissions**: Ensure GITHUB_TOKEN has required permissions
3. **Check runner availability**: Verify `cw-az-westus2-pd` runner is available
4. **Review merge status**: Confirm PR was successfully merged to `main`

## Additional Reading

### Official Documentation

- [Release Please Action](https://github.com/googleapis/release-please-action) - GitHub Action documentation
- [Release Please Core](https://github.com/googleapis/release-please) - Core library and concepts
- [Conventional Commits](https://www.conventionalcommits.org/) - Commit message specification
- [Semantic Versioning](https://semver.org/) - Version numbering scheme

### Advanced Configuration

- [Manifest Configuration](https://github.com/googleapis/release-please/blob/main/docs/manifest-releaser.md) - For monorepos and complex setups
- [Customizing Release Please](https://github.com/googleapis/release-please/blob/main/docs/customizing.md) - Advanced configuration options
- [Troubleshooting Guide](https://github.com/googleapis/release-please/blob/main/docs/troubleshooting.md) - Common issues and solutions

### Best Practices

- [GitHub Flow](https://guides.github.com/introduction/flow/) - Branching strategy that works well with Release Please
- [Writing Good Commit Messages](https://chris.beams.io/posts/git-commit/) - General commit message best practices

## Quick Reference

### Common Commands

```bash
# Regular feature commit
git commit -m "feat: add autoscaling configuration for AKS runners"

# Bug fix with scope
git commit -m "fix(openshift): correct RBAC permissions"

# Breaking change
git commit -m "feat!: change runner image registry" -m "BREAKING CHANGE: All runner images now use Quay.io"

# Force specific version
git commit --allow-empty -m "chore: release 1.0.0" -m "Release-As: 1.0.0"

# Skip CI for minor changes
git commit -m "docs: fix typo [skip ci]"

# Trigger release manually (create PR with empty commit)
git checkout -b trigger-release
git commit --allow-empty -m "chore: trigger release"
git push origin trigger-release
# Then merge PR to main
```

### Version Bumping Rules

| Commit Type | Version Impact | Example |
|-------------|----------------|---------|
| `feat:` | MINOR (0.1.0 → 0.2.0) | `feat: add GKE runner support` |
| `fix:`, `chore:`, `deps:`, `docs:`, `refactor:`, `perf:`, `build:`, `ci:`, `style:`, `test:`, `revert:` | PATCH (0.1.0 → 0.1.1) | `fix: resolve authentication issue` |
| `feat!:`, `fix!:`, or `BREAKING CHANGE:` footer | MAJOR (0.1.0 → 1.0.0) | `feat!: redesign configuration format` |

### PR Title Format

**Valid Formats:**
```
feat: add new feature
fix(scope): fix something
chore!: breaking change
```

**Rules:**

- Type: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`
- Scope: Optional, in parentheses
- Subject: Must start with lowercase letter
- Breaking change: Add `!` after type/scope

### Tracked File Paths

Changes to these paths trigger the Release Generator workflow:

- `images/scripts/ubi_tools.sh`
- `images/universal_base_image`
- `resources/**`
- `scripts/configure_runner_scale_set.sh`
- `CHANGELOG.md`
- `.release-please-manifest.json`

### Workflow Status Checks

- ✅ **Validate PR Title** - Must pass before PR merge (validates PR title only)
- ✅ **Release Generator** - Runs automatically on push to main (if tracked paths changed)
- 📝 **Release PR** - Created/updated automatically when needed
- 🏷️ **GitHub Release** - Created automatically when Release PR is merged
