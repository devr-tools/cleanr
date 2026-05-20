# Development Guide

This page holds contributor-facing workflows so the repository root `README` can stay focused on installation, onboarding, and product usage.

## Common Local Workflows

```bash
make fmt
make lint
make test
make check
make build
make release VERSION=v0.1.0
```

## What `make check` Runs

`make check` is the main local developer gate. It currently:

- validates Go file layout
- enforces repository file-placement rules
- checks `gofmt`
- runs `go vet`
- runs `go test ./...`

`make deploy` is an alias for `make release`.

## Build Outputs

- `make build` writes the CLI binary to `dist/cleanr`
- `make release VERSION=vX.Y.Z` writes release artifacts to `dist/releases/<version>/`

## Related Docs

- [CI guide](ci.md)
- [Configuration](configuration.md)
- [Roadmap](roadmap.md)
- [Taskboard](taskboard.md)
