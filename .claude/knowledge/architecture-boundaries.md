# Architecture & System Boundaries

Key architectural decisions, service boundaries, data flow, integration points, and why things are the way they are.

## Case execution concurrency

- Read-heavy engines (`prompt_injection`, `security`, `token_optimization`, `release_policy`, `provenance`) run scenarios concurrently via `runBoundedByIndex` in `cleanr/engines`. Results are written into a pre-sized slice by index, so output ordering stays deterministic despite concurrency.
- The limit comes from `Config.Concurrency` (accessor `Config.CaseConcurrency()`, default 4). The `config` package's `applyDefaults` does NOT set it; the accessor supplies the default itself.
- `drift`, `claim_trace`, and `shadow_state` are intentionally left serial: drift must re-issue the same request N times to measure non-determinism, and claim_trace/shadow_state capture filesystem state in a per-scenario before/after window (shared mutable state that concurrency would misattribute). Do not parallelize them.
- A per-run `responseCache` (in `cleanr/engines`, created in `Default()`) lets read-only engines share one live target Invoke per plain scenario request instead of each calling the target separately. Mutating engines always invoke fresh. Keyed by scenario name + system + prompt + transcript.

## MCP server trust boundary

- Configs supplied via the MCP surface (`cleanr_run`, `cleanr_generate_dataset`) are treated as untrusted. `toolkit.GuardMCPConfig` rejects `target.type: cli`, and any config declaring `plugins`/`state_adapters`/`probes`/`suites`, unless the env var `CLEANR_MCP_ALLOW_EXEC` is truthy. The normal CLI path is unaffected.
- MCP path arguments (`config_path`, `dataset_path`, etc.) must be CWD-relative — absolute paths and `..` traversal are rejected, and error messages never echo file contents.

## gRPC transport

- `cleanr/adapters/grpc.go` now defaults to **TLS** for non-loopback targets. Insecure/plaintext is only used for loopback (localhost/127.0.0.1/::1) or when `GRPCConfig.Plaintext` is set. A remote gRPC test target that isn't loopback needs a TLS server or `grpc.plaintext: true` in config.

## Integration credential egress

- `applyAuth` in `cleanr/integrations/runtime` only sends env vars whose names match provider-secret patterns (OPENAI/ANTHROPIC/AWS/etc.) to an egress allowlist (braintrust.dev, langfuse.com, posthog.com, langsmith) or loopback. Generic tokens still flow anywhere. This prevents a malicious config from exfiltrating provider keys to an arbitrary URL via `api_key_env`.
