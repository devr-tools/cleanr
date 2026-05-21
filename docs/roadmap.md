# cleanr Roadmap

## Product direction

`cleanr` is becoming a developer-first release gate for AI systems that take actions, mutate state, and cross trust boundaries.

The product direction is intentionally narrower than "general LLM evals." The market already has strong platforms for tracing, prompt iteration, observability, dashboards, and generic scoring. `cleanr` should win by being the CI-native system that tells a team whether an agent workflow is safe to ship.

That means `cleanr` should focus on questions such as:

- did the agent take the correct action, not just produce a plausible answer
- did untrusted input influence a privileged decision
- did the workflow claim to use a tool, source, or approval path that never actually happened
- did the system mutate the right state and avoid forbidden side effects
- did memory, retrieval, or tool context create longitudinal risk across sessions
- did a code, prompt, or model change widen the blast radius of failure

## Product thesis

`cleanr` should become the release gate for stateful agent systems.

The core value is not another hosted eval UI. The core value is deterministic, replayable, policy-aware verification that can run in CI before production changes ship.

## What we are not building

`cleanr` is not trying to out-compete the market on:

- hosted prompt playgrounds
- tracing dashboards
- generic observability
- broad experimentation suites for every AI workflow
- cloud-first team collaboration features as the primary product

Those capabilities can matter later, but they are not the wedge.

## Differentiation wedge

The roadmap is now centered on the remaining product wedges that are still under-served in the market.

### 1. Claim-vs-trace verification

Agents frequently claim they checked a source, called a tool, obtained approval, or completed an action when the execution trace does not support that claim.

`cleanr` should detect and block:

- claimed citations with no trace evidence
- claimed tool execution with no matching invocation
- claimed approval steps with no approval artifact
- claimed state changes that do not match observed side effects

### 2. Longitudinal memory and state safety

Single-turn evals miss risks that accumulate over time. `cleanr` should treat memory, saved context, and persistent state as first-class attack and regression surfaces.

Examples:

- stale memory replay
- revoked fact persistence
- cross-user memory bleed
- memory poisoning
- trust decay across long-running sessions

### 3. Release policy as code

Teams need machine-checkable rules for what an agent may and may not do before merge.

Examples:

- a tool may read but not mutate
- SQL generation must remain read-only
- an external communication tool may draft but not send
- human approval is required before any irreversible action
- sensitive data may cross only into approved sinks

## Guiding principles

- Keep the runtime fast enough for CI by default.
- Prefer deterministic and replayable tests over vague benchmark-style outputs.
- Treat action-level correctness as more important than output aesthetics.
- Model trust boundaries explicitly instead of flattening all context into plain text.
- Make side effects inspectable, diffable, and enforceable in policy.
- Produce artifacts that engineering, security, and compliance teams can all use.
- Stay local-first and pipeline-first until the release-gate experience is sharp.

## Current state

The foundation release is in place:

- Go module and CLI scaffold
- JSON and YAML config loading and validation
- HTTP target adapter
- native OpenAI and Anthropic target support
- prompt injection suite
- security suite
- load suite
- chaos suite
- drift suite
- token optimization suite
- text, JSON, and JUnit reporting

The first Phase 2 release-gate pieces are also now in place:

- normalized trace evidence for source uses, approvals, state changes, and memory operations
- claim-vs-trace verification for unsupported citations, tool claims, approvals, and state-change claims
- initial longitudinal memory safety coverage for stale, revoked, poisoned, and cross-user memory replay

The Phase 2 action-verification core is now in place as well:

- file-based shadow-state verification for approved write locations
- provenance-aware attacks driven by trust-tagged scenario context sources
- expected file-mutation assertions for create, modify, and delete checks
- provenance policy checks for approval-required tools and approved sink tools
- HTTP trace ingestion for provider-neutral workflow evidence on generic HTTP targets
- release-policy DSL for action-level tool, approval, trust-boundary, sink, and state-change rules
- exact expected state-change verification for non-file workflow surfaces
- end-to-end stateful sample project with a real workflow gate

Phase 3 is now in place as well:

- multi-session replay fixtures for revoked, stale, poisoned, and cross-user memory regressions
- replay validation with fixed session metadata for reproducible fixtures
- workflow-level regressions, grouped failure buckets, and blast-radius summaries across retained runs
- build-aware diffs for prompts, workflow inputs, and configured models between retained runs
- nightly replay artifacts that preserve failing workflows and retained evidence for triage

This establishes the longitudinal regression core. Phase 4 is now in place as well:

- org-level policy packs that can be layered into configs before validation
- signed release-gate attestations over the report and replay artifact for audit workflows
- plugin manifests for external suites, state adapters, and plugin-shipped policy rules
- SARIF output for IDE and PR review surfaces

This establishes the governance and ecosystem core. The next roadmap phase is about optional external eval and data integrations.
This final planned roadmap phase is now in place as well:

- best-effort result sink publishing for remote eval or experiment systems
- non-blocking remote trend-source comparisons against approved retained histories
- dataset import and export flows for promoting reviewed failures into replayable `cleanr` scenarios
- PR and release summary artifacts that link the local gate to deeper remote triage views

This establishes the local-first companion integration layer.

## Forward roadmap

Phase 1 through Phase 5 are complete. The roadmap below records the Phase 5 scope that was delivered.

### Phase 5: External eval and data integrations

Objective: add optional integration layers around `cleanr` for existing eval, tracing, and dataset systems without replacing the local-first release gate or turning `cleanr` into a hosted observability product.

Primary outcomes:

- add-on result sink integrations for external systems such as Braintrust so CI runs can publish machine-readable reports, regressions, and release-gate metadata remotely
- remote baseline and trend-source integrations for comparing the current run against approved prior experiments or retained histories
- dataset import and export flows that convert reviewed failures, production traces, or curated eval rows into replayable `cleanr` scenarios
- pull request and release summaries that link local gate failures to remote experiment views for deeper triage
- explicit contracts for what stays local and blocking versus what is add-on, best-effort, and asynchronous in remote systems

Exit criteria:

- teams can push `cleanr` CI results into external systems without changing or weakening the local pass or fail contract
- reviewed failures in external systems can be promoted back into durable `cleanr` regression fixtures
- integrations strengthen governance and review workflows as a companion layer rather than a replacement product surface

## Milestone sequence

### Milestone E

Add optional integrations with external eval and dataset systems.

- Braintrust-style result publishing for CI runs
- remote experiment and trend comparison sources
- dataset import and export for reviewed failures and production traces
- contracts that keep `cleanr` local-first while allowing optional add-on remote aggregation

## Non-goals for the near term

- hosted dashboard as the primary product surface
- generic observability platform work
- broad benchmark marketplace
- billing and account systems
- multi-tenant control plane as a prerequisite for usefulness

The immediate need is a sharp local and CI experience that blocks unsafe or incorrect agent releases before they ship.
