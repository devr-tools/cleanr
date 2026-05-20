# Getting Started

This guide walks through the shortest path to running `cleanr` against an HTTP-based AI endpoint.

## Prerequisites

- Go 1.20 or newer
- A reachable HTTP endpoint for the AI application you want to test
- A request and response shape you can describe with `cleanr` config fields

## Build the CLI

From the repository root:

```bash
make build
./dist/cleanr version
```

You can also run the CLI directly from source with `go run ./cmd/cleanr ...`, but using the built binary keeps command examples consistent with CI and release usage.

## Generate a Starter Config

Write the default JSON config:

```bash
./dist/cleanr init
```

Write a YAML version instead:

```bash
./dist/cleanr init -output cleanr.yaml
```

The generated file includes:

- an HTTP target definition
- example scenarios
- all currently supported suites enabled with starter thresholds
- text reporting as the default output mode

## Point It at Your Endpoint

At minimum, update these values in the generated config:

- `target.url`: the full endpoint URL
- `target.prompt_field`: the request field that should receive the end-user prompt
- `target.response_field`: the JSON path containing the model response text

If your API accepts a system prompt, also set `target.system_field`.

If your endpoint expects a larger payload shape, update `target.request_template` to match it. `cleanr` will inject the prompt and system fields into that template at runtime.

## Validate the Config

```bash
./dist/cleanr validate -config cleanr.json
./dist/cleanr validate -config cleanr.yaml
```

If you omit `-config`, `cleanr` looks for:

- `cleanr.json`
- `cleanr.yaml`
- `cleanr.yml`

Validation failures return exit code `2` and include field-level guidance for fixing the config.

## Run the Suites

Run with the default text report:

```bash
./dist/cleanr run -config cleanr.json
```

Write JSON or JUnit output:

```bash
./dist/cleanr run -config cleanr.json -format json -output cleanr-report.json
./dist/cleanr run -config cleanr.json -format junit -output cleanr-junit.xml
```

Set an overall execution timeout:

```bash
./dist/cleanr run -config cleanr.json -timeout 30s
```

CLI flags override `reporting.format` and `reporting.output` from the config file.

## Exit Codes

- `0`: all suites passed
- `1`: one or more suites or cases failed
- `2`: invalid configuration or runtime error

That makes the CLI suitable for CI gating without extra wrapper logic.

## Suggested First Iteration

For an initial rollout, keep the first config simple:

- start with a small set of representative scenarios
- confirm the response extraction path is correct
- tune load, chaos, and drift thresholds after a few real runs
- emit JUnit in CI so failures show up as native test results

The next reference to read is [configuration.md](configuration.md).
