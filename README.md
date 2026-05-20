# cleanr

<p align="center">
  <img src="img/cleanr.png" alt="cleanr logo" width="420">
</p>

`cleanr` is a Go-based AI testing SDK and CLI for validating AI applications in CI with adversarial, security, load, chaos, drift, and token-efficiency suites.

## Overview

`cleanr` is designed for teams that need repeatable release checks for AI systems, not just ad hoc prompt testing. The current codebase focuses on an HTTP-first target model so it can exercise chat, completion, agent, and tool-calling APIs without requiring a provider-specific integration layer.

Core capabilities:

- Prompt-injection testing with refusal and boundary checks
- Security scanning for secret leakage, PII-like output, and unsafe tool instructions
- Load testing with concurrent virtual users and latency or error-budget assertions
- Chaos testing with degraded request conditions such as tight deadlines and noisy context
- Drift testing for response stability across repeated runs
- Token optimization testing for prompt and completion budgets, duplication, and waste reduction opportunities
- CI-friendly reporting in text, JSON, and JUnit formats

## Project Status

`cleanr` is in a foundation-stage release. The core runner, config model, HTTP adapter, report formats, CI workflows, and release packaging are in place. The next major phase is focused on provider adapters, stronger assertion primitives, and richer documentation.

## Quick Start

Build the CLI:

```bash
make build
./dist/cleanr version
```

Generate an example config:

```bash
./dist/cleanr init
```

Use YAML if preferred:

```bash
./dist/cleanr init -output cleanr.yaml
```

Validate the config:

```bash
./dist/cleanr validate -config cleanr.json
./dist/cleanr validate -config cleanr.yaml
```

Run the suites:

```bash
./dist/cleanr run -config cleanr.json
```

Emit JUnit output for CI:

```bash
./dist/cleanr run -config cleanr.json -format junit -output cleanr-junit.xml
```

You can also run the CLI directly from source:

```bash
go run ./cmd/cleanr run -config cleanr.json
```

## Configuration

`cleanr` accepts both JSON and YAML configuration files. Format is selected by extension: `.json`, `.yaml`, or `.yml`.

The primary config sections are:

- `target`: endpoint, headers, timeout, request field mapping, and response extraction path
- `scenarios`: representative prompts and policy boundary cases
- `suites.prompt_injection`: adversarial refusal markers
- `suites.security`: leak patterns, dangerous tool indicators, and PII thresholds
- `suites.load`: concurrency settings and SLO thresholds
- `suites.chaos`: enabled fault injections and resilience thresholds
- `suites.drift`: repeated-run stability thresholds
- `suites.token_optimization`: token budgets, duplication limits, and optimization hints
- `reporting`: output format defaults

Run `init` to generate a working starter file.

## Documentation

- [Roadmap](docs/roadmap.md): product direction, workstreams, and milestone sequencing
- [Taskboard](docs/taskboard.md): execution-focused status of current and upcoming work

As the project grows, `docs/` should remain the home for guides, examples, and operational reference material. The main README should stay focused on orientation and onboarding.

## Repository Layout

```text
.
├── .github/workflows/   CI and release automation
├── cleanr/              Public SDK types, config, runner, engines, reporters, HTTP target adapter
├── cmd/cleanr/          CLI entrypoint
├── cmd/cleanr-dev/      Developer tooling entrypoint used by Make targets
├── docs/                Roadmap and execution docs
├── img/                 README and documentation assets
├── internal/cli/        Command parsing and CLI behavior
├── internal/devtools/   Format, lint, test, build, and release helpers
├── tests/               CLI and runner tests
├── Makefile             Common developer workflows
└── README.md            Project entrypoint
```

## Development

Common local workflows:

```bash
make fmt
make lint
make test
make check
make build
make release VERSION=v0.1.0
```

`make check` runs the main developer gate:

- validates Go file layout
- enforces repository file-placement rules
- checks `gofmt`
- runs `go vet`
- runs `go test ./...`

`make deploy` is an alias for `make release`.

## CI and Releases

The repository includes:

- `.github/workflows/ci.yml` for pull request and `main` branch verification
- `.github/workflows/release.yml` for tag-driven packaging and release publishing

Release artifacts are currently built for:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`

Each release generates compressed archives plus a `SHA256SUMS` file.

## Exit Codes

- `0`: all suites passed
- `1`: one or more tests failed
- `2`: invalid configuration or runtime error

That makes `cleanr` straightforward to wire into GitHub Actions, Buildkite, CircleCI, or other pipeline systems.

## Roadmap

Near-term priorities are:

- native provider adapters beginning with OpenAI
- provider-neutral response normalization
- reusable assertions and richer config ergonomics
- snapshot and semantic drift capabilities
- stronger docs, examples, and CI integration

Longer-term direction is tracked in [docs/roadmap.md](docs/roadmap.md).
