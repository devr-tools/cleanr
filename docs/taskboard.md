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
- [x] Add provider-neutral trace evidence model for source uses, approvals, state changes, and memory operations.
- [x] Add claim-vs-trace verification suite.
- [x] Add initial memory safety suite for stale, revoked, poisoned, and cross-user memory cases.
- [x] Add file-based shadow-state verification for approved write locations.
- [x] Add provenance-aware context-source attack testing for untrusted retrieved and tool-like inputs.
- [x] Add HTTP trace ingestion for provider-neutral workflow evidence on generic HTTP targets.
- [x] Add release-policy DSL for tools, trust boundaries, approvals, sinks, and state changes.
- [x] Add exact expected state-change verification for non-file workflow surfaces.
- [x] Add end-to-end stateful workflow sample project.
- [x] Add build-aware trend diffs for workflows, prompts, and configured models.
- [x] Add grouped failure buckets and workflow-level blast-radius summaries.
- [x] Add nightly replay artifacts for failing workflows and retained evidence.
- [x] Deepen longitudinal memory replay coverage for stale, revoked, and poisoned fixtures.
- [x] Add org-level policy packs for reusable release criteria.
- [x] Add signed release-gate attestations for audit workflows.
- [x] Add plugin manifests for external suites, state adapters, and plugin-shipped policy packs.
- [x] Add SARIF output for IDE and PR review surfaces.

## Now

### Config and DX

- [x] Add provider-native token usage ingestion.
Deliverable: adapters can populate exact input/output token counts instead of relying only on heuristic estimation.
Exit criteria: OpenAI and Anthropic adapters populate exact usage automatically when the provider returns it.

- [ ] Add reusable scenario templates.
Deliverable: scenario inheritance or shared variables for repeated prompt setups.
Exit criteria: example configs use shared defaults without duplication.

- [x] Add plugin manifest and discovery surface.
Deliverable: stable extension points for custom suites, state adapters, and policy rules.
Exit criteria: external teams can register at least one custom extension without forking core behavior.

### CI

- [x] Create GitHub Actions CI workflow.
Deliverable: repository workflow that runs `cleanr` QA and build steps in CI.
Exit criteria: workflow YAML is committed and validates the main developer path on pull requests.

- [ ] Stabilize machine-readable reports.
Deliverable: consistent JSON schema and JUnit output contract.
Exit criteria: docs specify report fields and compatibility expectations.

- [x] Add PR and IDE review integration outputs.
Deliverable: machine-readable findings that can be surfaced inline in pull requests and editor tooling.
Exit criteria: one documented integration path exists for PR annotations or IDE diagnostics.

## Next

### Assertions and regression

- [x] Add semantic drift scoring.
Deliverable: similarity-based drift scoring that is more robust than raw edit distance.
Exit criteria: drift report includes both lexical and semantic consistency metrics.

### Agent and tool testing

- [x] Add tool-call trace model.
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
- [x] Add trend reports across builds.
- [x] Add `cleanr trends` history summarizer.
- [x] Add signed attestations for compliance evidence.
- [x] Add plugin system for custom suites.
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

## Definition of Phase 1 done

- OpenAI and Anthropic are supported natively.
- YAML config is supported.
- Assertions are reusable and not hard-coded per engine.
- Drift can compare against saved baselines.
- CI integration is documented and usable.
- Release packaging and install paths are documented.

## Definition of Phase 2 done

- Provider-neutral workflow evidence is first-class in the target abstraction and reports.
- Tool calls, approvals, state changes, and memory operations are usable directly in assertions and policy checks.
- Developers can express action-level pass or fail policy without custom forks.
- At least one real tool-using or stateful workflow is covered by examples or sample projects.
- Reports show observed actions, claimed actions, and the first mismatch clearly enough for CI triage.

Status: complete.

## Definition of Phase 3 done

- Teams can compare workflow-level regressions, grouped failures, and blast radius instead of only aggregate pass or fail.
- Memory and state regressions are reproducible with stable multi-session replay fixtures.
- Trend summaries surface build, prompt, and configured model diffs alongside workflow evidence changes.
- Nightly runs can emit replay artifacts with failing workflows and retained evidence for triage.

Status: complete.

## Definition of Phase 4 done

- External teams can extend `cleanr` with plugin manifests for custom suites, state adapters, and plugin-shipped policy rules without forking core behavior.
- Organizations can standardize release policy across services with reusable policy packs.
- Signed release-gate attestations are available for audit and change-review workflows.
- SARIF output is available for IDE and PR review surfaces.

Status: complete.
