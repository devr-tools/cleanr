# MCP and MCPO

`cleanr` can run as an MCP server over stdio so AI agents can use the tool suite directly, or through MCPO as an OpenAPI server.

## Server mode

Start the MCP server with:

```bash
./dist/cleanr mcp
```

The server exposes these tools:

- `cleanr_example_config`: return a starter config in `json` or `yaml`
- `cleanr_describe_suites`: describe the built-in suites and their key config fields
- `cleanr_supported_targets`: describe the supported target types and their key config fields
- `cleanr_validate_config`: validate inline config content or a local `config_path`
- `cleanr_run`: execute suites from inline config content or a local `config_path`
- `cleanr_render_report`: render a JSON cleanr report as `text`, `json`, `junit`, `sarif`, or `agent`
- `cleanr_generate_dataset`: run `scenario_generation` and return a reviewable dataset artifact
- `cleanr_review_dataset`: review and approve generated datasets against a base config
- `cleanr_analyze_trends`: summarize retained trend history in `text` or `json`
- `cleanr_explain_failures`: summarize replay-artifact failures into grouped explanations for agents

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

For `cleanr_validate_config`, `cleanr_run`, and `cleanr_generate_dataset`, provide one of:

- `config`: raw JSON or YAML config content, plus optional `format`
- `config_path`: path to a local config file

For `cleanr_run`, you can also pass:

- `report_format`: `text`, `json`, `junit`, `sarif`, or `agent`
- `timeout_ms`: optional whole-run timeout

For `cleanr_render_report`, pass:

- `report_json`: serialized cleanr report JSON
- `format`: `text`, `json`, `junit`, `sarif`, or `agent`

For `cleanr_review_dataset`, provide:

- the base config via `config` or `config_path`
- the dataset via `dataset` or `dataset_path`
- optional `policy` or `policy_path`
- optional approval controls like `approve`, `reject`, `promote_stable`, and `promote_regression`

For `cleanr_analyze_trends`, provide:

- `history` or `history_path`
- optional `window`
- optional `output_format` (`text` or `json`)

For `cleanr_explain_failures`, provide:

- `replay` or `replay_path`
- optional `max_cases`

## Internal layout

The MCP implementation is intentionally split to avoid one growing server file:

- `internal/mcpserver/`: transport and JSON-RPC handling
- `internal/mcpserver/toolkit/`: shared MCP tool types and helpers
- `internal/mcpserver/catalog/`: read-only suite and target introspection tools
- `internal/mcpserver/runtime/`: config, run, and report execution tools
- `internal/mcpserver/tools/`: central tool registry and dispatch

## Notes

- `cleanr_run` returns a structured report plus rendered report text.
- `report_format: agent` emits a versioned AI-native contract with flattened findings, typed fix suggestions, and the full original report payload.
- A failed suite is still a successful tool call; the result returns `exit_code: 1`.
- Invalid config or runtime setup problems return `exit_code: 2`.
- If you use MCPO, secure the endpoint with an API key or stronger auth before exposing it beyond localhost.
