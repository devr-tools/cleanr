# Testing Patterns

Testing strategies, test infrastructure quirks, how to run/debug specific test suites, mocking conventions.

- CI and `cleanr-dev test` run `go test -race -shuffle=on ./...`. Keep the suite race-clean: a test that mutates process-global state (e.g. `http.DefaultTransport`) must NOT call `t.Parallel()` — sequential tests never overlap parallel ones, which is the only thing making the transport-swap pattern safe.
- Tests in `tests/cli` mock HTTP by swapping `http.DefaultTransport` (see `tests/cli/cli_test.go`, `snapshot_test.go`). Runner-built clients have a nil Transport and read the global at request time, so any parallel test doing real HTTP races with a swap.
- Tests that re-exec the test binary as a helper process (`tests/adapters/cli_adapter_test.go`) need generous timeouts (10s): race instrumentation makes the re-exec'd binary slow to start.
- Repo layout check (enforced by `cleanr-dev ci`): all Go test files must live under `tests/`, not next to the source packages.
- Config threshold fields with meaningful zero values (drift thresholds, `min_score`, `require_review`, trend-gate `enabled`) are pointer-typed with `*Value()` accessors that supply defaults; in tests, set them with the per-package `float64Ptr`/`boolPtr` helpers and read them through the accessors.
