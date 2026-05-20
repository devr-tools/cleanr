<p align="center">
  <img src="img/cleanr.png" alt="cleanr logo" width="420">
</p>

**cleanr** is a Go-based AI testing SDK and CLI for validating AI applications in CI with adversarial, security, load, chaos, drift, token-efficiency, and cross-build trend reporting.

## Installation

### Homebrew

```bash
brew install alxxjohn/cleanr/cleanr
```

This installs from the `alxxjohn/homebrew-cleanr` tap after a tagged release publishes the formula.

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

What each command does:

- `cleanr init`: generate a starter config file
- `cleanr setup`: launch an interactive setup flow with arrow-key provider selection, optional browser-open key setup, local token storage, and starter YAML generation
- `cleanr setup agent`: launch an interactive agent setup flow that reuses the provider profile, injects an agent prompt, and generates an agent-focused YAML config
- `cleanr snapshot -config <file>`: capture or refresh baseline snapshots for drift regression checks
- `cleanr validate -config <file>`: check config shape and required fields before execution
- `cleanr run -config <file>`: execute enabled suites and emit a report
- `cleanr run -config <file> -trend-file <file> -build-id <id>`: compare the current run to prior builds and append trend history
- `cleanr trends -config <file>`: summarize the retained trend history window
- `cleanr version`: print the installed CLI version
- `cleanr mcp`: start the MCP server for agent and tool integrations

For a step-by-step walkthrough, see [docs/getting-started.md](docs/getting-started.md).

## What `cleanr` Tests

- Prompt-injection resistance and refusal boundaries
- Secret leakage, PII-like output, and unsafe tool instructions
- Load behavior with concurrent virtual users and latency or error-budget assertions
- Chaos conditions such as tight deadlines, noisy context, and duplicate turns
- Drift across repeated runs of the same scenario, with lexical and semantic similarity checks
- Trend history across builds so drift and failure deltas are comparable over time
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

Starter configs for common targets:

- [examples/openai-responses.yaml](examples/openai-responses.yaml)
- [examples/openai-chat-completions.yaml](examples/openai-chat-completions.yaml)
- [examples/anthropic-messages.yaml](examples/anthropic-messages.yaml)
- [examples/openai-responses-tuned.yaml](examples/openai-responses-tuned.yaml)

Those examples now use `reporting.trend_gates.preset: moderate` by default. You can switch that to `strict` or `exploratory`, or keep the preset and override one field such as `max_duration_increase_pct`.

Interactive setup stores provider credentials in `~/.cleanr/profile.json` with local-only file permissions. Native provider targets automatically reuse those stored credentials when the configured API key env var is not already set in the shell.

For CI or non-interactive environments, use `cleanr setup --ci` or `cleanr setup agent --ci` with flags such as `-provider`, `-model`, and `-system-prompt`. CI mode skips the TUI, skips browser launch, and writes config without persisting secrets locally.

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
