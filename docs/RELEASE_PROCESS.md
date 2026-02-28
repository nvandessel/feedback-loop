# Release Process

This document describes the automated release process for floop using GoReleaser.

## Overview

The release pipeline is fully automated with semantic version bumping:
1. Code merges to `main` trigger automatic version detection from commit messages
2. Commits are parsed for conventional commit prefixes: `feat:` → minor, `fix:`/`perf:` → patch, breaking changes → major
3. Chore-only merges (docs, ci, test, refactor, etc.) skip the release entirely
4. GoReleaser builds binaries, publishes to GitHub Releases, and updates the Homebrew tap

For manual overrides, maintainers can trigger the version-bump workflow directly.

## Installation

### Homebrew (macOS/Linux)

```bash
brew install nvandessel/tap/floop
```

### Go

```bash
go install github.com/nvandessel/floop/cmd/floop@latest
```

### Manual

Download binaries from [GitHub Releases](https://github.com/nvandessel/floop/releases), extract, and add to your PATH.

## Auto-Release

Every push to `main` that changes code files triggers `auto-release.yml`. Documentation-only changes (`.md`, `docs/`, `.beads/`, `.floop/`, `LICENSE`) and workflow changes (`.github/`) are ignored via `paths-ignore`.

Before releasing, auto-release waits for the CI workflow to complete on the same commit. If CI fails, the release is skipped. If no CI run is found (e.g., the commit only touched paths ignored by CI), the release proceeds.

### Semantic Version Bumping

Since the repo uses squash-merge, PR titles become commit messages. The `pr-title.yml` workflow enforces conventional commit format on PR titles. Auto-release parses commits since the last tag to determine the bump type:

| Commit Type | Bump | Example |
|-------------|------|---------|
| `feat!:` or `BREAKING CHANGE:` in body | **major** | `feat!: remove legacy API` |
| `feat:` | **minor** | `feat: add --tags flag` |
| `fix:`, `perf:` | **patch** | `fix: handle empty input` |
| `chore:`, `ci:`, `test:`, `docs:`, `build:`, `style:`, `refactor:` | **skip** | `chore: update deps` |

### Skip Mechanisms

| Method | Use Case |
|--------|----------|
| `github-actions[bot]` actor | Automatic — prevents version-bump commits from re-triggering |
| `[skip release]` in commit message | Batch multiple PRs before releasing |
| `skip-release` label on PR | Mark a PR as not release-worthy before merging |

### Example: Batching Changes

When you want to merge several PRs before cutting a release:

1. Add the `skip-release` label to each PR (or include `[skip release]` in merge commits)
2. Merge the PRs
3. On the last PR, remove the label (or omit the skip marker) — the auto-release triggers

## Manual Release Override

Minor and major releases are now handled automatically by commit message parsing. Manual triggers are still available as an override. Release notes for minor and major releases automatically span back to the last tag of the same level — a minor release includes all commits since the previous `vX.Y.0`, and a major release since the previous `vX.0.0`. This means auto-patches between minor/major releases don't cause empty changelogs.

### 1. Prepare for Release

Ensure the `main` branch is ready:

```bash
# Pull latest changes
git checkout main
git pull origin main

# Verify tests pass
make test

# Verify CI suite passes
make ci

# Check for any uncommitted changes
git status
```

### 2. Choose Version Bump Type

| Bump Type | When to Use | Example |
|-----------|-------------|---------|
| **patch** | Bug fixes, minor improvements (usually auto-released) | 0.1.0 → 0.1.1 |
| **minor** | New features, backwards-compatible API additions | 0.1.0 → 0.2.0 |
| **major** | Breaking changes, major architectural changes | 0.1.0 → 1.0.0 |

**Current versioning stage**: Pre-1.0 (0.x.x)
- Use `minor` for new features or significant changes
- Use `patch` for bug fixes
- Reserve `major` for when ready to commit to API stability (1.0.0)

### 3. Trigger Version Bump

```bash
# For a minor release (0.1.0 → 0.2.0)
gh workflow run version-bump.yml -f bump=minor

# For a major release (0.1.0 → 1.0.0)
gh workflow run version-bump.yml -f bump=major
```

Alternatively, use the GitHub web UI:
1. Go to **Actions** tab
2. Select **Version Bump and Release** workflow
3. Click **Run workflow**
4. Choose bump type from dropdown
5. Click **Run workflow**

### 4. Monitor Release

```bash
# Watch the workflow
gh run watch --workflow=version-bump.yml

# Or list recent runs
gh run list --workflow=version-bump.yml
```

### 5. Verify Release

```bash
# View the release
gh release view v0.2.0

# Download and test a binary
gh release download v0.2.0 -p "floop-v0.2.0-linux-amd64.tar.gz"
tar xzf floop-v0.2.0-linux-amd64.tar.gz
./floop --version
```

Expected output:
```
floop version v0.2.0 (commit: abc1234, built: 2026-02-10T15:30:00Z)
```

### 6. Announce Release

After verification:
- Update any deployment documentation
- Notify users via appropriate channels
- Update integration guides if needed

## Release Artifacts

Each release produces:

| Artifact | Description |
|----------|-------------|
| **6 binaries** | linux/darwin/windows × amd64/arm64 |
| **Archives** | `.tar.gz` for Unix, `.zip` for Windows |
| **Checksums** | `checksums.txt` with SHA256 hashes |
| **Release notes** | Auto-generated by GoReleaser from conventional commits |

Archives include:
- `floop` binary
- `LICENSE`
- `README.md`
- `docs/` directory

## Local Testing

Test the release process locally before pushing:

```bash
# Install GoReleaser (if not already installed)
go install github.com/goreleaser/goreleaser/v2@v2.14.1

# Validate configuration
goreleaser check

# Test build without publishing
goreleaser build --snapshot --clean

# Check built binaries
ls -lh dist/*/

# Test version injection
./dist/floop_linux_amd64_v1/floop --version

# Test full release pipeline (doesn't publish)
goreleaser release --snapshot --clean

# Clean up
rm -rf dist/
```

## Version Information

All binaries include build metadata:

```bash
./floop --version
# Output: floop version v0.2.0 (commit: abc1234, built: 2026-02-10T15:30:00Z)

./floop version --json
# Output: {"version":"v0.2.0","commit":"abc1234","date":"2026-02-10T15:30:00Z"}
```

**Version sources:**
- `version` — From git tag (e.g., `v0.2.0`)
- `commit` — Short git commit SHA
- `date` — Build timestamp (RFC3339)

**Development builds:**
- Built with `make build` show `version=dev`
- Include current commit SHA and build time

## Troubleshooting

### Release Workflow Fails

**Check GoReleaser logs:**
```bash
gh run view --log
```

**Common issues:**
- **Missing tag:** Ensure version bump workflow completed
- **Build failure:** Check `go.mod` and dependencies are up to date
- **Invalid config:** Run `goreleaser check` locally

### Wrong Version Tagged

If you need to delete and recreate a tag:

```bash
# Delete local tag
git tag -d v0.2.0

# Delete remote tag (careful!)
git push origin :refs/tags/v0.2.0

# Create new tag
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0
```

**Note:** Only do this immediately after tagging, before anyone downloads the release.

### Release Published but Broken

If a release is published but has issues:

1. **Don't delete the release** — it breaks links
2. Create a hotfix and release a new patch version
3. Update the broken release description with a warning and link to the fix

### Emergency Hotfix

For critical bugs in production:

```bash
# Create hotfix branch from the release tag
git checkout -b hotfix/critical-bug v0.2.0

# Fix the bug
# ... make changes ...

# Commit fix
git add .
git commit -m "fix: critical bug description"

# Push hotfix branch
git push origin hotfix/critical-bug

# Create PR to main
gh pr create --base main --title "fix: critical bug" --body "Emergency hotfix for v0.2.0"

# After PR merge, auto-release creates the patch automatically
```

## CI/CD Workflows

### pr-title.yml

**Trigger:** Pull request opened, edited, or synchronized
**Purpose:** Enforce conventional commit format on PR titles (since squash-merge uses PR title as commit message)

**Steps:**
1. Validate PR title against allowed conventional commit types using `amannn/action-semantic-pull-request`

### auto-release.yml

**Trigger:** Push to `main` (code changes only, docs excluded)
**Purpose:** Automatically release with semantic version bumping from commit messages

**Jobs:**
1. `check-skip` — Evaluate skip conditions (bot actor, commit message, PR label), then wait for CI to pass
2. `determine-bump` — Parse commits since last tag for bump type (major/minor/patch/none)
3. `release` — If bump is needed, call `version-bump.yml` with the determined bump type

### version-bump.yml

**Trigger:** Manual `workflow_dispatch` or `workflow_call` from auto-release
**Permissions:** `contents: write`
**Purpose:** Calculate next version, create tag, and publish release

**Inputs:**
- `bump`: `patch`, `minor`, or `major`

**Steps:**
1. Checkout with full history
2. Calculate next version from latest tag
3. Create annotated tag
4. Push tag
5. Checkout the new tag
6. Calculate changelog base tag (for minor/major, overrides GoReleaser's default)
7. Run GoReleaser with `release --clean`
8. Publish GitHub release artifacts and notes

### test-release.yml

**Trigger:** PR changes to release files
**Permissions:** `contents: read`
**Purpose:** Validate release config before merge

**Steps:**
1. Checkout code
2. Run GoReleaser in snapshot mode
3. Verify binaries work
4. Check for expected builds

## Configuration Files

| File | Purpose |
|------|---------|
| `.goreleaser.yml` | GoReleaser configuration (builds, archives, changelog, Homebrew cask) |
| `.github/workflows/pr-title.yml` | Conventional commit enforcement on PR titles |
| `.github/workflows/auto-release.yml` | Semantic version bump on merge to main |
| `.github/workflows/version-bump.yml` | Version tagging and release workflow |
| `.github/workflows/test-release.yml` | PR validation workflow |
| `Makefile` | Local build with version injection |

## Homebrew Distribution

Each release automatically pushes a Homebrew cask to `nvandessel/homebrew-tap` via GoReleaser's `homebrew_casks` configuration.

**How it works:**
1. GoReleaser builds archives for all platforms
2. The `homebrew_casks` section creates/updates a cask file in the tap repo
3. Users install with `brew install nvandessel/tap/floop`

**Requirements:**
- `HOMEBREW_TAP_GITHUB_TOKEN` secret on the floop repo (PAT with repo scope to push to `homebrew-tap`)
- The `nvandessel/homebrew-tap` repository must exist

## Future Enhancements

Not currently implemented but can be added later:

- **Docker images** — Multi-arch container publishing
- **Scoop manifest** — Windows package manager
- **AUR package** — Arch Linux user repository
- **Binary signing** — GPG or cosign signatures
- **SBOM generation** — Software bill of materials
- **Release channels** — Beta/RC releases

To add these, extend `.goreleaser.yml` with the appropriate sections. See [GoReleaser documentation](https://goreleaser.com/customization/) for details.

## Questions?

For questions about the release process:
- Check the [GoReleaser docs](https://goreleaser.com)
- Review past releases: `gh release list`
- Open an issue with the `question` label
