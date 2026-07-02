# Local Development Setup

How to set up, run, and work with this project locally. Non-obvious dependencies, environment config, common setup issues.

- The shell's `GOROOT` points at a nonexistent `/Users/alex/apps/go`; Go commands fail with "cannot find GOROOT directory" unless you `export GOROOT=/opt/homebrew/opt/go/libexec` (Homebrew go1.26). The Makefile works around the same issue with `env -u GOROOT` in some targets.
- `golangci-lint run ./...` reports ~69 known issues (errcheck, staticcheck, unused) — it is configured in `.golangci.yml` but deliberately not wired into CI or `make lint` (which is only `go vet`), so don't assume a clean baseline.
