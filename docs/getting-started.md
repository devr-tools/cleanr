# Getting Started

This guide keeps the shortest path to a working `cleanr` run. Use it when you want to validate one target quickly, then branch into the deeper docs only where needed.

## Pick an Install Path

- CLI: `go install github.com/devr-tools/cleanr/cmd/cleanr@latest`
- Release binary: download a tagged archive from GitHub Releases
- Container: pull `ghcr.io/devr-tools/cleanr:<tag>`

The repository root [README](../README.md) has the copy-paste install commands.

## Generate a Starter Config

For a local interactive setup:

```bash
cleanr setup
```

For CI-safe config generation:

```bash
cleanr setup --ci -provider openai -model gpt-4.1-mini -output cleanr.yaml
```

For staged pipeline scaffolding:

```bash
cleanr setup --ci -provider openai -model gpt-4.1-mini -profile pr -output cleanr-pr.yaml
cleanr setup --ci -provider openai -model gpt-4.1-mini -profile main -output cleanr-main.yaml
cleanr setup --ci -provider openai -model gpt-4.1-mini -profile release -output cleanr-release.yaml
```

If you prefer to start from checked-in examples instead of the setup flow, use one of:

- `examples/openai-responses.yaml`
- `examples/openai-chat-completions.yaml`
- `examples/anthropic-messages.yaml`
- `examples/containerized-assistant/cleanr.yaml`
- `examples/openai-responses-tuned.yaml`
- `examples/best-practices/cleanr-pr.yaml`
- `examples/best-practices/cleanr-main.yaml`
- `examples/best-practices/cleanr-release.yaml`
- `examples/stateful-support-agent/cleanr.yaml`

## Validate and Run

```bash
cleanr validate -config cleanr.yaml
cleanr run -config cleanr.yaml
```

Common CI-oriented outputs:

```bash
cleanr run -config cleanr.yaml -format junit -output cleanr-junit.xml
cleanr run -config cleanr.yaml -trend-file reports/cleanr.trends.yaml -build-id "$GITHUB_SHA"
cleanr run -config cleanr.yaml -replay-artifact reports/cleanr.replay.json -build-id "$GITHUB_SHA"
```

## Capture a Baseline

Before enabling drift gates in CI, capture a known-good snapshot:

```bash
cleanr snapshot -config cleanr.yaml
```

Commit the resulting snapshot file once it reflects expected behavior.

## Choose the Next Guide

- [Configuration](configuration.md): target fields, suites, thresholds, and reporting options
- [Best practices](best-practices.md): staged rollout guidance for PR, main, nightly, and release pipelines
- [SDK guide](sdk.md): embed `cleanr` as a Go package
- [Docker guide](docker.md): run `cleanr` from GHCR in pipelines
- [CI guide](ci.md): wire reports, artifacts, and exit codes into automation
- [Release automation](release-automation.md): understand the repo's CD and publishing flow
