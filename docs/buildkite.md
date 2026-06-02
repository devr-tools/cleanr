# Buildkite Guide

`cleanr` does not need a Buildkite-specific backend to work well in Buildkite. The CLI already has stable exit codes, machine-readable outputs, and optional Buildkite-aware hooks for metadata, annotations, and artifact upload.

## Recommended Flow

Use Buildkite in two phases:

1. Validate and run the checked-in `cleanr` config.
2. Review generated or replay-backed scenario datasets and gate the step on review results.

## Example Pipeline

A checked-in example pipeline lives at [../.buildkite/pipeline.yml](../.buildkite/pipeline.yml).

If you want a reproducible artifact set before adapting the pipeline to your own Buildkite environment, run the repository smoke workflow first. [.github/workflows/cleanr-smoke.yml](../.github/workflows/cleanr-smoke.yml) now emits:

- `reports/github-actions-smoke.dataset.yaml`
- `reports/github-actions-smoke.reviewed.yaml`
- `reports/github-actions-smoke.review.json`
- `reports/github-actions-smoke.reviewed-config.yaml`

Those artifacts are the reference dataset/review loop that the Buildkite example mirrors.

The main pattern is:

```yaml
steps:
  - label: ":go: build cleanr"
    commands:
      - "go build -trimpath -o ./dist/cleanr ./cmd/cleanr"

  - label: ":mag: validate"
    commands:
      - "./dist/cleanr validate -profile pr"

  - label: ":test_tube: run"
    commands:
      - |
        ./dist/cleanr run \
          -profile pr \
          -format json \
          -output reports/cleanr-report.json \
          -replay-artifact reports/cleanr.replay.json \
          -buildkite-meta \
          -buildkite-annotation

  - label: ":clipboard: review dataset"
    commands:
      - |
        ./dist/cleanr dataset export \
          -profile pr \
          -replay-artifact reports/cleanr.replay.json \
          -output reports/cleanr.dataset.yaml
      - |
        ./dist/cleanr dataset review \
          -input reports/cleanr.dataset.yaml \
          -profile pr \
          -output reports/cleanr.reviewed.yaml \
          -merge-output .cleanr/pr.reviewed.yaml \
          -fail-on-pending \
          -min-approved 1 \
          -max-duplicates 0 \
          -buildkite-meta \
          -buildkite-annotation \
          -buildkite-upload-artifacts
```

## Buildkite Flags

### `cleanr run`

- `-buildkite-meta`: writes run metrics into Buildkite metadata via `buildkite-agent meta-data set`
- `-buildkite-annotation`: writes a failure annotation when the run fails

Current metadata keys:

- `cleanr.run.passed`
- `cleanr.run.total_suites`
- `cleanr.run.failed_suites`
- `cleanr.run.total_cases`
- `cleanr.run.failed_cases`
- `cleanr.run.report_format`
- `cleanr.run.report_output`
- `cleanr.run.build_id`
- `cleanr.run.provider_model`
- `cleanr.run.target_type`
- `cleanr.run.trend_gate_enabled`
- `cleanr.run.trend_gate_passed`

### `cleanr dataset review`

- `-buildkite-meta`: writes review metrics into Buildkite metadata
- `-buildkite-annotation`: writes an error annotation when the review gate fails
- `-buildkite-upload-artifacts`: uploads the reviewed dataset artifact and merged config output when present

Current metadata keys:

- `cleanr.review.gate_passed`
- `cleanr.review.total`
- `cleanr.review.approved`
- `cleanr.review.rejected`
- `cleanr.review.pending`
- `cleanr.review.new`
- `cleanr.review.modified`
- `cleanr.review.duplicates`
- `cleanr.review.unchanged`
- `cleanr.review.artifact`
- `cleanr.review.merge_output`
- `cleanr.review.top_candidate`
- `cleanr.review.top_score`

## Failure Semantics

Buildkite hooks are best-effort companions to the main CLI result:

- `cleanr run` still exits `0`, `1`, or `2` based on the local run result
- `cleanr dataset review` still exits `0`, `1`, or `2` based on review gates and runtime correctness
- if `buildkite-agent` is missing or one of the Buildkite helper actions fails, `cleanr` prints a warning to `stderr` and keeps the primary command result authoritative

That keeps the Buildkite integration useful without making Buildkite itself the source of truth for pass/fail.
