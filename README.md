<p align="center">
  <img src="img/cleanr.png" alt="cleanr logo" width="420">
</p>

**cleanr** is a Go-based AI testing SDK and CLI for validating AI applications in CI with adversarial, security, load, chaos, drift, token-efficiency, and cross-build trend reporting.

Phase 1 through Phase 4 are complete. Phase 5, external eval and data integrations, is now in place as an optional companion layer around the local-first release gate.

## Installation

### Homebrew

`cleanr` is not in `homebrew/core` yet. This repository now generates a source-based formula intended for a future `homebrew/core` submission, but users cannot install it with `brew install cleanr` until that PR is merged.

Until then, install from GitHub Releases or build from source. For submission prep details, see [docs/homebrew.md](docs/homebrew.md).

### Install with `curl`

```bash
VERSION="$(curl -fsSL https://api.github.com/repos/alxxjohn/cleanr/releases/latest | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p' | head -n 1)"
curl -fsSLo cleanr.tar.gz "https://github.com/alxxjohn/cleanr/releases/download/${VERSION}/cleanr_${VERSION}_darwin_arm64.tar.gz"
tar -xzf cleanr.tar.gz
sudo install -m 0755 ./cleanr /usr/local/bin/cleanr
cleanr version
```

Replace `darwin_arm64` with the artifact that matches your platform:

- `darwin_amd64`
- `darwin_arm64`
- `linux_amd64`
- `linux_arm64`

### Install from GitHub Releases

Download a packaged release artifact from:

- `https://github.com/alxxjohn/cleanr/releases`

Then unpack the file named `cleanr_<version>_<os>_<arch>.tar.gz` and install it:

```bash
tar -xzf cleanr_<version>_<os>_<arch>.tar.gz
sudo install -m 0755 ./cleanr /usr/local/bin/cleanr
cleanr version
```

### Build from source

```bash
make build
./dist/cleanr version
sudo install -m 0755 ./dist/cleanr /usr/local/bin/cleanr
cleanr version
```

## Quickstart

The core `cleanr` commands are:

```bash
cleanr init
cleanr setup
cleanr setup agent
cleanr snapshot -config cleanr.yaml
cleanr validate -config cleanr.yaml
cleanr run -config cleanr.yaml
cleanr trends -config cleanr.yaml
cleanr dataset export -config cleanr.yaml
cleanr dataset import -input cleanr.dataset.yaml
cleanr version
cleanr mcp
```

Typical first run:

```bash
cleanr setup
cleanr snapshot -config cleanr.yaml
cleanr validate -config cleanr.yaml
cleanr run -config cleanr.yaml
cleanr trends -config cleanr.yaml
```

Browser-assisted local setup:

```bash
cleanr setup --browser
cleanr setup agent --browser
```

Non-interactive CI setup:

```bash
cleanr setup --ci -provider openai -model gpt-4.1-mini -output cleanr.yaml
cleanr setup agent --ci -provider openai -model gpt-4.1-mini -name support-agent -system-prompt "You are a safe support agent." -user-prompt "Reset the password and confirm the email."
```

What each command does:

- `cleanr init`: generate a starter config file
- `cleanr setup`: launch an interactive setup flow with arrow-key provider selection, optional browser-open key setup, local token storage, and starter YAML generation
- `cleanr setup agent`: launch an interactive agent setup flow that reuses the provider profile, injects an agent prompt, and generates an agent-focused YAML config
- `cleanr snapshot -config <file>`: capture or refresh baseline snapshots for drift regression checks
- `cleanr validate -config <file>`: check config shape and required fields before execution
- `cleanr run -config <file>`: execute enabled suites and emit a report
- `cleanr run -config <file> -trend-file <file> -build-id <id>`: compare the current run to prior builds and append trend history
- `cleanr trends -config <file>`: summarize the retained trend history window
- `cleanr dataset export -config <file>`: convert replayed failures or all configured scenarios into a reusable scenario dataset
- `cleanr dataset import -input <file>`: merge a reviewed scenario dataset back into a runnable `cleanr` config
- `cleanr version`: print the installed CLI version
- `cleanr mcp`: start the MCP server for agent and tool integrations

For a step-by-step walkthrough, see [docs/getting-started.md](docs/getting-started.md).

## What `cleanr` Tests

- Prompt-injection resistance and refusal boundaries
- Secret leakage, PII-like output, and unsafe tool instructions
- Load behavior with concurrent virtual users and latency or error-budget assertions
- Chaos conditions such as tight deadlines, noisy context, and duplicate turns
- Drift across repeated runs of the same scenario, with lexical and semantic similarity checks
- File-system shadow-state verification for observed writes inside approved locations
- Provider-neutral workflow evidence for HTTP targets that emit normalized `trace` payloads
- Release-policy enforcement for allowed tools, read-only tools, approval-gated actions, trust boundaries, approved sinks, and expected state changes
- Exact expected file-mutation checks for create, modify, and delete behavior in controlled workspaces
- Exact expected state-change checks for provider-neutral workflow surfaces such as tickets, emails, and other action traces
- Provenance-aware context attacks that originate from untrusted retrieved, tool, memory, or approval content
- Approval-bypass and sink-restriction checks for tool-calling agents
- Trend history across builds so drift and failure deltas are comparable over time
- Optional best-effort external result publishing, including native Braintrust, Langfuse, and PostHog publishing, remote trend comparisons, and PR or release summaries
- Dataset promotion flows for turning reviewed failures into reusable regression scenarios
- Token budgets, duplication, and output-efficiency regressions
- CI-friendly reporting in text, JSON, and JUnit formats
- MCP server mode for agent and tool-based integrations

## Performance And Benchmark Metrics

`cleanr` is built to gate concrete operational metrics instead of vague eval scores. The current suite model lets teams enforce:

| Area | Metrics you can gate |
| --- | --- |
| Load | `virtual_users`, `requests_per_user`, `max_error_rate_pct`, `p95_latency_ms` |
| Scenario assertions | `status_code`, `latency_ms`, `finish_reason`, tool-call checks |
| Drift | `iterations`, `max_normalized_drift`, `max_semantic_drift`, `min_consistency_score`, `min_semantic_consistency_score` |
| Trend reporting | `reporting.trend_file`, `reporting.trend_limit`, `reporting.build_id`, `reporting.trend_gates.*` |
| Token efficiency | `max_input_tokens`, `max_output_tokens`, `max_total_tokens`, output/input ratio, duplication ratios |

The example configs currently ship with starter thresholds such as `8` virtual users, `8` requests per user, `5%` max error rate, and `2500ms` p95 latency so teams can tune from a realistic baseline instead of starting from zero. See [docs/configuration.md](docs/configuration.md) and the [`examples/`](examples) directory.

## Production Usage Examples

Typical production-facing ways teams use `cleanr`:

- Gate model or prompt changes in CI before merging a release.
- Compare provider or model upgrades before switching traffic.
- Run concurrency and latency checks against live-like staging endpoints.
- Catch drift and token-cost regressions in nightly or pre-release runs.
- Validate tool-using assistants for prompt injection, unsafe instructions, and boundary failures.
- Verify that local file-writing agents changed only approved paths in a controlled workspace.
- Assert the exact file mutations a workflow was expected to make, including content checks for created or modified files.
- Gate workflow actions with declarative release rules such as read-only SQL, draft-not-send email, trust-boundary tool bans, and approval-required actions.
- Verify provider-neutral state changes such as ticket updates or email drafts even when the underlying system is not file-based.
- Test whether untrusted retrieved or tool-provided context can cross into secret disclosure, approval-bypassed actions, or unapproved sink tools.

Starter configs for common targets:

- [examples/openai-responses.yaml](examples/openai-responses.yaml)
- [examples/openai-chat-completions.yaml](examples/openai-chat-completions.yaml)
- [examples/anthropic-messages.yaml](examples/anthropic-messages.yaml)
- [examples/openai-responses-tuned.yaml](examples/openai-responses-tuned.yaml)
- [examples/stateful-support-agent/cleanr.yaml](examples/stateful-support-agent/cleanr.yaml)

End-to-end stateful sample project:

- [examples/stateful-support-agent/README.md](examples/stateful-support-agent/README.md)

Those examples now use `reporting.trend_gates.preset: moderate` by default. You can switch that to `strict` or `exploratory`, or keep the preset and override one field such as `max_duration_increase_pct`.

Interactive setup stores provider credentials in `~/.cleanr/profile.json` with local-only file permissions. Native provider targets automatically reuse those stored credentials when the configured API key env var is not already set in the shell.

For CI or non-interactive environments, use `cleanr setup --ci` or `cleanr setup agent --ci` with flags such as `-provider`, `-model`, and `-system-prompt`. CI mode skips the TUI, skips browser launch, and writes config without persisting secrets locally.

Common setup commands:

```bash
# Interactive TUI with arrow-key provider selection
cleanr setup

# Interactive TUI plus automatic browser open for provider key setup
cleanr setup --browser

# Interactive agent config generation
cleanr setup agent

# CI-safe config generation with no prompts or local credential storage
cleanr setup --ci -provider anthropic -model claude-sonnet-4-20250514 -output cleanr.yaml
```

## Documentation

- [Getting started](docs/getting-started.md): first run, validation, and report generation
- [Configuration](docs/configuration.md): config schema, suites, assertions, and reporting
- [CI guide](docs/ci.md): pipeline integration and release packaging
- [MCP and MCPO](docs/mcp.md): expose `cleanr` as agent-callable tools
- [Developer guide](docs/development.md): local contributor workflows and repository checks
- [Docs index](docs/README.md): documentation map and reading order

## Exit Codes

- `0`: all suites passed
- `1`: one or more tests failed
- `2`: invalid configuration or runtime error
