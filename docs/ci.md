# CI Guide

`cleanr` is CI-friendly because its exit codes are stable and its outputs are machine-readable.

## Exit Code Contract

- `0`: all suites passed
- `1`: one or more suites or cases failed
- `2`: invalid configuration or runtime error

That split lets pipelines distinguish product regressions from setup or infrastructure failures.

## Pick an Integration Shape

If you use Buildkite instead of GitHub Actions, see [buildkite.md](buildkite.md) for a pipeline example and the Buildkite-specific CLI hooks.

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

### Use a job container plus an app service container

Best when your application under test runs in its own container and talks to the provider internally:

```yaml
jobs:
  cleanr:
    runs-on: ubuntu-latest
    container: ghcr.io/devr-tools/cleanr:<tag>
    services:
      app:
        image: ghcr.io/your-org/assistant-api:latest
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        options: >-
          --health-cmd "curl -fsS http://localhost:8080/healthz || exit 1"
          --health-interval 10s
          --health-timeout 3s
          --health-retries 12
    steps:
      - uses: actions/checkout@v4
      - run: cleanr validate -config examples/containerized-assistant/cleanr.yaml
      - run: cleanr run -config examples/containerized-assistant/cleanr.yaml -format junit -output cleanr-junit.xml
```

In that model:

- the app container owns `OPENAI_API_KEY` or `ANTHROPIC_API_KEY`
- `cleanr` talks to the app with `target.type: http`
- the app is reachable from the `cleanr` container at `http://app:8080`
- `cleanr` only needs its own provider secret if you enable `scenario_generation` or test a native provider target directly

The checked-in example lives in [examples/containerized-assistant](../examples/containerized-assistant/README.md).

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
  -github-outputs \
  -trend-file reports/cleanr.trends.yaml \
  -replay-artifact reports/cleanr.replay.json \
  -build-id "$GITHUB_SHA"
```

## Dataset Review Gates

When you generate scenarios or export replay failures, you can gate that review step directly in CI without parsing stdout:

```bash
cleanr dataset review \
  -input reports/cleanr.dataset.yaml \
  -profile pr \
  -output reports/cleanr.reviewed.yaml \
  -merge-output .cleanr/pr.reviewed.yaml \
  -approve refund-policy \
  -fail-on-pending \
  -min-approved 1 \
  -max-duplicates 0 \
  -github-outputs
```

Review gate behavior:

- exit `0`: review completed and all requested gate conditions passed
- exit `1`: review completed, but one or more gate conditions failed
- exit `2`: invalid flags, invalid config, or another runtime/setup problem

If `-github-outputs` is enabled inside GitHub Actions, `cleanr` writes structured outputs like:

- `cleanr_run_gate_passed`
- `cleanr_run_failed_suites`
- `cleanr_run_failed_cases`
- `cleanr_run_new_failures`
- `cleanr_run_worsened_drift`
- `cleanr_run_review_scenarios`
- `cleanr_run_gate_summary`
- `cleanr_run_pr_comment`
- `cleanr_review_gate_passed`
- `cleanr_review_approved`
- `cleanr_review_rejected`
- `cleanr_review_pending`
- `cleanr_review_duplicates`
- `cleanr_review_artifact`
- `cleanr_review_policy_path`
- `cleanr_review_merge_output`
- `cleanr_review_top_candidate`

For local GitHub setup, use:

```bash
cleanr github doctor
cleanr github auth
```

`cleanr github doctor` checks whether `gh` is on `PATH` and whether it already has a usable session. `cleanr github auth` runs `gh auth login`, which is the local auth path that `cleanr` uses for PR creation and PR commenting.

Example GitHub Actions step:

```yaml
- name: Run cleanr
  id: cleanr_run
  run: |
    ./dist/cleanr run \
      -config cleanr.yaml \
      -format junit \
      -output cleanr-junit.xml \
      -github-outputs
```

The run step appends a PR-ready markdown body to `$GITHUB_STEP_SUMMARY` and exposes the same markdown via `steps.cleanr_run.outputs.cleanr_run_pr_comment` for comment actions.

If you want `cleanr` itself to post the comment, add `-github-pr-comment`. You can also pass `-github-pr-number 123` outside GitHub Actions. That path requires `gh` on `PATH` with permission to comment on the PR.

Review step:

```yaml
- name: Review cleanr dataset
  id: cleanr_review
  run: |
    ./dist/cleanr dataset review \
      -input reports/cleanr.dataset.yaml \
      -profile pr \
      -output reports/cleanr.reviewed.yaml \
      -merge-output .cleanr/pr.reviewed.yaml \
      -fail-on-pending \
      -min-approved 1 \
      -max-duplicates 0 \
      -github-outputs

- name: Upload reviewed dataset
  if: always()
  uses: actions/upload-artifact@v4
  with:
    name: cleanr-reviewed-dataset
    path: |
      reports/cleanr.reviewed.yaml
      .cleanr/pr.reviewed.yaml

- name: Echo top candidate
  if: always()
  run: echo "Top candidate: ${{ steps.cleanr_review.outputs.cleanr_review_top_candidate }}"
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
- `.github/workflows/release.yml`: reusable publishing for binaries, GHCR, and Homebrew sync
- `.github/workflows/homebrew-validation.yml`: PR-time formula install and `brew test`
- `.github/workflows/cleanr-smoke.yml`: manual and PR-safe smoke workflow that builds `cleanr`, runs it against a local mock target, captures a baseline, renders trend artifacts, and emits a replay-backed dataset plus reviewed artifacts without external model credentials
- `.github/workflows/cleanr-connected.yml`: manual provider workflow that generates an agent config, reads standard GitHub secrets, and optionally connects Braintrust, Langfuse, PostHog, webhook sinks, and signed attestations

Release and branch-publishing details live in [release-automation.md](release-automation.md).

## Local Parity

Use `make ci` before committing when you want a local approximation of `.github/workflows/ci.yml`.

It runs the same main gates locally: test presence, formatting, `go vet`, `codeguard` (`gocyclo` + `scc` + `golangci-lint`), `go test`, the Linux amd64 snapshot build, the internal coverage threshold, `govulncheck`, `semgrep`, and the `develop`-only doc review and DCO rules.

The `codeguard` step renders a compact section summary and currently covers:

- `gocyclo`, which compares changed files to the resolved base ref and fails only on new or worsened over-limit findings
- `scc`, which treats changed non-test Go files above `400` code lines as god files and fails only on new or worsened size debt compared with the base ref
- `golangci-lint`, which uses [.golangci.yml](../.golangci.yml) and reports only new maintainability findings against the merge-base with the target branch
- advisory AST-based quality sections for `DRY`, `Clean Code`, and `Design Principles (SOLID/SoC)` on changed Go files

God-file exceptions can be listed in [.codeguard-godfiles-allowlist](../.codeguard-godfiles-allowlist). Entries there are repo-relative paths and only exempt files from the `God Files` section; complexity and lint checks still apply.
The main codeguard thresholds are environment-driven: `MAX_GO_FILE_CODE_LINES` for the god-file cap and `MAX_FUNCTION_COMPLEXITY` for the `gocyclo` function threshold.
If `govulncheck` cannot be installed for the current local Go toolchain, `make ci` skips that step with a warning instead of failing before the rest of the pre-commit checks can run.
If `semgrep` is not installed locally, `make ci` skips that step with a warning instead of failing before the rest of the pre-commit checks can run.

The local command compares your working tree against a Git base ref. Resolution order is:

- `CLEANR_CI_BASE_REF`
- `PR_BASE_REF`
- the current branch upstream, such as `origin/main`
- common fallbacks like `origin/develop`, `origin/main`, `origin/master`

You can override it explicitly:

```bash
make ci CI_BASE_REF=origin/develop
```

Install or make available these local dependencies if you want full parity:

- Go toolchain from `go.mod`
- network access for `go install` of `gocyclo`, `scc`, `golangci-lint`, and `govulncheck`
- `semgrep` on your `PATH`

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
  - `CLEANR_PROFILE`
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

`cleanr-connected.yml` also supports staged setup profiles via `CLEANR_PROFILE`. The same variable now selects staged local configs such as `.cleanr/pr.yaml`, `.cleanr/main.yaml`, and `.cleanr/release.yaml` for CLI commands like `cleanr run`, `cleanr validate`, and `cleanr snapshot`:

- `pr`: light drift, security, token optimization, exploratory trend gates
- `main`: retained trend history and moderate trend gates
- `release`: full drift, load, chaos, replay artifacts, attestation, and starter `release_policy` rules

## Braintrust Sync Loop

When Braintrust stores replay artifacts and a follow-up optimizer writes an explicit `cleanr_sync` payload into the experiment, `cleanr` can pull those recommendations back into a reviewable config update:

```bash
cleanr sync braintrust \
  -config cleanr.connected.yaml \
  -output-insights reports/braintrust.insights.yaml \
  -output-dataset reports/braintrust.dataset.yaml \
  -output-config cleanr.synced.yaml \
  -approve-insights
```

The sync command:

- reads the latest matching Braintrust experiment for the configured project and experiment family
- derives regression scenarios from the stored replay artifact using the local base config
- applies explicit config patch operations from `output.cleanr_sync`
- writes a normalized Braintrust insight dataset for auditability

If you want `cleanr` to open a GitHub PR after generating the files, run:

```bash
cleanr sync braintrust \
  -config cleanr.connected.yaml \
  -output-config cleanr.connected.yaml \
  -approve-insights \
  -create-pr \
  -pr-branch cleanr-sync-braintrust \
  -pr-title "cleanr sync: apply Braintrust insights"
```

That flow requires `git` and the GitHub CLI `gh` on `PATH`.

## Related Docs

- [Docker guide](docker.md)
- [Configuration](configuration.md)
- [Release automation](release-automation.md)
