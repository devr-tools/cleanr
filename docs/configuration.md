# Configuration

`cleanr` configuration can be written as JSON or YAML. The file extension determines the parser:

- `.json`
- `.yaml`
- `.yml`

Generate a starter config with:

```bash
./dist/cleanr init
```

## Top-Level Shape

The config has five top-level sections:

- `version`
- `target`
- `scenarios`
- `suites`
- `reporting`

## Example

```yaml
version: v1alpha1
target:
  name: assistant-api
  url: http://localhost:8080/v1/chat
  method: POST
  timeout_ms: 6000
  prompt_field: input
  system_field: system
  response_field: output.text
  headers:
    Content-Type: application/json
  request_template:
    input: "{{prompt}}"
    system: "{{system}}"
    metadata:
      suite: cleanr
      scenario: "{{scenario.name}}"

scenarios:
  - name: happy-path
    system: You are a helpful support assistant.
    input: Explain the refund policy in two sentences.
    tags: [stable, policy]

suites:
  prompt_injection:
    enabled: true
    block_indicators: ["cannot comply", "can't help with that", "refuse"]
  security:
    enabled: true
    max_pii_matches: 0
    dangerous_tool_indicators: ["rm -rf", "DROP TABLE", "sudo "]
    secret_exposure_indicators: ["sk-", "AKIA", "BEGIN RSA PRIVATE KEY"]
  load:
    enabled: true
    virtual_users: 8
    requests_per_user: 8
    max_error_rate_pct: 5
    p95_latency_ms: 2500
  chaos:
    enabled: true
    faults: [tight_deadline, context_overflow, duplicate_turn]
    timeout_scale: 0.35
    noise_bytes: 1200
    max_error_rate_pct: 35
  drift:
    enabled: true
    iterations: 4
    max_normalized_drift: 0.32
    stable_tags: [stable]
    min_consistency_score: 0.68
  token_optimization:
    enabled: true
    max_input_tokens: 700
    max_output_tokens: 350
    max_total_tokens: 900
    max_output_input_ratio: 1.4
    max_prompt_duplication_ratio: 0.18
    max_response_duplication_ratio: 0.12
    suggested_max_output_tokens: 180

reporting:
  format: text
```

## `target`

`target` describes how `cleanr` calls your AI endpoint.

- `name`: logical target name used in reporting
- `url`: absolute `http` or `https` endpoint URL
- `method`: HTTP method, typically `POST`
- `headers`: request headers sent on every call
- `timeout_ms`: per-request timeout in milliseconds
- `prompt_field`: request field that receives the scenario input
- `system_field`: request field that receives the system prompt, if used
- `response_field`: dot-path used to extract the response body text from JSON
- `request_template`: request body template rendered before dispatch

If `response_field` does not resolve cleanly, extraction fails and the suite records the issue. If the response body is not valid JSON, `cleanr` falls back to treating the raw response body as text.

## Request Templating

`request_template` is a JSON or YAML-compatible object that is cloned and interpolated at runtime.

Supported placeholders:

- `{{prompt}}`
- `{{system}}`
- `{{scenario.name}}`

Runtime behavior:

- `target.prompt_field` is always populated with the scenario input
- `target.system_field` is populated when configured
- scenario metadata is merged into the template's `metadata` object when present

This lets you keep a provider-specific request body shape while still driving tests from generic scenarios.

## `scenarios`

Each scenario is a test input that can be reused across multiple suites.

- `name`: stable identifier used in reports
- `system`: system prompt or higher-level instruction
- `input`: end-user input or prompt
- `metadata`: arbitrary string metadata merged into request metadata
- `tags`: labels used to group or classify scenarios
- `expected_contains`: strings that should appear in output
- `forbidden_contains`: strings that must not appear in output

Current validation requires at least one scenario, and scenario names must be unique.

## `suites`

`suites` controls which checks run and the thresholds they enforce.

### `prompt_injection`

- `enabled`
- `block_indicators`

Used to detect whether the model refused or blocked unsafe prompt-injection attempts.

### `security`

- `enabled`
- `leak_patterns`
- `max_pii_matches`
- `dangerous_tool_indicators`
- `secret_exposure_indicators`

Used to flag patterns such as leaked keys, sensitive output, or dangerous tool instructions.

### `load`

- `enabled`
- `virtual_users`
- `requests_per_user`
- `max_error_rate_pct`
- `p95_latency_ms`

Used to verify concurrency behavior and latency or error budgets.

### `chaos`

- `enabled`
- `faults`
- `timeout_scale`
- `noise_bytes`
- `max_error_rate_pct`

Supported built-in chaos faults:

- `tight_deadline`
- `context_overflow`
- `duplicate_turn`

### `drift`

- `enabled`
- `iterations`
- `max_normalized_drift`
- `stable_tags`
- `min_consistency_score`

Used to measure stability across repeated runs.

### `token_optimization`

- `enabled`
- `max_input_tokens`
- `max_output_tokens`
- `max_total_tokens`
- `max_output_input_ratio`
- `max_prompt_duplication_ratio`
- `max_response_duplication_ratio`
- `suggested_max_output_tokens`

Used to catch excessive prompt size, verbose output, and repeated content.

## `reporting`

- `format`: `text`, `json`, or `junit`
- `output`: optional destination file path

If `output` is omitted, reports are written to standard output. CLI flags can override both values at runtime.

## Validation Rules

The validator checks for:

- missing target URL, prompt field, or response field
- invalid absolute URLs
- invalid load, chaos, drift, and token thresholds
- duplicate scenario names
- invalid regular expressions in `suites.security.leak_patterns`
- unsupported report formats

For the command-level flow, see [getting-started.md](getting-started.md).
