# SDK Guide

`cleanr` exposes a public Go module path for embedding:

```bash
go get github.com/devr-tools/cleanr@latest
```

Import the root package:

```go
import cleanr "github.com/devr-tools/cleanr"
```

## Minimal Example

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

## What the Facade Exposes

The root package is a stable facade over the internal `cleanr/` package tree. It exposes the main config, runner, target, reporting, trend, replay, attestation, and integration entry points without forcing downstream code to import subpackages directly.

Use the facade when you want:

- config loading and validation
- default runner construction for HTTP, OpenAI, or Anthropic targets
- text, JSON, JUnit, or SARIF report generation
- trend analysis, replay artifacts, and attestation helpers

## When to Use the CLI Instead

Prefer the CLI when:

- you only need CI execution and machine-readable reports
- you want setup flows or snapshot generation
- you do not need to embed `cleanr` inside another Go service

The CLI install path is:

```bash
go install github.com/devr-tools/cleanr/cmd/cleanr@latest
```

## Related Docs

- [Getting started](getting-started.md)
- [Configuration](configuration.md)
- [CI guide](ci.md)
