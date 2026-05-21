# Getting Started

This guide walks through the shortest path to running `cleanr` against an HTTP-based AI endpoint or a native OpenAI or Anthropic target.

## Prerequisites

- Go 1.20 or newer
- A reachable HTTP endpoint for the AI application you want to test
- A request and response shape you can describe with `cleanr` config fields

## Build the CLI

From the repository root:

```bash
make build
./dist/cleanr version
```

You can also run the CLI directly from source with `go run ./cmd/cleanr ...`, but using the built binary keeps command examples consistent with CI and release usage.

## Generate a Starter Config

Write the default JSON config:

```bash
./dist/cleanr init
```

Write a YAML version instead:

```bash
./dist/cleanr init -output cleanr.yaml
```

The generated file includes:

- an HTTP target definition
- example scenarios
- a starter assertion example
- all currently supported suites enabled with starter thresholds
- text reporting as the default output mode

If you want to start from an org-level policy baseline instead of a raw starter, add a policy pack such as:

```yaml
policy_packs:
  - ./examples/policy-packs/support-strict.yaml
```

If you also want organization-specific extension points, add a plugin manifest such as:

```yaml
plugins:
  - ./examples/plugins/release-audit.yaml
```

If you want `cleanr` to set up a native provider for you instead of editing the starter manually, use:

```bash
./dist/cleanr setup
```

That flow:

- shows a terminal UI with arrow-key provider selection when run in a real terminal
- can open the provider dashboard in your browser so you can sign in and create an API key
- prompts for the model, API key env var name, and API key
- stores the API key in `~/.cleanr/profile.json`
- writes a starter `cleanr.yaml`

If you want the same flow to open the provider key page in your browser immediately, use:

```bash
./dist/cleanr setup --browser
```

If you are testing an agent and want to seed the config with a specific system prompt, use:

```bash
./dist/cleanr setup agent
```

That flow reuses the saved provider profile, asks for the agent prompt and a primary user task, then writes an agent-focused YAML config such as `cleanr.agent.yaml`.

If you want the agent flow to open the provider key page in your browser first, use:

```bash
./dist/cleanr setup agent --browser
```

If you need the same setup flow in CI without prompts, browser launch, or local token storage, use:

```bash
./dist/cleanr setup --ci -provider openai -model gpt-4.1-mini -output cleanr.yaml
./dist/cleanr setup agent --ci -provider openai -model gpt-4.1-mini -name support-agent -system-prompt "You are a safe support agent." -user-prompt "Reset the password and confirm the email."
```

CI mode generates the config only. It does not write `~/.cleanr/profile.json`, so your pipeline should provide the actual provider secret through the env var referenced by the generated config.

Recommended setup command matrix:

```bash
# Local TUI
./dist/cleanr setup

# Local TUI with browser-open provider setup
./dist/cleanr setup --browser

# Local agent config generation
./dist/cleanr setup agent

# CI-safe provider config generation
./dist/cleanr setup --ci -provider openai -model gpt-4.1-mini -output cleanr.yaml

# CI-safe agent config generation
./dist/cleanr setup agent --ci -provider openai -model gpt-4.1-mini -name support-agent -system-prompt "You are a safe support agent." -user-prompt "Reset the password and confirm the email."
```

## Capture a Baseline

Before using drift as a regression gate, write a baseline snapshot file from a known-good build:

```bash
./dist/cleanr snapshot -config cleanr.json
./dist/cleanr snapshot -config cleanr.yaml
```

If `suites.drift.baseline_file` is set, `cleanr snapshot` writes there. Otherwise it defaults to `cleanr.snapshots.yaml`.

Commit that snapshot file to the repository once it reflects expected behavior.

If you want a native provider config instead of the default HTTP starter, begin from one of these examples:

- `examples/openai-responses.yaml`
- `examples/openai-chat-completions.yaml`
- `examples/anthropic-messages.yaml`
- `examples/openai-responses-tuned.yaml`
- `examples/stateful-support-agent/cleanr.yaml`

Use `examples/openai-responses-tuned.yaml` when you want a concrete example of preset-based trend gating with one tuned override instead of a fully custom threshold block.

If you want a real stateful workflow instead of a minimal starter, use [examples/stateful-support-agent](../examples/stateful-support-agent/README.md). It shows an HTTP agent that emits normalized trace evidence, mutates local state, and is gated by `release_policy`, `claim_trace`, `provenance`, and `shadow_state`.

## Point It at Your Endpoint

For an HTTP target, update these values in the generated config:

- `target.url`: the full endpoint URL
- `target.prompt_field`: the request field that should receive the end-user prompt
- `target.response_field`: the JSON path containing the model response text

If your API accepts a system prompt, also set `target.system_field`.

If your endpoint expects a larger payload shape, update `target.request_template` to match it. `cleanr` will inject the prompt and system fields into that template at runtime.

If your HTTP endpoint can emit workflow evidence, have it return a top-level `trace` object with fields such as `tool_calls`, `approvals`, `state_changes`, and `memory_operations`. The generic HTTP adapter ingests that normalized evidence directly, so you can use `assertions`, `claim_trace`, and `release_policy` without writing a native provider adapter.

For a native OpenAI target:

- set `target.type: openai`
- set `target.openai.model`
- choose `target.openai.api_mode: responses` or `chat_completions`
- export the API key env var, usually `OPENAI_API_KEY`

For a native Anthropic target:

- set `target.type: anthropic`
- set `target.anthropic.model`
- set `target.anthropic.max_tokens` or use the default
- export the API key env var, usually `ANTHROPIC_API_KEY`

## Validate the Config

```bash
./dist/cleanr validate -config cleanr.json
./dist/cleanr validate -config cleanr.yaml
```

If you omit `-config`, `cleanr` looks for:

- `cleanr.json`
- `cleanr.yaml`
- `cleanr.yml`

Validation failures return exit code `2` and include field-level guidance for fixing the config.

## Run the Suites

Run with the default text report:

```bash
./dist/cleanr run -config cleanr.json
```

Write JSON or JUnit output:

```bash
./dist/cleanr run -config cleanr.json -format json -output cleanr-report.json
./dist/cleanr run -config cleanr.json -format junit -output cleanr-junit.xml
```

Track trend history across builds:

```bash
./dist/cleanr run -config cleanr.json -trend-file reports/cleanr.trends.yaml -build-id "$GITHUB_SHA"
```

Write a nightly replay artifact for failing workflows:

```bash
./dist/cleanr run -config cleanr.json -trend-file reports/cleanr.trends.yaml -replay-artifact reports/cleanr.replay.json -build-id "$GITHUB_SHA"
```

Write a signed attestation for audit or release review:

```bash
export CLEANR_ATTESTATION_KEY=...
./dist/cleanr run -config cleanr.json -trend-file reports/cleanr.trends.yaml -replay-artifact reports/cleanr.replay.json -build-id "$GITHUB_SHA"
```

Summarize the retained history window:

```bash
./dist/cleanr trends -config cleanr.json
./dist/cleanr trends -config cleanr.json -format json
./dist/cleanr plugins -config cleanr.json
```

Set an overall execution timeout:

```bash
./dist/cleanr run -config cleanr.json -timeout 30s
```

CLI flags override `reporting.format` and `reporting.output` from the config file.

If `suites.drift.baseline_file` is configured and the baseline file exists, the drift suite also compares the current response against the checked-in snapshot and fails on meaningful semantic baseline regressions while still reporting lexical drift for review.

## Exit Codes

- `0`: all suites passed
- `1`: one or more suites or cases failed
- `2`: invalid configuration or runtime error

That makes the CLI suitable for CI gating without extra wrapper logic.

## Suggested First Iteration

For an initial rollout, keep the first config simple:

- start with a small set of representative scenarios
- add a few scenario assertions for output text, status code, or finish reason
- capture a baseline snapshot from a known-good run and check it into the repo
- confirm the response extraction path is correct
- tune load, chaos, and drift thresholds after a few real runs
- add `reporting.trend_file` once the suite is stable enough to track build-over-build drift deltas
- upload the replay artifact in CI once you want workflow-level nightly triage instead of only score deltas
- emit JUnit in CI so failures show up as native test results

The next reference to read is [configuration.md](configuration.md).
