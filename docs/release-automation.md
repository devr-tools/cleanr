# Release Automation

This repository now follows the same tool-family release shape as `szr`.

## Workflows

### `.github/workflows/cd.yml`

Branch-driven CD entry point.

It currently:

- runs on pushes to `develop`, `master`, and `main`
- computes prerelease tags automatically for `develop`
- reuses `.github/workflows/release.yml` for prerelease packaging
- runs Release Please on `main` and `master`

### `.github/workflows/release.yml`

Reusable publisher invoked by CD or manual dispatch.

It currently:

- supports `workflow_dispatch` and `workflow_call`
- normalizes and validates tags before release
- runs GoReleaser using `.goreleaser.yaml`
- uploads release archives and checksums
- publishes a GHCR image for Linux `amd64` and `arm64`
- publishes a multi-arch GHCR manifest for the release tag
- syncs `Formula/cleanr.rb` in `devr-tools/homebrew-tap` for stable releases

## Release Please Files

Stable branch release preparation is driven by:

- `.github/release-please-config.json`
- `.release-please-manifest.json`
- `CHANGELOG.md`
- `internal/version/version.go`

## Required Secrets

- `GITHUB_TOKEN`: used by the release workflow for GitHub Releases and GHCR publishing
- `RELEASE_PLEASE_TOKEN`: used for Release Please PRs and Homebrew tap automation

## Published Outputs

Each tagged release currently publishes:

- `darwin/amd64` archive
- `darwin/arm64` archive
- `linux/amd64` archive
- `linux/arm64` archive
- `SHA256SUMS`
- `ghcr.io/devr-tools/cleanr:<tag>`

## Local Developer Commands

```bash
make build
make release VERSION=v0.1.0
make homebrew-formula VERSION=v0.1.0 REPOSITORY=devr-tools/cleanr SOURCE_SHA256=<sha256> HOMEBREW_LICENSE=Apache-2.0
```

## Related Docs

- [CI guide](ci.md)
- [Homebrew packaging](homebrew.md)
- [Developer guide](development.md)
