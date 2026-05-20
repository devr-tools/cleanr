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

The roadmap is now centered on five product wedges that are still under-served in the market.

### 1. Shadow-state verification

Verify what the agent actually changed in a simulated or controlled environment, not just what it said it changed.

Examples:

- verify that a ticket was updated with the correct fields
- verify that a database mutation stayed within row and column constraints
- verify that a draft email was created but not sent
- verify that a file write occurred only in approved locations

### 2. Provenance-aware attack testing

Model inputs should not be treated as a flat blob. `cleanr` should track trust tiers across user input, system instructions, retrieved content, tool output, memory, and human approvals.

Examples:

- fail if untrusted retrieved text overrides system policy
- fail if tool output is treated as authority without validation
- fail if indirect prompt injection crosses from untrusted content into privileged actions

### 3. Claim-vs-trace verification

Agents frequently claim they checked a source, called a tool, obtained approval, or completed an action when the execution trace does not support that claim.

`cleanr` should detect and block:

- claimed citations with no trace evidence
- claimed tool execution with no matching invocation
- claimed approval steps with no approval artifact
- claimed state changes that do not match observed side effects

### 4. Longitudinal memory and state safety

Single-turn evals miss risks that accumulate over time. `cleanr` should treat memory, saved context, and persistent state as first-class attack and regression surfaces.

Examples:

- stale memory replay
- revoked fact persistence
- cross-user memory bleed
- memory poisoning
- trust decay across long-running sessions

### 5. Release policy as code

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
- early native OpenAI target support
- prompt injection suite
- security suite
- load suite
- chaos suite
- drift suite
- token optimization suite
- text, JSON, and JUnit reporting

This proves the basic runner model, but the current product still focuses mostly on response inspection. The next phase needs to expand the abstraction from "prompt in, text out" toward "workflow in, evidence out."

## Strategic direction by phase

### Phase 2: Agent release-gate core

Status: current focus

Objective: turn `cleanr` from a foundation eval runner into a credible CI gate for tool-using and stateful agents.

Primary outcomes:

- provider-neutral workflow evidence model
- tool-call and trace capture as first-class inputs to assertions
- release-policy DSL for tool permissions, trust boundaries, and side effects
- claim-vs-trace verification suite
- shadow-state verification harnesses for common action surfaces
- provenance-aware prompt-injection and data-exfiltration tests
- stronger docs and sample projects for real agent stacks

Exit criteria:

- developers can express action-level pass or fail rules without custom forks
- CI runs can fail on policy violations even when the final answer sounds plausible
- reports show what action occurred, what was claimed, and where the mismatch happened
- example projects demonstrate at least one real stateful workflow end to end

### Phase 3: Longitudinal and blast-radius analysis

Objective: make `cleanr` credible for higher-risk and longer-lived agent systems.

Primary outcomes:

- longitudinal memory safety suite
- stale-memory and memory-poisoning regression packs
- seeded replay with fixed workflow metadata where supported
- change-impact replay and blast-radius summaries
- richer trace diffs across builds, prompts, and models
- grouped failure triage for repeated workflow-level failure modes

Exit criteria:

- teams can compare not just pass or fail, but which workflows regressed and how broadly
- memory and state regressions can be reproduced with stable fixtures
- nightly runs produce actionable replay artifacts rather than generic score deltas

### Phase 4: Governance and ecosystem

Objective: standardize agent release policy across teams and services.

Primary outcomes:

- signed release-gate artifacts and attestations
- org-level policy packs for common agent risk profiles
- plugin system for custom suites, state adapters, and policy rules
- remote result aggregation for multi-service governance
- IDE and PR integrations that surface policy failures inline

Exit criteria:

- external teams can extend `cleanr` without forking core behavior
- organizations can standardize agent release criteria across multiple systems
- governance artifacts are strong enough for internal audit and change review workflows

## Phase 2 workstreams

### 1. Evidence model and target abstraction

- Expand the target abstraction from text responses to workflow evidence.
- Normalize tool calls, approvals, retrieval events, state mutations, and final outputs into one provider-neutral envelope.
- Preserve support for HTTP-first targets while making richer adapters possible.
- Keep evidence exportable in text, JSON, and JUnit-compatible forms where practical.

### 2. Release-policy DSL

- Add policy primitives for allowed tools, blocked tools, read-only tools, approval requirements, and sink restrictions.
- Add trust-tier primitives for system, user, retrieved, memory, tool, and approved-human context.
- Add assertion support for side effects, argument shape, ordering, and irreversible actions.
- Keep the syntax readable enough for CI ownership by application teams.

### 3. Claim-vs-trace verification

- Add checks for claimed tool use with no invocation.
- Add checks for claimed citations or approvals with no evidence.
- Add checks for claimed state changes that do not match observed mutations.
- Produce focused failure output that pinpoints the first unsupported claim.

### 4. Shadow-state verification

- Introduce pluggable state verifiers for common surfaces such as SQL, HTTP side effects, files, queues, and outbound communications.
- Support pre-run and post-run snapshots with diffing.
- Add allowlists and deny rules for mutation scope.
- Report both intended and observed state changes.

### 5. Provenance-aware adversarial testing

- Add trust-tagged scenario inputs and context sources.
- Add indirect prompt-injection attacks that originate from retrieved or tool-provided content.
- Add exfiltration tests that target cross-boundary data flow instead of only unsafe text output.
- Add role-confusion and approval-bypass tests for multi-step workflows.

### 6. Docs and developer adoption

- Write an opinionated guide for testing stateful agents in CI.
- Add sample projects for support agents, RAG agents, and action-taking internal agents.
- Add policy cookbook examples such as draft-not-send, read-only SQL, and approved-sink-only data flow.
- Document recommended fast PR checks versus deeper nightly workflow replay.

## Phase 3 workstreams

### 1. Longitudinal memory safety

- Add fixtures for persistent memory reads and writes.
- Test stale facts, revoked facts, poisoned facts, and cross-session bleed.
- Add expiry and freshness assertions.
- Add replay support for multi-session scenarios.

### 2. Change-impact replay

- Re-run stored evidence packs against new builds.
- Group regressions by workflow, tool, trust tier, and policy family.
- Summarize blast radius for pull requests and release candidates.
- Distinguish local regressions from broad systemic failures.

### 3. Trace diffs and triage

- Produce diffs that show changed actions, not just changed text.
- Group repeated failures into root-cause-oriented buckets.
- Highlight first divergence points in workflow execution.
- Improve artifacts for engineering review and audit workflows.

## Milestone sequence

### Milestone A

Establish the release-gate core.

- provider-neutral evidence envelope
- first policy DSL primitives
- claim-vs-trace verification
- example project for a tool-using agent
- docs that reposition `cleanr` around action verification

### Milestone B

Make state and trust boundaries enforceable.

- shadow-state verification for at least one mutation surface
- trust-tiered adversarial scenarios
- policy assertions for approvals, sinks, and irreversible actions
- indirect prompt-injection coverage for retrieved and tool-provided content

### Milestone C

Add longitudinal and replay intelligence.

- memory safety suite
- change-impact replay
- workflow-level diffs
- grouped regression summaries

## Non-goals for the near term

- hosted dashboard as the primary product surface
- generic observability platform work
- broad benchmark marketplace
- billing and account systems
- multi-tenant control plane as a prerequisite for usefulness

The immediate need is a sharp local and CI experience that blocks unsafe or incorrect agent releases before they ship.
