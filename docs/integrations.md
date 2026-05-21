# Integrations

This page lists the integrations that are currently implemented in `cleanr`.

The Phase 5 integration layer is additive:

- local suite execution, trend gates, replay artifacts, and exit codes remain the source of truth
- remote sinks and remote comparison sources are best-effort companions
- integration failures are reported, but they do not turn a passing local run into exit code `2`

## Current Support

### Result Sinks

These integrations publish `cleanr` run data outward after the local run finishes.

| Integration | Type | Status | Current behavior |
| --- | --- | --- | --- |
| Generic webhook | `http` | implemented | POSTs the machine-readable `cleanr` sink payload to an arbitrary HTTP endpoint |
| Braintrust | `braintrust` | implemented | Publishes native Braintrust experiments and events, and returns experiment URLs when available |
| Langfuse | `langfuse` | implemented | Publishes a Langfuse trace plus numeric scores such as pass or fail and failed case counts |
| PostHog | `posthog` | implemented | Publishes batch events for the overall run and case-level failures or findings |

### Trend Sources

These integrations pull comparison history into `cleanr` before remote comparison summaries are attached to the report.

| Integration | Type | Status | Current behavior |
| --- | --- | --- | --- |
| Local retained history | `file` | implemented | Loads a local `cleanr` trend history file |
| Generic remote history | `http` | implemented | Fetches a remote `cleanr` trend history file over HTTP |
| Braintrust | `braintrust` | implemented | Loads prior `cleanr` history rows from Braintrust experiments for comparison |

### Summary Outputs

These integrations write local summary artifacts after the run completes.

| Integration | Type | Status | Current behavior |
| --- | --- | --- | --- |
| Markdown summary | `summaries[].format: markdown` | implemented | Writes a human-readable PR or release summary |
| JSON summary | `summaries[].format: json` | implemented | Writes a machine-readable summary for downstream automation |

## Current Gaps

The following are not implemented yet:

- native Langfuse trend-source loading
- native PostHog trend-source loading
- provider-backed dataset import or export flows
- provider-specific UI or PR annotation integrations beyond returned run URLs and local summary files

## Where To Configure

- [configuration.md](configuration.md): field-level config reference for `integrations.*`
- [ci.md](ci.md): CI usage patterns and example integration blocks
- [roadmap.md](roadmap.md): forward-looking integration direction
