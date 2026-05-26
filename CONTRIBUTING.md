# Contributing to cleanr

Thanks for contributing to `cleanr`.

This file covers the fast path for contributors. Keep the repository root [README](README.md) focused on installation and usage, and use [docs/development.md](docs/development.md) for deeper workflow detail.

## Development Setup

Install Go, then verify the local toolchain works:

```bash
make build
make test
```

The built CLI is written to `dist/cleanr`.

## Common Commands

Run these before opening or updating a pull request:

```bash
make fmt
make lint
make test
make check
make ci
```

`make check` is the main local developer gate. It validates formatting, runs `go vet`, and runs `go test ./...`.

`make ci` is the closest local match to GitHub Actions and includes additional maintainability, security, and packaging checks.

## Pull Requests

- keep changes scoped and update docs when behavior changes
- add or update tests with code changes
- run `make check` before pushing
- run `make ci` for CI-parity validation when the change is substantial or release-adjacent

If your change touches install, release, or packaging behavior, also update [README.md](README.md) and any relevant files in [docs/](docs/README.md).

## Release and Packaging

Use [docs/release-automation.md](docs/release-automation.md) for Release Please, tagged releases, GHCR publishing, and Homebrew sync.

## More Detail

- [Development guide](docs/development.md)
- [CI guide](docs/ci.md)
- [Docs index](docs/README.md)
