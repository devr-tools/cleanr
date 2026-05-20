# cleanr

<p align="center">
  <img src="img/cleanr.png" alt="cleanr logo" width="420">
</p>

`cleanr` is a Go-based AI testing SDK and CLI for exercising AI applications in CI with adversarial, security, load, chaos, and drift test suites.

## What it does

- Prompt-injection testing with refusal and boundary checks
- Security scanning for secret leakage, PII-like output, and unsafe tool instructions
- Load testing with concurrent virtual users and latency/error SLO assertions
- Chaos testing with degraded request conditions such as tight deadlines and noisy context
- Drift testing for response stability across repeated runs
- Token optimization testing for prompt/completion budgets, duplication, and waste reduction opportunities
- CI-friendly summaries with text, JSON, and JUnit output

## Architecture

`cleanr` is structured as:

- `cleanr/`: public SDK types, runner, engines, HTTP target adapter, reporters
- `internal/cli/`: CLI entrypoint logic
- `cmd/cleanr/`: main package

The initial target adapter is HTTP-first so teams can point `cleanr` at chat, completion, agent, or tool-calling APIs. The SDK surface is intentionally simple: implement the `Target` interface if you want to test non-HTTP runtimes in-process.

## Developer workflow

Local development is wired through `make` and the Go-based `cleanr-dev` helper:

```bash
make fmt
make lint
make test
make check
make build
make release VERSION=v0.1.0
```

`make check` runs the full developer gate:

- validates Go file layout
- enforces that all `_test.go` files live under `tests/`
- checks `gofmt`
- runs `go vet`
- runs `go test ./...`

`make deploy` is an alias for `make release`. It packages local release artifacts into `dist/releases/<version>/`.

## Quick start

Initialize a config:

```bash
go run ./cmd/cleanr init
```

Validate it:

```bash
go run ./cmd/cleanr validate -config cleanr.json
```

Run the test suites:

```bash
go run ./cmd/cleanr run -config cleanr.json
```

Emit JUnit for CI:

```bash
go run ./cmd/cleanr run -config cleanr.json -format junit -output cleanr-junit.xml
```

## Config model

The config file is JSON to keep the runtime dependency-free and deterministic in CI. Key sections:

- `target`: endpoint, headers, timeout, request field mapping, and response extraction path
- `scenarios`: representative prompts and policy boundary cases
- `suites.prompt_injection`: adversarial refusal markers
- `suites.security`: custom leak patterns, dangerous tool indicators, PII threshold
- `suites.load`: concurrency and SLO thresholds
- `suites.chaos`: enabled fault injections and resilience threshold
- `suites.drift`: repeated-run stability thresholds
- `suites.token_optimization`: prompt/output token budgets, duplication limits, and optimization hints
- `reporting`: output format defaults

Generate `cleanr.json` with `init` to see a working example.

## CI behavior

- Exit code `0`: all suites passed
- Exit code `1`: one or more tests failed
- Exit code `2`: invalid configuration or runtime error

That makes it easy to drop into GitHub Actions, Buildkite, CircleCI, or any other pipeline.

## CI and release automation

The repository now includes:

- `.github/workflows/ci.yml` for pull request and `main` branch QA
- `.github/workflows/release.yml` for tag-driven release packaging and GitHub release publishing

Release artifacts are built for:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`

Each release run generates compressed archives plus a `SHA256SUMS` file.

## Next extensions

- Native OpenAI/Anthropic/Gemini target adapters with richer eval metadata
- Provider-native token usage ingestion instead of heuristic estimation
- Tool-call tracing assertions and policy graphs
- Distributed load execution and percentile histograms
- Snapshot baselines and semantic drift scoring
- Signed attestations for compliance evidence
