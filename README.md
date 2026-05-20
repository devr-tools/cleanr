<p align="center">
  <img src="img/cleanr.png" alt="cleanr logo" width="420">
</p>

`cleanr` is a Go-based AI testing SDK and CLI for validating AI applications in CI with adversarial, security, load, chaos, drift, and token-efficiency suites.

## Overview

`cleanr` is designed for teams that need repeatable release checks for AI systems, not just ad hoc prompt testing. The current codebase keeps an HTTP-first target model for broad compatibility, and now includes native provider adapters for OpenAI and Anthropic.

Core capabilities:

- Prompt-injection testing with refusal and boundary checks
- Security scanning for secret leakage, PII-like output, and unsafe tool instructions
- Load testing with concurrent virtual users and latency or error-budget assertions
- Chaos testing with degraded request conditions such as tight deadlines and noisy context
- Drift testing for response stability across repeated runs
- Token optimization testing for prompt and completion budgets, duplication, and waste reduction opportunities
- CI-friendly reporting in text, JSON, and JUnit formats

## Project Status

`cleanr` is in a foundation-stage release. The core runner, config model, HTTP/OpenAI/Anthropic adapters, report formats, CI workflows, and release packaging are in place. The next major phase is focused on stronger assertion primitives, more provider coverage, and richer documentation.

## Quick Start

```bash
make build
./dist/cleanr init
./dist/cleanr validate -config cleanr.json
./dist/cleanr run -config cleanr.json
```

For a fuller setup walkthrough, see [docs/getting-started.md](docs/getting-started.md).

## Documentation

- [Docs index](docs/README.md): documentation map and recommended reading order
- [Getting started](docs/getting-started.md): first run, validation, and report generation
- [Configuration](docs/configuration.md): config schema, request templating, suites, and reporting
- [CI guide](docs/ci.md): GitHub Actions, release flow, and pipeline integration
- [Roadmap](docs/roadmap.md): product direction, workstreams, and milestone sequencing
- [Taskboard](docs/taskboard.md): execution-focused status of current and upcoming work

As the project grows, `docs/` should remain the home for guides, examples, and operational reference material. The main README should stay focused on orientation and onboarding.


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

The repository includes CI and release workflows under `.github/workflows/`. For the current pipeline behavior and integration patterns, see [docs/ci.md](docs/ci.md).

## Exit Codes

- `0`: all suites passed
- `1`: one or more tests failed
- `2`: invalid configuration or runtime error

That makes `cleanr` straightforward to wire into GitHub Actions, Buildkite, CircleCI, or other pipeline systems.
