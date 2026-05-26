# Best Practices

Use `cleanr` as a release gate for behavior you actually care about, not as a broad catch-all probe that floods CI with unstable failures.

## Keep It High Signal

- Keep PR scenarios narrow and tied to deterministic, business-critical flows.
- Tag only those deterministic scenarios as `stable`.
- Treat `stable` as a contract: if a scenario is naturally variable, keep it out of the stable baseline set.
- Prefer one clear assertion per scenario over a long list of loose checks.

## Baselines And Drift

Use `suites.drift.baseline_file` together with `cleanr snapshot` for expected-answer regression checks.

Recommended flow:

1. Capture a known-good baseline with `cleanr snapshot -config ...`.
2. Commit the baseline snapshot once the output is reviewed.
3. Point `suites.drift.baseline_file` at that committed snapshot.
4. Keep `stable_tags` limited to scenarios where answer drift should be treated as a regression.

This gives you two different signals:

- baseline comparison: did a known-good answer change in a meaningful way
- repeated-run drift: is the model unstable across equivalent runs right now

## Trend Gates

Start trend gates in a non-blocking posture and tighten them only after you have retained history.

Recommended rollout:

1. Start with `reporting.trend_gates.preset: exploratory`.
2. Retain at least a few runs with `reporting.trend_file`.
3. Review the real variance and duration growth in your target.
4. Move to `preset: moderate` when the run history is stable enough to gate on.

This avoids turning day-one CI into noise while still collecting the data needed for a useful gate.

## Split By Pipeline Stage

Do not run the heaviest suites on every pull request unless the target is cheap and highly deterministic.

Recommended split:

- PR: assertions, security, token optimization, and light drift
- `main`: add retained trend history and moderate trend gates
- nightly or pre-release: add load, chaos, and larger drift iterations
- release: add replay artifacts, attestations, and any `release_policy` rules

If model cost or variance is high, move `load`, `chaos`, and larger drift windows out of PR validation.

## Reports And Artifacts

Use each output format for a specific job:

- `junit`: CI test UIs and pass/fail visibility
- `json`: automation, post-processing, and downstream integrations
- replay artifacts: failure debugging and rerun context

Recommended reporting defaults:

```yaml
reporting:
  format: text
  trend_file: reports/cleanr.trends.yaml
  replay_artifact_file: reports/cleanr.replay.json
```

Then override the primary report format in CI with CLI flags when needed:

```bash
cleanr run -config cleanr.yaml -format junit -output cleanr-junit.xml
cleanr run -config cleanr.yaml -format json -output cleanr-report.json
```

## Agentic Targets

If your app is agentic, return top-level `trace` and `usage` fields in the target response whenever possible.

That allows `cleanr` to evaluate more than plain text output:

- workflow policy and `release_policy`
- provenance and trust-boundary behavior
- approvals and privileged action handling
- tool usage and state changes
- token budgets and token-efficiency checks

Minimal shape:

```json
{
  "output": { "text": "drafted the reply and requested approval" },
  "usage": {
    "input_tokens": 412,
    "output_tokens": 96,
    "total_tokens": 508
  },
  "trace": {
    "provider": "openai",
    "model": "gpt-4.1-mini",
    "tool_calls": [{ "name": "draft_email" }],
    "approvals": [{ "status": "required" }],
    "state_changes": [{ "kind": "email", "action": "draft" }]
  }
}
```

## Recommended Config Shape

If you want the highest-value rollout, structure configs by pipeline stage:

- `cleanr-pr.yaml`: assertions, security, token optimization, light drift
- `cleanr-main.yaml`: adds trend tracking and moderate trend gates
- `cleanr-release.yaml`: adds full drift, load, chaos, replay artifacts, attestation, and `release_policy`

Reference examples live in:

- `examples/best-practices/cleanr-pr.yaml`
- `examples/best-practices/cleanr-main.yaml`
- `examples/best-practices/cleanr-release.yaml`

## Example Rollout

1. Start with the PR profile and one or two reviewed stable scenarios.
2. Capture and commit a baseline snapshot.
3. Enable retained trend history on `main`.
4. Move trend gates from `exploratory` to `moderate`.
5. Add load, chaos, and release policy only where they are operationally worth the cost.
