# cleanr Roadmap

## Product direction

`cleanr` is becoming a developer-first AI testing platform for CI pipelines, release gates, and continuous safety validation. The goal is not just to run prompts against a model, but to give engineering teams a repeatable way to measure whether an AI application is secure, stable, resilient, and production-ready.

The long-term product needs to cover:

- adversarial testing
- policy and security validation
- regression and drift detection
- load and resilience testing
- tool-call and agent workflow verification
- CI-native reporting and governance artifacts

## Guiding principles

- Keep the runtime fast enough for CI by default.
- Prefer deterministic and replayable tests over vague benchmark-style outputs.
- Make the SDK extensible so teams can test HTTP APIs, agent runtimes, tools, and in-process adapters.
- Treat security and prompt-safety failures as first-class release blockers.
- Build reporting that works for both developers and compliance-minded teams.

## Current state

Phase 1 foundation is in place:

- Go module and CLI scaffold
- JSON config loading and validation
- HTTP target adapter
- prompt injection suite
- security suite
- load suite
- chaos suite
- drift suite
- token optimization suite
- text, JSON, and JUnit reporting

This is enough to prove the shape of the product, but it is still a foundation release. The next phase needs to make the system materially more useful for real AI teams.

## Phase 2: Production-grade core

Status: current focus

Objective: turn the prototype into a serious SDK and CI product that teams can adopt without rewriting their stack.

Primary outcomes:

- native provider adapters for OpenAI, Anthropic, and Gemini
- YAML config support in addition to JSON
- a richer scenario and assertion DSL
- token cost governance and provider-native usage ingestion
- semantic drift and baseline snapshot support
- tool-call and agent trace assertions
- first-party GitHub Actions integration
- stronger docs and sample projects

Exit criteria:

- developers can onboard without customizing internal packages
- test configs support common AI app patterns without hacks
- CI output is stable enough for gating pull requests
- core suites are documented with examples and expected failure modes

## Phase 3: Advanced testing and scale

Objective: make `cleanr` credible for larger engineering orgs and more complex agent systems.

Primary outcomes:

- distributed load execution
- percentile histograms and trend reports
- seeded replay runs for deterministic regression analysis
- dataset-backed evaluation packs
- red-team scenario bundles
- failure triage with grouped root-cause summaries
- signed test attestations and audit artifacts

Exit criteria:

- large test runs can execute within reasonable CI budgets
- teams can compare behavior across builds, models, and prompt versions
- reports can support engineering review and internal audit workflows

## Phase 4: Platform and ecosystem

Objective: move from SDK to ecosystem.

Primary outcomes:

- plugin system for custom suites and org-specific policies
- remote result aggregation service
- hosted dashboard and historical analytics
- org-level policy packs
- managed scenario libraries
- IDE and PR review integrations

Exit criteria:

- external teams can extend `cleanr` without forking the core
- organizations can standardize AI release criteria across multiple services

## Phase 2 workstreams

### 1. Provider adapters

- Add OpenAI target adapter with request/response normalization.
- Add Anthropic target adapter with message format normalization.
- Add Gemini target adapter with content block normalization.
- Standardize a provider-neutral response envelope for assertions and reporting.

### 2. Config and DSL

- Add YAML parsing while preserving JSON compatibility.
- Introduce reusable scenario templates.
- Add assertions for output contains, not contains, regex, JSON path, tool-call count, and latency budgets.
- Add config schema documentation and validation errors with actionable messages.

### 3. Drift and regression

- Add snapshot baselines checked into repo.
- Add semantic similarity scoring instead of only string distance.
- Allow deterministic replay inputs with fixed metadata and seeds where supported.
- Produce diffs that show what changed, not just that something changed.

### 3a. Token efficiency and cost control

- Upgrade from heuristic token estimation to provider-native usage capture where available.
- Add suite-level cost budgets by model or endpoint.
- Track prompt duplication, retrieval bloat, and verbose response patterns across builds.
- Surface optimization guidance that teams can use to reduce spend before release.

### 4. Agent and tool verification

- Support tool-call traces in the target abstraction.
- Add assertions for allowed tools, blocked tools, argument shape, and tool ordering.
- Add loop detection and runaway-agent protections.
- Add jailbreak tests that target tool misuse, data exfiltration, and role confusion.

### 5. CI and delivery

- Ship an official GitHub Action.
- Add machine-readable summary artifacts and stable exit semantics.
- Add release binaries and install docs.
- Add sample CI workflows for common stacks.

### 6. Docs and developer adoption

- Write quickstart guides for API apps, agents, and internal tools.
- Add example configs for customer support bots, RAG apps, and tool-using agents.
- Add a security testing guide and a drift testing guide.
- Document recommended suite selection for fast PR checks versus deep nightly runs.

## Milestone sequence

### Milestone A

Foundation hardening and adoption readiness.

- YAML config
- improved validation
- OpenAI adapter
- example projects
- GitHub Action draft

### Milestone B

Test depth and agent-awareness.

- Anthropic and Gemini adapters
- assertion DSL
- tool-call trace model
- snapshot baselines

### Milestone C

Advanced regression and operational maturity.

- semantic drift
- richer CI artifacts
- release packaging
- nightly load and chaos workflows

## Non-goals for Phase 2

- hosted dashboard
- multi-tenant control plane
- billing or account systems
- broad plugin marketplace

Those can come later. The immediate need is a sharp local and CI experience with strong testing primitives.
