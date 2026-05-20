# CI Guide

`cleanr` is intended to be easy to run in CI because its exit codes are stable and its report formats are machine-readable.

## Current Repository Workflows

The repository currently ships with two GitHub Actions workflows.

### `.github/workflows/ci.yml`

Runs on pull requests and pushes to `main`.

It currently:

- checks out the repository
- sets up Go from `go.mod`
- runs `make check`
- runs `make build`
- uploads `dist/cleanr` as a workflow artifact

### `.github/workflows/release.yml`

Runs on tag pushes matching `v*` and also supports manual dispatch.

It currently:

- resolves the release version from the tag or manual input
- runs `make release VERSION=<version>`
- publishes the generated archives with `softprops/action-gh-release`

Release packaging currently targets:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`

Each release directory includes compressed archives plus a `SHA256SUMS` file.

## Exit Code Contract

- `0`: all suites passed
- `1`: one or more suites or cases failed
- `2`: invalid configuration or runtime error

That split is useful in pipelines because it distinguishes product failures from infrastructure or config failures.

## Minimal GitHub Actions Integration

If you want to run `cleanr` inside another repository's workflow, a minimal pattern looks like this:

```yaml
name: AI QA

on:
  pull_request:
  push:
    branches: [main]

jobs:
  cleanr:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.20"

      - name: Build cleanr
        run: go build -trimpath -o ./dist/cleanr ./cmd/cleanr

      - name: Validate config
        run: ./dist/cleanr validate -config cleanr.yaml

      - name: Run cleanr
        run: ./dist/cleanr run -config cleanr.yaml -format junit -output cleanr-junit.xml -trend-file reports/cleanr.trends.yaml -build-id "${{ github.sha }}"

      - name: Upload report
        uses: actions/upload-artifact@v4
        with:
          name: cleanr-report
          path: |
            cleanr-junit.xml
            reports/cleanr.trends.yaml
```

## Local-to-CI Workflow

A practical rollout path is:

- develop and tune the config locally
- validate it with `cleanr validate`
- run text reports while tuning thresholds
- switch CI output to `junit` or `json`
- upload the generated artifact for inspection on failures

## When to Use Each Report Format

- `text`: good for local development and terminal-first review
- `json`: good for automation, custom ingestion, or post-processing
- `junit`: good for CI systems that already understand test reports

Trend history is orthogonal to the main report format. If `reporting.trend_file` or `-trend-file` is set, `cleanr` also writes a compact JSON or YAML history file that can be persisted as a CI artifact and compared by later runs.

## Release Workflow Notes

`make release VERSION=vX.Y.Z` writes artifacts to `dist/releases/<version>/`.

That output structure is what the repository release workflow publishes, so local release dry runs should use the same command.

## Future CI Direction

The roadmap includes stronger CI-native support such as:

- a first-party GitHub Action
- more stable machine-readable contracts
- richer example workflows for common AI app setups

Track that work in [roadmap.md](roadmap.md) and [taskboard.md](taskboard.md).
