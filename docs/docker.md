# Docker Guide

`cleanr` now publishes a container image to GitHub Container Registry:

```text
ghcr.io/devr-tools/cleanr:<tag>
```

Release tags also publish a multi-arch image manifest for Linux `amd64` and `arm64`.

## Pull and Inspect

```bash
TAG=v0.1.0
docker pull ghcr.io/devr-tools/cleanr:${TAG}
docker run --rm ghcr.io/devr-tools/cleanr:${TAG} version
```

## Run Against a Checked-In Config

```bash
TAG=v0.1.0
docker run --rm \
  -v "$PWD:/workspace" \
  -w /workspace \
  ghcr.io/devr-tools/cleanr:${TAG} \
  run -config cleanr.yaml
```

If your config needs provider credentials, pass them through with `-e`, for example:

```bash
TAG=v0.1.0
docker run --rm \
  -v "$PWD:/workspace" \
  -w /workspace \
  -e OPENAI_API_KEY \
  -e CLEANR_ATTESTATION_KEY \
  ghcr.io/devr-tools/cleanr:${TAG} \
  run -config cleanr.yaml -format junit -output cleanr-junit.xml
```

## GitHub Actions Container Job

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

## When to Use the Image

Prefer the published image when:

- you want the exact released CLI without compiling in CI
- your pipeline already runs as a container job
- you want a simpler install path than bootstrapping Go

Prefer the release binary or `go install` path when:

- your runners do not permit container jobs
- you need native host integration instead of a containerized step

## Related Docs

- [Getting started](getting-started.md)
- [CI guide](ci.md)
- [Release automation](release-automation.md)
