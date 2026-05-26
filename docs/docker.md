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

## Test an App Container Over a Shared Network

When your application already runs as a separate container and owns its own provider credentials, prefer `target.type: http`.

That means:

- the app container talks to OpenAI or Anthropic internally
- the `cleanr` container calls the app endpoint over the container network
- provider credentials usually stay in the app container, not the `cleanr` container

Minimal target example:

```yaml
target:
  type: http
  name: assistant-api
  url: http://app:8080/v1/chat
  prompt_field: input
  system_field: system
  response_field: output.text
```

The repository now includes a full example in [examples/containerized-assistant](../examples/containerized-assistant/README.md).

## Protected Stress-Test Compose Example

Use this pattern when you want `cleanr` load, chaos, and prompt-injection suites to exercise an app container while keeping the `cleanr` container constrained:

```yaml
services:
  app:
    image: ghcr.io/your-org/assistant-api:latest
    environment:
      OPENAI_API_KEY: ${OPENAI_API_KEY}
    expose:
      - "8080"
    healthcheck:
      test: ["CMD", "curl", "-fsS", "http://localhost:8080/healthz"]
    networks: [cleanr]

  cleanr:
    image: ghcr.io/devr-tools/cleanr:${CLEANR_TAG:-v0.1.0}
    depends_on:
      app:
        condition: service_healthy
    command:
      - run
      - -config
      - /workspace/examples/containerized-assistant/cleanr.yaml
      - -format
      - junit
      - -output
      - /tmp/cleanr-junit.xml
    working_dir: /workspace
    volumes:
      - ../..:/workspace:ro
    read_only: true
    tmpfs:
      - /tmp:size=128m,mode=1777
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    pids_limit: 256
    mem_limit: 512m
    cpus: 1.0
    user: "65532:65532"
    networks: [cleanr]
```

Those restrictions are a practical baseline when you want the `cleanr` container to stress-test the app without giving the test runner broad filesystem or privilege access.

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

## GitHub Actions Job Container Plus App Service

This is the closest CI equivalent to the compose example above:

```yaml
jobs:
  cleanr:
    runs-on: ubuntu-latest
    container: ghcr.io/devr-tools/cleanr:<tag>
    services:
      app:
        image: ghcr.io/your-org/assistant-api:latest
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        options: >-
          --health-cmd "curl -fsS http://localhost:8080/healthz || exit 1"
          --health-interval 10s
          --health-timeout 3s
          --health-retries 12
    steps:
      - uses: actions/checkout@v4
      - run: cleanr validate -config examples/containerized-assistant/cleanr.yaml
      - run: cleanr run -config examples/containerized-assistant/cleanr.yaml -format junit -output cleanr-junit.xml
```

From the `cleanr` job container, the app service is reachable as `http://app:8080`.

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
