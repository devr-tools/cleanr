# Development Guide

This page is for contributor workflows. Keep the repository root [README](../README.md) for installation and product usage, and use `docs/` for deeper operational material.

## Common Local Commands

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
- checks `gofmt`
- runs `go vet`
- runs `go test ./...`

`make deploy` remains an alias for `make release`.

## Local Build Outputs

- `make build` writes the CLI binary to `dist/cleanr`
- `make release VERSION=vX.Y.Z` writes release artifacts to `dist/releases/<version>/`
- `make homebrew-formula ...` writes `dist/releases/<version>/cleanr.rb`

## Release and Packaging

The detailed release pipeline now lives in [release-automation.md](release-automation.md). Use that page for:

- Release Please metadata
- branch-driven prerelease automation
- tagged GitHub Releases
- GHCR publishing
- Homebrew tap sync

## Related Docs

- [CI guide](ci.md)
- [Release automation](release-automation.md)
- [Homebrew packaging](homebrew.md)
- [Roadmap](roadmap.md)
- [Taskboard](taskboard.md)
