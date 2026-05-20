# cleanr Taskboard

## Board rules

- Keep this file execution-focused.
- Move items between sections instead of duplicating them.
- Each in-flight item should have a clear deliverable and exit criteria.
- Do not start new infrastructure-heavy work until the current core adoption blockers are closed.

## Done

- [x] Scaffold Go module and CLI entrypoints.
- [x] Add JSON config loader and validator.
- [x] Add HTTP target adapter.
- [x] Add prompt injection, security, load, chaos, and drift suites.
- [x] Add token optimization suite for prompt/output budgets and redundancy checks.
- [x] Add text, JSON, and JUnit report output.
- [x] Add initial README and quickstart.
- [x] Improve validation error quality.
- [x] Add YAML config support.
- [x] Add OpenAI adapter.
- [x] Add Anthropic adapter.
- [x] Define provider-neutral target response envelope.
- [x] Add assertion DSL.
- [x] Add MCP server mode for agent and MCPO access.
- [x] Add snapshot baselines.

## Now

### Config and DX

- [x] Add provider-native token usage ingestion.
Deliverable: adapters can populate exact input/output token counts instead of relying only on heuristic estimation.
Exit criteria: OpenAI and Anthropic adapters populate exact usage automatically when the provider returns it.

- [ ] Add reusable scenario templates.
Deliverable: scenario inheritance or shared variables for repeated prompt setups.
Exit criteria: example configs use shared defaults without duplication.

### CI

- [x] Create GitHub Actions CI workflow.
Deliverable: repository workflow that runs `cleanr` QA and build steps in CI.
Exit criteria: workflow YAML is committed and validates the main developer path on pull requests.

- [ ] Stabilize machine-readable reports.
Deliverable: consistent JSON schema and JUnit output contract.
Exit criteria: docs specify report fields and compatibility expectations.

## Next

### Assertions and regression

- [x] Add semantic drift scoring.
Deliverable: similarity-based drift scoring that is more robust than raw edit distance.
Exit criteria: drift report includes both lexical and semantic consistency metrics.

### Agent and tool testing

- [ ] Add tool-call trace model.
Deliverable: normalized tool invocation structure in the SDK.
Exit criteria: assertions can inspect tool name, arguments, order, and counts.

- [ ] Add agent safety cases.
Deliverable: prebuilt adversarial scenarios for data exfiltration, unsafe tools, runaway loops, and instruction override.
Exit criteria: sample policy packs exist and are documented.

### Packaging

- [x] Add release builds.
Deliverable: versioned binaries for macOS and Linux.
Exit criteria: tagged release process documented and repeatable.

## Later

- [ ] Add Gemini adapter.
- [ ] Add distributed load workers.
- [ ] Add trend reports across builds.
- [ ] Add signed attestations for compliance evidence.
- [ ] Add plugin system for custom suites.
- [ ] Add hosted result aggregation.

## Blockers and decisions

- [ ] Decide whether provider adapters live in `cleanr/providers/...` or behind build tags.
- [ ] Decide whether YAML support should use a third-party dependency or a thin compatibility layer.
- [ ] Decide whether semantic drift ships with pluggable embeddings or a single default implementation.
- [ ] Decide whether snapshots are plain text, JSON, or a custom result bundle format.

## Suggested execution order

1. semantic drift
2. tool-call tracing
3. Gemini adapter
4. distributed load workers

## Definition of Phase 2 done

- OpenAI and Anthropic are supported natively.
- YAML config is supported.
- Assertions are reusable and not hard-coded per engine.
- Drift can compare against saved baselines.
- CI integration is documented and usable.
- At least one agent or tool-using workflow is covered by examples.
