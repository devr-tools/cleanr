<p align="center">
  <img src="img/cleanr.png" alt="cleanr logo" width="420">
</p>

<p align="center">
  <a href="https://github.com/devr-tools/cleanr/actions/workflows/ci.yml"><img src="https://github.com/devr-tools/cleanr/actions/workflows/ci.yml/badge.svg" alt="CI status"></a>
  <a href="https://github.com/devr-tools/cleanr/actions/workflows/cd.yml"><img src="https://github.com/devr-tools/cleanr/actions/workflows/cd.yml/badge.svg" alt="CD status"></a>
  <a href="https://github.com/devr-tools/cleanr/actions/workflows/release.yml"><img src="https://github.com/devr-tools/cleanr/actions/workflows/release.yml/badge.svg" alt="Release status"></a>
  <a href="https://github.com/devr-tools/cleanr/actions/workflows/homebrew-validation.yml"><img src="https://github.com/devr-tools/cleanr/actions/workflows/homebrew-validation.yml/badge.svg" alt="Homebrew validation status"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="Apache 2.0 license"></a>
</p>

**cleanr** is a Go-based AI testing CLI and SDK for validating AI applications in CI. It is built for adversarial, security, load, chaos, drift, token-efficiency, and workflow-policy checks, with machine-readable outputs for release gates.

## Quick Install

Install the CLI with Go:

```bash
go install github.com/devr-tools/cleanr/cmd/cleanr@latest
cleanr version
```

Install from GitHub Releases:

```bash
VERSION="$(curl -fsSL https://api.github.com/repos/devr-tools/cleanr/releases/latest | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p' | head -n 1)"
curl -fsSLo cleanr.tar.gz "https://github.com/devr-tools/cleanr/releases/download/${VERSION}/cleanr_${VERSION}_darwin_arm64.tar.gz"
tar -xzf cleanr.tar.gz
sudo install -m 0755 ./cleanr /usr/local/bin/cleanr
cleanr version
```

Build from source:

```bash
make build
./dist/cleanr version
sudo install -m 0755 ./dist/cleanr /usr/local/bin/cleanr
```

Homebrew:

```bash
brew tap devr-tools/tap
brew install cleanr
cleanr version
```

```bash
brew install devr-tools/tap/cleanr
cleanr version
```

`cleanr` is not in `homebrew/core` yet. For tap and formula details, see [docs/homebrew.md](docs/homebrew.md).

## Quickstart

### CLI

```bash
make build
./dist/cleanr setup --ci -provider openai -model gpt-4.1-mini -output cleanr.yaml
./dist/cleanr validate -config cleanr.yaml
./dist/cleanr run -config cleanr.yaml
```

For an interactive local setup flow, use:

```bash
./dist/cleanr setup
./dist/cleanr snapshot -config cleanr.yaml
./dist/cleanr trends -config cleanr.yaml
```

### SDK

Use the root module path for embedding:

```go
package main

import (
	"context"
	"fmt"

	cleanr "github.com/devr-tools/cleanr"
)

func main() {
	cfg, err := cleanr.LoadConfigFile("cleanr.yaml")
	if err != nil {
		panic(err)
	}

	report := cleanr.NewHTTPRunner(cfg).Run(context.Background())
	fmt.Print(cleanr.TextReport(report))
}
```

### Docker Pipeline

Use the published GitHub Container Registry image:

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

For fuller SDK, Docker, CI, and release patterns, see [docs/sdk.md](docs/sdk.md), [docs/docker.md](docs/docker.md), [docs/ci.md](docs/ci.md), and [docs/release-automation.md](docs/release-automation.md).

## Uninstall

Remove the installed binary:

```bash
sudo rm -f /usr/local/bin/cleanr
```

Remove the local profile and cached credentials if you used `cleanr setup`:

```bash
rm -rf ~/.cleanr
```

## Docs

- [Getting started](docs/getting-started.md)
- [SDK guide](docs/sdk.md)
- [Docker guide](docs/docker.md)
- [Configuration](docs/configuration.md)
- [CI guide](docs/ci.md)
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
- [Best Practice Profiles](examples/best-practices/cleanr-pr.yaml)
- [Stateful Support Agent](examples/stateful-support-agent/README.md)

## Exit Codes

- `0`: all suites passed
- `1`: one or more tests failed
- `2`: invalid configuration or runtime error
