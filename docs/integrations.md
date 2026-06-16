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
| LangSmith import | `langsmith` | implemented | Imports exported LangSmith run JSON from `path` or `url` when records contain embedded `cleanr` report/history metadata or normalized run rows |
| OpenLLMetry import | `openllmetry` | implemented | Imports exported trace or log JSON from `path` or `url` when records contain embedded `cleanr` report/history metadata or normalized run rows |
| Provider-log import | `provider_logs` | implemented | Imports generic provider trace or log exports from `path` or `url` and normalizes embedded `cleanr` metadata or simple run rows into retained history |

### Summary Outputs

These integrations write local summary artifacts after the run completes.

| Integration | Type | Status | Current behavior |
| --- | --- | --- | --- |
| Markdown summary | `summaries[].format: markdown` | implemented | Writes a human-readable PR or release summary |
| JSON summary | `summaries[].format: json` | implemented | Writes a machine-readable summary for downstream automation |

### Sync Workflows

These workflows pull Braintrust-stored artifacts back into a reviewable `cleanr` change set.

| Workflow | Command | Status | Current behavior |
| --- | --- | --- | --- |
| Braintrust sync | `cleanr sync braintrust` | implemented | Reads replay artifacts and explicit `cleanr_sync` insight payloads from Braintrust, writes a normalized insight dataset, merges scenario updates into config, applies explicit config patch operations, and can optionally open a GitHub PR through `gh` |

## Current Gaps

The following are not implemented yet:

- native Langfuse trend-source loading
- native PostHog trend-source loading
- provider-backed dataset import or export flows beyond file or HTTP exports
- provider-specific UI integrations beyond returned run URLs, local summary files, imported run rows, and the `cleanr sync braintrust` review loop

## Where To Configure

- [configuration.md](configuration.md): field-level config reference for `integrations.*`
- [ci.md](ci.md): CI usage patterns and example integration blocks
- [roadmap.md](roadmap.md): forward-looking integration direction
