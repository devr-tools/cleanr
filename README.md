<p align="center">
  <img src="img/cleanr.png" alt="cleanr logo" width="420">
</p>

**cleanr** is a Go-based AI testing SDK and CLI for validating AI applications in CI with adversarial, security, load, chaos, drift, and token-efficiency suites.

## Installation

### Homebrew

```bash
brew install cleanr
```

Homebrew is the intended macOS installation path, but this repository does not currently ship a published tap or formula. Until that exists, use the `curl` or GitHub Releases methods below.

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
cleanr snapshot -config cleanr.json
cleanr validate -config cleanr.json
cleanr run -config cleanr.json
cleanr version
cleanr mcp
```

Typical first run:

```bash
cleanr init
cleanr snapshot -config cleanr.json
cleanr validate -config cleanr.json
cleanr run -config cleanr.json
```

What each command does:

- `cleanr init`: generate a starter config file
- `cleanr snapshot -config <file>`: capture or refresh baseline snapshots for drift regression checks
- `cleanr validate -config <file>`: check config shape and required fields before execution
- `cleanr run -config <file>`: execute enabled suites and emit a report
- `cleanr version`: print the installed CLI version
- `cleanr mcp`: start the MCP server for agent and tool integrations

For a step-by-step walkthrough, see [docs/getting-started.md](docs/getting-started.md).

## What `cleanr` Tests

- Prompt-injection resistance and refusal boundaries
- Secret leakage, PII-like output, and unsafe tool instructions
- Load behavior with concurrent virtual users and latency or error-budget assertions
- Chaos conditions such as tight deadlines, noisy context, and duplicate turns
- Drift across repeated runs of the same scenario
- Token budgets, duplication, and output-efficiency regressions
- CI-friendly reporting in text, JSON, and JUnit formats
- MCP server mode for agent and tool-based integrations

## Performance And Benchmark Metrics

`cleanr` is built to gate concrete operational metrics instead of vague eval scores. The current suite model lets teams enforce:

| Area | Metrics you can gate |
| --- | --- |
| Load | `virtual_users`, `requests_per_user`, `max_error_rate_pct`, `p95_latency_ms` |
| Scenario assertions | `status_code`, `latency_ms`, `finish_reason`, tool-call checks |
| Drift | `iterations`, `max_normalized_drift`, `min_consistency_score` |
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
