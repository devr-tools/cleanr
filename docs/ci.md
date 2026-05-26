# CI Guide

`cleanr` is CI-friendly because its exit codes are stable and its outputs are machine-readable.

## Exit Code Contract

- `0`: all suites passed
- `1`: one or more suites or cases failed
- `2`: invalid configuration or runtime error

That split lets pipelines distinguish product regressions from setup or infrastructure failures.

## Pick an Integration Shape

### Use the released container

Best when your CI supports container jobs and you want the exact tagged CLI:

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

### Use `go install`

Best when you want a simple host-runner install path:

```yaml
jobs:
  cleanr:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.20"
      - run: go install github.com/devr-tools/cleanr/cmd/cleanr@latest
      - run: cleanr validate -config cleanr.yaml
      - run: cleanr run -config cleanr.yaml -format junit -output cleanr-junit.xml
```

### Build from source

Best when you need to pin the exact repo state under test:

```yaml
jobs:
  cleanr:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make build
      - run: ./dist/cleanr validate -config cleanr.yaml
      - run: ./dist/cleanr run -config cleanr.yaml -format junit -output cleanr-junit.xml
```

## Recommended Outputs

For CI, the usual combination is:

- main report as `junit` or `json`
- trend history via `-trend-file`
- replay bundle via `-replay-artifact`
- signed attestation via `CLEANR_ATTESTATION_KEY`

Example:

```bash
cleanr run \
  -config cleanr.yaml \
  -format junit \
  -output cleanr-junit.xml \
  -trend-file reports/cleanr.trends.yaml \
  -replay-artifact reports/cleanr.replay.json \
  -build-id "$GITHUB_SHA"
```

## Artifact Retention

Persist these files between runs when you want trend and replay workflows to be useful:

- `cleanr-junit.xml`
- `reports/cleanr.trends.yaml`
- `reports/cleanr.replay.json`
- `reports/cleanr.attestation.json`

## Repository Workflows

This repository already ships with:

- `.github/workflows/ci.yml`: PR validation and quality gates
- `.github/workflows/cd.yml`: branch-driven prerelease and Release Please orchestration
- `.github/workflows/release.yml`: tag-driven publishing for binaries, GHCR, and Homebrew sync
- `.github/workflows/homebrew-validation.yml`: PR-time formula install and `brew test`

Release and branch-publishing details live in [release-automation.md](release-automation.md).

## Related Docs

- [Docker guide](docker.md)
- [Configuration](configuration.md)
- [Release automation](release-automation.md)
