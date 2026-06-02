<p align="center">
  <img src="img/cleanr.png" alt="cleanr logo" width="420">
</p>

<p align="center">
  <a href="https://github.com/devr-tools/cleanr/actions/workflows/ci.yml"><img src="https://github.com/devr-tools/cleanr/actions/workflows/ci.yml/badge.svg" alt="CI status"></a>
  <a href="https://github.com/devr-tools/cleanr/actions/workflows/cd.yml"><img src="https://github.com/devr-tools/cleanr/actions/workflows/cd.yml/badge.svg" alt="CD status"></a>
  <a href="https://github.com/devr-tools/cleanr/releases">
    <img src="https://img.shields.io/github/v/release/devr-tools/cleanr?display_name=tag&include_prereleases" alt="release version" />
  </a>
  <a href="https://github.com/devr-tools/cleanr/actions/workflows/homebrew-validation.yml"><img src="https://github.com/devr-tools/cleanr/actions/workflows/homebrew-validation.yml/badge.svg" alt="Homebrew validation status"></a>
  <a href="CONTRIBUTING.md"><img src="https://img.shields.io/badge/contributing-guide-brightgreen.svg" alt="Contributing guide"></a>
    <a href="https://pkg.go.dev/github.com/devr-tools/cleanr/pkg/cleanr">
    <img src="https://pkg.go.dev/badge/github.com/devr-tools/cleanr/pkg/cleanr.svg" alt="Go Reference" />
  </a>
  <a href="https://goreportcard.com/report/github.com/devr-tools/cleanr">
    <img src="https://goreportcard.com/badge/github.com/devr-tools/cleanr" alt="Go Report Card" />
  </a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="Apache 2.0 license"></a>
    <a href="https://www.linkedin.com/in/alxjohn">
    <img src="https://img.shields.io/badge/LinkedIn-alxjohn-blue?logo=linkedin" alt="LinkedIn" />
  </a>
</p>

**cleanr** is a Go-based AI testing CLI and SDK for validating AI applications in CI. Use it to generate configs, run suites against providers or HTTP targets, and emit machine-readable reports for release gates.

## Install

Recommended:

```bash
go install github.com/devr-tools/cleanr/cmd/cleanr@latest
cleanr version
```

Other install paths:

- GitHub Releases: tagged binaries for direct download
- Homebrew: `brew install devr-tools/tap/cleanr`
- Source build: `make build`

Install details and packaging notes live in [docs/getting-started.md](docs/getting-started.md) and [docs/homebrew.md](docs/homebrew.md).

## Quickstart

Generate a config, validate it, and run your first suite:

```bash
cleanr setup --ci -provider openai -model gpt-4.1-mini -output cleanr.yaml
cleanr validate -config cleanr.yaml
cleanr run -config cleanr.yaml
```

If you prefer an interactive local flow:

```bash
cleanr setup
cleanr snapshot -config cleanr.yaml
```

For staged CI configs:

```bash
cleanr setup --ci -provider openai -model gpt-4.1-mini -profile pr -output cleanr-pr.yaml
cleanr setup --ci -provider openai -model gpt-4.1-mini -profile main -output cleanr-main.yaml
cleanr setup --ci -provider openai -model gpt-4.1-mini -profile release -output cleanr-release.yaml
```

For scenario generation and dataset import flows, start with [docs/getting-started.md](docs/getting-started.md).

Review generated or replay-backed scenarios before merging them into your checked-in config:

```bash
cleanr dataset review -input generated/cleanr.dataset.yaml -base-config cleanr.yaml -output reviewed/cleanr.reviewed.yaml
cleanr dataset review -input reviewed/cleanr.dataset.yaml -profile pr -approve happy-path -promote-regression happy-path -merge-output .cleanr/pr.reviewed.yaml
```

## CI Example

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

More CI, Docker, SDK, and release patterns are in [docs/ci.md](docs/ci.md), [docs/docker.md](docs/docker.md), [docs/sdk.md](docs/sdk.md), and [docs/release-automation.md](docs/release-automation.md).

## Docs

- [Getting started](docs/getting-started.md)
- [SDK guide](docs/sdk.md)
- [Docker guide](docs/docker.md)
- [Configuration](docs/configuration.md)
- [CI guide](docs/ci.md)
- [Buildkite guide](docs/buildkite.md)
- [Best practices](docs/best-practices.md)
- [Release automation](docs/release-automation.md)
- [Homebrew packaging](docs/homebrew.md)
- [MCP and MCPO](docs/mcp.md)
- [Developer guide](docs/development.md)
- [Docs index](docs/README.md)

## Examples

- [OpenAI Responses](examples/openai-responses.yaml)
- [OpenAI Chat Completions](examples/openai-chat-completions.yaml)
- [Anthropic Messages](examples/anthropic-messages.yaml)
- [Containerized Assistant](examples/containerized-assistant/README.md)
- [Best Practice Profiles](examples/best-practices/cleanr-pr.yaml)
- [Stateful Support Agent](examples/stateful-support-agent/README.md)


## updates
