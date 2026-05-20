# MCP and MCPO

`cleanr` can run as an MCP server over stdio so AI agents can use the tool suite directly, or through MCPO as an OpenAPI server.

## Server mode

Start the MCP server with:

```bash
./dist/cleanr mcp
```

The server exposes three tools:

- `cleanr_example_config`: return a starter config in `json` or `yaml`
- `cleanr_validate_config`: validate inline config content or a local `config_path`
- `cleanr_run`: execute suites from inline config content or a local `config_path`

The stdio transport follows newline-delimited JSON-RPC messages and writes logs only to `stderr`.

## MCPO bridge

To expose `cleanr` as OpenAPI for agent frameworks that do not speak MCP natively:

```bash
uvx mcpo --port 8000 --api-key "top-secret" -- ./dist/cleanr mcp
```

That gives you:

- OpenAPI schema at `http://localhost:8000/openapi.json`
- interactive docs at `http://localhost:8000/docs`
- HTTP tool endpoints generated from the `cleanr` MCP tools

Example:

```bash
curl -X POST http://localhost:8000/cleanr_example_config \
  -H "Authorization: Bearer top-secret" \
  -H "Content-Type: application/json" \
  -d '{"format":"yaml"}'
```

## Tool inputs

For `cleanr_validate_config` and `cleanr_run`, provide one of:

- `config`: raw JSON or YAML config content, plus optional `format`
- `config_path`: path to a local config file

For `cleanr_run`, you can also pass:

- `report_format`: `text`, `json`, or `junit`
- `timeout_ms`: optional whole-run timeout

## Notes

- `cleanr_run` returns a structured report plus rendered report text.
- A failed suite is still a successful tool call; the result returns `exit_code: 1`.
- Invalid config or runtime setup problems return `exit_code: 2`.
- If you use MCPO, secure the endpoint with an API key or stronger auth before exposing it beyond localhost.
