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
- computes the SHA256 for the tagged GitHub source tarball
- runs `make homebrew-formula VERSION=<version> SOURCE_SHA256=<sha256>`
- publishes the generated archives with `softprops/action-gh-release`

Release packaging currently targets:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`

Each release directory includes compressed archives plus a `SHA256SUMS` file.

The release job also generates `dist/releases/<version>/cleanr.rb`, which is a source-build Homebrew formula intended to be used as the starting point for a future `homebrew/core` submission.

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
        run: ./dist/cleanr run -config cleanr.yaml -format junit -output cleanr-junit.xml -trend-file reports/cleanr.trends.yaml -replay-artifact reports/cleanr.replay.json -build-id "${{ github.sha }}"

      - name: Upload report
        uses: actions/upload-artifact@v4
        with:
          name: cleanr-report
          path: |
            cleanr-junit.xml
            reports/cleanr.trends.yaml
            reports/cleanr.replay.json
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

Replay artifacts are the nightly triage companion to that retained history. If `reporting.replay_artifact_file` or `-replay-artifact` is set, `cleanr` writes a JSON or YAML bundle that includes:

- failing workflows and findings
- curated retained evidence from the failing cases
- run metadata such as build ID, target type, configured model, and scenario fingerprints
- the latest build diff, including prompt and model changes when they changed between retained runs

If `trend_file` is set and `replay_artifact_file` is omitted, `cleanr` derives a sibling replay artifact path automatically.

If you want CI to fail only on meaningful regressions instead of every informational delta, enable `reporting.trend_gates` in the config. That lets you gate on metrics like additional failed cases, semantic drift delta, or duration growth between builds.

A sane starter policy is:

```yaml
reporting:
  trend_file: reports/cleanr.trends.yaml
  replay_artifact_file: reports/cleanr.replay.json
  trend_limit: 30
  trend_gates:
    preset: moderate
```

Use `preset: strict` when you want tighter budgets, or `preset: exploratory` when you want the same trend history and summaries without CI-blocking gates.

If you want to keep a preset but tune one threshold, add only that field. For example:

```yaml
reporting:
  trend_file: reports/cleanr.trends.yaml
  replay_artifact_file: reports/cleanr.replay.json
  trend_limit: 30
  trend_gates:
    preset: moderate
    max_duration_increase_pct: 40
```

That exact policy is checked in as `examples/openai-responses-tuned.yaml` so teams can copy a working partial-override pattern directly.

To summarize the retained window for dashboards or release notes, run:

```bash
./dist/cleanr trends -trend-file reports/cleanr.trends.yaml -format json > cleanr-trends-summary.json
```

For nightly failures, upload the replay artifact alongside that summary so reviewers can inspect the exact failing workflows instead of only aggregate score deltas.

## Release Workflow Notes

`make release VERSION=vX.Y.Z` writes artifacts to `dist/releases/<version>/`.

That output structure is what the repository release workflow publishes, so local release dry runs should use the same command.

`make homebrew-formula VERSION=vX.Y.Z REPOSITORY=owner/name SOURCE_SHA256=<sha256>` generates the matching Homebrew formula from the tagged source archive checksum.

If the repository has a committed open-source license and you know the SPDX identifier, include it when generating the formula:

```bash
make homebrew-formula VERSION=vX.Y.Z REPOSITORY=owner/name SOURCE_SHA256=<sha256> HOMEBREW_LICENSE=MIT
```

The generated formula is structured for `homebrew/core`, which means it builds `cleanr` from source instead of downloading platform-specific release binaries.

This repository is not installable with `brew install cleanr` yet. That only becomes true after a formula PR is accepted into `Homebrew/homebrew-core`.

## Future CI Direction

The roadmap includes stronger CI-native support such as:

- a first-party GitHub Action
- more stable machine-readable contracts
- richer example workflows for common AI app setups

Track that work in [roadmap.md](roadmap.md) and [taskboard.md](taskboard.md).
