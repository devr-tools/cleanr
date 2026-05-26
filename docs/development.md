# Development Guide

This page is for contributor workflows. Keep the repository root [README](../README.md) for installation and product usage, and use `docs/` for deeper operational material.

## Common Local Commands

```bash
make fmt
make lint
make test
make check
make ci
make build
make release VERSION=v0.1.0
```

## What `make check` Runs

`make check` is the main local developer gate. It currently:

- validates Go file layout
- checks `gofmt`
- runs `go vet`
- runs `go test ./...`

`make deploy` remains an alias for `make release`.

## What `make ci` Runs

`make ci` is the local parity command for `.github/workflows/ci.yml`. It runs:

- changed-file test-presence validation against your local base ref
- `gofmt` drift checks
- `go vet`
- `gocyclo` with the repository complexity budget
- `go test ./...` on the current OS
- the Linux amd64 snapshot build
- the internal coverage gate
- `govulncheck`
- `semgrep scan --config auto --baseline-commit <merge-base> --error`
- doc review and DCO checks when the base branch is `develop`

Local behavior differs from hosted GitHub Actions in two places:

- the test suite runs only on your current OS instead of the GitHub Ubuntu and macOS matrix
- PR-only checks use a local Git base ref, resolved from `CLEANR_CI_BASE_REF`, `PR_BASE_REF`, your upstream branch, or common `origin/*` defaults

Set `CI_BASE_REF=<ref>` when you want to force the comparison target, for example `make ci CI_BASE_REF=origin/develop`.

## Local Build Outputs

- `make build` writes the CLI binary to `dist/cleanr`
- `make release VERSION=vX.Y.Z` writes release artifacts to `dist/releases/<version>/`
- `make homebrew-formula ...` writes `dist/releases/<version>/cleanr.rb`

## Release and Packaging

The detailed release pipeline now lives in [release-automation.md](release-automation.md). Use that page for:

- Release Please metadata
- branch-driven prerelease automation
- tagged GitHub Releases
- GHCR publishing
- Homebrew tap sync

## Related Docs

- [CI guide](ci.md)
- [Release automation](release-automation.md)
- [Homebrew packaging](homebrew.md)
- [Roadmap](roadmap.md)
- [Taskboard](taskboard.md)
