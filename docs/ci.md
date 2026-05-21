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
        env:
          CLEANR_ATTESTATION_KEY: ${{ secrets.CLEANR_ATTESTATION_KEY }}
        run: ./dist/cleanr run -config cleanr.yaml -format junit -output cleanr-junit.xml -trend-file reports/cleanr.trends.yaml -replay-artifact reports/cleanr.replay.json -build-id "${{ github.sha }}"

      - name: Upload report
        uses: actions/upload-artifact@v4
        with:
          name: cleanr-report
          path: |
            cleanr-junit.xml
            reports/cleanr.trends.yaml
            reports/cleanr.replay.json
            reports/cleanr.attestation.json
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
- `sarif`: good for IDEs, GitHub code scanning, and PR review surfaces that understand SARIF

Trend history is orthogonal to the main report format. If `reporting.trend_file` or `-trend-file` is set, `cleanr` also writes a compact JSON or YAML history file that can be persisted as a CI artifact and compared by later runs.

Replay artifacts are the nightly triage companion to that retained history. If `reporting.replay_artifact_file` or `-replay-artifact` is set, `cleanr` writes a JSON or YAML bundle that includes:

- failing workflows and findings
- curated retained evidence from the failing cases
- run metadata such as build ID, target type, configured model, and scenario fingerprints
- the latest build diff, including prompt and model changes when they changed between retained runs

If `trend_file` is set and `replay_artifact_file` is omitted, `cleanr` derives a sibling replay artifact path automatically.

Signed attestations are the governance companion to those artifacts. If `governance.attestation.enabled` is true, `cleanr run` signs the release-gate statement with an Ed25519 key from the configured env var and writes an attestation file for audit or change-review workflows.

Phase 5 integrations sit beside that local gate instead of replacing it. Remote trend-source fetches, result publishing, and summary writing are best-effort companions. A local passing run still exits `0` even if a remote sink is unavailable, and a local failing run still exits `1` even if the remote publish succeeds.

If you want CI to fail only on meaningful regressions instead of every informational delta, enable `reporting.trend_gates` in the config. That lets you gate on metrics like additional failed cases, semantic drift delta, or duration growth between builds.

A sane starter policy is:

```yaml
reporting:
  trend_file: reports/cleanr.trends.yaml
  replay_artifact_file: reports/cleanr.replay.json
  trend_limit: 30
  trend_gates:
    preset: moderate
governance:
  attestation:
    enabled: true
    output: reports/cleanr.attestation.json
    key_env: CLEANR_ATTESTATION_KEY
    key_id: ci-ed25519
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

To attach optional remote publishing and PR summaries, add an `integrations` block such as:

```yaml
integrations:
  trend_sources:
    - name: approved-history
      type: braintrust
      project: qa-gates
      experiment: release-gate
      view_url: https://braintrust.example/runs/approved-history
      api_key_env: CLEANR_BRAINTRUST_TOKEN
  result_sinks:
    - name: braintrust
      type: braintrust
      project: qa-gates
      experiment: release-gate
      api_key_env: CLEANR_BRAINTRUST_TOKEN
      include_replay_artifact: true
      include_attestation: true
  summaries:
    - name: pr
      format: markdown
      output: reports/cleanr-summary.md
```

That gives CI a durable local gate plus add-on remote triage links without turning `cleanr` into the system of record for hosted observability.

For Braintrust Cloud EU or self-hosted deployments, set `base_url` on the sink and source to your data plane URL.

If you prefer Langfuse as the remote sink companion, a minimal native sink looks like this:

```yaml
integrations:
  result_sinks:
    - name: langfuse
      type: langfuse
      base_url: https://cloud.langfuse.com
      public_key_env: LANGFUSE_PUBLIC_KEY
      secret_key_env: LANGFUSE_SECRET_KEY
      experiment: release-gate
      include_replay_artifact: true
      run_url_template: https://cloud.langfuse.com/project/demo/traces/{{trace_id}}
```

That mode publishes the local CI result as a Langfuse trace plus numeric scores such as pass or fail, failed suite count, and failed case count.

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

The next CI-facing work is narrower:

- a first-party GitHub Action wrapper around the existing CLI
- broader example workflows for common remote sink and dataset promotion flows
- additional machine-readable contracts for downstream automation beyond the current JSON, JUnit, SARIF, replay, and summary outputs

Track that work in [roadmap.md](roadmap.md) and [taskboard.md](taskboard.md).
