# Containerized Assistant Example

This example shows the intended CI shape when your application runs in one container and `cleanr` runs in another:

- the app container owns the provider credentials and talks to OpenAI or Anthropic internally
- the `cleanr` container tests the app over HTTP with `target.type: http`
- both containers share a private network
- the `cleanr` container is locked down with a read-only root filesystem, dropped Linux capabilities, `no-new-privileges`, and resource limits

## Files

- `cleanr.yaml`: `cleanr` config that points at `http://app:8080/v1/chat`
- `docker-compose.yaml`: container orchestration example with the hardened `cleanr` service

## Run With Docker Compose

Replace `ghcr.io/your-org/assistant-api:latest` with your app image, then run:

```bash
export OPENAI_API_KEY=...
export CLEANR_TAG=v0.1.0
docker compose -f examples/containerized-assistant/docker-compose.yaml up --abort-on-container-exit --exit-code-from cleanr
```

The app service is expected to expose:

- `POST /v1/chat`
- `GET /healthz`

If your endpoint shape differs, update `cleanr.yaml`:

- `target.url`
- `target.prompt_field`
- `target.system_field`
- `target.response_field`
- `target.request_template`

## Why The Credentials Stay In The App Container

In this model, `cleanr` is testing your application, not calling the upstream provider directly.

That means:

- `OPENAI_API_KEY` or `ANTHROPIC_API_KEY` usually belongs only in the app container
- `cleanr` only needs its own provider credentials if you use `target.type: openai`, `target.type: anthropic`, or `scenario_generation`

## CI Adaptation

The same network model maps cleanly to GitHub Actions container jobs:

- run the job inside `ghcr.io/devr-tools/cleanr:<tag>`
- declare your app as a `services:` container
- point `target.url` at `http://app:8080/...`

See the Docker and CI docs for the copy-paste workflow snippet.
