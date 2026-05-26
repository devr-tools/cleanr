# CI Guide

`cleanr` is CI-friendly because its exit codes are stable and its outputs are machine-readable.

## Exit Code Contract

- `0`: all suites passed
- `1`: one or more suites or cases failed
- `2`: invalid configuration or runtime error

That split lets pipelines distinguish product regressions from setup or infrastructure failures.

## Pick an Integration Shape

### Use the released container

Best when your CI supports container jobs and you want the exact tagged CLI:

```yaml
jobs:
  cleanr:
    runs-on: ubuntu-latest
    container: ghcr.io/devr-tools/cleanr:<tag>
    steps:
      - uses: actions/checkout@v4
      - run: cleanr validate -config cleanr.yaml
      - run: cleanr run -config cleanr.yaml -format junit -output cleanr-junit.xml
```

### Use `go install`

Best when you want a simple host-runner install path:

```yaml
jobs:
  cleanr:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.20"
      - run: go install github.com/devr-tools/cleanr/cmd/cleanr@latest
      - run: cleanr validate -config cleanr.yaml
      - run: cleanr run -config cleanr.yaml -format junit -output cleanr-junit.xml
```

### Build from source

Best when you need to pin the exact repo state under test:

```yaml
jobs:
  cleanr:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make build
      - run: ./dist/cleanr validate -config cleanr.yaml
      - run: ./dist/cleanr run -config cleanr.yaml -format junit -output cleanr-junit.xml
```

## Recommended Outputs

For CI, the usual combination is:

- main report as `junit` or `json`
- trend history via `-trend-file`
- replay bundle via `-replay-artifact`
- signed attestation via `CLEANR_ATTESTATION_KEY`

Example:

```bash
cleanr run \
  -config cleanr.yaml \
  -format junit \
  -output cleanr-junit.xml \
  -trend-file reports/cleanr.trends.yaml \
  -replay-artifact reports/cleanr.replay.json \
  -build-id "$GITHUB_SHA"
```

## Artifact Retention

Persist these files between runs when you want trend and replay workflows to be useful:

- `cleanr-junit.xml`
- `reports/cleanr.trends.yaml`
- `reports/cleanr.replay.json`
- `reports/cleanr.attestation.json`

## Repository Workflows

This repository already ships with:

- `.github/workflows/ci.yml`: PR validation and quality gates
- `.github/workflows/cd.yml`: branch-driven prerelease and Release Please orchestration
- `.github/workflows/release.yml`: tag-driven publishing for binaries, GHCR, and Homebrew sync
- `.github/workflows/homebrew-validation.yml`: PR-time formula install and `brew test`
- `.github/workflows/cleanr-smoke.yml`: manual and PR-safe smoke workflow that builds `cleanr`, runs it against a local mock target, captures a baseline, and renders trend artifacts without external model credentials
- `.github/workflows/cleanr-connected.yml`: manual provider workflow that generates an agent config, reads standard GitHub secrets, and optionally connects Braintrust, Langfuse, PostHog, webhook sinks, and signed attestations

Release and branch-publishing details live in [release-automation.md](release-automation.md).

## Connected Secrets Workflow

Use `.github/workflows/cleanr-connected.yml` when you want a repo owner to wire credentials once and immediately run a provider-backed `cleanr` job.

The workflow expects a small fixed contract:

- Repository secrets:
  - `OPENAI_API_KEY`
  - `ANTHROPIC_API_KEY`
  - `BRAINTRUST_API_KEY`
  - `LANGFUSE_PUBLIC_KEY`
  - `LANGFUSE_SECRET_KEY`
  - `POSTHOG_PROJECT_TOKEN`
  - `CLEANR_RESULTS_WEBHOOK_TOKEN`
  - `CLEANR_ATTESTATION_KEY`
- Repository variables:
  - `CLEANR_PROVIDER`
  - `CLEANR_MODEL`
  - `CLEANR_OPENAI_API_MODE`
  - `CLEANR_AGENT_NAME`
  - `CLEANR_SYSTEM_PROMPT`
  - `CLEANR_USER_PROMPT`
  - `CLEANR_TREND_GATE_PRESET`
  - `CLEANR_BRAINTRUST_PROJECT`
  - `CLEANR_BRAINTRUST_EXPERIMENT`
  - `CLEANR_BRAINTRUST_BASE_URL`
  - `CLEANR_LANGFUSE_BASE_URL`
  - `CLEANR_LANGFUSE_EXPERIMENT`
  - `CLEANR_POSTHOG_BASE_URL`
  - `CLEANR_POSTHOG_EXPERIMENT`
  - `CLEANR_RESULTS_WEBHOOK_URL`

The setup command now supports the same pattern directly, for example:

```bash
cleanr setup agent --ci \
  -provider openai \
  -model gpt-4.1-mini \
  -name support-agent \
  -system-prompt "You are a concise support agent." \
  -user-prompt "Help the customer reset their password." \
  -with-braintrust \
  -braintrust-project support-ai \
  -with-langfuse \
  -with-posthog \
  -with-webhook \
  -webhook-endpoint https://example.com/cleanr \
  -with-attestation \
  -output cleanr.agent.yaml
```

That generated config points to standard env var names instead of embedding credentials, so a user can set secrets once and run without editing the config.

## Related Docs

- [Docker guide](docker.md)
- [Configuration](configuration.md)
- [Release automation](release-automation.md)
